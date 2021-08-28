import { expect } from '../../../../setup'

/* External Imports */
import { ethers } from 'hardhat'
import { Signer, ContractFactory, Contract } from 'ethers'
import { smoddit } from '@eth-optimism/smock'

/* Internal Imports */
import { getContractInterface, predeploys } from '../../../../../src'

describe('OVM_L2StandardERC721Factory', () => {
  let signer: Signer
  let Factory__L1ERC721: ContractFactory
  let L1ERC721: Contract
  let OVM_L2StandardERC721Factory: Contract
  before(async () => {
    ;[signer] = await ethers.getSigners()
    // deploy an ERC20 contract on L1
    Factory__L1ERC721 = await smoddit(
      '@openzeppelin/contracts/token/ERC721/ERC721.sol:ERC721'
    )
    L1ERC721 = await Factory__L1ERC721.deploy('L1ERC721', 'NFT')

    OVM_L2StandardERC721Factory = await (
      await ethers.getContractFactory('OVM_L2StandardERC721Factory')
    ).deploy()
  })

  describe('Standard ERC721 token factory', () => {
    it('should be able to create a standard ERC721 token', async () => {
      const tx = await OVM_L2StandardERC721Factory.createStandardL2ERC721(
        L1ERC721.address,
        'L2ERC721',
        'NFT'
      )
      const receipt = await tx.wait()
      const [tokenCreatedEvent] = receipt.events

      // Expect there to be an event emitted for the standard token creation
      expect(tokenCreatedEvent.event).to.be.eq('StandardL2ERC721Created')

      // Get the L2 token address from the emitted event and check it was created correctly
      const l2TokenAddress = tokenCreatedEvent.args._l2Token
      const l2Token = new Contract(
        l2TokenAddress,
        getContractInterface('L2StandardERC721'),
        signer
      )

      expect(await l2Token.l2Bridge()).to.equal(predeploys.OVM_L2StandardBridge)
      expect(await l2Token.l1Token()).to.equal(L1ERC721.address)
      expect(await l2Token.name()).to.equal('L2ERC721')
      expect(await l2Token.symbol()).to.equal('NFT')
    })

    it('should not be able to create a standard token with a 0 address for l1 token', async () => {
      await expect(
        OVM_L2StandardERC721Factory.createStandardL2ERC721(
          ethers.constants.AddressZero,
          'L2ERC721',
          'NFT'
        )
      ).to.be.revertedWith('Must provide L1 token address')
    })
  })
})
