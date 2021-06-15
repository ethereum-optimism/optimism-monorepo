/* External Imports */
import { Signer, ContractFactory, Contract, constants } from 'ethers'
import { TransactionResponse } from '@ethersproject/abstract-provider'
import { Overrides } from '@ethersproject/contracts'

/* Internal Imports */
import { getContractFactory } from '../contract-defs'
import { predeploys } from '../predeploys'

export interface RollupDeployConfig {
  deploymentSigner: Signer
  ovmGasMeteringConfig: {
    minTransactionGasLimit: number
    maxTransactionGasLimit: number
    maxGasPerQueuePerEpoch: number
    secondsPerEpoch: number
  }
  ovmGlobalContext: {
    ovmCHAINID: number
    L2CrossDomainMessengerAddress: string
  }
  transactionChainConfig: {
    sequencer: string | Signer
    forceInclusionPeriodSeconds: number
    forceInclusionPeriodBlocks: number
  }
  stateChainConfig: {
    fraudProofWindowSeconds: number
    sequencerPublishWindowSeconds: number
  }
  l1CrossDomainMessengerConfig: {
    relayerAddress?: string | Signer
  }
  whitelistConfig: {
    owner: string | Signer
    allowArbitraryContractDeployment: boolean
  }
  gasPriceOracleConfig: {
    owner: string | Signer
    initialGasPrice: number
  }
  addressManager?: string
  dependencies?: string[]
  deployOverrides: Overrides
  waitForReceipts: boolean
}

export interface ContractDeployParameters {
  factory: ContractFactory
  params?: any[]
  afterDeploy?: (contracts?: { [name: string]: Contract }) => Promise<void>
}

export interface ContractDeployConfig {
  [name: string]: ContractDeployParameters
}

export const makeContractDeployConfig = async (
  config: RollupDeployConfig,
  AddressManager: Contract
): Promise<ContractDeployConfig> => {
  const _sendTx = async (
    txPromise: Promise<TransactionResponse>
  ): Promise<TransactionResponse> => {
    const res = await txPromise
    if (config.waitForReceipts) {
      await res.wait()
    }
    return res
  }

  return {
    OVM_L2CrossDomainMessenger: {
      factory: getContractFactory('OVM_L2CrossDomainMessenger'),
      params: [AddressManager.address],
    },
    OVM_L1CrossDomainMessenger: {
      factory: getContractFactory('OVM_L1CrossDomainMessenger'),
      params: [],
      afterDeploy: async (contracts): Promise<void> => {
        if (config.l1CrossDomainMessengerConfig.relayerAddress) {
          const relayer = config.l1CrossDomainMessengerConfig.relayerAddress
          const address =
            typeof relayer === 'string' ? relayer : await relayer.getAddress()
          await _sendTx(
            AddressManager.setAddress('OVM_L2MessageRelayer', address)
          )
        }
      },
    },
    Proxy__OVM_L1CrossDomainMessenger: {
      factory: getContractFactory('Lib_ResolvedDelegateProxy'),
      params: [AddressManager.address, 'OVM_L1CrossDomainMessenger'],
      afterDeploy: async (contracts): Promise<void> => {
        const xDomainMessenger = getContractFactory(
          'OVM_L1CrossDomainMessenger'
        )
          .connect(config.deploymentSigner)
          .attach(contracts.Proxy__OVM_L1CrossDomainMessenger.address)
        await _sendTx(
          xDomainMessenger.initialize(
            AddressManager.address,
            config.deployOverrides
          )
        )
        await _sendTx(
          AddressManager.setAddress(
            'OVM_L2CrossDomainMessenger',
            config.ovmGlobalContext.L2CrossDomainMessengerAddress,
            config.deployOverrides
          )
        )
      },
    },
    OVM_L1StandardBridge: {
      factory: getContractFactory('OVM_L1StandardBridge'),
      params: [],
    },
    Proxy__OVM_L1StandardBridge: {
      factory: getContractFactory('Lib_ResolvedDelegateProxy'),
      params: [AddressManager.address, 'OVM_L1StandardBridge'],
    },
    OVM_L2StandardBridge: {
      factory: getContractFactory('OVM_L2StandardBridge'),
      params: [
        predeploys.OVM_L2CrossDomainMessenger,
        constants.AddressZero, // we'll set this to the L1 Bridge address in genesis.go
      ],
    },
    OVM_L1MultiMessageRelayer: {
      factory: getContractFactory('OVM_L1MultiMessageRelayer'),
      params: [AddressManager.address],
    },
    OVM_CanonicalTransactionChain: {
      factory: getContractFactory('OVM_CanonicalTransactionChain'),
      params: [
        AddressManager.address,
        config.transactionChainConfig.forceInclusionPeriodSeconds,
        config.transactionChainConfig.forceInclusionPeriodBlocks,
        config.ovmGasMeteringConfig.maxTransactionGasLimit,
      ],
      afterDeploy: async (): Promise<void> => {
        const sequencer = config.transactionChainConfig.sequencer
        const sequencerAddress =
          typeof sequencer === 'string'
            ? sequencer
            : await sequencer.getAddress()
        await _sendTx(
          AddressManager.setAddress(
            'OVM_DecompressionPrecompileAddress',
            predeploys.OVM_SequencerEntrypoint
          )
        )
        await _sendTx(
          AddressManager.setAddress('OVM_Sequencer', sequencerAddress)
        )
        await _sendTx(
          AddressManager.setAddress('OVM_Proposer', sequencerAddress)
        )
        await _sendTx(AddressManager.setAddress('Sequencer', sequencerAddress))
      },
    },
    OVM_StateCommitmentChain: {
      factory: getContractFactory('OVM_StateCommitmentChain'),
      params: [
        AddressManager.address,
        config.stateChainConfig.fraudProofWindowSeconds,
        config.stateChainConfig.sequencerPublishWindowSeconds,
      ],
    },
    OVM_DeployerWhitelist: {
      factory: getContractFactory('OVM_DeployerWhitelist', undefined, true),
      params: [],
    },
    OVM_SafetyChecker: {
      factory: getContractFactory('OVM_SafetyChecker'),
      params: [],
    },
    OVM_L1MessageSender: {
      factory: getContractFactory('OVM_L1MessageSender'),
      params: [],
    },
    OVM_L2ToL1MessagePasser: {
      factory: getContractFactory('OVM_L2ToL1MessagePasser'),
      params: [],
    },
    OVM_ExecutionManager: {
      factory: getContractFactory('OVM_ExecutionManager'),
      params: [
        AddressManager.address,
        config.ovmGasMeteringConfig,
        config.ovmGlobalContext,
      ],
    },
    OVM_StateManager: {
      factory: getContractFactory('OVM_StateManager'),
      params: [await config.deploymentSigner.getAddress()],
      afterDeploy: async (contracts): Promise<void> => {
        await _sendTx(
          contracts.OVM_StateManager.setExecutionManager(
            contracts.OVM_ExecutionManager.address,
            config.deployOverrides
          )
        )
      },
    },
    OVM_StateManagerFactory: {
      factory: getContractFactory('OVM_StateManagerFactory'),
      params: [],
    },
    OVM_FraudVerifier: {
      factory: getContractFactory('OVM_FraudVerifier'),
      params: [AddressManager.address],
    },
    OVM_StateTransitionerFactory: {
      factory: getContractFactory('OVM_StateTransitionerFactory'),
      params: [AddressManager.address],
    },
    OVM_ECDSAContractAccount: {
      factory: getContractFactory('OVM_ECDSAContractAccount', undefined, true),
    },
    OVM_SequencerEntrypoint: {
      factory: getContractFactory('OVM_SequencerEntrypoint', undefined, true),
    },
    OVM_BondManager: {
      factory: getContractFactory('mockOVM_BondManager'),
      params: [AddressManager.address],
    },
    OVM_ETH: {
      factory: getContractFactory('OVM_ETH'),
      params: [],
    },
    'OVM_ChainStorageContainer-CTC-batches': {
      factory: getContractFactory('OVM_ChainStorageContainer'),
      params: [AddressManager.address, 'OVM_CanonicalTransactionChain'],
    },
    'OVM_ChainStorageContainer-CTC-queue': {
      factory: getContractFactory('OVM_ChainStorageContainer'),
      params: [AddressManager.address, 'OVM_CanonicalTransactionChain'],
    },
    'OVM_ChainStorageContainer-SCC-batches': {
      factory: getContractFactory('OVM_ChainStorageContainer'),
      params: [AddressManager.address, 'OVM_StateCommitmentChain'],
    },
    ERC1820Registry: {
      factory: getContractFactory('ERC1820Registry'),
    },
    OVM_ProxyEOA: {
      factory: getContractFactory('OVM_ProxyEOA', undefined, true),
    },
    OVM_ExecutionManagerWrapper: {
      factory: getContractFactory(
        'OVM_ExecutionManagerWrapper',
        undefined,
        true
      ),
    },
    OVM_GasPriceOracle: {
      factory: getContractFactory('OVM_GasPriceOracle'),
      params: [
        (() => {
          if (typeof config.gasPriceOracleConfig.owner !== 'string') {
            return config.gasPriceOracleConfig.owner.getAddress()
          }
          return config.gasPriceOracleConfig.owner
        })(),
        config.gasPriceOracleConfig.initialGasPrice,
      ],
    },
    OVM_SequencerFeeVault: {
      factory: getContractFactory('OVM_SequencerFeeVault'),
      params: [`0x${'11'.repeat(20)}`],
    },
  }
}
