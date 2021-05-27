// SPDX-License-Identifier: MIT
// @unsupported: evm
pragma solidity >=0.7.0;

contract ValueCallsWithCompiler {

    receive() external payable { }

    function getBalance(
        address _address
    ) external payable returns(uint256) {
        return _address.balance;
    }

    function simpleSend(
        address _address,
        uint _value
    ) external payable returns (bool, bytes memory) {
        return sendWithData(_address, _value, hex"");
    }

    function sendWithData(
        address _address,
        uint _value,
        bytes memory _calldata
    ) public returns (bool, bytes memory) {
        return _address.call{value: _value}(_calldata);
    }

    function verifyCallValueAndRevert(
        uint256 _expectedValue
    ) external payable {
        bool correct = _checkCallValue(_expectedValue);
        // do the opposite of expected if the value is wrong.
        if (correct) {
            revert("expected revert");
        } else {
            return;
        }
    }

    function getCallValue() public payable returns(uint256) {
        return msg.value;
    }

    function verifyCallValueAndReturn(
        uint256 _expectedValue
    ) external payable {
        bool correct = _checkCallValue(_expectedValue);
        // do the opposite of expected if the value is wrong.
        if (correct) {
            return;
        } else {
            revert("unexpected revert");
        }
    }

    function _checkCallValue(
        uint256 _expectedValue
    ) internal returns(bool) {
        return getCallValue() == _expectedValue;
    }
}