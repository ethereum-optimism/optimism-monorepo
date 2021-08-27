import { expect } from '../../../setup'
import { deployContractCode } from '../../../helpers/utils'

/* External Imports */
import { ethers } from 'hardhat'
import { Contract, ContractFactory, Signer } from 'ethers'
import { smockit, MockContract } from '@eth-optimism/smock'

/* Internal Imports */
import {
  makeAddressManager,
  NON_ZERO_ADDRESS,
  NON_NULL_BYTES32,
  GasMeasurement,
} from '../../../helpers'

const DUMMY_GASMETERCONFIG = {
  minTransactionGasLimit: 0,
  maxTransactionGasLimit: NON_NULL_BYTES32,
  maxGasPerQueuePerEpoch: NON_NULL_BYTES32,
  secondsPerEpoch: NON_NULL_BYTES32,
}

const DUMMY_GLOBALCONTEXT = {
  ovmCHAINID: 420,
}

const QUEUE_ORIGIN = {
  SEQUENCER_QUEUE: 0,
  L1TOL2_QUEUE: 1,
}

const DUMMY_TRANSACTION = {
  timestamp: 111111111111,
  blockNumber: 20,
  l1QueueOrigin: QUEUE_ORIGIN.SEQUENCER_QUEUE,
  l1TxOrigin: NON_ZERO_ADDRESS,
  entrypoint: NON_ZERO_ADDRESS, // update this below
  gasLimit: 10_000_000,
  data: 0,
}

describe('OVM_ExecutionManager gas consumption', () => {
  let wallet: Signer
  before(async () => {
    ;[wallet] = await ethers.getSigners()
  })

  let Factory__OVM_ExecutionManager: ContractFactory
  let MOCK__STATE_MANAGER: MockContract
  let AddressManager: Contract
  let targetContractAddress: string
  let gasMeasurement: GasMeasurement
  before(async () => {
    Factory__OVM_ExecutionManager = await ethers.getContractFactory(
      'OVM_ExecutionManager'
    )

    // Deploy a simple contract that just returns successfully with no data
    targetContractAddress = await deployContractCode(
      '60206001f3',
      wallet,
      10_000_000
    )
    DUMMY_TRANSACTION.entrypoint = targetContractAddress

    AddressManager = await makeAddressManager()

    // deploy the state manager and mock it for the state transitioner
    MOCK__STATE_MANAGER = await smockit(
      await (
        await ethers.getContractFactory('OVM_StateManager')
      ).deploy(NON_ZERO_ADDRESS)
    )

    // Setup the SM to satisfy all the checks executed during EM.run()
    MOCK__STATE_MANAGER.smocked.isAuthenticated.will.return.with(true)
    MOCK__STATE_MANAGER.smocked.getAccountEthAddress.will.return.with(
      targetContractAddress
    )
    MOCK__STATE_MANAGER.smocked.hasAccount.will.return.with(true)
    MOCK__STATE_MANAGER.smocked.testAndSetAccountLoaded.will.return.with(true)

    await AddressManager.setAddress(
      'OVM_StateManagerFactory',
      MOCK__STATE_MANAGER.address
    )

    gasMeasurement = new GasMeasurement()
    await gasMeasurement.init(wallet)
  })

  let OVM_ExecutionManager: Contract
  beforeEach(async () => {
    OVM_ExecutionManager = (
      await Factory__OVM_ExecutionManager.deploy(
        AddressManager.address,
        DUMMY_GASMETERCONFIG,
        DUMMY_GLOBALCONTEXT
      )
    ).connect(wallet)
  })

  describe('Measure cost of a very simple contract  [ @skip-on-coverage ]', async () => {
    it('Gas cost of run', async () => {
      const gasCost = await gasMeasurement.getGasCost(
        OVM_ExecutionManager,
        'run',
        [DUMMY_TRANSACTION, MOCK__STATE_MANAGER.address]
      )
      console.log(`calculated gas cost of ${gasCost}`)

      const benchmark: number = 111_000
      expect(gasCost).to.be.lte(benchmark)
      expect(gasCost).to.be.gte(
        benchmark - 1_000,
        'Gas cost has significantly decreased, consider updating the benchmark to reflect the change'
      )
    })
  })
})
