// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { CommonTest } from "test/setup/CommonTest.sol";
import { IOptimismMintableERC20 } from "src/universal/interfaces/IOptimismMintableERC20.sol";
import { ILegacyMintableERC20 } from "src/universal/interfaces/ILegacyMintableERC20.sol";
import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";

contract OptimismMintableERC20_Test is CommonTest {
    event Mint(address indexed account, uint256 amount);
    event Burn(address indexed account, uint256 amount);

    function test_remoteToken_succeeds() external view {
        assertEq(L2Token.remoteToken(), address(L1Token));
    }

    function test_bridge_succeeds() external view {
        assertEq(L2Token.bridge(), address(l2StandardBridge));
    }

    function test_l1Token_succeeds() external view {
        assertEq(L2Token.l1Token(), address(L1Token));
    }

    function test_l2Bridge_succeeds() external view {
        assertEq(L2Token.l2Bridge(), address(l2StandardBridge));
    }

    function test_legacy_succeeds() external view {
        // Getters for the remote token
        assertEq(L2Token.REMOTE_TOKEN(), address(L1Token));
        assertEq(L2Token.remoteToken(), address(L1Token));
        assertEq(L2Token.l1Token(), address(L1Token));
        // Getters for the bridge
        assertEq(L2Token.BRIDGE(), address(l2StandardBridge));
        assertEq(L2Token.bridge(), address(l2StandardBridge));
        assertEq(L2Token.l2Bridge(), address(l2StandardBridge));
    }

    function test_mint_succeeds() external {
        vm.expectEmit(true, true, true, true);
        emit Mint(alice, 100);

        vm.prank(address(l2StandardBridge));
        L2Token.mint(alice, 100);

        assertEq(L2Token.balanceOf(alice), 100);
    }

    function test_allowance_permit2Max_works() external view {
        assertEq(L2Token.allowance(alice, L2Token.PERMIT2()), type(uint256).max);
    }

    function test_permit2_transferFrom_succeeds() external {
        vm.prank(address(l2StandardBridge));
        L2Token.mint(alice, 100);

        assertEq(L2Token.balanceOf(bob), 0);
        vm.prank(L2Token.PERMIT2());
        L2Token.transferFrom(alice, bob, 100);
        assertEq(L2Token.balanceOf(bob), 100);
    }

    function test_mint_notBridge_reverts() external {
        // NOT the bridge
        vm.expectRevert("OptimismMintableERC20: only bridge can mint and burn");
        vm.prank(address(alice));
        L2Token.mint(alice, 100);
    }

    function test_burn_succeeds() external {
        vm.prank(address(l2StandardBridge));
        L2Token.mint(alice, 100);

        vm.expectEmit(true, true, true, true);
        emit Burn(alice, 100);

        vm.prank(address(l2StandardBridge));
        L2Token.burn(alice, 100);

        assertEq(L2Token.balanceOf(alice), 0);
    }

    function test_burn_notBridge_reverts() external {
        // NOT the bridge
        vm.expectRevert("OptimismMintableERC20: only bridge can mint and burn");
        vm.prank(address(alice));
        L2Token.burn(alice, 100);
    }

    function test_erc165_supportsInterface_succeeds() external view {
        // The assertEq calls in this test are comparing the manual calculation of the iface,
        // with what is returned by the solidity's type().interfaceId, just to be safe.
        bytes4 iface1 = bytes4(keccak256("supportsInterface(bytes4)"));
        assertEq(iface1, type(IERC165).interfaceId);
        assert(L2Token.supportsInterface(iface1));

        bytes4 iface2 = L2Token.l1Token.selector ^ L2Token.mint.selector ^ L2Token.burn.selector;
        assertEq(iface2, type(ILegacyMintableERC20).interfaceId);
        assert(L2Token.supportsInterface(iface2));

        bytes4 iface3 =
            L2Token.remoteToken.selector ^ L2Token.bridge.selector ^ L2Token.mint.selector ^ L2Token.burn.selector;
        assertEq(iface3, type(IOptimismMintableERC20).interfaceId);
        assert(L2Token.supportsInterface(iface3));
    }
}
