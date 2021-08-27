/* Imports: External */
import { DeployFunction } from 'hardhat-deploy/dist/types'

/* Imports: Internal */
import { getDeployedContract } from '../src/hardhat-deploy-ethers'

const deployFn: DeployFunction = async (hre) => {
  const { deploy } = hre.deployments
  const { deployer } = await hre.getNamedAccounts()

  const Lib_AddressManager = await getDeployedContract(
    hre,
    'Lib_AddressManager',
    {
      signerOrProvider: deployer,
    }
  )

  const result = await deploy('Proxy__OVM_L1CrossDomainMessenger', {
    contract: 'Lib_ResolvedDelegateProxy',
    from: deployer,
    args: [Lib_AddressManager.address, 'OVM_L1CrossDomainMessenger'],
    log: true,
  })

  if (!result.newlyDeployed) {
    return
  }

  const Proxy__OVM_L1CrossDomainMessenger = await getDeployedContract(
    hre,
    'Proxy__OVM_L1CrossDomainMessenger',
    {
      signerOrProvider: deployer,
      iface: 'OVM_L1CrossDomainMessenger',
    }
  )

  await Proxy__OVM_L1CrossDomainMessenger.initialize(Lib_AddressManager.address)

  const libAddressManager =
    await Proxy__OVM_L1CrossDomainMessenger.libAddressManager()
  if (
    libAddressManager.toLowerCase() !== Lib_AddressManager.address.toLowerCase()
  ) {
    throw new Error(
      `\n**FATAL ERROR. THIS SHOULD NEVER HAPPEN. CHECK YOUR DEPLOYMENT.**:\n` +
        `Proxy__OVM_L1CrossDomainMessenger could not be succesfully initialized.\n` +
        `Attempted to set Lib_AddressManager to: ${Lib_AddressManager.address}\n` +
        `Actual address after initialization: ${libAddressManager}\n` +
        `This could indicate a compromised deployment.`
    )
  }

  await Lib_AddressManager.setAddress(
    'Proxy__OVM_L1CrossDomainMessenger',
    result.address
  )
}

deployFn.dependencies = ['Lib_AddressManager', 'OVM_L1CrossDomainMessenger']
deployFn.tags = ['Proxy__OVM_L1CrossDomainMessenger']

export default deployFn
