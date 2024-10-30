// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { OptimismMintableERC20Factory } from "src/universal/OptimismMintableERC20Factory.sol";
import { Initializable } from "@openzeppelin/contracts/proxy/utils/Initializable.sol";

/// @custom:proxied true
/// @title L1OptimismMintableERC20Factory
/// @notice Allows users to create L1 tokens that represent L2 native tokens.
contract L1OptimismMintableERC20Factory is OptimismMintableERC20Factory, Initializable {
    /// @custom:semver 1.3.1-beta.5
    /// @notice Semantic version.
    ///         The semver MUST be bumped any time that there is a change in
    ///         the OptimismMintableERC20 token contract since this contract
    ///         is responsible for deploying OptimismMintableERC20 contracts.
    string public constant version = "1.3.1-beta.5";

    /// @custom:spacer
    /// @notice Spacer to fill the remainder of the _initialized slot, preventing the standardBridge
    ///         address from being packed with it.
    bytes30 private spacer_51_2_30;

    /// @notice Address of the bridge on this domain.
    address internal standardBridge;

    constructor() {
        _disableInitializers();
    }

    /// @notice Initializes the contract.
    /// @param _bridge Contract of the bridge on this domain.
    function initialize(address _bridge) public initializer {
        standardBridge = _bridge;
    }

    /// @notice Getter function for the bridge contract.
    /// @return Contract of the bridge on this domain.
    function bridge() public view virtual override returns (address) {
        return standardBridge;
    }
}
