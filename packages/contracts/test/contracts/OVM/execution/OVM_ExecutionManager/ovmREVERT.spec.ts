/* Internal Imports */
import {
  ExecutionManagerTestRunner,
  TestDefinition,
  OVM_TX_GAS_LIMIT,
  NON_NULL_BYTES32,
  REVERT_FLAGS,
} from '../../../../helpers'

const test_ovmREVERT: TestDefinition = {
  name: 'Basic tests for ovmREVERT',
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
      },
    },
  },
  parameters: [
    {
      name: 'ovmCALL => ovmREVERT',
      steps: [
        {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT / 2,
            target: '$DUMMY_OVM_ADDRESS_1',
            subSteps: [
              {
                functionName: 'ovmREVERT',
                revertData: '0xdeadbeef',
                expectedReturnStatus: false,
                expectedReturnValue: {
                  flag: REVERT_FLAGS.INTENTIONAL_REVERT,
                  data: '0xdeadbeef',
                  nuisanceGasLeft: OVM_TX_GAS_LIMIT / 2,
                  ovmGasRefund: 0,
                },
              },
            ],
          },
          expectedReturnStatus: false,
          expectedReturnValue: '0xdeadbeef',
        },
      ],
    },
    // TODO: fix this.  only way to do it is manually set up and call ovmREVERT directly inside a context which mirrors that during creation.
    // {
    //   name: "ovmREVERT inside ovmCREATE ?",
    //   parameters: [
    //     {
    //       steps: [
    //         {
    //           functionName: "ovmCALL",
    //           functionParams: [
    //             GAS_LIMIT / 2,
    //             "$DUMMY_OVM_ADDRESS_1",
    //             [
    //               {
    //                 functionName: "ovmCREATE",
    //                 functionParams: [
    //                   USELESS_BYTECODE,
    //                   false, // "create will be successful?"
    //                   [
    //                     {
    //                       functionName: "ovmREVERT",
    //                       functionParams: [ "0xdeadbeef" ],
    //                       expectedReturnStatus: false,
    //                       expectedReturnValues: [ "0x00" ] // no return values for reversion in constructor
    //                     },
    //                     // TODO: check internally flagged storage here
    //                   ]
    //                 ],
    //                 expectedReturnStatus: true,
    //                 expectedReturnValues: [ CREATED_CONTRACT_1 ]
    //               }
    //             ],
    //           ],
    //           expectedReturnStatus: true,
    //           expectedReturnValues: []
    //         }
    //       ]
    //     }
    //   ]
    // }
  ],
}

const runner = new ExecutionManagerTestRunner()
runner.run(test_ovmREVERT)
