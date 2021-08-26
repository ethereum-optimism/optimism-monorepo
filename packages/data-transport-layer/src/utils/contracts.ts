/* Imports: External */
import { constants, Contract, Signer } from 'ethers'
import { BaseProvider } from '@ethersproject/providers'
import { getContractInterface } from '@eth-optimism/contracts'

export const loadContract = (
  name: string,
  address: string,
  provider: BaseProvider
): Contract => {
  return new Contract(address, getContractInterface(name) as any, provider)
}

export const loadProxyFromManager = async (
  name: string,
  proxy: string,
  Lib_AddressManager: Contract,
  provider: BaseProvider
): Promise<Contract> => {
  const address = await Lib_AddressManager.getAddress(proxy)

  if (address === constants.AddressZero) {
    throw new Error(
      `Lib_AddressManager does not have a record for a contract named: ${proxy}`
    )
  }

  return loadContract(name, address, provider)
}

export interface OptimismContracts {
  Lib_AddressManager: Contract
  OVM_StateCommitmentChain: Contract
  OVM_CanonicalTransactionChain: Contract
  OVM_ExecutionManager: Contract
}

export const loadOptimismContracts = async (
  l1RpcProvider: BaseProvider,
  addressManagerAddress: string,
  signer?: Signer
): Promise<OptimismContracts> => {
  const Lib_AddressManager = loadContract(
    'Lib_AddressManager',
    addressManagerAddress,
    l1RpcProvider
  )

  const inputs = [
    {
      name: 'OVM_StateCommitmentChain',
      interface: 'iOVM_StateCommitmentChain',
    },
    {
      name: 'OVM_CanonicalTransactionChain',
      interface: 'iOVM_CanonicalTransactionChain',
    },
    {
      name: 'OVM_ExecutionManager',
      interface: 'iOVM_ExecutionManager',
    },
  ]

  const contracts = {}
  for (const input of inputs) {
    contracts[input.name] = await loadProxyFromManager(
      input.interface,
      input.name,
      Lib_AddressManager,
      l1RpcProvider
    )

    if (signer) {
      contracts[input.name] = contracts[input.name].connect(signer)
    }
  }

  contracts['Lib_AddressManager'] = Lib_AddressManager

  // TODO: sorry
  return contracts as OptimismContracts
}
