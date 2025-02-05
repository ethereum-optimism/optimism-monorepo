// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { Ownable } from "@openzeppelin/contracts/access/Ownable.sol";
import { IERC1271 } from "@openzeppelin/contracts/interfaces/IERC1271.sol";
import { ECDSA } from "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";

/// @notice Simple ERC1271 wallet that can be used to test the ERC1271 signature checker.
/// @notice https://github.com/OpenZeppelin/openzeppelin-contracts/
///         blob/master/contracts/mocks/ERC1271WalletMock.sol
contract TestERC1271Wallet is Ownable, IERC1271 {
    constructor(address originalOwner) {
        transferOwnership(originalOwner);
    }

    function isValidSignature(
        bytes32 _hash,
        bytes memory _signature
    )
        public
        view
        override
        returns (bytes4 magicValue_)
    {
        return ECDSA.recover(_hash, _signature) == owner() ? this.isValidSignature.selector : bytes4(0);
    }
}
