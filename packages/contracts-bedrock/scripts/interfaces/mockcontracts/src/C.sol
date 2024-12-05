// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.13;


contract C {
    uint256 public cVar;
    type CType is uint128;

    error CError();
    event CEvent();

    struct CStruct {
        CType var1;
        uint256 var2;
    }

    constructor(string memory constructorC) {

    }

    function cFuncPublic() public {

    }

    function cFuncExternal() public {

    }
}
