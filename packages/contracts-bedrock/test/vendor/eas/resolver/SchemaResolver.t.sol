// SPDX-License-Identifier: MIT
pragma solidity 0.8.19;

import { Test } from "forge-std/Test.sol";
import { SchemaResolver } from "src/vendor/eas/resolver/SchemaResolver.sol";
import { IEAS, Attestation } from "src/vendor/eas/IEAS.sol";
import { ISchemaResolver } from "src/vendor/eas/resolver/ISchemaResolver.sol";

// =============================================================
//                        MOCK CONTRACTS 
// =============================================================

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

/// @dev Minimal mock EAS implementation for testing
contract MockEAS {
    /// @dev Mock attestation that always succeeds
    function attest(Attestation calldata) external payable returns (bool) {
        return true;
    }
}

/// @dev Mock resolver that accepts payments for testing fee handling
contract MockPayableResolver is ISchemaResolver {
    /// @dev Indicates resolver accepts payments
    function isPayable() external pure override returns (bool) {
        return true;
    }

    /// @dev Mock payable attestation
    function attest(Attestation calldata) external payable override returns (bool) {
        return true;
    }

    /// @dev Mock payable multi-attestation
    function multiAttest(Attestation[] calldata, uint256[] calldata) external payable override returns (bool) {
        return true;
    }

    /// @dev Mock payable revocation
    function revoke(Attestation calldata) external payable override returns (bool) {
        return true;
    }

    /// @dev Mock payable multi-revocation
    function multiRevoke(Attestation[] calldata, uint256[] calldata) external payable override returns (bool) {
        return true;
    }
}

// =============================================================
//                        MAIN TEST CONTRACT
// =============================================================

contract SchemaResolverTest is Test {

    // =============================================================
    //                           CONSTANTS
    // =============================================================
    uint64 constant NO_EXPIRATION = 0;

    // =============================================================
    //                          TEST STATE
    // =============================================================
    TestSchemaResolver public resolver;
    MockEAS public eas;
    address public recipient;

    // =============================================================
    //                         ERROR TYPES
    // =============================================================
    error AccessDenied();
    error InvalidLength();
    error NotPayable();
    error InsufficientValue();

    // =============================================================
    //                           SETUP
    // =============================================================
    /// @dev Deploys mock contracts and sets up test environment
    function setUp() public {
        eas = new MockEAS();
        resolver = new TestSchemaResolver(IEAS(address(eas)));
        recipient = makeAddr("recipient");
    }

    // =============================================================
    //                      BASIC STATE TESTS
    // =============================================================
    /// @dev Tests initial resolver configuration
    function testInitialState() public view {
        assertEq(resolver.version(), "1.3.0");
        assertEq(resolver.isPayable(), false);
    }

    // =============================================================
    //                    ACCESS CONTROL TESTS
    // =============================================================
    /// @dev Tests that only EAS can call resolver functions
    function testOnlyEASCanCall() public {
        Attestation memory attestation = Attestation({
            uid: bytes32(0),
            schema: bytes32(0),
            time: uint64(block.timestamp),
            expirationTime: uint64(0),
            revocationTime: uint64(0),
            refUID: bytes32(0),
            recipient: recipient,
            attester: address(this),
            revocable: true,
            data: new bytes(0)
        });

        // Should revert when called by non-EAS address
        vm.expectRevert(AccessDenied.selector);
        resolver.attest(attestation);

        vm.expectRevert(AccessDenied.selector);
        resolver.revoke(attestation);
    }

    // =============================================================
    //                     VALIDATION TESTS
    // =============================================================
    /// @dev Tests rejection of ETH transfers to non-payable resolver
    function testNonPayableResolver() public {
        // Should revert when sending ETH to non-payable resolver
        vm.expectRevert(NotPayable.selector);
        payable(address(resolver)).transfer(1 ether);
    }

    /// @dev Tests array length validation in multi-attestations
    function testMultiAttestInvalidLength() public {
        Attestation[] memory attestations = new Attestation[](2);
        uint256[] memory values = new uint256[](1);

        vm.prank(address(eas));
        vm.expectRevert(InvalidLength.selector);
        resolver.multiAttest(attestations, values);
    }

    /// @dev Tests array length validation in multi-revocations
    function testMultiRevokeInvalidLength() public {
        Attestation[] memory attestations = new Attestation[](2);
        uint256[] memory values = new uint256[](1);

        vm.prank(address(eas));
        // Should revert with InvalidLength
        vm.expectRevert(InvalidLength.selector);
        resolver.multiRevoke(attestations, values);
    }

    /// @dev Tests value validation for paid attestations
    function testInsufficientValue() public {
        Attestation[] memory attestations = new Attestation[](2);
        uint256[] memory values = new uint256[](2);
        values[0] = 1 ether;
        values[1] = 1 ether;

        // Fund the EAS contract with ETH
        vm.deal(address(eas), 2 ether);

        vm.prank(address(eas));
        // Should revert with InsufficientValue
        vm.expectRevert(InsufficientValue.selector);
        resolver.multiAttest{value: 1 ether}(attestations, values);
    }

    // =============================================================
    //                    ATTESTATION TESTS
    // =============================================================
    /// @dev Tests multi-attestation with value transfers
    function testMultiAttestationWithValues() public {
        Attestation[] memory attestations = new Attestation[](2);
        uint256[] memory values = new uint256[](2);
        
        // Setup attestations
        for(uint i = 0; i < 2; i++) {
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
                data: new bytes(0)
            });
            values[i] = 1 ether;
        }

        // Fund the contract
        vm.deal(address(eas), 2 ether);

        vm.prank(address(eas));
        resolver.multiAttest{value: 2 ether}(attestations, values);
    }

    // =============================================================
    //                    REVOCATION TESTS
    // =============================================================
    /// @dev Tests various revocation scenarios
    function testRevocationScenarios() public {
        Attestation memory attestation = Attestation({
            uid: bytes32(uint256(1)),
            schema: bytes32(0),
            time: uint64(block.timestamp),
            expirationTime: NO_EXPIRATION,
            revocationTime: 0,
            refUID: bytes32(0),
            recipient: recipient,
            attester: address(this),
            revocable: true,
            data: new bytes(0)
        });

        vm.prank(address(eas));
        resolver.revoke(attestation);

        // Test revocation with value
        vm.deal(address(eas), 1 ether);
        vm.prank(address(eas));
        resolver.revoke{value: 1 ether}(attestation);
    }

    // =============================================================
    //                    INTEGRATION TESTS
    // =============================================================
    /// @dev Tests complex attestation scenarios with multiple configurations
    function testComplexResolverScenarios() public {
        Attestation[] memory attestations = new Attestation[](3);
        uint256[] memory values = new uint256[](3);
        
        // Setup attestations with different configurations
        for(uint i = 0; i < 3; i++) {
            attestations[i] = Attestation({
                uid: bytes32(uint256(i + 1)),
                schema: bytes32(0),
                time: uint64(block.timestamp),
                expirationTime: i == 0 ? uint64(block.timestamp + 1 days) : NO_EXPIRATION,
                revocationTime: 0,
                refUID: i == 0 ? bytes32(0) : attestations[0].uid,
                recipient: recipient,
                attester: address(this),
                revocable: true,
                data: new bytes(i + 1)
            });
            values[i] = i * 0.5 ether;
        }

        // Fund the contract
        vm.deal(address(eas), 1.5 ether);

        vm.prank(address(eas));
        resolver.multiAttest{value: 1.5 ether}(attestations, values);
    }

    /// @dev Tests interactions with payable resolver implementation
    function testPayableResolverInteractions() public {
        // Create a payable resolver instance
        MockPayableResolver payableResolver = new MockPayableResolver();
        
        Attestation memory attestation = Attestation({
            uid: bytes32(uint256(1)),
            schema: bytes32(0),
            time: uint64(block.timestamp),
            expirationTime: NO_EXPIRATION,
            revocationTime: 0,
            refUID: bytes32(0),
            recipient: recipient,
            attester: address(this),
            revocable: true,
            data: new bytes(0)
        });

        // Test attestation with value
        vm.deal(address(eas), 1 ether);
        vm.prank(address(eas));
        payableResolver.attest{value: 1 ether}(attestation);

        // Test revocation with value
        vm.deal(address(eas), 1 ether);
        vm.prank(address(eas));
        payableResolver.revoke{value: 1 ether}(attestation);
    }
}
