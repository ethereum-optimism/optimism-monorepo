// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { ERC721, IERC721 } from "@openzeppelin/contracts/token/ERC721/ERC721.sol";
import { IERC721Enumerable } from "@openzeppelin/contracts/token/ERC721/extensions/ERC721Enumerable.sol";
import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";
import { Strings } from "@openzeppelin/contracts/utils/Strings.sol";
import { CommonTest } from "test/setup/CommonTest.sol";
import { OptimismMintableERC721, IOptimismMintableERC721 } from "src/universal/OptimismMintableERC721.sol";

contract OptimismMintableERC721_Test is CommonTest {
    ERC721 internal L1NFT;
    OptimismMintableERC721 internal L2NFT;

    event Transfer(address indexed from, address indexed to, uint256 indexed tokenId);

    event Mint(address indexed account, uint256 tokenId);

    event Burn(address indexed account, uint256 tokenId);

    function setUp() public override {
        super.setUp();

        // Set up the token pair.
        L1NFT = new ERC721("L1NFT", "L1T");
        L2NFT = new OptimismMintableERC721(address(l2ERC721Bridge), 1, address(L1NFT), "L2NFT", "L2T");

        // Label the addresses for nice traces.
        vm.label(address(L1NFT), "L1ERC721Token");
        vm.label(address(L2NFT), "L2ERC721Token");
    }

    /// @notice Tests that the constructor works as expected.
    function test_constructor_succeeds() external view {
        assertEq(L2NFT.name(), "L2NFT");
        assertEq(L2NFT.symbol(), "L2T");
        assertEq(L2NFT.remoteToken(), address(L1NFT));
        assertEq(L2NFT.bridge(), address(l2ERC721Bridge));
        assertEq(L2NFT.remoteChainId(), 1);
        assertEq(L2NFT.REMOTE_TOKEN(), address(L1NFT));
        assertEq(L2NFT.BRIDGE(), address(l2ERC721Bridge));
        assertEq(L2NFT.REMOTE_CHAIN_ID(), 1);
    }

    /// @notice Tests that the bridge cannot be address(0) at construction time.
    function test_constructor_bridgeAsAddress0_reverts() external {
        vm.expectRevert("OptimismMintableERC721: bridge cannot be address(0)");
        L2NFT = new OptimismMintableERC721(address(0), 1, address(L1NFT), "L2NFT", "L2T");
    }

    /// @notice Tests that the remote chain ID cannot be zero at construction time.
    function test_constructor_remoteChainId0_reverts() external {
        vm.expectRevert("OptimismMintableERC721: remote chain id cannot be zero");
        L2NFT = new OptimismMintableERC721(address(l2ERC721Bridge), 0, address(L1NFT), "L2NFT", "L2T");
    }

    /// @notice Tests that the remote token cannot be address(0) at construction time.
    function test_constructor_remoteTokenAsAddress0_reverts() external {
        vm.expectRevert("OptimismMintableERC721: remote token cannot be address(0)");
        L2NFT = new OptimismMintableERC721(address(l2ERC721Bridge), 1, address(0), "L2NFT", "L2T");
    }

    /// @notice Ensure that the contract supports the expected interfaces.
    function test_supportsInterfaces_succeeds() external view {
        // Checks if the contract supports the IOptimismMintableERC721 interface.
        assertTrue(L2NFT.supportsInterface(type(IOptimismMintableERC721).interfaceId));
        // Checks if the contract supports the IERC721Enumerable interface.
        assertTrue(L2NFT.supportsInterface(type(IERC721Enumerable).interfaceId));
        // Checks if the contract supports the IERC721 interface.
        assertTrue(L2NFT.supportsInterface(type(IERC721).interfaceId));
        // Checks if the contract supports the IERC165 interface.
        assertTrue(L2NFT.supportsInterface(type(IERC165).interfaceId));
    }

    function test_safeMint_succeeds() external {
        // Expect a transfer event.
        vm.expectEmit(true, true, true, true);
        emit Transfer(address(0), alice, 1);

        // Expect a mint event.
        vm.expectEmit(true, true, true, true);
        emit Mint(alice, 1);

        // Mint the token.
        vm.prank(address(l2ERC721Bridge));
        L2NFT.safeMint(alice, 1);

        // Token should be owned by alice.
        assertEq(L2NFT.ownerOf(1), alice);
    }

    function test_safeMint_notBridge_reverts() external {
        // Try to mint the token.
        vm.expectRevert("OptimismMintableERC721: only bridge can call this function");
        vm.prank(address(alice));
        L2NFT.safeMint(alice, 1);
    }

    function test_burn_succeeds() external {
        // Mint the token first.
        vm.prank(address(l2ERC721Bridge));
        L2NFT.safeMint(alice, 1);

        // Expect a transfer event.
        vm.expectEmit(true, true, true, true);
        emit Transfer(alice, address(0), 1);

        // Expect a burn event.
        vm.expectEmit(true, true, true, true);
        emit Burn(alice, 1);

        // Burn the token.
        vm.prank(address(l2ERC721Bridge));
        L2NFT.burn(alice, 1);

        // Token should be owned by address(0).
        vm.expectRevert("ERC721: invalid token ID");
        L2NFT.ownerOf(1);
    }

    function test_burn_notBridge_reverts() external {
        // Mint the token first.
        vm.prank(address(l2ERC721Bridge));
        L2NFT.safeMint(alice, 1);

        // Try to burn the token.
        vm.expectRevert("OptimismMintableERC721: only bridge can call this function");
        vm.prank(address(alice));
        L2NFT.burn(alice, 1);
    }

    function test_tokenURI_succeeds() external {
        // Mint the token first.
        vm.prank(address(l2ERC721Bridge));
        L2NFT.safeMint(alice, 1);

        // Token URI should be correct.
        assertEq(
            L2NFT.tokenURI(1),
            string(
                abi.encodePacked(
                    "ethereum:",
                    Strings.toHexString(uint160(address(L1NFT)), 20),
                    "@",
                    Strings.toString(1),
                    "/tokenURI?uint256=",
                    Strings.toString(1)
                )
            )
        );
    }
}
