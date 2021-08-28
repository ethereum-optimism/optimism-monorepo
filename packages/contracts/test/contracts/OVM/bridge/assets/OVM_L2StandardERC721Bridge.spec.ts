import { expect } from '../../../../setup'

/* External Imports */
import { ethers } from 'hardhat'
import { Signer, ContractFactory, Contract } from 'ethers'
import { smockit, MockContract } from '@eth-optimism/smock'

/* Internal Imports */
import { NON_NULL_BYTES32, NON_ZERO_ADDRESS } from '../../../../helpers'

const ERR_INVALID_MESSENGER = 'OVM_XCHAIN: messenger contract unauthenticated'
const ERR_INVALID_X_DOMAIN_MSG_SENDER =
  'OVM_XCHAIN: wrong sender of cross-domain message'
const DUMMY_L1BRIDGE_ADDRESS: string =
  '0x1234123412341234123412341234123412341234'
const DUMMY_L1_ERC721_ADDRESS: string = ethers.utils.getAddress(
  '0x' + 'baab'.repeat(10)
)

describe('OVM_L2StandardERC721Bridge', () => {
  let alice: Signer
  let aliceAddress: string
  let bob: Signer
  let bobsAddress: string
  let l2MessengerImpersonator: Signer
  let Factory__OVM_L1StandardERC721Bridge: ContractFactory
  before(async () => {
    // Create a special signer which will enable us to send messages from the L2Messenger contract
    ;[alice, bob, l2MessengerImpersonator] = await ethers.getSigners()
    aliceAddress = await alice.getAddress()
    bobsAddress = await bob.getAddress()
    Factory__OVM_L1StandardERC721Bridge = await ethers.getContractFactory(
      'OVM_L1StandardERC721Bridge'
    )
  })

  let OVM_L2StandardERC721Bridge: Contract
  let L2ERC721: Contract
  let Mock__OVM_L2CrossDomainMessenger: MockContract
  beforeEach(async () => {
    // Get a new mock L2 messenger
    Mock__OVM_L2CrossDomainMessenger = await smockit(
      await ethers.getContractFactory('OVM_L2CrossDomainMessenger'),
      // This allows us to use an ethers override {from: Mock__OVM_L2CrossDomainMessenger.address} to mock calls
      { address: await l2MessengerImpersonator.getAddress() }
    )

    // Deploy the contract under test
    OVM_L2StandardERC721Bridge = await (
      await ethers.getContractFactory('OVM_L2StandardERC721Bridge')
    ).deploy(Mock__OVM_L2CrossDomainMessenger.address, DUMMY_L1BRIDGE_ADDRESS)

    // Deploy an L2 ERC721
    L2ERC721 = await (
      await ethers.getContractFactory('L2StandardERC721', alice)
    ).deploy(
      OVM_L2StandardERC721Bridge.address,
      DUMMY_L1_ERC721_ADDRESS,
      'L2NFT',
      'NFT'
    )
  })

  describe('finalizeERC721Deposit', () => {
    it('onlyFromCrossDomainAccount: should revert on calls from a non-crossDomainMessenger L2 account', async () => {
      await expect(
        OVM_L2StandardERC721Bridge.finalizeERC721Deposit(
          DUMMY_L1_ERC721_ADDRESS,
          NON_ZERO_ADDRESS,
          NON_ZERO_ADDRESS,
          NON_ZERO_ADDRESS,
          0,
          NON_NULL_BYTES32
        )
      ).to.be.revertedWith(ERR_INVALID_MESSENGER)
    })

    it('onlyFromCrossDomainAccount: should revert on calls from the right crossDomainMessenger, but wrong xDomainMessageSender (ie. not the L1L1StandardBridge)', async () => {
      Mock__OVM_L2CrossDomainMessenger.smocked.xDomainMessageSender.will.return.with(
        NON_ZERO_ADDRESS
      )

      await expect(
        OVM_L2StandardERC721Bridge.connect(
          l2MessengerImpersonator
        ).finalizeERC721Deposit(
          DUMMY_L1_ERC721_ADDRESS,
          NON_ZERO_ADDRESS,
          NON_ZERO_ADDRESS,
          NON_ZERO_ADDRESS,
          0,
          NON_NULL_BYTES32,
          {
            from: Mock__OVM_L2CrossDomainMessenger.address,
          }
        )
      ).to.be.revertedWith(ERR_INVALID_X_DOMAIN_MSG_SENDER)
    })

    it('should initialize a withdrawal if the L2 token is not compliant', async () => {
      // Deploy a non compliant ERC721
      const NonCompliantERC721 = await (
        await ethers.getContractFactory(
          'contracts/test-helpers/TestERC721.sol:TestERC721'
        )
      ).deploy()

      OVM_L2StandardERC721Bridge.connect(
        l2MessengerImpersonator
      ).finalizeERC721Deposit(
        DUMMY_L1_ERC721_ADDRESS,
        NON_ZERO_ADDRESS,
        NON_ZERO_ADDRESS,
        NON_ZERO_ADDRESS,
        0,
        NON_NULL_BYTES32,
        {
          from: Mock__OVM_L2CrossDomainMessenger.address,
        }
      )

      Mock__OVM_L2CrossDomainMessenger.smocked.xDomainMessageSender.will.return.with(
        () => DUMMY_L1BRIDGE_ADDRESS
      )

      await OVM_L2StandardERC721Bridge.connect(
        l2MessengerImpersonator
      ).finalizeERC721Deposit(
        DUMMY_L1_ERC721_ADDRESS,
        NonCompliantERC721.address,
        aliceAddress,
        bobsAddress,
        0,
        NON_NULL_BYTES32,
        {
          from: Mock__OVM_L2CrossDomainMessenger.address,
        }
      )

      const withdrawalCallToMessenger =
        Mock__OVM_L2CrossDomainMessenger.smocked.sendMessage.calls[0]

      expect(withdrawalCallToMessenger._target).to.equal(DUMMY_L1BRIDGE_ADDRESS)
      expect(withdrawalCallToMessenger._message).to.equal(
        Factory__OVM_L1StandardERC721Bridge.interface.encodeFunctionData(
          'finalizeERC721Withdrawal',
          [
            DUMMY_L1_ERC721_ADDRESS,
            NonCompliantERC721.address,
            bobsAddress,
            aliceAddress,
            0,
            NON_NULL_BYTES32,
          ]
        )
      )
    })

    it('should credit funds to the depositor', async () => {
      const depositTokenId = 1

      Mock__OVM_L2CrossDomainMessenger.smocked.xDomainMessageSender.will.return.with(
        () => DUMMY_L1BRIDGE_ADDRESS
      )

      await OVM_L2StandardERC721Bridge.connect(
        l2MessengerImpersonator
      ).finalizeERC721Deposit(
        DUMMY_L1_ERC721_ADDRESS,
        L2ERC721.address,
        aliceAddress,
        bobsAddress,
        depositTokenId,
        NON_NULL_BYTES32,
        {
          from: Mock__OVM_L2CrossDomainMessenger.address,
        }
      )

      const bobsBalance = await L2ERC721.balanceOf(bobsAddress)
      expect(bobsBalance).to.equal(1)

      const nftOwner = await L2ERC721.ownerOf(depositTokenId)
      expect(nftOwner).to.equal(bobsAddress)
    })
  })

  describe('ERC721 withdrawals', () => {
    const tokenId = 0
    let L2token: Contract
    beforeEach(async () => {
      // Deploy a smodded gateway so we can give some balances to withdraw
      L2token = await (
        await ethers.getContractFactory('TestL2StandardERC721', alice)
      ).deploy(
        OVM_L2StandardERC721Bridge.address,
        DUMMY_L1_ERC721_ADDRESS,
        'L2NFT',
        'NFT'
      )
      await L2token.mintTestToken(aliceAddress, 0)
    })

    it('withdraw() burns and sends the correct withdrawal message', async () => {
      await OVM_L2StandardERC721Bridge.withdrawERC721(
        L2token.address,
        tokenId,
        0,
        NON_NULL_BYTES32
      )
      const withdrawalCallToMessenger =
        Mock__OVM_L2CrossDomainMessenger.smocked.sendMessage.calls[0]

      const aliceBalance = await L2token.balanceOf(await alice.getAddress())
      expect(aliceBalance).to.deep.equal(0)

      // Assert the correct cross-chain call was sent:
      // Message should be sent to the L1L1StandardBridge on L1
      expect(withdrawalCallToMessenger._target).to.equal(DUMMY_L1BRIDGE_ADDRESS)
      // Message data should be a call telling the L1L1StandardBridge to finalize the withdrawal
      expect(withdrawalCallToMessenger._message).to.equal(
        Factory__OVM_L1StandardERC721Bridge.interface.encodeFunctionData(
          'finalizeERC721Withdrawal',
          [
            DUMMY_L1_ERC721_ADDRESS,
            L2token.address,
            await alice.getAddress(),
            await alice.getAddress(),
            tokenId,
            NON_NULL_BYTES32,
          ]
        )
      )
      // gaslimit should be correct
      expect(withdrawalCallToMessenger._gasLimit).to.equal(0)
    })

    it('withdrawTo() burns and sends the correct withdrawal message', async () => {
      await OVM_L2StandardERC721Bridge.withdrawERC721To(
        L2token.address,
        await bob.getAddress(),
        tokenId,
        0,
        NON_NULL_BYTES32
      )
      const withdrawalCallToMessenger =
        Mock__OVM_L2CrossDomainMessenger.smocked.sendMessage.calls[0]

      // Assert Alice's balance went down
      const aliceBalance = await L2token.balanceOf(await alice.getAddress())
      expect(aliceBalance).to.deep.equal(0)

      // Assert the correct cross-chain call was sent.
      // Message should be sent to the L1L1StandardBridge on L1
      expect(withdrawalCallToMessenger._target).to.equal(DUMMY_L1BRIDGE_ADDRESS)
      // The message data should be a call telling the L1L1StandardBridge to finalize the withdrawal
      expect(withdrawalCallToMessenger._message).to.equal(
        Factory__OVM_L1StandardERC721Bridge.interface.encodeFunctionData(
          'finalizeERC721Withdrawal',
          [
            DUMMY_L1_ERC721_ADDRESS,
            L2token.address,
            await alice.getAddress(),
            await bob.getAddress(),
            tokenId,
            NON_NULL_BYTES32,
          ]
        )
      )
      // gas value is ignored and set to 0.
      expect(withdrawalCallToMessenger._gasLimit).to.equal(0)
    })
  })

  describe('standard erc721', () => {
    it('should not allow anyone but the L2 bridge to mint and burn', async () => {
      expect(L2ERC721.connect(alice).mint(aliceAddress, 1)).to.be.revertedWith(
        'Only L2 Bridge can mint and burn'
      )
      expect(L2ERC721.connect(alice).burn(aliceAddress, 1)).to.be.revertedWith(
        'Only L2 Bridge can mint and burn'
      )
    })

    it('should return the correct interface support', async () => {
      const supportsERC165 = await L2ERC721.supportsInterface(0x01ffc9a7)
      expect(supportsERC165).to.be.true

      const supportsL2ERC721Interface = await L2ERC721.supportsInterface(
        0x1d1d8b63
      )
      expect(supportsL2ERC721Interface).to.be.true

      const badSupports = await L2ERC721.supportsInterface(0xffffffff)
      expect(badSupports).to.be.false
    })
  })
})
