/* External Imports */
import { ethers } from 'ethers'
import * as Ganache from 'ganache-core'
import { keccak256 } from 'ethers/lib/utils'
import { fromHexString, toHexString, remove0x } from '@eth-optimism/core-utils'

/* Internal Imports */
import { StorageDump, StateDump } from './get-dump'
import { deploy, RollupDeployConfig } from '../contract-deployment'
import { getContractDefinition } from '../contract-defs'
import { predeploys } from '../predeploys'

/**
 * Generates a storage dump for a given address.
 * @param cStateManager Instance of the callback-based internal vm StateManager.
 * @param address Address to generate a state dump for.
 */
const getStorageDump = async (
  cStateManager: any,
  address: string
): Promise<StorageDump> => {
  return new Promise<StorageDump>((resolve, reject) => {
    cStateManager._getStorageTrie(address, (err: any, trie: any) => {
      if (err) {
        reject(err)
      }

      const storage: StorageDump = {}
      const stream = trie.createReadStream()

      stream.on('data', (val: any) => {
        const storageSlotValue = ethers.utils.RLP.decode(
          '0x' + val.value.toString('hex')
        )
        storage['0x' + val.key.toString('hex')] = storageSlotValue
      })

      stream.on('end', () => {
        resolve(storage)
      })
    })
  })
}

/**
 * Replaces old addresses found in a storage dump with new ones.
 * @param storageDump Storage dump to sanitize.
 * @param accounts Set of accounts to sanitize with.
 * @returns Sanitized storage dump.
 */
const sanitizeStorageDump = (
  storageDump: StorageDump,
  accounts: Array<{
    originalAddress: string
    deadAddress: string
  }>
): StorageDump => {
  for (const account of accounts) {
    account.originalAddress = remove0x(account.originalAddress).toLowerCase()
    account.deadAddress = remove0x(account.deadAddress).toLowerCase()
  }

  for (const [key, value] of Object.entries(storageDump)) {
    let parsedKey = key
    let parsedValue = value
    for (const account of accounts) {
      const re = new RegExp(`${account.originalAddress}`, 'g')
      parsedValue = parsedValue.replace(re, account.deadAddress)
      parsedKey = parsedKey.replace(re, account.deadAddress)
    }

    if (parsedKey !== key) {
      delete storageDump[key]
    }

    storageDump[parsedKey] = parsedValue
  }

  return storageDump
}

export const makeStateDump = async (cfg: RollupDeployConfig): Promise<any> => {
  const ganache = (Ganache as any).provider({
    gasLimit: 100_000_000,
    allowUnlimitedContractSize: true,
    accounts: [
      {
        secretKey:
          '0x29f3edee0ad3abf8e2699402e0e28cd6492c9be7eaab00d732a791c33552f797',
        balance: 10000000000000000000000000000000000,
      },
    ],
  })

  const provider = new ethers.providers.Web3Provider(ganache)
  const signer = provider.getSigner(0)

  let config: RollupDeployConfig = {
    deploymentSigner: signer,
    ovmGasMeteringConfig: {
      minTransactionGasLimit: 0,
      maxTransactionGasLimit: 11_000_000,
      maxGasPerQueuePerEpoch: 1_000_000_000_000,
      secondsPerEpoch: 0,
    },
    ovmGlobalContext: {
      ovmCHAINID: 420,
      L2CrossDomainMessengerAddress: predeploys.OVM_L2CrossDomainMessenger,
    },
    transactionChainConfig: {
      sequencer: signer,
      forceInclusionPeriodSeconds: 600,
      forceInclusionPeriodBlocks: 600 / 12,
    },
    stateChainConfig: {
      fraudProofWindowSeconds: 600,
      sequencerPublishWindowSeconds: 60_000,
    },
    whitelistConfig: {
      owner: signer,
      allowArbitraryContractDeployment: true,
    },
    l1CrossDomainMessengerConfig: {},
    dependencies: [
      'ERC1820Registry',
      'Lib_AddressManager',
      'OVM_DeployerWhitelist',
      'OVM_L1MessageSender',
      'OVM_L2ToL1MessagePasser',
      'OVM_ProxyEOA',
      'OVM_ECDSAContractAccount',
      'OVM_SequencerEntrypoint',
      'OVM_L2CrossDomainMessenger',
      'OVM_SafetyChecker',
      'OVM_ExecutionManager',
      'OVM_StateManager',
      'OVM_ETH',
      'OVM_ExecutionManagerWrapper',
      'OVM_GasPriceOracle',
      'OVM_SequencerFeeVault',
      'OVM_L2StandardBridge',
    ],
    deployOverrides: {},
    waitForReceipts: false,
    gasPriceOracleConfig: {
      owner: signer,
      initialGasPrice: 0,
    },
  }

  config = { ...config, ...cfg }

  const ovmCompiled = [
    'OVM_L2ToL1MessagePasser',
    'OVM_L2CrossDomainMessenger',
    'OVM_SequencerEntrypoint',
    'Lib_AddressManager',
    'OVM_DeployerWhitelist',
    'OVM_ETH',
    'OVM_ECDSAContractAccount',
    'OVM_ProxyEOA',
    'OVM_ExecutionManagerWrapper',
    'OVM_GasPriceOracle',
    'OVM_SequencerFeeVault',
    'OVM_L2StandardBridge',
    'OVM_SafetyChecker',
  ]

  const deploymentResult = await deploy(config)
  deploymentResult.contracts['Lib_AddressManager'] =
    deploymentResult.AddressManager

  if (deploymentResult.failedDeployments.length > 0) {
    throw new Error(
      `Could not generate state dump, deploy failed for: ${deploymentResult.failedDeployments}`
    )
  }

  const pStateManager = ganache.engine.manager.state.blockchain.vm.pStateManager
  const cStateManager = pStateManager._wrapped

  const dump: StateDump = {
    accounts: {},
  }

  for (let i = 0; i < Object.keys(deploymentResult.contracts).length; i++) {
    const name = Object.keys(deploymentResult.contracts)[i]
    const contract = deploymentResult.contracts[name]
    let code: string
    if (ovmCompiled.includes(name)) {
      const ovmDeployedBytecode = getContractDefinition(name, true)
        .deployedBytecode
      // TODO remove: deployedBytecode is missing the find and replace in solidity
      code = ovmDeployedBytecode
        .split(
          '336000905af158601d01573d60011458600c01573d6000803e3d621234565260ea61109c52'
        )
        .join(
          '336000905af158600e01573d6000803e3d6000fd5b3d6001141558600a015760016000f35b'
        )
    } else {
      const codeBuf = await pStateManager.getContractCode(
        fromHexString(contract.address)
      )
      code = toHexString(codeBuf)
    }

    const deadAddress =
      predeploys[name] ||
      `0xdeaddeaddeaddeaddeaddeaddeaddeaddead${i.toString(16).padStart(4, '0')}`

    let def: any
    try {
      def = getContractDefinition(name.replace('Proxy__', ''))
    } catch (err) {
      def = getContractDefinition(name.replace('Proxy__', ''), true)
    }

    dump.accounts[name] = {
      address: deadAddress,
      code,
      codeHash: keccak256(code),
      storage: await getStorageDump(cStateManager, contract.address),
      abi: def.abi,
    }
  }

  const addressMap = Object.keys(dump.accounts).map((name) => {
    return {
      originalAddress: deploymentResult.contracts[name].address,
      deadAddress: dump.accounts[name].address,
    }
  })

  for (const name of Object.keys(dump.accounts)) {
    dump.accounts[name].storage = sanitizeStorageDump(
      dump.accounts[name].storage,
      addressMap
    )
  }

  dump.accounts['OVM_GasMetadata'] = {
    address: '0x06a506a506a506a506a506a506a506a506a506a5',
    code: '0x00',
    codeHash: keccak256('0x00'),
    storage: {},
    abi: [],
  }

  return dump
}
