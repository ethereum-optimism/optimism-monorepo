// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.13;

import "./B.sol";

contract A is B {
    uint256 public aVar;
    type AType is uint128;

    error AError();
    event AEvent();

    struct AStruct {
        AType var1;
        uint256 var2;
    }

    constructor() B(2) {

    }

    function aFuncPublic() public {

    }

    function aFuncExternal() public {

    }
}
