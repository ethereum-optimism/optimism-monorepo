// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { GnosisSafe as Safe } from "safe-contracts/GnosisSafe.sol";
import { ISemver } from "interfaces/universal/ISemver.sol";
import { IDeputyGuardianModule } from "interfaces/safe/IDeputyGuardianModule.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";

interface IDeputyPauseModule is ISemver {
    error DeputyPauseModule_InvalidDeputy();
    error DeputyPauseModule_InvalidDeputyGuardianModule();
    error DeputyPauseModule_ExecutionFailed(string);
    error DeputyPauseModule_SuperchainNotPaused();
    error DeputyPauseModule_Unauthorized();
    error DeputyPauseModule_NonceAlreadyUsed();
    error DeputyPauseModule_NotFromSafe();

    error ECDSAInvalidSignature();
    error ECDSAInvalidSignatureLength(uint256 length);
    error ECDSAInvalidSignatureS(bytes32 s);
    error InvalidShortString();
    error StringTooLong(string str);

    struct PauseMessage {
        bytes32 nonce;
    }

    struct DeputyAuthMessage {
        address deputy;
    }

    event DeputySet(address indexed deputy);
    event DeputyGuardianModuleSet(IDeputyGuardianModule indexed deputyGuardianModule);
    event PauseTriggered(address indexed deputy, bytes32 nonce);
    event EIP712DomainChanged();

    function version() external view returns (string memory);
    function __constructor__(
        Safe _foundationSafe,
        IDeputyGuardianModule _deputyGuardianModule,
        ISuperchainConfig _superchainConfig,
        address _deputy,
        bytes memory _deputySignature
    )
        external;
    function foundationSafe() external view returns (Safe foundationSafe_);
    function deputyGuardianModule() external view returns (IDeputyGuardianModule);
    function superchainConfig() external view returns (ISuperchainConfig superchainConfig_);
    function deputy() external view returns (address);
    function pauseMessageTypehash() external pure returns (bytes32 pauseMessageTypehash_);
    function deputyAuthMessageTypehash() external pure returns (bytes32 deputyAuthMessageTypehash_);
    function usedNonces(bytes32) external view returns (bool);
    function pause(bytes32 _nonce, bytes memory _signature) external;
    function setDeputy(address _deputy, bytes memory _deputySignature) external;
    function setDeputyGuardianModule(IDeputyGuardianModule _deputyGuardianModule) external;
    function eip712Domain()
        external
        view
        returns (
            bytes1 fields,
            string memory name,
            string memory version,
            uint256 chainId,
            address verifyingContract,
            bytes32 salt,
            uint256[] memory extensions
        );
}
