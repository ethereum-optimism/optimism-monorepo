// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";

// Interfaces
import { ISemver } from "interfaces/universal/ISemver.sol";
import { IL1Block } from "interfaces/L2/IL1Block.sol";

/// @custom:legacy true
/// @custom:proxied true
/// @custom:predeploy 0x4200000000000000000000000000000000000013
/// @title L1BlockNumber
/// @notice L1BlockNumber is a legacy contract that fills the roll of the OVM_L1BlockNumber contract
///        in the old version of the Optimism system. Only necessary for backwards compatibility.
///        If you want to access the L1 block number going forward, you should use the L1Block
///        contract instead.
contract L1BlockNumber is ISemver {
    /// @notice Semantic version.
    /// @custom:semver 1.1.1-beta.3
    string public constant version = "1.1.1-beta.3";

    /// @notice Returns the L1 block number.
    receive() external payable {
        uint256 l1BlockNumber = getL1BlockNumber();
        assembly {
            mstore(0, l1BlockNumber)
            return(0, 32)
        }
    }

    /// @notice Returns the L1 block number.
    fallback() external payable {
        uint256 l1BlockNumber = getL1BlockNumber();
        assembly {
            mstore(0, l1BlockNumber)
            return(0, 32)
        }
    }

    /// @notice Retrieves the latest L1 block number.
    /// @return Latest L1 block number.
    function getL1BlockNumber() public view returns (uint256) {
        return IL1Block(Predeploys.L1_BLOCK_ATTRIBUTES).number();
    }
}
