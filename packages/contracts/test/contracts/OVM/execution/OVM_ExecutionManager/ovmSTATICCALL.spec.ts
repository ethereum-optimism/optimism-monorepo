/* External Imports */
import { ethers } from 'ethers'

/* Internal Imports */
import {
  ExecutionManagerTestRunner,
  TestDefinition,
  OVM_TX_GAS_LIMIT,
  NON_NULL_BYTES32,
  REVERT_FLAGS,
  DUMMY_BYTECODE,
  getStorageXOR,
} from '../../../../helpers'

const test_ovmSTATICCALL: TestDefinition = {
  name: 'Basic tests for ovmSTATICCALL',
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
          codeHash: NON_NULL_BYTES32,
          ethAddress: '$OVM_CALL_HELPER',
        },
      },
      contractStorage: {
        $DUMMY_OVM_ADDRESS_1: {
          [NON_NULL_BYTES32]: getStorageXOR(ethers.constants.HashZero),
        },
        $DUMMY_OVM_ADDRESS_3: {
          [NON_NULL_BYTES32]: getStorageXOR(ethers.constants.HashZero),
        },
      },
      verifiedContractStorage: {
        $DUMMY_OVM_ADDRESS_1: {
          [NON_NULL_BYTES32]: true,
        },
        $DUMMY_OVM_ADDRESS_3: {
          [NON_NULL_BYTES32]: true,
        },
      },
    },
  },
  parameters: [
    {
      name: 'ovmSTATICCALL => ovmSSTORE',
      steps: [
        {
          functionName: 'ovmSTATICCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT / 2,
            target: '$DUMMY_OVM_ADDRESS_1',
            subSteps: [
              {
                functionName: 'ovmSSTORE',
                functionParams: {
                  key: ethers.constants.HashZero,
                  value: ethers.constants.HashZero,
                },
                expectedReturnStatus: false,
                expectedReturnValue: {
                  flag: REVERT_FLAGS.STATIC_VIOLATION,
                  nuisanceGasLeft: OVM_TX_GAS_LIMIT / 2,
                },
              },
            ],
          },
          expectedReturnStatus: false,
        },
      ],
    },
    {
      name: 'ovmSTATICCALL => ovmSLOAD',
      steps: [
        {
          functionName: 'ovmSTATICCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT / 2,
            target: '$DUMMY_OVM_ADDRESS_1',
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
        },
      ],
    },
    {
      name: 'ovmSTATICCALL => ovmCREATE',
      steps: [
        {
          functionName: 'ovmSTATICCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT / 2,
            target: '$DUMMY_OVM_ADDRESS_1',
            subSteps: [
              {
                functionName: 'ovmCREATE',
                functionParams: {
                  bytecode: DUMMY_BYTECODE,
                },
                expectedReturnStatus: false,
                expectedReturnValue: {
                  flag: REVERT_FLAGS.STATIC_VIOLATION,
                  nuisanceGasLeft: OVM_TX_GAS_LIMIT / 2,
                },
              },
            ],
          },
          expectedReturnStatus: false,
        },
      ],
    },
    {
      name: 'ovmCALL(ADDRESS_1) => ovmSTATICCALL(ADDRESS_2) => ovmCALLER',
      steps: [
        {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT / 2,
            target: '$DUMMY_OVM_ADDRESS_1',
            subSteps: [
              {
                functionName: 'ovmSTATICCALL',
                functionParams: {
                  gasLimit: OVM_TX_GAS_LIMIT / 2,
                  target: '$DUMMY_OVM_ADDRESS_2',
                  subSteps: [
                    {
                      functionName: 'ovmCALLER',
                      expectedReturnValue: '$DUMMY_OVM_ADDRESS_1',
                    },
                  ],
                },
                expectedReturnStatus: true,
              },
            ],
          },
          expectedReturnStatus: true,
        },
      ],
    },
    {
      name: 'ovmCALL(ADDRESS_1) => ovmSTATICCALL(ADDRESS_2) => ovmADDRESS',
      steps: [
        {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT / 2,
            target: '$DUMMY_OVM_ADDRESS_1',
            subSteps: [
              {
                functionName: 'ovmSTATICCALL',
                functionParams: {
                  gasLimit: OVM_TX_GAS_LIMIT / 2,
                  target: '$DUMMY_OVM_ADDRESS_2',
                  subSteps: [
                    {
                      functionName: 'ovmADDRESS',
                      expectedReturnValue: '$DUMMY_OVM_ADDRESS_2',
                    },
                  ],
                },
                expectedReturnStatus: true,
              },
            ],
          },
          expectedReturnStatus: true,
        },
      ],
    },
    {
      name:
        'ovmCALL(ADDRESS_1) => ovmSTATICCALL(ADDRESS_2) => ovmCALL(ADDRESS_3) => ovmSSTORE',
      steps: [
        {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT,
            target: '$DUMMY_OVM_ADDRESS_1',
            subSteps: [
              {
                functionName: 'ovmSTATICCALL',
                functionParams: {
                  gasLimit: OVM_TX_GAS_LIMIT,
                  target: '$DUMMY_OVM_ADDRESS_2',
                  subSteps: [
                    {
                      functionName: 'ovmCALL',
                      functionParams: {
                        gasLimit: OVM_TX_GAS_LIMIT / 2,
                        target: '$DUMMY_OVM_ADDRESS_3',
                        subSteps: [
                          {
                            functionName: 'ovmSSTORE',
                            functionParams: {
                              key: ethers.constants.HashZero,
                              value: ethers.constants.HashZero,
                            },
                            expectedReturnStatus: false,
                            expectedReturnValue: {
                              flag: REVERT_FLAGS.STATIC_VIOLATION,
                              nuisanceGasLeft: OVM_TX_GAS_LIMIT / 2,
                            },
                          },
                        ],
                      },
                      expectedReturnStatus: false,
                    },
                  ],
                },
                expectedReturnStatus: true,
              },
            ],
          },
          expectedReturnStatus: true,
        },
      ],
    },
    {
      name:
        'ovmCALL(ADDRESS_1) => ovmSTATICCALL(ADDRESS_2) => ovmCALL(ADDRESS_3) => ovmSLOAD',
      steps: [
        {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT / 2,
            target: '$DUMMY_OVM_ADDRESS_1',
            subSteps: [
              {
                functionName: 'ovmSTATICCALL',
                functionParams: {
                  gasLimit: OVM_TX_GAS_LIMIT / 2,
                  target: '$DUMMY_OVM_ADDRESS_2',
                  subSteps: [
                    {
                      functionName: 'ovmCALL',
                      functionParams: {
                        gasLimit: OVM_TX_GAS_LIMIT / 2,
                        target: '$DUMMY_OVM_ADDRESS_3',
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
                    },
                  ],
                },
                expectedReturnStatus: true,
              },
            ],
          },
          expectedReturnStatus: true,
        },
      ],
    },
    {
      name:
        'ovmCALL(ADDRESS_1) => ovmSTATICCALL(ADDRESS_2) => ovmCALL(ADDRESS_3) => ovmCREATE',
      steps: [
        {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT,
            target: '$DUMMY_OVM_ADDRESS_1',
            subSteps: [
              {
                functionName: 'ovmSTATICCALL',
                functionParams: {
                  gasLimit: OVM_TX_GAS_LIMIT,
                  target: '$DUMMY_OVM_ADDRESS_2',
                  subSteps: [
                    {
                      functionName: 'ovmCALL',
                      functionParams: {
                        gasLimit: OVM_TX_GAS_LIMIT / 2,
                        target: '$DUMMY_OVM_ADDRESS_3',
                        subSteps: [
                          {
                            functionName: 'ovmCREATE',
                            functionParams: {
                              bytecode: DUMMY_BYTECODE,
                            },
                            expectedReturnStatus: false,
                            expectedReturnValue: {
                              flag: REVERT_FLAGS.STATIC_VIOLATION,
                              nuisanceGasLeft: OVM_TX_GAS_LIMIT / 2,
                            },
                          },
                        ],
                      },
                      expectedReturnStatus: false,
                    },
                  ],
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
}

const runner = new ExecutionManagerTestRunner()
runner.run(test_ovmSTATICCALL)
