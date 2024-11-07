// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { StandardBridge } from "src/universal/StandardBridge.sol";
import { CommonTest } from "test/setup/CommonTest.sol";
import { OptimismMintableERC20, ILegacyMintableERC20 } from "src/universal/OptimismMintableERC20.sol";
import { ERC20 } from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import { Constants } from "src/libraries/Constants.sol";

/// @title StandardBridgeTester
/// @notice Simple wrapper around the StandardBridge contract that exposes
///         internal functions so they can be more easily tested directly.
contract StandardBridgeTester is StandardBridge {
    constructor() StandardBridge() { }

    function isOptimismMintableERC20(address _token) external view returns (bool) {
        return _isOptimismMintableERC20(_token);
    }

    function isCorrectTokenPair(address _mintableToken, address _otherToken) external view returns (bool) {
        return _isCorrectTokenPair(_mintableToken, _otherToken);
    }

    function gasPayingToken() internal pure override returns (address, uint8) {
        return (Constants.ETHER, 18);
    }

    receive() external payable override { }
}

/// @title LegacyMintable
/// @notice Simple implementation of the legacy OptimismMintableERC20.
contract LegacyMintable is ERC20 {
    constructor(string memory _name, string memory _ticker) ERC20(_name, _ticker) { }

    function l1Token() external pure returns (address) {
        return address(0);
    }

    function mint(address _to, uint256 _amount) external pure { }

    function burn(address _from, uint256 _amount) external pure { }

    /// @notice Implements ERC165. This implementation should not be changed as
    ///         it is how the actual legacy optimism mintable token does the
    ///         check. Allows for testing against code that is has been deployed,
    ///         assuming different compiler version is no problem.
    function supportsInterface(bytes4 _interfaceId) external pure returns (bool) {
        bytes4 firstSupportedInterface = bytes4(keccak256("supportsInterface(bytes4)")); // ERC165
        bytes4 secondSupportedInterface = ILegacyMintableERC20.l1Token.selector ^ ILegacyMintableERC20.mint.selector
            ^ ILegacyMintableERC20.burn.selector;
        return _interfaceId == firstSupportedInterface || _interfaceId == secondSupportedInterface;
    }
}

/// @title StandardBridge_Stateless_Test
/// @notice Tests internal functions that require no existing state or contract
///         interactions with the messenger.
contract StandardBridge_Stateless_Test is CommonTest {
    StandardBridgeTester internal bridge;
    OptimismMintableERC20 internal mintable;
    ERC20 internal erc20;
    LegacyMintable internal legacy;

    function setUp() public override {
        super.setUp();

        bridge = new StandardBridgeTester();

        mintable = new OptimismMintableERC20({
            _bridge: address(0),
            _remoteToken: address(0),
            _name: "Stonks",
            _symbol: "STONK",
            _decimals: 18
        });

        erc20 = new ERC20("Altcoin", "ALT");
        legacy = new LegacyMintable("Legacy", "LEG");
    }

    /// @notice Test coverage for identifying OptimismMintableERC20 tokens.
    ///         This function should return true for both modern and legacy
    ///         OptimismMintableERC20 tokens and false for any accounts that
    ///         do not implement the interface.
    function test_isOptimismMintableERC20_succeeds() external view {
        // Both the modern and legacy mintable tokens should return true
        assertTrue(bridge.isOptimismMintableERC20(address(mintable)));
        assertTrue(bridge.isOptimismMintableERC20(address(legacy)));
        // A regular ERC20 should return false
        assertFalse(bridge.isOptimismMintableERC20(address(erc20)));
        // Non existent contracts should return false and not revert
        assertEq(address(0x20).code.length, 0);
        assertFalse(bridge.isOptimismMintableERC20(address(0x20)));
    }

    /// @notice Test coverage of isCorrectTokenPair under different types of
    ///         tokens.
    function test_isCorrectTokenPair_succeeds() external {
        // Modern + known to be correct remote token
        assertTrue(bridge.isCorrectTokenPair(address(mintable), mintable.remoteToken()));
        // Modern + known to be correct l1Token (legacy interface)
        assertTrue(bridge.isCorrectTokenPair(address(mintable), mintable.l1Token()));
        // Modern + known to be incorrect remote token
        assertTrue(mintable.remoteToken() != address(0x20));
        assertFalse(bridge.isCorrectTokenPair(address(mintable), address(0x20)));
        // Legacy + known to be correct l1Token
        assertTrue(bridge.isCorrectTokenPair(address(legacy), legacy.l1Token()));
        // Legacy + known to be incorrect l1Token
        assertTrue(legacy.l1Token() != address(0x20));
        assertFalse(bridge.isCorrectTokenPair(address(legacy), address(0x20)));
        // A token that doesn't support either modern or legacy interface
        // will revert
        vm.expectRevert(bytes(""));
        bridge.isCorrectTokenPair(address(erc20), address(1));
    }

    /// @notice The bridge by default should be unpaused.
    function test_paused_succeeds() external view {
        assertFalse(bridge.paused());
    }
}
