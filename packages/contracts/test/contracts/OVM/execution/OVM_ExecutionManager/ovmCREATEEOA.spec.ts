/* External Imports */
import { fromHexString } from '@eth-optimism/core-utils'

/* Internal Imports */
import {
  ExecutionManagerTestRunner,
  TestDefinition,
  OVM_TX_GAS_LIMIT,
  NON_NULL_BYTES32,
  VERIFIED_EMPTY_CONTRACT_HASH,
} from '../../../../helpers'
import { getContractDefinition } from '../../../../../src'

const test_ovmCREATEEOA: TestDefinition = {
  name: 'Basic tests for CREATEEOA',
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
        '0x17ec8597ff92C3F44523bDc65BF0f1bE632917ff': {
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
      name: 'ovmCREATEEOA, ovmEXTCODESIZE(CREATED)',
      steps: [
        {
          functionName: 'ovmCREATEEOA',
          functionParams: {
            _messageHash:
              '0x92d658d25f963af824e9d4bd533c165773d4a694a67d88135d119d5bca97c001',
            _v: 1,
            _r:
              '0x73757c671fae2c3fb6825766c724b7715720bda4b309d3612f2c623364556967',
            _s:
              '0x2fc9b7222783390b9f10e22e92a52871beaff2613193d6e2dbf18d0e2d2eb8ff',
          },
          expectedReturnStatus: true,
          expectedReturnValue: undefined,
        },
        {
          functionName: 'ovmGETNONCE',
          expectedReturnValue: 0,
        },
        {
          functionName: 'ovmEXTCODESIZE',
          functionParams: {
            address: '0x17ec8597ff92C3F44523bDc65BF0f1bE632917ff',
          },
          expectedReturnStatus: true,
          expectedReturnValue: fromHexString(
            getContractDefinition('OVM_ProxyEOA', true).deployedBytecode
          ).length,
        },
      ],
    },
    {
      name: 'ovmCALL(ADDRESS_1) => ovmGETNONCE',
      steps: [
        {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT,
            target: '$DUMMY_OVM_ADDRESS_1',
            subSteps: [
              {
                functionName: 'ovmGETNONCE',
                expectedReturnValue: 0,
              },
            ],
          },
          expectedReturnStatus: true,
        },
      ],
    },
    {
      name: 'ovmCALL(ADDRESS_1) => ovmINCREMENTNONCEx3 => ovmGETNONCE',
      steps: [
        {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT,
            target: '$DUMMY_OVM_ADDRESS_1',
            subSteps: [
              {
                functionName: 'ovmINCREMENTNONCE',
                expectedReturnStatus: true,
              },
              {
                functionName: 'ovmINCREMENTNONCE',
                expectedReturnStatus: true,
              },
              {
                functionName: 'ovmINCREMENTNONCE',
                expectedReturnStatus: true,
              },
              {
                functionName: 'ovmGETNONCE',
                expectedReturnValue: 3,
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
runner.run(test_ovmCREATEEOA)
