/* External Imports */
import { ethers } from 'ethers'

/* Internal Imports */
import {
  ExecutionManagerTestRunner,
  TestDefinition,
  OVM_TX_GAS_LIMIT,
  NON_NULL_BYTES32,
  getStorageXOR,
} from '../../../../helpers'

const test_ovmSLOAD: TestDefinition = {
  name: 'Basic tests for ovmSLOAD',
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
      },
      contractStorage: {
        $DUMMY_OVM_ADDRESS_1: {
          [NON_NULL_BYTES32]: getStorageXOR(ethers.constants.HashZero),
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
      name: 'ovmCALL => ovmSLOAD',
      steps: [
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
                expectedReturnValue: ethers.constants.HashZero,
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
runner.run(test_ovmSLOAD)
