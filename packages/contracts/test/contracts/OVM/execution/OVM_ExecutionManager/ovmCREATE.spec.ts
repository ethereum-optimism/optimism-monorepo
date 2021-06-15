/* Internal Imports */
import { constants, ethers } from 'ethers'
import {
  ExecutionManagerTestRunner,
  TestDefinition,
  OVM_TX_GAS_LIMIT,
  NON_NULL_BYTES32,
  REVERT_FLAGS,
  DUMMY_BYTECODE,
  UNSAFE_BYTECODE,
  VERIFIED_EMPTY_CONTRACT_HASH,
  DUMMY_BYTECODE_BYTELEN,
  DUMMY_BYTECODE_HASH,
  getStorageXOR,
  encodeSolidityError,
} from '../../../../helpers'
import { predeploys } from '../../../../../src'

const CREATED_CONTRACT_1 = '0x2bda4a99d5be88609d23b1e4ab5d1d34fb1c2feb'
const CREATED_CONTRACT_2 = '0x2bda4a99d5be88609d23b1e4ab5d1d34fb1c2feb'
const CREATED_CONTRACT_BY_2_1 = '0xe0d8be8101f36ebe6b01abacec884422c39a1f62'
const CREATED_CONTRACT_BY_2_2 = '0x15ac629e1a3866b17179ee4ae86de5cbda744335'
const NESTED_CREATED_CONTRACT = '0xcb964b3f4162a0d4f5c997b40e19da5a546bc36f'
const DUMMY_REVERT_DATA =
  '0xdeadbeef1e5420deadbeef1e5420deadbeef1e5420deadbeef1e5420deadbeef1e5420'

const NON_WHITELISTED_DEPLOYER = '0x1234123412341234123412341234123412341234'
const NON_WHITELISTED_DEPLOYER_KEY = ethers.utils.keccak256(
  '0x' +
    '0000000000000000000000001234123412341234123412341234123412341234' +
    '0000000000000000000000000000000000000000000000000000000000000001'
)
const CREATED_BY_NON_WHITELISTED_DEPLOYER =
  '0x794e4aa3be128b0fc01ba12543b70bf9d77072fc'

const WHITELISTED_DEPLOYER = '0x3456345634563456345634563456345634563456'
const WHITELISTED_DEPLOYER_KEY = ethers.utils.keccak256(
  '0x' +
    '0000000000000000000000003456345634563456345634563456345634563456' +
    '0000000000000000000000000000000000000000000000000000000000000001'
)

const CREATED_BY_WHITELISTED_DEPLOYER =
  '0x9f397a91ccb7cc924d1585f1053bc697d30f343f'

const test_ovmCREATE: TestDefinition = {
  name: 'Basic tests for ovmCREATE',
  preState: {
    ExecutionManager: {
      ovmStateManager: '$OVM_STATE_MANAGER',
      messageRecord: {
        nuisanceGasLeft: OVM_TX_GAS_LIMIT,
      },
    },
    StateManager: {
      owner: '$OVM_EXECUTION_MANAGER',
      accounts: {
        $DUMMY_OVM_ADDRESS_1: {
          codeHash: NON_NULL_BYTES32,
          ethAddress: '$OVM_CALL_HELPER',
        },
        $DUMMY_OVM_ADDRESS_2: {
          codeHash: NON_NULL_BYTES32,
          ethAddress: '$OVM_CALL_HELPER',
        },
        [CREATED_CONTRACT_1]: {
          codeHash: VERIFIED_EMPTY_CONTRACT_HASH,
          ethAddress: '0x' + '00'.repeat(20),
        },
        [CREATED_CONTRACT_2]: {
          codeHash: VERIFIED_EMPTY_CONTRACT_HASH,
          ethAddress: '0x' + '00'.repeat(20),
        },
        [CREATED_CONTRACT_BY_2_1]: {
          codeHash: VERIFIED_EMPTY_CONTRACT_HASH,
          ethAddress: '0x' + '00'.repeat(20),
        },
        [CREATED_CONTRACT_BY_2_2]: {
          codeHash: VERIFIED_EMPTY_CONTRACT_HASH,
          ethAddress: '0x' + '00'.repeat(20),
        },
        [NESTED_CREATED_CONTRACT]: {
          codeHash: VERIFIED_EMPTY_CONTRACT_HASH,
          ethAddress: '0x' + '00'.repeat(20),
        },
      },
      contractStorage: {
        $DUMMY_OVM_ADDRESS_2: {
          [ethers.constants.HashZero]: getStorageXOR(ethers.constants.HashZero),
          [NON_NULL_BYTES32]: getStorageXOR(ethers.constants.HashZero),
        },
      },
      verifiedContractStorage: {
        $DUMMY_OVM_ADDRESS_1: {
          [NON_NULL_BYTES32]: true,
        },
        $DUMMY_OVM_ADDRESS_2: {
          [ethers.constants.HashZero]: true,
          [NON_NULL_BYTES32]: true,
        },
      },
    },
  },
  parameters: [
    {
      name: 'ovmCREATE, ovmEXTCODESIZE(CREATED)',
      steps: [
        {
          functionName: 'ovmCREATE',
          functionParams: {
            bytecode: DUMMY_BYTECODE,
          },
          expectedReturnStatus: true,
          expectedReturnValue: CREATED_CONTRACT_1,
        },
        {
          functionName: 'ovmEXTCODESIZE',
          functionParams: {
            address: CREATED_CONTRACT_1,
          },
          expectedReturnStatus: true,
          expectedReturnValue: DUMMY_BYTECODE_BYTELEN,
        },
      ],
    },
    {
      name: 'ovmCREATE, ovmEXTCODEHASH(CREATED)',
      steps: [
        {
          functionName: 'ovmCREATE',
          functionParams: {
            bytecode: DUMMY_BYTECODE,
          },
          expectedReturnStatus: true,
          expectedReturnValue: CREATED_CONTRACT_1,
        },
        {
          functionName: 'ovmEXTCODEHASH',
          functionParams: {
            address: CREATED_CONTRACT_1,
          },
          expectedReturnStatus: true,
          expectedReturnValue: DUMMY_BYTECODE_HASH,
        },
      ],
    },
    {
      name: 'ovmCREATE, ovmEXTCODECOPY(CREATED)',
      steps: [
        {
          functionName: 'ovmCREATE',
          functionParams: {
            bytecode: DUMMY_BYTECODE,
          },
          expectedReturnStatus: true,
          expectedReturnValue: CREATED_CONTRACT_1,
        },
        {
          functionName: 'ovmEXTCODECOPY',
          functionParams: {
            address: CREATED_CONTRACT_1,
            offset: 0,
            length: DUMMY_BYTECODE_BYTELEN,
          },
          expectedReturnStatus: true,
          expectedReturnValue: DUMMY_BYTECODE,
        },
      ],
    },
    {
      name: 'ovmCREATE => ovmREVERT',
      steps: [
        {
          functionName: 'ovmCREATE',
          functionParams: {
            subSteps: [
              {
                functionName: 'ovmREVERT',
                revertData: DUMMY_REVERT_DATA,
                expectedReturnStatus: false,
                expectedReturnValue: {
                  flag: REVERT_FLAGS.INTENTIONAL_REVERT,
                  onlyValidateFlag: true,
                },
              },
            ],
          },
          expectedReturnStatus: true,
          expectedReturnValue: {
            address: constants.AddressZero,
            revertData: DUMMY_REVERT_DATA,
          },
        },
      ],
    },
    {
      name: 'ovmCREATE => ovmREVERT, ovmEXTCODESIZE(CREATED)',
      steps: [
        {
          functionName: 'ovmCREATE',
          functionParams: {
            subSteps: [
              {
                functionName: 'ovmREVERT',
                revertData: DUMMY_REVERT_DATA,
                expectedReturnStatus: false,
                expectedReturnValue: {
                  flag: REVERT_FLAGS.INTENTIONAL_REVERT,
                  onlyValidateFlag: true,
                },
              },
            ],
          },
          expectedReturnStatus: true,
          expectedReturnValue: {
            address: constants.AddressZero,
            revertData: DUMMY_REVERT_DATA,
          },
        },
        {
          functionName: 'ovmEXTCODESIZE',
          functionParams: {
            address: CREATED_CONTRACT_1,
          },
          expectedReturnStatus: true,
          expectedReturnValue: 0,
        },
      ],
    },
    {
      name: 'ovmCREATE => ovmREVERT, ovmEXTCODEHASH(CREATED)',
      steps: [
        {
          functionName: 'ovmCREATE',
          functionParams: {
            subSteps: [
              {
                functionName: 'ovmREVERT',
                revertData: DUMMY_REVERT_DATA,
                expectedReturnStatus: false,
                expectedReturnValue: {
                  flag: REVERT_FLAGS.INTENTIONAL_REVERT,
                  onlyValidateFlag: true,
                },
              },
            ],
          },
          expectedReturnStatus: true,
          expectedReturnValue: {
            address: constants.AddressZero,
            revertData: DUMMY_REVERT_DATA,
          },
        },
        {
          functionName: 'ovmEXTCODEHASH',
          functionParams: {
            address: CREATED_CONTRACT_1,
          },
          expectedReturnStatus: true,
          expectedReturnValue: ethers.constants.HashZero,
        },
      ],
    },
    {
      name: 'ovmCREATE => ovmREVERT, ovmEXTCODECOPY(CREATED)',
      steps: [
        {
          functionName: 'ovmCREATE',
          functionParams: {
            subSteps: [
              {
                functionName: 'ovmREVERT',
                revertData: DUMMY_REVERT_DATA,
                expectedReturnStatus: false,
                expectedReturnValue: {
                  flag: REVERT_FLAGS.INTENTIONAL_REVERT,
                  onlyValidateFlag: true,
                },
              },
            ],
          },
          expectedReturnStatus: true,
          expectedReturnValue: {
            address: constants.AddressZero,
            revertData: DUMMY_REVERT_DATA,
          },
        },
        {
          functionName: 'ovmEXTCODECOPY',
          functionParams: {
            address: CREATED_CONTRACT_1,
            offset: 0,
            length: 256,
          },
          expectedReturnStatus: true,
          expectedReturnValue: '0x' + '00'.repeat(256),
        },
      ],
    },
    {
      name: 'ovmCREATE => ovmADDRESS',
      steps: [
        {
          functionName: 'ovmCREATE',
          functionParams: {
            subSteps: [
              {
                functionName: 'ovmADDRESS',
                expectedReturnValue: CREATED_CONTRACT_1,
              },
            ],
          },
          expectedReturnStatus: true,
          expectedReturnValue: CREATED_CONTRACT_1,
        },
      ],
    },
    {
      name: 'ovmCREATE => ovmSLOAD',
      steps: [
        {
          functionName: 'ovmCREATE',
          functionParams: {
            subSteps: [
              {
                functionName: 'ovmSLOAD',
                functionParams: {
                  key: NON_NULL_BYTES32,
                },
                expectedReturnStatus: true,
                expectedReturnValue: ethers.constants.HashZero,
              },
            ],
          },
          expectedReturnStatus: true,
          expectedReturnValue: CREATED_CONTRACT_1,
        },
      ],
    },
    {
      name: 'ovmCALL => ovmCREATE => ovmCALLER',
      steps: [
        {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT,
            target: '$DUMMY_OVM_ADDRESS_1',
            subSteps: [
              {
                functionName: 'ovmCREATE',
                functionParams: {
                  subSteps: [
                    {
                      functionName: 'ovmSLOAD',
                      functionParams: {
                        key: NON_NULL_BYTES32,
                      },
                      expectedReturnStatus: true,
                      expectedReturnValue: ethers.constants.HashZero,
                    },
                  ],
                },
                expectedReturnStatus: true,
                expectedReturnValue: CREATED_CONTRACT_2,
              },
            ],
          },
          expectedReturnStatus: true,
        },
      ],
    },
    {
      name: 'ovmCREATE => ovmSSTORE + ovmSLOAD',
      steps: [
        {
          functionName: 'ovmCREATE',
          functionParams: {
            subSteps: [
              {
                functionName: 'ovmSSTORE',
                functionParams: {
                  key: NON_NULL_BYTES32,
                  value: NON_NULL_BYTES32,
                },
                expectedReturnStatus: true,
              },
              {
                functionName: 'ovmSLOAD',
                functionParams: {
                  key: NON_NULL_BYTES32,
                },
                expectedReturnStatus: true,
                expectedReturnValue: NON_NULL_BYTES32,
              },
            ],
          },
          expectedReturnStatus: true,
          expectedReturnValue: CREATED_CONTRACT_1,
        },
      ],
    },
    {
      name:
        'ovmCREATE => ovmSSTORE, ovmCALL(CREATED) => ovmSLOAD(EXIST) + ovmSLOAD(NONEXIST)',
      steps: [
        {
          functionName: 'ovmCREATE',
          functionParams: {
            subSteps: [
              {
                functionName: 'ovmSSTORE',
                functionParams: {
                  key: NON_NULL_BYTES32,
                  value: NON_NULL_BYTES32,
                },
                expectedReturnStatus: true,
              },
            ],
          },
          expectedReturnStatus: true,
          expectedReturnValue: CREATED_CONTRACT_1,
        },
        {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT,
            target: CREATED_CONTRACT_1,
            subSteps: [
              {
                functionName: 'ovmSLOAD',
                functionParams: {
                  key: NON_NULL_BYTES32,
                },
                expectedReturnStatus: true,
                expectedReturnValue: NON_NULL_BYTES32,
              },
              {
                functionName: 'ovmSLOAD',
                functionParams: {
                  key: ethers.constants.HashZero,
                },
                expectedReturnStatus: true,
                expectedReturnValue: ethers.constants.HashZero,
              },
            ],
          },
          expectedReturnStatus: true,
        },
      ],
    },
    {
      name:
        'ovmCREATE => ovmCALL(ADDRESS_1) => ovmSSTORE, ovmCALL(ADDRESS_1) => ovmSLOAD',
      steps: [
        {
          functionName: 'ovmCREATE',
          functionParams: {
            subSteps: [
              {
                functionName: 'ovmCALL',
                functionParams: {
                  gasLimit: OVM_TX_GAS_LIMIT,
                  target: '$DUMMY_OVM_ADDRESS_1',
                  subSteps: [
                    {
                      functionName: 'ovmSSTORE',
                      functionParams: {
                        key: NON_NULL_BYTES32,
                        value: NON_NULL_BYTES32,
                      },
                      expectedReturnStatus: true,
                    },
                  ],
                },
                expectedReturnStatus: true,
              },
            ],
          },
          expectedReturnStatus: true,
          expectedReturnValue: CREATED_CONTRACT_1,
        },
        {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT,
            target: '$DUMMY_OVM_ADDRESS_1',
            subSteps: [
              {
                functionName: 'ovmSLOAD',
                functionParams: {
                  key: NON_NULL_BYTES32,
                },
                expectedReturnStatus: true,
                expectedReturnValue: NON_NULL_BYTES32,
              },
            ],
          },
          expectedReturnStatus: true,
        },
      ],
    },
    {
      // TODO: appears to be failing due to a smoddit issue
      skip: true,
      name:
        'ovmCREATE => (ovmCALL(ADDRESS_2) => ovmSSTORE) + ovmREVERT, ovmCALL(ADDRESS_2) => ovmSLOAD',
      steps: [
        {
          functionName: 'ovmCREATE',
          functionParams: {
            subSteps: [
              {
                functionName: 'ovmCALL',
                functionParams: {
                  gasLimit: OVM_TX_GAS_LIMIT,
                  target: '$DUMMY_OVM_ADDRESS_2',
                  subSteps: [
                    {
                      functionName: 'ovmSLOAD',
                      functionParams: {
                        key: NON_NULL_BYTES32,
                      },
                      expectedReturnStatus: true,
                      expectedReturnValue: NON_NULL_BYTES32,
                    },
                    {
                      functionName: 'ovmSSTORE',
                      functionParams: {
                        key: NON_NULL_BYTES32,
                        value: NON_NULL_BYTES32,
                      },
                      expectedReturnStatus: true,
                    },
                  ],
                },
                expectedReturnStatus: true,
              },
              {
                functionName: 'ovmREVERT',
                revertData: DUMMY_REVERT_DATA,
                expectedReturnStatus: false,
                expectedReturnValue: {
                  flag: REVERT_FLAGS.INTENTIONAL_REVERT,
                  onlyValidateFlag: true,
                },
              },
            ],
          },
          expectedReturnStatus: true,
          expectedReturnValue: {
            address: constants.AddressZero,
            revertData: DUMMY_REVERT_DATA,
          },
        },
        {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT,
            target: '$DUMMY_OVM_ADDRESS_2',
            subSteps: [
              {
                functionName: 'ovmSLOAD',
                functionParams: {
                  key: NON_NULL_BYTES32,
                },
                expectedReturnStatus: true,
                expectedReturnValue: NON_NULL_BYTES32,
              },
            ],
          },
          expectedReturnStatus: true,
        },
      ],
    },
    {
      name: 'ovmCREATE => ovmCALL(ADDRESS_NONEXIST)',
      expectInvalidStateAccess: true,
      steps: [
        {
          functionName: 'ovmCREATE',
          functionParams: {
            subSteps: [
              {
                functionName: 'ovmCALL',
                functionParams: {
                  gasLimit: OVM_TX_GAS_LIMIT,
                  target: '$DUMMY_OVM_ADDRESS_3',
                  calldata: '0x',
                },
                expectedReturnStatus: false,
                expectedReturnValue: {
                  flag: REVERT_FLAGS.INVALID_STATE_ACCESS,
                  onlyValidateFlag: true,
                },
              },
            ],
          },
          expectedReturnStatus: false,
          expectedReturnValue: {
            flag: REVERT_FLAGS.INVALID_STATE_ACCESS,
          },
        },
      ],
    },
    {
      name: 'ovmCALL => ovmCREATE => ovmCREATE',
      steps: [
        {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT,
            target: '$DUMMY_OVM_ADDRESS_2',
            subSteps: [
              {
                functionName: 'ovmCREATE',
                functionParams: {
                  subSteps: [
                    {
                      functionName: 'ovmCREATE',
                      functionParams: {
                        bytecode: '0x', // this will still succeed with empty bytecode
                      },
                      expectedReturnStatus: true,
                      expectedReturnValue: CREATED_CONTRACT_BY_2_2,
                    },
                  ],
                },
                expectedReturnStatus: true,
                expectedReturnValue: CREATED_CONTRACT_BY_2_1,
              },
            ],
          },
          expectedReturnStatus: true,
        },
      ],
    },
    {
      name: 'ovmCREATE => ovmCREATE => ovmCALL(ADDRESS_NONEXIST)',
      expectInvalidStateAccess: true,
      steps: [
        {
          functionName: 'ovmCREATE',
          functionParams: {
            subSteps: [
              {
                functionName: 'ovmCREATE',
                functionParams: {
                  subSteps: [
                    {
                      functionName: 'ovmCALL',
                      functionParams: {
                        gasLimit: OVM_TX_GAS_LIMIT,
                        target: '$DUMMY_OVM_ADDRESS_3',
                        calldata: '0x',
                      },
                      expectedReturnStatus: false,
                      expectedReturnValue: {
                        flag: REVERT_FLAGS.INVALID_STATE_ACCESS,
                        onlyValidateFlag: true,
                      },
                    },
                  ],
                },
                expectedReturnStatus: false,
                expectedReturnValue: {
                  flag: REVERT_FLAGS.INVALID_STATE_ACCESS,
                  onlyValidateFlag: true,
                },
              },
            ],
          },
          expectedReturnStatus: false,
          expectedReturnValue: {
            flag: REVERT_FLAGS.INVALID_STATE_ACCESS,
            onlyValidateFlag: true,
          },
        },
      ],
    },
    {
      name: 'OZ-AUDIT: ovmCREATE => ((ovmCREATE => ovmADDRESS), ovmREVERT)',
      steps: [
        {
          functionName: 'ovmCREATE',
          functionParams: {
            subSteps: [
              {
                functionName: 'ovmCREATE',
                functionParams: {
                  subSteps: [
                    {
                      functionName: 'ovmADDRESS',
                      expectedReturnValue: NESTED_CREATED_CONTRACT,
                    },
                  ],
                },
                expectedReturnStatus: true,
                expectedReturnValue: NESTED_CREATED_CONTRACT,
              },
              {
                functionName: 'ovmREVERT',
                revertData: DUMMY_REVERT_DATA,
                expectedReturnStatus: false,
                expectedReturnValue: {
                  flag: REVERT_FLAGS.INTENTIONAL_REVERT,
                  onlyValidateFlag: true,
                },
              },
            ],
          },
          expectedReturnStatus: true,
          expectedReturnValue: {
            address: constants.AddressZero,
            revertData: DUMMY_REVERT_DATA,
          },
        },
      ],
    },
    {
      name: 'ovmCREATE => OUT_OF_GAS',
      steps: [
        {
          functionName: 'ovmCREATE',
          functionParams: {
            subSteps: [
              {
                functionName: 'evmINVALID',
              },
            ],
          },
          expectedReturnStatus: true,
          expectedReturnValue: constants.AddressZero,
        },
      ],
    },
    {
      name: 'ovmCREATE(UNSAFE_CODE)',
      steps: [
        {
          functionName: 'ovmCREATE',
          functionParams: {
            bytecode: UNSAFE_BYTECODE,
          },
          expectedReturnStatus: true,
          expectedReturnValue: {
            address: constants.AddressZero,
            revertData: encodeSolidityError(
              'Constructor attempted to deploy unsafe bytecode.'
            ),
          },
        },
      ],
    },
  ],
  subTests: [
    {
      name: 'Deployer whitelist tests',
      preState: {
        StateManager: {
          accounts: {
            [NON_WHITELISTED_DEPLOYER]: {
              codeHash: NON_NULL_BYTES32,
              ethAddress: '$OVM_CALL_HELPER',
            },
            [WHITELISTED_DEPLOYER]: {
              codeHash: NON_NULL_BYTES32,
              ethAddress: '$OVM_CALL_HELPER',
            },
            [CREATED_BY_WHITELISTED_DEPLOYER]: {
              codeHash: VERIFIED_EMPTY_CONTRACT_HASH,
              ethAddress: '0x' + '00'.repeat(20),
            },
          },
          contractStorage: {
            [predeploys.OVM_DeployerWhitelist]: {
              // initialized? true, allowArbitraryDeployment? false
              '0x0000000000000000000000000000000000000000000000000000000000000000': getStorageXOR(
                '0x0000000000000000000000000000000000000000000000000000000000000001'
              ),
              // non-whitelisted deployer is whitelisted? false
              [NON_WHITELISTED_DEPLOYER_KEY]: getStorageXOR(
                ethers.constants.HashZero
              ),
              // whitelisted deployer is whitelisted? true
              [WHITELISTED_DEPLOYER_KEY]: getStorageXOR(
                '0x' + '00'.repeat(31) + '01'
              ),
            },
          },
          verifiedContractStorage: {
            [predeploys.OVM_DeployerWhitelist]: {
              '0x0000000000000000000000000000000000000000000000000000000000000000': 1,
              [NON_WHITELISTED_DEPLOYER_KEY]: 1,
              [WHITELISTED_DEPLOYER_KEY]: 1,
            },
          },
        },
      },
      parameters: [
        {
          name: 'ovmCREATE by WHITELISTED_DEPLOYER',
          steps: [
            {
              functionName: 'ovmCALL',
              functionParams: {
                gasLimit: OVM_TX_GAS_LIMIT / 2,
                target: WHITELISTED_DEPLOYER,
                subSteps: [
                  {
                    functionName: 'ovmCREATE',
                    functionParams: {
                      bytecode: DUMMY_BYTECODE,
                    },
                    expectedReturnStatus: true,
                    expectedReturnValue: CREATED_BY_WHITELISTED_DEPLOYER,
                  },
                ],
              },
              expectedReturnStatus: true,
            },
          ],
        },
        {
          name: 'ovmCREATE by NON_WHITELISTED_DEPLOYER',
          steps: [
            {
              functionName: 'ovmCALL',
              functionParams: {
                gasLimit: OVM_TX_GAS_LIMIT / 2,
                target: NON_WHITELISTED_DEPLOYER,
                subSteps: [
                  {
                    functionName: 'ovmCREATE',
                    functionParams: {
                      subSteps: [],
                    },
                    expectedReturnStatus: false,
                    expectedReturnValue: {
                      flag: REVERT_FLAGS.CREATOR_NOT_ALLOWED,
                      onlyValidateFlag: true,
                    },
                  },
                ],
              },
              expectedReturnStatus: true,
              expectedReturnValue: {
                ovmSuccess: false,
                returnData: '0x',
              },
            },
          ],
        },
        {
          name: 'ovmCREATE2 by NON_WHITELISTED_DEPLOYER',
          steps: [
            {
              functionName: 'ovmCALL',
              functionParams: {
                gasLimit: OVM_TX_GAS_LIMIT / 2,
                target: NON_WHITELISTED_DEPLOYER,
                subSteps: [
                  {
                    functionName: 'ovmCREATE2',
                    functionParams: {
                      salt: ethers.constants.HashZero,
                      bytecode: '0x',
                    },
                    expectedReturnStatus: false,
                    expectedReturnValue: {
                      flag: REVERT_FLAGS.CREATOR_NOT_ALLOWED,
                      onlyValidateFlag: true,
                    },
                  },
                ],
              },
              expectedReturnStatus: true,
              expectedReturnValue: {
                ovmSuccess: false,
                returnData: '0x',
              },
            },
          ],
        },
      ],
    },
    {
      name: 'Deployer whitelist tests',
      preState: {
        StateManager: {
          accounts: {
            [NON_WHITELISTED_DEPLOYER]: {
              codeHash: NON_NULL_BYTES32,
              ethAddress: '$OVM_CALL_HELPER',
            },
            [WHITELISTED_DEPLOYER]: {
              codeHash: NON_NULL_BYTES32,
              ethAddress: '$OVM_CALL_HELPER',
            },
            [CREATED_BY_NON_WHITELISTED_DEPLOYER]: {
              codeHash: VERIFIED_EMPTY_CONTRACT_HASH,
              ethAddress: '0x' + '00'.repeat(20),
            },
          },
          contractStorage: {
            [predeploys.OVM_DeployerWhitelist]: {
              // initialized? true, allowArbitraryDeployment? true
              '0x0000000000000000000000000000000000000000000000000000000000000000': getStorageXOR(
                '0x0000000000000000000000000000000000000000000000000000000000000101'
              ),
              // non-whitelisted deployer is whitelisted? false
              [NON_WHITELISTED_DEPLOYER_KEY]: getStorageXOR(
                ethers.constants.HashZero
              ),
              // whitelisted deployer is whitelisted? true
              [WHITELISTED_DEPLOYER_KEY]: getStorageXOR(
                '0x' + '00'.repeat(31) + '01'
              ),
            },
          },
          verifiedContractStorage: {
            [predeploys.OVM_DeployerWhitelist]: {
              '0x0000000000000000000000000000000000000000000000000000000000000000': 1,
              [NON_WHITELISTED_DEPLOYER_KEY]: 1,
              [WHITELISTED_DEPLOYER_KEY]: 1,
            },
          },
        },
      },
      subTests: [
        {
          name: 'when arbitrary contract deployment is enabled',
          parameters: [
            {
              name: 'ovmCREATE by NON_WHITELISTED_DEPLOYER',
              steps: [
                {
                  functionName: 'ovmCALL',
                  functionParams: {
                    gasLimit: OVM_TX_GAS_LIMIT / 2,
                    target: NON_WHITELISTED_DEPLOYER,
                    subSteps: [
                      {
                        functionName: 'ovmCREATE',
                        functionParams: {
                          subSteps: [],
                        },
                        expectedReturnStatus: true,
                        expectedReturnValue: CREATED_BY_NON_WHITELISTED_DEPLOYER,
                      },
                    ],
                  },
                  expectedReturnStatus: true,
                },
              ],
            },
          ],
        },
      ],
    },
  ],
}

const runner = new ExecutionManagerTestRunner()
runner.run(test_ovmCREATE)
