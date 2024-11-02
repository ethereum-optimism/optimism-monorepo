// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { CommonTest } from "test/setup/CommonTest.sol";

// Libraries
import { CREATE3, Bytes32AddressLib } from "@rari-capital/solmate/src/utils/CREATE3.sol";

// Target contract
import { IOptimismSuperchainERC20 } from "src/L2/interfaces/IOptimismSuperchainERC20.sol";
import { IERC20Metadata } from "@openzeppelin/contracts/interfaces/IERC20Metadata.sol";

/// @title OptimismSuperchainERC20FactoryTest
/// @notice Contract for testing the OptimismSuperchainERC20Factory contract.
contract OptimismSuperchainERC20FactoryTest is CommonTest {
    using Bytes32AddressLib for bytes32;

    event OptimismSuperchainERC20Created(
        address indexed superchainToken, address indexed remoteToken, address deployer
    );

    /// @notice Sets up the test suite.
    function setUp() public override {
        super.enableInterop();
        super.setUp();
    }

    /// @notice Test that calling `deploy` with valid parameters succeeds.
    function test_deploy_succeeds(
        address _caller,
        address _remoteToken,
        string memory _name,
        string memory _symbol,
        uint8 _decimals
    )
        public
    {
        // Arrange
        bytes32 salt = keccak256(abi.encode(_remoteToken, _name, _symbol, _decimals));
        address deployment = _calculateTokenAddress(salt, address(l2OptimismSuperchainERC20Factory));

        vm.expectEmit(address(l2OptimismSuperchainERC20Factory));
        emit OptimismSuperchainERC20Created(deployment, _remoteToken, _caller);

        // Act
        vm.prank(_caller);
        address addr = l2OptimismSuperchainERC20Factory.deploy(_remoteToken, _name, _symbol, _decimals);

        // Assert
        assertTrue(addr == deployment);
        assertTrue(IERC20Metadata(deployment).decimals() == _decimals);
        assertTrue(IOptimismSuperchainERC20(deployment).remoteToken() == _remoteToken);
        assertEq(IERC20Metadata(deployment).name(), _name);
        assertEq(IERC20Metadata(deployment).symbol(), _symbol);
        assertEq(l2OptimismSuperchainERC20Factory.deployments(deployment), _remoteToken);
    }

    /// @notice Test that calling `deploy` with the same parameters twice reverts.
    function test_deploy_sameTwice_reverts(
        address _caller,
        address _remoteToken,
        string memory _name,
        string memory _symbol,
        uint8 _decimals
    )
        external
    {
        // Arrange
        vm.prank(_caller);
        l2OptimismSuperchainERC20Factory.deploy(_remoteToken, _name, _symbol, _decimals);

        vm.expectRevert(bytes("DEPLOYMENT_FAILED"));

        // Act
        vm.prank(_caller);
        l2OptimismSuperchainERC20Factory.deploy(_remoteToken, _name, _symbol, _decimals);
    }

    /// @notice Precalculates the address of the token contract using CREATE3.
    function _calculateTokenAddress(bytes32 _salt, address _deployer) internal pure returns (address) {
        address proxy =
            keccak256(abi.encodePacked(bytes1(0xFF), _deployer, _salt, CREATE3.PROXY_BYTECODE_HASH)).fromLast20Bytes();

        return keccak256(abi.encodePacked(hex"d694", proxy, hex"01")).fromLast20Bytes();
    }
}
