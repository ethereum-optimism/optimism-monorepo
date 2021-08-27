// SPDX-License-Identifier: MIT
pragma solidity >=0.7.0 <0.8.0;

contract BlockHashGetter {
    function getBlockHash(uint256 _blockNumber) public view returns (bytes32) {
        return blockhash(_blockNumber);
    }
}
