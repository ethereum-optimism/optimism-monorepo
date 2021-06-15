/* Internal Imports */
import {
  ExecutionManagerTestRunner,
  TestDefinition,
  NON_NULL_BYTES32,
  OVM_TX_GAS_LIMIT,
} from '../../../../helpers'

const globalContext = {
  ovmCHAINID: 420,
}

const transactionContext = {
  ovmTIMESTAMP: 12341234,
  ovmNUMBER: 13371337,
  ovmGASLIMIT: 45674567,
  ovmTXGASLIMIT: 78907890,
  ovmL1QUEUEORIGIN: 1,
  ovmL1TXORIGIN: '0x1234123412341234123412341234123412341234',
}

const messageContext = {
  ovmCALLER: '0x6789678967896789678967896789678967896789',
  ovmADDRESS: '0x4567456745674567456745674567456745674567',
}

const test_contextOpcodes: TestDefinition = {
  name: 'unit tests for basic getter opcodes',
  preState: {
    ExecutionManager: {
      ovmStateManager: '$OVM_STATE_MANAGER',
      messageRecord: {
        nuisanceGasLeft: OVM_TX_GAS_LIMIT,
      },
      globalContext,
      transactionContext,
      messageContext,
    },
    StateManager: {
      owner: '$OVM_EXECUTION_MANAGER',
      accounts: {
        $DUMMY_OVM_ADDRESS_1: {
          codeHash: NON_NULL_BYTES32,
          ethAddress: '$OVM_CALL_HELPER',
        },
      },
    },
  },
  parameters: [
    // TODO: re-enable when we can unwrap tests' ovmCALL
    // {
    //   name: 'gets ovmCALLER',
    //   steps: [
    //           {
    //               functionName: 'ovmCALLER',
    //               expectedReturnValue: messageContext.ovmCALLER
    //           }
    //       ],
    // },
    // {
    //   name: 'gets ovmADDRESS',
    //       steps: [
    //           {
    //               functionName: 'ovmADDRESS',
    //               expectedReturnValue: messageContext.ovmADDRESS
    //           }
    //       ],
    // },
    {
      name: 'gets ovmTIMESTAMP',
      steps: [
        {
          functionName: 'ovmTIMESTAMP',
          expectedReturnValue: transactionContext.ovmTIMESTAMP,
        },
      ],
    },
    {
      name: 'gets ovmNUMBER',
      steps: [
        {
          functionName: 'ovmNUMBER',
          expectedReturnValue: transactionContext.ovmNUMBER,
        },
      ],
    },
    {
      name: 'gets ovmGASLIMIT',
      steps: [
        {
          functionName: 'ovmGASLIMIT',
          expectedReturnValue: transactionContext.ovmGASLIMIT,
        },
      ],
    },
    {
      name: 'gets ovmL1QUEUEORIGIN',
      steps: [
        {
          functionName: 'ovmL1QUEUEORIGIN',
          expectedReturnValue: transactionContext.ovmL1QUEUEORIGIN,
        },
      ],
    },
    {
      name: 'gets ovmL1TXORIGIN',
      steps: [
        {
          functionName: 'ovmL1TXORIGIN',
          expectedReturnValue: transactionContext.ovmL1TXORIGIN,
        },
      ],
    },
    {
      name: 'gets ovmCHAINID',
      steps: [
        {
          functionName: 'ovmCHAINID',
          expectedReturnValue: globalContext.ovmCHAINID,
        },
      ],
    },
  ],
}

const runner = new ExecutionManagerTestRunner()
runner.run(test_contextOpcodes)
