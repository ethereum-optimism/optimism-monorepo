// SPDX-License-Identifier: MIT
pragma solidity 0.8.19;

import { Test } from "forge-std/Test.sol";
import { SchemaResolver } from "src/vendor/eas/resolver/SchemaResolver.sol";
import { IEAS, Attestation } from "src/vendor/eas/IEAS.sol";
import { Vm } from "forge-std/Vm.sol"; 
import { ISchemaResolver } from "src/vendor/eas/resolver/ISchemaResolver.sol";

/// Mock Contracts
/// @dev Test implementation of SchemaResolver that always returns true
contract TestSchemaResolver is SchemaResolver {
    constructor(IEAS eas) SchemaResolver(eas) {}

    /// @dev Mock attestation validation that always succeeds
    function onAttest(Attestation calldata, uint256) internal pure override returns (bool) {
        return true;
    }

    /// @dev Mock revocation validation that always succeeds
    function onRevoke(Attestation calldata, uint256) internal pure override returns (bool) {
        return true;
    }

    /// @dev Returns fixed version string
    function version() external pure returns (string memory) {
        return "1.3.0";
    }
}

///  Main test contract
contract SchemaResolverTest is Test {
    

    // Constants
    uint64 constant NO_EXPIRATION = 0;

    // Test State
    TestSchemaResolver public resolver;
     ISchemaResolver public payableResolver;
    address public recipient;
    IEAS public eas;

    // Errors
    error AccessDenied();
    error InvalidLength();
    error NotPayable();
    error InsufficientValue();

    // Setup
    /// @dev Deploys mock contracts and sets up test environment
    function setUp() public {
        eas = IEAS(makeAddr("eas"));
        vm.mockCall(
            address(eas),
            abi.encodeWithSelector(IEAS.attest.selector),
            abi.encode(true)
        );
        resolver = new TestSchemaResolver(eas);
        // Mock PayableResolver
        payableResolver = ISchemaResolver(makeAddr("payableResolver"));
        vm.mockCall(
            address(payableResolver),
            abi.encodeWithSelector(ISchemaResolver.isPayable.selector),
            abi.encode(true)
        );
        vm.mockCall(
            address(payableResolver),
            abi.encodeWithSelector(ISchemaResolver.attest.selector),
            abi.encode(true)
        );
        vm.mockCall(
            address(payableResolver),
            abi.encodeWithSelector(ISchemaResolver.multiAttest.selector),
            abi.encode(true)
        );
        vm.mockCall(
            address(payableResolver),
            abi.encodeWithSelector(ISchemaResolver.revoke.selector),
            abi.encode(true)
        );
        vm.mockCall(
            address(payableResolver),
            abi.encodeWithSelector(ISchemaResolver.multiRevoke.selector),
            abi.encode(true)
        );

        recipient = makeAddr("recipient");
    }

    // Helper Functions
    /// @dev Helper function to get minimum of two numbers
    function min(uint256 a, uint256 b) internal pure returns (uint256) {
        return a < b ? a : b;
    }

    /// @dev Helper function to get maximum of two numbers
    function max(uint256 a, uint256 b) internal pure returns (uint256) {
        return a > b ? a : b;
    }

    // Basic State Tests
    /// @dev Tests initial resolver configuration
    function testInitialState() public view {
        assertEq(resolver.version(), "1.3.0");
        assertEq(resolver.isPayable(), false);
    }

    // Access Control Tests
    /// @dev Tests resolver access control restrictions.
    ///      Verifies only EAS contract can call resolver functions
    function testOnlyEASCanCall(
        address _attester,
        address _recipient,
        uint64 _time,
        uint64 _expirationTime,
        bytes calldata _data
    ) public {
        // Avoid special addresses
        vm.assume(_attester != address(eas));
        vm.assume(_recipient != address(0));
        vm.assume(_time > 0);
        
        Attestation memory attestation = Attestation({
            uid: bytes32(0),
            schema: bytes32(0),
            time: _time,
            expirationTime: _expirationTime,
            revocationTime: uint64(0),
            refUID: bytes32(0),
            recipient: _recipient,
            attester: _attester,
            revocable: true,
            data: _data
        });

        // Should revert when called by non-EAS address
        vm.expectRevert(AccessDenied.selector);
        resolver.attest(attestation);

        vm.expectRevert(AccessDenied.selector);
        resolver.revoke(attestation);
    }

    // Validation Tests
    /// @dev Tests ETH transfer rejection by non-payable resolver.
    ///      Ensures resolver properly rejects direct ETH transfers
    function testNonPayableResolver(uint256 _ethAmount) public {
        vm.assume(_ethAmount > 0);
        vm.assume(_ethAmount <= 100 ether); // Reasonable upper bound
        
        vm.expectRevert(NotPayable.selector);
        payable(address(resolver)).transfer(_ethAmount);
    }

    /// @dev Tests multi-attestation array length validation.
    ///      Ensures proper validation of array lengths
    ///      in multi-attestation operations
    function testMultiAttestInvalidLength(uint256 _attestationsLength, uint256 _valuesLength) public {
        // Ensure lengths are different and reasonable
        vm.assume(_attestationsLength > _valuesLength);
        vm.assume(_attestationsLength <= 100); // Reasonable upper bound
        vm.assume(_valuesLength > 0);
        
        Attestation[] memory attestations = new Attestation[](_attestationsLength);
        uint256[] memory values = new uint256[](_valuesLength);

        vm.prank(address(eas));
        vm.expectRevert(InvalidLength.selector);
        resolver.multiAttest(attestations, values);
    }

    /// @dev Tests multi-revocation array length validation.
    ///      1. Setup:
    ///         - Creates mismatched arrays with fuzzed lengths
    ///      2. Verification:
    ///         - Attempts multi-revoke as EAS
    ///         - Confirms revert with InvalidLength
    ///      Ensures proper validation of array lengths
    ///      in multi-revocation operations
    function testMultiRevokeInvalidLength(uint256 _length1, uint256 _length2) public {
        // Ensure lengths are different and reasonable
        vm.assume(_length1 != _length2);
        vm.assume(_length1 > 0 && _length2 > 0);
        vm.assume(_length1 <= 100 && _length2 <= 100); // Reasonable upper bounds
        
        Attestation[] memory attestations = new Attestation[](max(_length1, _length2));
        uint256[] memory values = new uint256[](min(_length1, _length2));

        vm.prank(address(eas));
        vm.expectRevert(InvalidLength.selector);
        resolver.multiRevoke(attestations, values);
    }

    /// @dev Tests value validation for paid multi-attestations.
    function testInsufficientValue(uint256 _numAttestations, uint256 _valuePerAttestation) public {
        // Ensure reasonable bounds
        vm.assume(_numAttestations > 1 && _numAttestations <= 10);
        vm.assume(_valuePerAttestation > 0 && _valuePerAttestation <= 10 ether);
        
        Attestation[] memory attestations = new Attestation[](_numAttestations);
        uint256[] memory values = new uint256[](_numAttestations);
        
        uint256 totalRequired = _numAttestations * _valuePerAttestation;
        uint256 insufficientAmount = totalRequired - 1;
        
        // Set up values array
        for(uint i = 0; i < _numAttestations; i++) {
            values[i] = _valuePerAttestation;
        }

        vm.deal(address(eas), totalRequired);

        vm.prank(address(eas));
        vm.expectRevert(InsufficientValue.selector);
        resolver.multiAttest{value: insufficientAmount}(attestations, values);
    }

    /// @dev Tests multi-attestation with ETH value transfers.
    function testMultiAttestationWithValues(
        uint256 _numAttestations,
        uint256 _valuePerAttestation,
        bytes[] calldata _data
    ) public {
        // Ensure reasonable bounds
        vm.assume(_numAttestations > 1 && _numAttestations <= 10);
        vm.assume(_valuePerAttestation > 0 && _valuePerAttestation <= 10 ether);
        vm.assume(_data.length >= _numAttestations);
        
        Attestation[] memory attestations = new Attestation[](_numAttestations);
        uint256[] memory values = new uint256[](_numAttestations);
        uint256 totalValue = _numAttestations * _valuePerAttestation;
        
        // Setup attestations
        for(uint i = 0; i < _numAttestations; i++) {
            attestations[i] = Attestation({
                uid: bytes32(uint256(i + 1)),
                schema: bytes32(0),
                time: uint64(block.timestamp),
                expirationTime: NO_EXPIRATION,
                revocationTime: 0,
                refUID: bytes32(0),
                recipient: recipient,
                attester: address(this),
                revocable: true,
                data: _data[i]
            });
            values[i] = _valuePerAttestation;
        }

        vm.deal(address(eas), totalValue);
        vm.prank(address(eas));
        resolver.multiAttest{value: totalValue}(attestations, values);
    }

    /// @dev Tests revocation functionality with and without value.
    function testRevocationScenarios(
        uint256 _uid,
        uint256 _value,
        bytes calldata _data
    ) public {
        vm.assume(_uid > 0);
        vm.assume(_value > 0 && _value <= 100 ether);
        
        Attestation memory attestation = Attestation({
            uid: bytes32(_uid),
            schema: bytes32(0),
            time: uint64(block.timestamp),
            expirationTime: NO_EXPIRATION,
            revocationTime: 0,
            refUID: bytes32(0),
            recipient: recipient,
            attester: address(this),
            revocable: true,
            data: _data
        });

        // Test free revocation
        vm.prank(address(eas));
        resolver.revoke(attestation);

        // Test paid revocation
        vm.deal(address(eas), _value);
        vm.prank(address(eas));
        resolver.revoke{value: _value}(attestation);
    }

    // Integration Tests
    /// @dev Tests complex multi-attestation scenarios with varying configurations.
    function testComplexResolverScenarios(
        uint64 _expirationOffset,
        uint256[] calldata _values,
        bytes[] calldata _data
    ) public {
        // Ensure valid inputs
        vm.assume(_expirationOffset > 0 && _expirationOffset <= 365 days);
        vm.assume(_values.length >= 3);
        vm.assume(_data.length >= 3);
        
        // Add bounds for values
        for(uint i = 0; i < _values.length; i++) {
            vm.assume(_values[i] <= 100 ether); // Reasonable upper bound for values
        }
        
        // Add bounds for data length
        for(uint i = 0; i < _data.length; i++) {
            vm.assume(_data[i].length <= 1024); // Reasonable upper bound for data size
        }

        Attestation[] memory attestations = new Attestation[](3);
        uint256[] memory values = new uint256[](3);
        uint256 totalValue = 0;
        
        for(uint i = 0; i < 3; i++) {
            attestations[i] = Attestation({
                uid: bytes32(uint256(i + 1)),
                schema: bytes32(0),
                time: uint64(block.timestamp),
                expirationTime: i == 0 ? uint64(block.timestamp + _expirationOffset) : NO_EXPIRATION,
                revocationTime: 0,
                refUID: i == 0 ? bytes32(0) : attestations[0].uid,
                recipient: recipient,
                attester: address(this),
                revocable: true,
                data: _data[i]
            });
            values[i] = _values[i];
            totalValue += _values[i];
        }

        vm.assume(totalValue <= address(this).balance); // Ensure we can afford the total value
        vm.deal(address(eas), totalValue);

        vm.prank(address(eas));
        resolver.multiAttest{value: totalValue}(attestations, values);
    }

    /// @dev Tests interactions with payable resolver implementation.
    function testPayableResolverInteractions(
        uint256 _uid,
        uint256 _attestValue,
        uint256 _revokeValue,
        bytes calldata _data
    ) public {
        // Ensure valid inputs
        vm.assume(_uid > 0);
        vm.assume(_attestValue > 0 && _attestValue <= 100 ether);
        vm.assume(_revokeValue > 0 && _revokeValue <= 100 ether);
        
        Attestation memory attestation = Attestation({
            uid: bytes32(_uid),
            schema: bytes32(0),
            time: uint64(block.timestamp),
            expirationTime: NO_EXPIRATION,
            revocationTime: 0,
            refUID: bytes32(0),
            recipient: recipient,
            attester: address(this),
            revocable: true,
            data: _data
        });

        // Test paid attestation
        vm.deal(address(eas), _attestValue);
        vm.prank(address(eas));
        payableResolver.attest{value: _attestValue}(attestation);

        // Test paid revocation
        vm.deal(address(eas), _revokeValue);
        vm.prank(address(eas));
        payableResolver.revoke{value: _revokeValue}(attestation);
    }
}
