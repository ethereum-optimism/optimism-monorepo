import { expect } from '../../../setup'

/* External Imports */
import { ethers } from 'hardhat'
import { ContractFactory, Contract, Signer } from 'ethers'

describe('OVM_GasPriceOracle', () => {
  const initialGasPrice = 0
  let signer1: Signer
  let signer2: Signer
  before(async () => {
    ;[signer1, signer2] = await ethers.getSigners()
  })

  let Factory__OVM_GasPriceOracle: ContractFactory
  before(async () => {
    Factory__OVM_GasPriceOracle = await ethers.getContractFactory(
      'OVM_GasPriceOracle'
    )
  })

  let OVM_GasPriceOracle: Contract
  beforeEach(async () => {
    OVM_GasPriceOracle = await Factory__OVM_GasPriceOracle.deploy(
      await signer1.getAddress(),
      initialGasPrice
    )
  })

  describe('owner', () => {
    it('should have an owner', async () => {
      expect(await OVM_GasPriceOracle.owner()).to.equal(
        await signer1.getAddress()
      )
    })
  })

  describe('setGasPrice', () => {
    it('should revert if called by someone other than the owner', async () => {
      await expect(OVM_GasPriceOracle.connect(signer2).setGasPrice(1234)).to.be
        .reverted
    })

    it('should succeed if called by the owner and is equal to `0`', async () => {
      await expect(OVM_GasPriceOracle.connect(signer1).setGasPrice(0)).to.not.be
        .reverted
    })
  })

  describe('get gasPrice', () => {
    it('should return zero at first', async () => {
      expect(await OVM_GasPriceOracle.getGasPrice()).to.equal(initialGasPrice)
    })

    it('should change when setGasPrice is called', async () => {
      const gasPrice = 1234

      await OVM_GasPriceOracle.connect(signer1).setGasPrice(gasPrice)

      expect(await OVM_GasPriceOracle.getGasPrice()).to.equal(gasPrice)
    })

    it('is the 1st storage slot', async () => {
      const gasPrice = 1234
      const slot = 1

      // set the price
      await OVM_GasPriceOracle.connect(signer1).setGasPrice(gasPrice)

      // get the storage slot value
      const priceAtSlot = await signer1.provider.getStorageAt(
        OVM_GasPriceOracle.address,
        slot
      )
      expect(await OVM_GasPriceOracle.getGasPrice()).to.equal(
        ethers.BigNumber.from(priceAtSlot)
      )
    })
  })
})
