/* Internal Imports */
import {
  ExecutionManagerTestRunner,
  TestDefinition,
  OVM_TX_GAS_LIMIT,
  NON_NULL_BYTES32,
  DUMMY_BYTECODE,
  VERIFIED_EMPTY_CONTRACT_HASH,
} from '../../../../helpers'

const CREATED_CONTRACT_1 = '0x2bda4a99d5be88609d23b1e4ab5d1d34fb1c2feb'
const CREATED_CONTRACT_2 = '0xe0d8be8101f36ebe6b01abacec884422c39a1f62'

const test_ovmDELEGATECALL: TestDefinition = {
  name: 'Basic tests for ovmDELEGATECALL',
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
        $DUMMY_OVM_ADDRESS_4: {
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
      },
      verifiedContractStorage: {
        $DUMMY_OVM_ADDRESS_1: {
          [NON_NULL_BYTES32]: true,
        },
      },
    },
  },
  parameters: [
    {
      name: 'ovmCALL(ADDRESS_1) => ovmDELEGATECALL(ADDRESS_2) => ovmADDRESS',
      steps: [
        {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT,
            target: '$DUMMY_OVM_ADDRESS_1',
            subSteps: [
              {
                functionName: 'ovmDELEGATECALL',
                functionParams: {
                  gasLimit: OVM_TX_GAS_LIMIT,
                  target: '$DUMMY_OVM_ADDRESS_2',
                  subSteps: [
                    {
                      functionName: 'ovmADDRESS',
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
      name:
        'ovmCALL(ADDRESS_1) => ovmCALL(ADDRESS_2) => ovmDELEGATECALL(ADDRESS_3) => ovmCALLER',
      steps: [
        {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT,
            target: '$DUMMY_OVM_ADDRESS_1',
            subSteps: [
              {
                functionName: 'ovmCALL',
                functionParams: {
                  gasLimit: OVM_TX_GAS_LIMIT,
                  target: '$DUMMY_OVM_ADDRESS_2',
                  subSteps: [
                    {
                      functionName: 'ovmDELEGATECALL',
                      functionParams: {
                        gasLimit: OVM_TX_GAS_LIMIT,
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
          expectedReturnStatus: true,
        },
      ],
    },
    {
      name:
        'ovmCALL(ADDRESS_1) => (ovmDELEGATECALL(ADDRESS_2) => ovmSSTORE) + ovmSLOAD',
      steps: [
        {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT,
            target: '$DUMMY_OVM_ADDRESS_1',
            subSteps: [
              {
                functionName: 'ovmDELEGATECALL',
                functionParams: {
                  gasLimit: OVM_TX_GAS_LIMIT,
                  target: '$DUMMY_OVM_ADDRESS_2',
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
      name: 'ovmCALL(ADDRESS_1) => (ovmDELEGATECALL(ADDRESS_2) => ovmCREATE)',
      steps: [
        {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT,
            target: '$DUMMY_OVM_ADDRESS_1',
            subSteps: [
              {
                functionName: 'ovmDELEGATECALL',
                functionParams: {
                  gasLimit: OVM_TX_GAS_LIMIT,
                  target: '$DUMMY_OVM_ADDRESS_2',
                  subSteps: [
                    {
                      functionName: 'ovmCREATE',
                      functionParams: {
                        bytecode: DUMMY_BYTECODE,
                      },
                      expectedReturnStatus: true,
                      expectedReturnValue: CREATED_CONTRACT_1,
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
        'ovmCALL(ADDRESS_1) => ovmCALL(ADDRESS_2) => ovmDELEGATECALL(ADDRESS_3) => ovmDELEGATECALL(ADDRESS_4) => ovmCALLER',
      steps: [
        {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT,
            target: '$DUMMY_OVM_ADDRESS_1',
            subSteps: [
              {
                functionName: 'ovmCALL',
                functionParams: {
                  gasLimit: OVM_TX_GAS_LIMIT,
                  target: '$DUMMY_OVM_ADDRESS_2',
                  subSteps: [
                    {
                      functionName: 'ovmDELEGATECALL',
                      functionParams: {
                        gasLimit: OVM_TX_GAS_LIMIT,
                        target: '$DUMMY_OVM_ADDRESS_2',
                        subSteps: [
                          {
                            functionName: 'ovmDELEGATECALL',
                            functionParams: {
                              gasLimit: OVM_TX_GAS_LIMIT,
                              target: '$DUMMY_OVM_ADDRESS_3',
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
        'ovmCALL(ADDRESS_1) => ovmCALL(ADDRESS_2) => ovmDELEGATECALL(ADDRESS_3) => ovmDELEGATECALL(ADDRESS_4) => ovmADDRESS',
      steps: [
        {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT,
            target: '$DUMMY_OVM_ADDRESS_1',
            subSteps: [
              {
                functionName: 'ovmCALL',
                functionParams: {
                  gasLimit: OVM_TX_GAS_LIMIT,
                  target: '$DUMMY_OVM_ADDRESS_2',
                  subSteps: [
                    {
                      functionName: 'ovmDELEGATECALL',
                      functionParams: {
                        gasLimit: OVM_TX_GAS_LIMIT,
                        target: '$DUMMY_OVM_ADDRESS_2',
                        subSteps: [
                          {
                            functionName: 'ovmDELEGATECALL',
                            functionParams: {
                              gasLimit: OVM_TX_GAS_LIMIT,
                              target: '$DUMMY_OVM_ADDRESS_3',
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
        'ovmCALL(ADDRESS_1) => ovmCALL(ADDRESS_2) => ovmDELEGATECALL(ADDRESS_3) => ovmDELEGATECALL(ADDRESS_4) => ovmCREATE',
      steps: [
        {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT,
            target: '$DUMMY_OVM_ADDRESS_1',
            subSteps: [
              {
                functionName: 'ovmCALL',
                functionParams: {
                  gasLimit: OVM_TX_GAS_LIMIT,
                  target: '$DUMMY_OVM_ADDRESS_2',
                  subSteps: [
                    {
                      functionName: 'ovmDELEGATECALL',
                      functionParams: {
                        gasLimit: OVM_TX_GAS_LIMIT,
                        target: '$DUMMY_OVM_ADDRESS_2',
                        subSteps: [
                          {
                            functionName: 'ovmDELEGATECALL',
                            functionParams: {
                              gasLimit: OVM_TX_GAS_LIMIT,
                              target: '$DUMMY_OVM_ADDRESS_3',
                              subSteps: [
                                {
                                  functionName: 'ovmCREATE',
                                  functionParams: {
                                    bytecode: DUMMY_BYTECODE,
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
  ],
}

const runner = new ExecutionManagerTestRunner()
runner.run(test_ovmDELEGATECALL)
