// SPDX-License-Identifier: MIT
pragma solidity 0.8.19;

import { Test } from "forge-std/Test.sol";
import { SchemaResolver } from "src/vendor/eas/resolver/SchemaResolver.sol";
import { IEAS, Attestation } from "src/vendor/eas/IEAS.sol";
import { ISchemaResolver } from "src/vendor/eas/resolver/ISchemaResolver.sol";

contract TestSchemaResolver is SchemaResolver {
    constructor(IEAS eas) SchemaResolver(eas) {}

    function onAttest(Attestation calldata, uint256) internal pure override returns (bool) {
        return true;
    }

    function onRevoke(Attestation calldata, uint256) internal pure override returns (bool) {
        return true;
    }

    function version() external pure returns (string memory) {
        return "1.3.0";
    }
}

contract MockEAS {
    function attest(Attestation calldata) external payable returns (bool) {
        return true;
    }
}

contract MockPayableResolver is ISchemaResolver {
    function isPayable() external pure override returns (bool) {
        return true;
    }

    function attest(Attestation calldata) external payable override returns (bool) {
        return true;
    }

    function multiAttest(Attestation[] calldata, uint256[] calldata) external payable override returns (bool) {
        return true;
    }

    function revoke(Attestation calldata) external payable override returns (bool) {
        return true;
    }

    function multiRevoke(Attestation[] calldata, uint256[] calldata) external payable override returns (bool) {
        return true;
    }
}

contract SchemaResolverTest is Test {
    TestSchemaResolver public resolver;
    MockEAS public eas;
    address public recipient;
    uint64 constant NO_EXPIRATION = 0;

    error AccessDenied();
    error InvalidLength();
    error NotPayable();
    error InsufficientValue();


    function setUp() public {
        eas = new MockEAS();
        resolver = new TestSchemaResolver(IEAS(address(eas)));
        recipient = makeAddr("recipient");
    }

    function testInitialState() public view {
        assertEq(resolver.version(), "1.3.0");
        assertEq(resolver.isPayable(), false);
    }

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

    function testNonPayableResolver() public {
        // Should revert when sending ETH to non-payable resolver
        vm.expectRevert(NotPayable.selector);
        payable(address(resolver)).transfer(1 ether);
    }

    function testMultiAttestInvalidLength() public {
        Attestation[] memory attestations = new Attestation[](2);
        uint256[] memory values = new uint256[](1);

        vm.prank(address(eas));
        vm.expectRevert(InvalidLength.selector);
        resolver.multiAttest(attestations, values);
    }

    function testMultiRevokeInvalidLength() public {
        Attestation[] memory attestations = new Attestation[](2);
        uint256[] memory values = new uint256[](1);

        vm.prank(address(eas));
        // Should revert with InvalidLength
        vm.expectRevert(InvalidLength.selector);
        resolver.multiRevoke(attestations, values);
    }

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
