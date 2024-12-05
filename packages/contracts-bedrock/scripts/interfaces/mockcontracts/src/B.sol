// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.13;

import "./C.sol";

contract B is C {
    uint256 public bVar;
    type BType is uint128;

    error BError();
    event BEvent();

    struct BStruct {
        BType var1;
        uint256 var2;
    }

    constructor(uint256 constructorB) C ("test") {

    }

    function bFuncPublic() public {

    }

    function bFuncExternal() public {

    }
}
