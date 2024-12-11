// SPDX-License-Identifier: MIT
pragma solidity 0.8.19;

import { Test } from "forge-std/Test.sol";
import { SchemaResolver } from "src/vendor/eas/resolver/SchemaResolver.sol";
import { IEAS, Attestation } from "src/vendor/eas/IEAS.sol";
import { Vm } from "forge-std/Vm.sol"; 
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

// /// @dev Minimal mock EAS implementation for testing
// contract MockEAS {
//     /// @dev Mock attestation that always succeeds
//     function attest(Attestation calldata) external payable returns (bool) {
//         return true;
//     }
// }


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
     ISchemaResolver public payableResolver;
    address public recipient;
    IEAS public eas;

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
    /// @dev Tests resolver access control restrictions.
    ///      1. Setup:
    ///         - Creates minimal attestation data
    ///         - Sets current timestamp
    ///         - Uses empty data fields
    ///      2. Attestation Test:
    ///         - Attempts direct attestation call
    ///         - Verifies revert with AccessDenied
    ///      3. Revocation Test:
    ///         - Attempts direct revocation call
    ///         - Verifies revert with AccessDenied
    ///      Demonstrates:
    ///         - Resolver's EAS-only access control
    ///         - Protection against unauthorized calls
    ///         - Both attest and revoke protection
    ///      Note: Only EAS contract should be able to
    ///      call resolver functions directly
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
    /// @dev Tests ETH transfer rejection by non-payable resolver.
    ///      1. Attempt:
    ///         - Tries to transfer 1 ETH to resolver
    ///      2. Verification:
    ///         - Confirms revert with NotPayable error
    ///      Ensures resolver properly rejects direct ETH transfers
    function testNonPayableResolver() public {
        vm.expectRevert(NotPayable.selector);
        payable(address(resolver)).transfer(1 ether);
    }

    /// @dev Tests multi-attestation array length validation.
    ///      1. Setup:
    ///         - Creates mismatched arrays:
    ///           * 2 attestations
    ///           * 1 value
    ///      2. Verification:
    ///         - Attempts multi-attest as EAS
    ///         - Confirms revert with InvalidLength
    ///      Ensures proper validation of array lengths
    ///      in multi-attestation operations
    function testMultiAttestInvalidLength() public {
        Attestation[] memory attestations = new Attestation[](2);
        uint256[] memory values = new uint256[](1);

        vm.prank(address(eas));
        vm.expectRevert(InvalidLength.selector);
        resolver.multiAttest(attestations, values);
    }

    /// @dev Tests multi-revocation array length validation.
    ///      1. Setup:
    ///         - Creates mismatched arrays:
    ///           * 2 attestations
    ///           * 1 value
    ///      2. Verification:
    ///         - Attempts multi-revoke as EAS
    ///         - Confirms revert with InvalidLength
    ///      Ensures proper validation of array lengths
    ///      in multi-revocation operations
    function testMultiRevokeInvalidLength() public {
        Attestation[] memory attestations = new Attestation[](2);
        uint256[] memory values = new uint256[](1);

        vm.prank(address(eas));
        vm.expectRevert(InvalidLength.selector);
        resolver.multiRevoke(attestations, values);
    }

    /// @dev Tests value validation for paid multi-attestations.
    ///      1. Setup:
    ///         - Creates 2 attestations requiring 1 ETH each
    ///         - Funds EAS with 2 ETH
    ///      2. Attempt:
    ///         - Tries multi-attest with insufficient value (1 ETH)
    ///      3. Verification:
    ///         - Confirms revert with InsufficientValue
    ///      Ensures proper validation of sent ETH value
    ///      against required attestation costs
    function testInsufficientValue() public {
        Attestation[] memory attestations = new Attestation[](2);
        uint256[] memory values = new uint256[](2);
        values[0] = 1 ether;
        values[1] = 1 ether;

        vm.deal(address(eas), 2 ether);

        vm.prank(address(eas));
        vm.expectRevert(InsufficientValue.selector);
        resolver.multiAttest{value: 1 ether}(attestations, values);
    }

    // =============================================================
    //                    ATTESTATION TESTS
    // =============================================================
    /// @dev Tests multi-attestation with ETH value transfers.
    ///      1. Setup:
    ///         - Creates two attestations:
    ///           * Each with unique UID
    ///           * No expiration time
    ///           * No reference UID
    ///           * Empty data
    ///         - Sets value of 1 ETH per attestation
    ///      2. Funding:
    ///         - Provides EAS contract with 2 ETH total
    ///      3. Execution:
    ///         - Calls multiAttest as EAS
    ///         - Transfers full required value (2 ETH)
    ///      Demonstrates:
    ///         - Proper handling of multiple paid attestations
    ///         - Correct value distribution
    ///         - Batch processing with payments
    ///      Note: Ensures system correctly processes
    ///      multiple attestations with associated ETH values
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
    /// @dev Tests revocation functionality with and without value.
    ///      1. Setup:
    ///         - Creates basic attestation:
    ///           * Specific UID (1)
    ///           * No expiration
    ///           * Marked as revocable
    ///           * Empty data
    ///      2. Free Revocation Test:
    ///         - Executes revocation as EAS
    ///         - No value transferred
    ///      3. Paid Revocation Test:
    ///         - Funds EAS with 1 ETH
    ///         - Executes revocation with value
    ///      Demonstrates:
    ///         - Basic revocation functionality
    ///         - Paid revocation handling
    ///         - EAS permission validation
    ///      Note: Tests both zero-value and paid revocation
    ///      scenarios in single attestation context
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

        vm.deal(address(eas), 1 ether);
        vm.prank(address(eas));
        resolver.revoke{value: 1 ether}(attestation);
    }

    // =============================================================
    //                    INTEGRATION TESTS
    // =============================================================
    /// @dev Tests complex multi-attestation scenarios with varying configurations.
    ///      1. Setup:
    ///         - Creates three attestations with different settings:
    ///           First Attestation:
    ///           * 1-day expiration
    ///           * No reference UID
    ///           * 0 ETH value
    ///           Second/Third Attestations:
    ///           * No expiration
    ///           * References first attestation
    ///           * 0.5/1.0 ETH values respectively
    ///           * Increasing data sizes
    ///      2. Funding:
    ///         - Provides EAS with 1.5 ETH total
    ///      3. Execution:
    ///         - Processes all attestations in single transaction
    ///      Demonstrates:
    ///         - Mixed configuration handling
    ///         - Reference chaining
    ///         - Variable payment processing
    ///         - Batch attestation capabilities
    function testComplexResolverScenarios() public {
        Attestation[] memory attestations = new Attestation[](3);
        uint256[] memory values = new uint256[](3);
        
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

        vm.deal(address(eas), 1.5 ether);

        vm.prank(address(eas));
        resolver.multiAttest{value: 1.5 ether}(attestations, values);
    }

    /// @dev Tests interactions with payable resolver implementation.
    ///      1. Setup:
    ///         - Deploys MockPayableResolver
    ///         - Creates basic attestation
    ///      2. Paid Attestation:
    ///         - Funds EAS with 1 ETH
    ///         - Executes attestation with value
    ///      3. Paid Revocation:
    ///         - Funds EAS with 1 ETH
    ///         - Executes revocation with value
    ///      Demonstrates:
    ///         - Payable resolver functionality
    ///         - Value handling in attestations
    ///         - Value handling in revocations
    ///         - EAS interaction with payable resolver
    function testPayableResolverInteractions() public {
        
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

        vm.deal(address(eas), 1 ether);
        vm.prank(address(eas));
        payableResolver.attest{value: 1 ether}(attestation);

        vm.deal(address(eas), 1 ether);
        vm.prank(address(eas));
        payableResolver.revoke{value: 1 ether}(attestation);
    }
}
