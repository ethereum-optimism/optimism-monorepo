// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { CommonTest } from "test/setup/CommonTest.sol";

import { LegacyMintableERC20 } from "src/legacy/LegacyMintableERC20.sol";
import { ILegacyMintableERC20 } from "interfaces/universal/ILegacyMintableERC20.sol";

contract LegacyMintableERC20_Test is CommonTest {
    LegacyMintableERC20 legacyMintableERC20;

    function setUp() public override {
        super.setUp();

        legacyMintableERC20 = new LegacyMintableERC20(address(l2StandardBridge), address(L1Token), "_L2Token_", "_L2T_");
    }

    /// @notice Tests that the constructor sets the correct values
    function test_constructor_works() public view {
        assertEq(legacyMintableERC20.l2Bridge(), address(l2StandardBridge));
        assertEq(legacyMintableERC20.l1Token(), address(L1Token));
        assertEq(legacyMintableERC20.name(), "_L2Token_");
        assertEq(legacyMintableERC20.symbol(), "_L2T_");
        assertEq(legacyMintableERC20.decimals(), 18);
    }

    /// @notice Tests that the contract supports the correct interfaces
    function test_supportsInterface_works() public view {
        assertEq(legacyMintableERC20.supportsInterface(bytes4(keccak256("supportsInterface(bytes4)"))), true);
        assertEq(
            legacyMintableERC20.supportsInterface(
                ILegacyMintableERC20.l1Token.selector ^ ILegacyMintableERC20.mint.selector
                    ^ ILegacyMintableERC20.burn.selector
            ),
            true
        );
    }

    /// @notice Tests that the mint function works when called by the bridge
    function test_mint_byBridge_succeeds() public {
        vm.prank(address(l2StandardBridge));
        legacyMintableERC20.mint(address(this), 1000);
        assertEq(legacyMintableERC20.balanceOf(address(this)), 1000);
    }

    /// @notice Tests that the mint function fails when called by an address other than the bridge
    function test_mint_byNonBridge_reverts() public {
        vm.expectRevert(bytes("Only L2 Bridge can mint and burn"));
        legacyMintableERC20.mint(address(this), 1000);
    }

    /// @notice Tests that the burn function works when called by the bridge
    function test_burn_byBridge_succeeds() public {
        vm.prank(address(l2StandardBridge));
        legacyMintableERC20.mint(address(this), 1000);

        vm.prank(address(l2StandardBridge));
        legacyMintableERC20.burn(address(this), 1000);
        assertEq(legacyMintableERC20.balanceOf(address(this)), 0);
    }

    /// @notice Tests that the burn function fails when called by an address other than the bridge
    function test_burn_byNonBridge_reverts() public {
        vm.expectRevert(bytes("Only L2 Bridge can mint and burn"));
        legacyMintableERC20.burn(address(this), 1000);
    }
}
