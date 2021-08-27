import { injectL2Context } from '@eth-optimism/core-utils'
import { Wallet, Contract, ContractFactory } from 'ethers'
import { ethers } from 'hardhat'
import chai, { expect } from 'chai'
import {
  sleep,
  l2WebsocketProvider,
  l2Provider,
  IS_LIVE_NETWORK,
} from './shared/utils'
import chaiAsPromised from 'chai-as-promised'
import { OptimismEnv } from './shared/env'
import { solidity } from 'ethereum-waffle'
chai.use(chaiAsPromised)
chai.use(solidity)

describe('Basic Websocket tests', () => {
  let env: OptimismEnv
  let wallet: Wallet
  let ERC20: Contract
  const provider = injectL2Context(l2Provider)
  let other: Wallet

  before(async () => {
    env = await OptimismEnv.new()
    wallet = env.l2Wallet
    const Factory__ERC20 = await ethers.getContractFactory('ERC20', wallet)

    other = Wallet.createRandom().connect(provider)
    ERC20 = await Factory__ERC20.deploy(100, 'OVM Test', 8, 'OVM')
  })

  describe('eth_subscribe', () => {
    it('should subscribe to new blocks', async () => {
      let seen = false
      const height = await provider.getBlockNumber()

      l2WebsocketProvider.once('block', (blockNumber) => {
        seen = true
        expect(blockNumber).to.deep.eq(height + 1)
      })

      const transfer = await ERC20.transfer(other.address, 10)
      transfer.wait()
      while (!seen) {
        await sleep(500)
      }

      expect(seen).to.equal(true)
    })

    it('should filter by topic', async () => {
      let seen = false
      const filter = ERC20.filters.Transfer()

      let event
      l2WebsocketProvider.once(filter, (ev) => {
        seen = true
        event = ev
      })

      const transfer = await ERC20.transfer(other.address, 10)
      const receipt = await transfer.wait()
      while (!seen) {
        await sleep(500)
      }

      expect(seen).to.equal(true)
      expect(event.blockNumber).to.deep.eq(receipt.blockNumber)
      expect(event.blockHash).to.deep.eq(receipt.blockHash)
      expect(event.transactionHash).to.deep.eq(receipt.transactionHash)
    })
  })
})
