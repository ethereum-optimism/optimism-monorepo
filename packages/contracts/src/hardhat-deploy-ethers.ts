/* Imports: External */
import { ethers, Contract } from 'ethers'
import { Provider } from '@ethersproject/abstract-provider'
import { Signer } from '@ethersproject/abstract-signer'
import { sleep } from '@eth-optimism/core-utils'

export const registerAddress = async ({
  hre,
  name,
  address,
}): Promise<void> => {
  // TODO: Cache these 2 across calls?
  const { deployer } = await hre.getNamedAccounts()
  const Lib_AddressManager = await getDeployedContract(
    hre,
    'Lib_AddressManager',
    {
      signerOrProvider: deployer,
    }
  )

  const currentAddress = await Lib_AddressManager.getAddress(name)
  if (address.toLowerCase() === currentAddress.toLowerCase()) {
    console.log(
      `✓ Not registering address for ${name} because it's already been correctly registered`
    )
    return
  }

  console.log(`Registering address for ${name} to ${address}...`)
  const tx = await Lib_AddressManager.setAddress(name, address)
  await tx.wait()

  let remoteAddress = await Lib_AddressManager.getAddress(name)
  let retries = 10
  while (remoteAddress === ethers.constants.AddressZero) {
    await sleep(5000)
    remoteAddress = await Lib_AddressManager.getAddress(name)
    retries--
    if (retries === 0) {
      throw new Error(`Address for ${name} is still zero after 10 retries`)
    }
  }

  if (remoteAddress.toLowerCase() !== address.toLowerCase()) {
    throw new Error(
      `\n**FATAL ERROR. THIS SHOULD NEVER HAPPEN. CHECK YOUR DEPLOYMENT.**:\n` +
        `Call to Lib_AddressManager.setAddress(${name}) was unsuccessful.\n` +
        `Attempted to set address to: ${address}\n` +
        `Actual address was set to: ${remoteAddress}\n` +
        `This could indicate a compromised deployment.`
    )
  }

  console.log(`✓ Registered address for ${name}`)
}

export const deployAndRegister = async ({
  hre,
  name,
  args,
  contract,
}: {
  hre: any
  name: string
  args: any[]
  contract?: string
}) => {
  const { deploy } = hre.deployments
  const { deployer } = await hre.getNamedAccounts()

  const result = await deploy(name, {
    contract,
    from: deployer,
    args,
    log: true,
  })

  await hre.ethers.provider.waitForTransaction(result.transactionHash)

  if (result.newlyDeployed) {
    await registerAddress({
      hre,
      name,
      address: result.address,
    })
  }
}

export const getDeployedContract = async (
  hre: any,
  name: string,
  options: {
    iface?: string
    signerOrProvider?: Signer | Provider | string
  } = {}
): Promise<Contract> => {
  const deployed = await hre.deployments.get(name)

  await hre.ethers.provider.waitForTransaction(deployed.receipt.transactionHash)

  // Get the correct interface.
  let iface = new hre.ethers.utils.Interface(deployed.abi)
  if (options.iface) {
    const factory = await hre.ethers.getContractFactory(options.iface)
    iface = factory.interface
  }

  let signerOrProvider: Signer | Provider = hre.ethers.provider
  if (options.signerOrProvider) {
    if (typeof options.signerOrProvider === 'string') {
      signerOrProvider = hre.ethers.provider.getSigner(options.signerOrProvider)
    } else {
      signerOrProvider = options.signerOrProvider
    }
  }

  // Temporarily override Object.defineProperty to bypass ether's object protection.
  const def = Object.defineProperty
  Object.defineProperty = (obj, propName, prop) => {
    prop.writable = true
    return def(obj, propName, prop)
  }

  const contract = new Contract(deployed.address, iface, signerOrProvider)

  // Now reset Object.defineProperty
  Object.defineProperty = def

  // Override each function call to also `.wait()` so as to simplify the deploy scripts' syntax.
  for (const fnName of Object.keys(contract.functions)) {
    const fn = contract[fnName].bind(contract)
    ;(contract as any)[fnName] = async (...args: any) => {
      const result = await fn(...args)
      if (typeof result === 'object' && typeof result.wait === 'function') {
        await result.wait()
      }
      return result
    }
  }

  return contract
}
