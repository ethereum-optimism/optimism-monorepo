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

  const result = await deploy('OVM_L1CrossDomainMessenger', {
    from: deployer,
    args: [],
    log: true,
  })

  if (!result.newlyDeployed) {
    return
  }

  const OVM_L1CrossDomainMessenger = await getDeployedContract(
    hre,
    'OVM_L1CrossDomainMessenger',
    {
      signerOrProvider: deployer,
    }
  )

  // NOTE: this initialization is *not* technically required (we only need to initialize the proxy)
  // but it feels safer to initialize this anyway. Otherwise someone else could come along and
  // initialize this.
  await OVM_L1CrossDomainMessenger.initialize(Lib_AddressManager.address)

  const libAddressManager = await OVM_L1CrossDomainMessenger.libAddressManager()
  if (
    libAddressManager.toLowerCase() !== Lib_AddressManager.address.toLowerCase()
  ) {
    throw new Error(
      `\n**FATAL ERROR. THIS SHOULD NEVER HAPPEN. CHECK YOUR DEPLOYMENT.**:\n` +
        `OVM_L1CrossDomainMessenger could not be succesfully initialized.\n` +
        `Attempted to set Lib_AddressManager to: ${Lib_AddressManager.address}\n` +
        `Actual address after initialization: ${libAddressManager}\n` +
        `This could indicate a compromised deployment.`
    )
  }

  await Lib_AddressManager.setAddress(
    'OVM_L1CrossDomainMessenger',
    result.address
  )
}

deployFn.dependencies = ['Lib_AddressManager']
deployFn.tags = ['OVM_L1CrossDomainMessenger']

export default deployFn
