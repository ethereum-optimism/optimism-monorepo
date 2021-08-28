import { expect } from '../../../../setup'

/* External Imports */
import { ethers } from 'hardhat'
import { Signer, ContractFactory, Contract } from 'ethers'
import { smoddit } from '@eth-optimism/smock'

/* Internal Imports */
import { predeploys, getContractInterface } from '../../../../../src'

describe('OVM_L2StandardTokenFactory', () => {
  let signer: Signer
  let Factory__L1ERC20: ContractFactory
  let L1ERC20: Contract
  let OVM_L2StandardTokenFactory: Contract
  before(async () => {
    ;[signer] = await ethers.getSigners()
    // deploy an ERC20 contract on L1
    Factory__L1ERC20 = await smoddit(
      '@openzeppelin/contracts/token/ERC20/ERC20.sol:ERC20'
    )
    L1ERC20 = await Factory__L1ERC20.deploy('L1ERC20', 'ERC')

    OVM_L2StandardTokenFactory = await (
      await ethers.getContractFactory('OVM_L2StandardTokenFactory')
    ).deploy()
  })

  describe('Standard token factory', () => {
    it('should be able to create a standard token', async () => {
      const tx = await OVM_L2StandardTokenFactory.createStandardL2Token(
        L1ERC20.address,
        'L2ERC20',
        'ERC'
      )
      const receipt = await tx.wait()
      const [tokenCreatedEvent] = receipt.events

      // Expect there to be an event emitted for the standard token creation
      expect(tokenCreatedEvent.event).to.be.eq('StandardL2TokenCreated')

      // Get the L2 token address from the emitted event and check it was created correctly
      const l2TokenAddress = tokenCreatedEvent.args._l2Token
      const l2Token = new Contract(
        l2TokenAddress,
        getContractInterface('L2StandardERC20'),
        signer
      )

      expect(await l2Token.l2Bridge()).to.equal(predeploys.OVM_L2StandardBridge)
      expect(await l2Token.l1Token()).to.equal(L1ERC20.address)
      expect(await l2Token.name()).to.equal('L2ERC20')
      expect(await l2Token.symbol()).to.equal('ERC')
    })

    it('should not be able to create a standard token with a 0 address for l1 token', async () => {
      await expect(
        OVM_L2StandardTokenFactory.createStandardL2Token(
          ethers.constants.AddressZero,
          'L2ERC20',
          'ERC'
        )
      ).to.be.revertedWith('Must provide L1 token address')
    })
  })
})
