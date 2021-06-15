/* Internal Imports */
import { constants } from 'ethers'
import {
  ExecutionManagerTestRunner,
  TestDefinition,
  OVM_TX_GAS_LIMIT,
  NON_NULL_BYTES32,
  VERIFIED_EMPTY_CONTRACT_HASH,
  NUISANCE_GAS_COSTS,
  Helper_TestRunner_BYTELEN,
} from '../../../../helpers'

const CREATED_CONTRACT_1 = '0x2bda4a99d5be88609d23b1e4ab5d1d34fb1c2feb'

const FRESH_CALL_NUISANCE_GAS_COST =
  Helper_TestRunner_BYTELEN *
    NUISANCE_GAS_COSTS.NUISANCE_GAS_PER_CONTRACT_BYTE +
  NUISANCE_GAS_COSTS.MIN_NUISANCE_GAS_PER_CONTRACT

const test_nuisanceGas: TestDefinition = {
  name: 'Basic tests for nuisance gas',
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
        $DUMMY_OVM_ADDRESS_3: {
          codeHash: VERIFIED_EMPTY_CONTRACT_HASH,
          ethAddress: '0x' + '00'.repeat(20),
        },
        [CREATED_CONTRACT_1]: {
          codeHash: VERIFIED_EMPTY_CONTRACT_HASH,
          ethAddress: '0x' + '00'.repeat(20),
        },
      },
    },
  },
  subTests: [
    {
      name:
        'ovmCALL consumes nuisance gas of CODESIZE * NUISANCE_GAS_PER_CONTRACT_BYTE',
      postState: {
        ExecutionManager: {
          messageRecord: {
            nuisanceGasLeft: OVM_TX_GAS_LIMIT - FRESH_CALL_NUISANCE_GAS_COST,
          },
        },
      },
      parameters: [
        {
          name: 'single ovmCALL',
          steps: [
            // do a non-nuisance-gas-consuming opcode (test runner auto-wraps in ovmCALL)
            {
              functionName: 'ovmADDRESS',
              expectedReturnValue: '$DUMMY_OVM_ADDRESS_1',
            },
          ],
        },
        {
          name: 'nested ovmCALL, same address',
          steps: [
            {
              functionName: 'ovmCALL',
              functionParams: {
                gasLimit: OVM_TX_GAS_LIMIT,
                target: '$DUMMY_OVM_ADDRESS_1',
                subSteps: [],
              },
              expectedReturnStatus: true,
            },
          ],
        },
      ],
    },
    {
      name:
        'ovmCALL consumes nuisance gas of CODESIZE * NUISANCE_GAS_PER_CONTRACT_BYTE twice for two unique ovmCALLS',
      postState: {
        ExecutionManager: {
          messageRecord: {
            nuisanceGasLeft:
              OVM_TX_GAS_LIMIT - 2 * FRESH_CALL_NUISANCE_GAS_COST,
          },
        },
      },
      parameters: [
        {
          name: 'directly nested ovmCALL',
          steps: [
            {
              functionName: 'ovmCALL',
              functionParams: {
                gasLimit: OVM_TX_GAS_LIMIT,
                target: '$DUMMY_OVM_ADDRESS_2',
                subSteps: [],
              },
              expectedReturnStatus: true,
            },
          ],
        },
        {
          name: 'with a call to previously called contract too',
          steps: [
            {
              functionName: 'ovmCALL',
              functionParams: {
                gasLimit: OVM_TX_GAS_LIMIT,
                target: '$DUMMY_OVM_ADDRESS_2',
                subSteps: [
                  {
                    functionName: 'ovmCALL',
                    functionParams: {
                      gasLimit: OVM_TX_GAS_LIMIT,
                      target: '$DUMMY_OVM_ADDRESS_1',
                      subSteps: [],
                    },
                    expectedReturnStatus: true,
                  },
                ],
              },
              expectedReturnStatus: true,
            },
          ],
        },
      ],
    },
    {
      name:
        'ovmCALL consumes all allotted nuisance gas if code contract throws unknown exception',
      postState: {
        ExecutionManager: {
          messageRecord: {
            nuisanceGasLeft:
              OVM_TX_GAS_LIMIT -
              FRESH_CALL_NUISANCE_GAS_COST -
              OVM_TX_GAS_LIMIT / 2,
          },
        },
      },
      parameters: [
        {
          name: 'give 1/2 gas to evmINVALID',
          steps: [
            {
              functionName: 'ovmCALL',
              functionParams: {
                gasLimit: OVM_TX_GAS_LIMIT / 2,
                target: '$DUMMY_OVM_ADDRESS_1',
                subSteps: [
                  {
                    functionName: 'evmINVALID',
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
      name:
        'ovmCREATE consumes all allotted nuisance gas if creation code throws data-less exception',
      parameters: [
        {
          name: 'give 1/2 gas to ovmCALL => ovmCREATE, evmINVALID',
          steps: [
            {
              functionName: 'ovmCALL',
              functionParams: {
                target: '$DUMMY_OVM_ADDRESS_1',
                gasLimit: OVM_TX_GAS_LIMIT / 2,
                subSteps: [
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
              expectedReturnStatus: true,
            },
          ],
        },
      ],
    },
  ],
}

const runner = new ExecutionManagerTestRunner()
runner.run(test_nuisanceGas)
