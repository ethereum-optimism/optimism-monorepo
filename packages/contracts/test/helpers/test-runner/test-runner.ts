import { expect } from '../../setup'

/* External Imports */
import { ethers } from 'hardhat'
import { Contract, BigNumber, ContractFactory } from 'ethers'
import { cloneDeep, merge } from 'lodash'
import { smoddit, smockit, ModifiableContract } from '@eth-optimism/smock'

/* Internal Imports */
import {
  TestDefinition,
  ParsedTestStep,
  TestParameter,
  TestStep,
  TestStep_CALLType,
  TestStep_Run,
  isRevertFlagError,
  isTestStep_SSTORE,
  isTestStep_SLOAD,
  isTestStep_CALLType,
  isTestStep_CREATE,
  isTestStep_CREATE2,
  isTestStep_CREATEEOA,
  isTestStep_Context,
  isTestStep_evm,
  isTestStep_Run,
  isTestStep_EXTCODESIZE,
  isTestStep_EXTCODEHASH,
  isTestStep_EXTCODECOPY,
  isTestStep_BALANCE,
  isTestStep_REVERT,
  isTestStep_CALL,
} from './test.types'
import { encodeRevertData, REVERT_FLAGS } from '../codec'
import {
  OVM_TX_GAS_LIMIT,
  RUN_OVM_TEST_GAS,
  NON_NULL_BYTES32,
} from '../constants'
import { getStorageXOR } from '../'
import { UNSAFE_BYTECODE } from '../dummy'
import { getContractFactory, predeploys } from '../../../src'
import { safetyCheckerPrestate } from './safety-checker-prestate'

export class ExecutionManagerTestRunner {
  private snapshot: string
  private contracts: {
    OVM_SafetyChecker: Contract
    OVM_StateManager: ModifiableContract
    OVM_ExecutionManager: ModifiableContract
    Helper_TestRunner: Contract
    Factory__Helper_TestRunner_CREATE: ContractFactory
    OVM_DeployerWhitelist: Contract
    OVM_ProxyEOA: Contract
    OVM_ETH: Contract
  } = {
    OVM_SafetyChecker: undefined,
    OVM_StateManager: undefined,
    OVM_ExecutionManager: undefined,
    Helper_TestRunner: undefined,
    Factory__Helper_TestRunner_CREATE: undefined,
    OVM_DeployerWhitelist: undefined,
    OVM_ProxyEOA: undefined,
    OVM_ETH: undefined,
  }

  // Default pre-state with contract deployer whitelist NOT initialized.
  private defaultPreState = {
    StateManager: {
      owner: '$OVM_EXECUTION_MANAGER',
      accounts: {
        [predeploys.OVM_DeployerWhitelist]: {
          codeHash: NON_NULL_BYTES32,
          ethAddress: '$OVM_DEPLOYER_WHITELIST',
        },
        [predeploys.OVM_ETH]: {
          codeHash: NON_NULL_BYTES32,
          ethAddress: '$OVM_ETH',
        },
        [predeploys.OVM_ProxyEOA]: {
          codeHash: NON_NULL_BYTES32,
          ethAddress: '$OVM_PROXY_EOA',
        },
        [predeploys.OVM_SafetyChecker]: {
          codeHash: NON_NULL_BYTES32,
          ethAddress: '$OVM_SAFETY_CHECKER',
        },
        // Address of hardhat/console.sol, comes in handy sometimes
        ['0x000000000000000000636F6e736F6c652e6c6f67']: {
          codeHash: NON_NULL_BYTES32,
          ethAddress: '0x000000000000000000636F6e736F6c652e6c6f67',
        },
      },
      contractStorage: {
        [predeploys.OVM_DeployerWhitelist]: {
          [ethers.constants.HashZero]: {
            getStorageXOR: true,
            value: ethers.constants.HashZero,
          },
        },
      },
      verifiedContractStorage: {
        [predeploys.OVM_DeployerWhitelist]: {
          [ethers.constants.HashZero]: true,
        }
      },
    },
    ExecutionManager: {
      transactionRecord: {
        ovmGasRefund: 0,
      },
    },
  }

  public run(test: TestDefinition) {
    // tslint:disable-next-line:ban-comma-operator
    ;(test.preState = merge(
      cloneDeep(this.defaultPreState),
      cloneDeep(safetyCheckerPrestate),
      cloneDeep(test.preState)
    )),
      (test.postState = test.postState || {})

    describe(`OVM_ExecutionManager Test: ${test.name}`, () => {
      test.subTests?.map((subTest) => {
        this.run({
          ...subTest,
          preState: merge(
            cloneDeep(test.preState),
            cloneDeep(subTest.preState)
          ),
          postState: merge(
            cloneDeep(test.postState),
            cloneDeep(subTest.postState)
          ),
        })
      })

      test.parameters?.map((parameter) => {
        beforeEach(async () => {
          await this.initContracts()
        })

        let replacedTest: TestDefinition
        let replacedParameter: TestParameter
        beforeEach(async () => {
          replacedTest = this.setPlaceholderStrings(test)
          replacedParameter = this.setPlaceholderStrings(parameter)
        })

        beforeEach(async () => {
          await this.contracts.OVM_StateManager.smodify.put({
            accounts: {
              [this.contracts.Helper_TestRunner.address]: {
                nonce: 0,
                codeHash: NON_NULL_BYTES32,
                ethAddress: this.contracts.Helper_TestRunner.address,
              },
            },
          })
        })

        beforeEach(async () => {
          await this.contracts.OVM_ExecutionManager.smodify.put(
            replacedTest.preState.ExecutionManager
          )
          await this.contracts.OVM_StateManager.smodify.put(
            replacedTest.preState.StateManager
          )
        })

        afterEach(async () => {
          expect(
            await this.contracts.OVM_ExecutionManager.smodify.check(
              replacedTest.postState.ExecutionManager
            )
          ).to.equal(true)

          expect(
            await this.contracts.OVM_StateManager.smodify.check(
              replacedTest.postState.StateManager
            )
          ).to.equal(true)
        })

        let itfn: any = it
        if (parameter.focus) {
          itfn = it.only
        } else if (parameter.skip) {
          itfn = it.skip
        }

        itfn(`should execute: ${parameter.name}`, async () => {
          try {
            for (const step of replacedParameter.steps) {
              await this.runTestStep(step)
            }
          } catch (err) {
            if (parameter.expectInvalidStateAccess) {
              expect(err.toString()).to.contain(
                'VM Exception while processing transaction: revert'
              )
            } else {
              throw err
            }
          }
        })
      })
    })
  }

  private async initContracts() {
    if (this.snapshot) {
      await ethers.provider.send('evm_revert', [this.snapshot])
      this.snapshot = await ethers.provider.send('evm_snapshot', [])
      return
    }

    const AddressManager = await (
      await ethers.getContractFactory('Lib_AddressManager')
    ).deploy()

    const SafetyChecker = await (
      await ethers.getContractFactory('OVM_SafetyChecker')
    ).deploy()

    const MockSafetyChecker = await smockit(SafetyChecker)
    MockSafetyChecker.smocked.checkAndRegisterSafeBytecode.will.return.with(
      (bytecode: string) => {
        return bytecode !== UNSAFE_BYTECODE
      }
    )

    this.contracts.OVM_SafetyChecker = MockSafetyChecker


    const DeployerWhitelist = await getContractFactory(
      'OVM_DeployerWhitelist',
      AddressManager.signer,
      true
    ).deploy()

    this.contracts.OVM_DeployerWhitelist = DeployerWhitelist

    const OvmEth = await getContractFactory(
      'OVM_ETH',
      AddressManager.signer,
      true
    ).deploy()

    this.contracts.OVM_ETH = OvmEth

    this.contracts.OVM_ProxyEOA = await getContractFactory(
      'OVM_ProxyEOA',
      AddressManager.signer,
      true
    ).deploy()

    this.contracts.OVM_ExecutionManager = await (
      await smoddit('OVM_ExecutionManager')
    ).deploy(
      AddressManager.address,
      {
        minTransactionGasLimit: 0,
        maxTransactionGasLimit: 1_000_000_000,
        maxGasPerQueuePerEpoch: 1_000_000_000_000,
        secondsPerEpoch: 600,
      },
      {
        ovmCHAINID: 420,
      }
    )

    this.contracts.OVM_StateManager = await (
      await smoddit('OVM_StateManager')
    ).deploy(await this.contracts.OVM_ExecutionManager.signer.getAddress())
    await this.contracts.OVM_StateManager.setExecutionManager(
      this.contracts.OVM_ExecutionManager.address
    )

    this.contracts.Helper_TestRunner = await (
      await ethers.getContractFactory('Helper_TestRunner')
    ).deploy()

    this.contracts.Factory__Helper_TestRunner_CREATE = await ethers.getContractFactory(
      'Helper_TestRunner_CREATE'
    )

    this.snapshot = await ethers.provider.send('evm_snapshot', [])
  }

  public static getDummyAddress(placeholder: string): string {
    return '0x' + (placeholder.split('$DUMMY_OVM_ADDRESS_')[1] + '0').repeat(20)
  }

  private setPlaceholderStrings(obj: any) {
    const getReplacementString = (kv: string): string => {
      if (kv === '$OVM_EXECUTION_MANAGER') {
        return this.contracts.OVM_ExecutionManager.address
      } else if (kv === '$OVM_STATE_MANAGER') {
        return this.contracts.OVM_StateManager.address
      } else if (kv === '$OVM_SAFETY_CHECKER') {
        return this.contracts.OVM_SafetyChecker.address
      } else if (kv === '$OVM_CALL_HELPER') {
        return this.contracts.Helper_TestRunner.address
      } else if (kv === '$OVM_DEPLOYER_WHITELIST') {
        return this.contracts.OVM_DeployerWhitelist.address
      } else if (kv === '$OVM_ETH') {
        return this.contracts.OVM_ETH.address
      } else if (kv === '$OVM_PROXY_EOA') {
        return this.contracts.OVM_ProxyEOA.address
      } else if (kv.startsWith('$DUMMY_OVM_ADDRESS_')) {
        return ExecutionManagerTestRunner.getDummyAddress(kv)
      } else {
        return kv
      }
    }

    let ret: any = cloneDeep(obj)
    if (Array.isArray(ret)) {
      ret = ret.map((element: any) => {
        return this.setPlaceholderStrings(element)
      })
    } else if (typeof ret === 'object' && ret !== null) {
      if (ret.getStorageXOR) {
        // Special case allowing us to set prestate with an object which will be
        // padded to 32 bytes and XORd with STORAGE_XOR_VALUE
        return getStorageXOR(
          ethers.utils.hexZeroPad(getReplacementString(ret.value), 32)
        )
      }

      for (const key of Object.keys(ret)) {
        const replacedKey = getReplacementString(key)

        if (replacedKey !== key) {
          ret[replacedKey] = ret[key]
          delete ret[key]
        }

        ret[replacedKey] = this.setPlaceholderStrings(ret[replacedKey])
      }
    } else if (typeof ret === 'string') {
      ret = getReplacementString(ret)
    }

    return ret
  }

  private async runTestStep(step: TestStep | TestStep_Run) {
    if (isTestStep_Run(step)) {
      let calldata: string
      if (step.functionParams.data) {
        calldata = step.functionParams.data
      } else {
        const runStep: TestStep_CALLType = {
          functionName: 'ovmCALL',
          functionParams: {
            gasLimit: OVM_TX_GAS_LIMIT,
            target: ExecutionManagerTestRunner.getDummyAddress(
              '$DUMMY_OVM_ADDRESS_1'
            ),
            subSteps: step.functionParams.subSteps,
          },
          expectedReturnStatus: true,
        }

        calldata = this.encodeFunctionData(runStep)
      }

      const toRun = this.contracts.OVM_ExecutionManager.run(
        {
          timestamp: step.functionParams.timestamp,
          blockNumber: 0,
          l1QueueOrigin: step.functionParams.queueOrigin,
          l1TxOrigin: step.functionParams.origin,
          entrypoint: step.functionParams.entrypoint,
          gasLimit: step.functionParams.gasLimit,
          data: calldata,
        },
        this.contracts.OVM_StateManager.address,
        { gasLimit: step.suppliedGas || RUN_OVM_TEST_GAS }
      )
      if (!!step.expectedRevertValue) {
        await expect(toRun).to.be.revertedWith(step.expectedRevertValue)
      } else {
        await toRun
      }
    } else {
      await this.contracts.OVM_ExecutionManager[
        'ovmCALL(uint256,address,uint256,bytes)'
      ](
        OVM_TX_GAS_LIMIT,
        ExecutionManagerTestRunner.getDummyAddress('$DUMMY_OVM_ADDRESS_1'),
        0,
        this.contracts.Helper_TestRunner.interface.encodeFunctionData(
          'runSingleTestStep',
          [this.parseTestStep(step)]
        ),
        { gasLimit: RUN_OVM_TEST_GAS }
      )
    }
  }

  private parseTestStep(step: TestStep): ParsedTestStep {
    return {
      functionName: step.functionName,
      functionData: this.encodeFunctionData(step),
      expectedReturnStatus: this.getReturnStatus(step),
      expectedReturnData: this.encodeExpectedReturnData(step),
      onlyValidateFlag: this.shouldStepOnlyValidateFlag(step),
    }
  }

  private shouldStepOnlyValidateFlag(step: TestStep): boolean {
    if (!!(step as any).expectedReturnValue) {
      if (!!((step as any).expectedReturnValue as any).onlyValidateFlag) {
        return true
      }
    }
    return false
  }

  private getReturnStatus(step: TestStep): boolean {
    if (isTestStep_evm(step)) {
      return false
    } else if (isTestStep_Context(step)) {
      return true
    } else if (isTestStep_CALLType(step)) {
      if (
        isRevertFlagError(step.expectedReturnValue) &&
        (step.expectedReturnValue.flag === REVERT_FLAGS.INVALID_STATE_ACCESS ||
          step.expectedReturnValue.flag === REVERT_FLAGS.STATIC_VIOLATION ||
          step.expectedReturnValue.flag === REVERT_FLAGS.CREATOR_NOT_ALLOWED)
      ) {
        return step.expectedReturnStatus
      } else {
        return true
      }
    } else {
      return step.expectedReturnStatus
    }
  }

  private encodeFunctionData(step: TestStep): string {
    if (isTestStep_evm(step)) {
      if (isRevertFlagError(step.returnData)) {
        return encodeRevertData(
          step.returnData.flag,
          step.returnData.data,
          step.returnData.nuisanceGasLeft,
          step.returnData.ovmGasRefund
        )
      } else {
        return step.returnData || '0x'
      }
    }

    let functionParams: any[] = []
    if (
      isTestStep_SSTORE(step) ||
      isTestStep_SLOAD(step) ||
      isTestStep_EXTCODESIZE(step) ||
      isTestStep_EXTCODEHASH(step) ||
      isTestStep_EXTCODECOPY(step) ||
      isTestStep_BALANCE(step) ||
      isTestStep_CREATEEOA(step)
    ) {
      functionParams = Object.values(step.functionParams)
    } else if (isTestStep_CALLType(step)) {
      const innnerCalldata =
        step.functionParams.calldata ||
        this.contracts.Helper_TestRunner.interface.encodeFunctionData(
          'runMultipleTestSteps',
          [
            step.functionParams.subSteps.map((subStep) => {
              return this.parseTestStep(subStep)
            }),
          ]
        )
      // only ovmCALL accepts a value parameter.
      if (isTestStep_CALL(step)) {
        functionParams = [
          step.functionParams.gasLimit,
          step.functionParams.target,
          step.functionParams.value || 0,
          innnerCalldata,
        ]
      } else {
        functionParams = [
          step.functionParams.gasLimit,
          step.functionParams.target,
          innnerCalldata,
        ]
      }
    } else if (isTestStep_CREATE(step)) {
      functionParams = [
        this.contracts.Factory__Helper_TestRunner_CREATE.getDeployTransaction(
          step.functionParams.bytecode || '0x',
          step.functionParams.subSteps?.map((subStep) => {
            return this.parseTestStep(subStep)
          }) || []
        ).data,
      ]
    } else if (isTestStep_CREATE2(step)) {
      functionParams = [
        this.contracts.Factory__Helper_TestRunner_CREATE.getDeployTransaction(
          step.functionParams.bytecode || '0x',
          step.functionParams.subSteps?.map((subStep) => {
            return this.parseTestStep(subStep)
          }) || []
        ).data,
        step.functionParams.salt,
      ]
    } else if (isTestStep_REVERT(step)) {
      functionParams = [step.revertData || '0x']
    }

    // legacy ovmCALL causes multiple matching functions without the full signature
    let functionName
    if (step.functionName === 'ovmCALL') {
      functionName = 'ovmCALL(uint256,address,uint256,bytes)'
    } else {
      functionName = step.functionName
    }

    return this.contracts.OVM_ExecutionManager.interface.encodeFunctionData(
      functionName,
      functionParams
    )
  }

  private encodeExpectedReturnData(step: TestStep): string {
    if (isTestStep_evm(step)) {
      return '0x'
    }

    if (isRevertFlagError(step.expectedReturnValue)) {
      return encodeRevertData(
        step.expectedReturnValue.flag,
        step.expectedReturnValue.data,
        step.expectedReturnValue.nuisanceGasLeft,
        step.expectedReturnValue.ovmGasRefund
      )
    }

    if (isTestStep_REVERT(step)) {
      return step.expectedReturnValue || '0x'
    }

    let returnData: any[] = []
    if (isTestStep_CALLType(step)) {
      if (step.expectedReturnValue === '0x00') {
        return step.expectedReturnValue
      } else if (
        typeof step.expectedReturnValue === 'string' ||
        step.expectedReturnValue === undefined
      ) {
        returnData = [
          step.expectedReturnStatus,
          step.expectedReturnValue || '0x',
        ]
      } else {
        returnData = [
          step.expectedReturnValue.ovmSuccess,
          step.expectedReturnValue.returnData,
        ]
      }
    } else if (BigNumber.isBigNumber(step.expectedReturnValue)) {
      returnData = [step.expectedReturnValue.toHexString()]
    } else if (step.expectedReturnValue !== undefined) {
      if (step.expectedReturnValue === '0x00') {
        return step.expectedReturnValue
      } else {
        returnData = [step.expectedReturnValue]
      }
    }

    if (isTestStep_CREATE(step) || isTestStep_CREATE2(step)) {
      if (!isRevertFlagError(step.expectedReturnValue)) {
        if (typeof step.expectedReturnValue === 'string') {
          returnData = [step.expectedReturnValue, '0x']
        } else {
          returnData = [
            step.expectedReturnValue.address,
            step.expectedReturnValue.revertData || '0x',
          ]
        }
      }
    }

    // legacy ovmCALL causes multiple matching functions without the full signature
    let functionName
    if (step.functionName === 'ovmCALL') {
      functionName = 'ovmCALL(uint256,address,uint256,bytes)'
    } else {
      functionName = step.functionName
    }

    return this.contracts.OVM_ExecutionManager.interface.encodeFunctionResult(
      functionName,
      returnData
    )
  }
}
