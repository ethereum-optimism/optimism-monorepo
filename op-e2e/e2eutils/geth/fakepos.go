package geth

import (
	"encoding/binary"
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/clock"
	opeth "github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

type Beacon interface {
	StoreBlobsBundle(slot uint64, bundle *engine.BlobsBundleV1) error
}

// fakePoS is a testing-only utility to attach to Geth,
// to build a fake proof-of-stake L1 chain with fixed block time and basic lagging safe/finalized blocks.
type fakePoS struct {
	clock     clock.Clock
	eth       *eth.Ethereum
	log       log.Logger
	blockTime uint64

	withdrawalsIndex uint64

	finalizedDistance uint64
	safeDistance      uint64

	engineAPI *catalyst.ConsensusAPI
	sub       ethereum.Subscription

	beacon Beacon
}

func (f *fakePoS) FakeBeaconBlockRoot(time uint64) common.Hash {
	var dat [8]byte
	binary.LittleEndian.PutUint64(dat[:], time)
	return crypto.Keccak256Hash(dat[:])
}

func (f *fakePoS) Start() error {
	if advancing, ok := f.clock.(*clock.AdvancingClock); ok {
		advancing.Start()
	}
	withdrawalsRNG := rand.New(rand.NewSource(450368975843)) // avoid generating the same address as any test
	f.sub = event.NewSubscription(func(quit <-chan struct{}) error {
		// poll every half a second: enough to catch up with any block time when ticks are missed
		t := f.clock.NewTicker(time.Second / 2)
		for {
			select {
			case now := <-t.Ch():
				chain := f.eth.BlockChain()
				head := chain.CurrentBlock()
				finalized := chain.CurrentFinalBlock()
				if finalized == nil { // fallback to genesis if nothing is finalized
					finalized = chain.Genesis().Header()
				}
				safe := chain.CurrentSafeBlock()
				if safe == nil { // fallback to finalized if nothing is safe
					safe = finalized
				}
				if head.Number.Uint64() > f.finalizedDistance { // progress finalized block, if we can
					finalized = f.eth.BlockChain().GetHeaderByNumber(head.Number.Uint64() - f.finalizedDistance)
				}
				if head.Number.Uint64() > f.safeDistance { // progress safe block, if we can
					safe = f.eth.BlockChain().GetHeaderByNumber(head.Number.Uint64() - f.safeDistance)
				}
				// start building the block as soon as we are past the current head time
				if head.Time >= uint64(now.Unix()) {
					continue
				}
				newBlockTime := head.Time + f.blockTime
				if time.Unix(int64(newBlockTime), 0).Add(5 * time.Minute).Before(f.clock.Now()) {
					// We're a long way behind, let's skip some blocks...
					newBlockTime = uint64(f.clock.Now().Unix())
				}
				// create some random withdrawals
				withdrawals := make([]*types.Withdrawal, withdrawalsRNG.Intn(4))
				for i := 0; i < len(withdrawals); i++ {
					withdrawals[i] = &types.Withdrawal{
						Index:     f.withdrawalsIndex + uint64(i),
						Validator: withdrawalsRNG.Uint64() % 100_000_000, // 100 million fake validators
						Address:   testutils.RandomAddress(withdrawalsRNG),
						// in gwei, consensus-layer quirk. withdraw non-zero value up to 50 ETH
						Amount: uint64(withdrawalsRNG.Intn(50_000_000_000) + 1),
					}
				}
				attrs := &engine.PayloadAttributes{
					Timestamp:             newBlockTime,
					Random:                common.Hash{},
					SuggestedFeeRecipient: head.Coinbase,
					Withdrawals:           withdrawals,
				}
				parentBeaconBlockRoot := f.FakeBeaconBlockRoot(head.Time) // parent beacon block root
				isCancun := f.eth.BlockChain().Config().IsCancun(new(big.Int).SetUint64(head.Number.Uint64()+1), newBlockTime)
				isPrague := f.eth.BlockChain().Config().IsPrague(new(big.Int).SetUint64(head.Number.Uint64()+1), newBlockTime)
				if isCancun {
					attrs.BeaconRoot = &parentBeaconBlockRoot
				}
				fcState := engine.ForkchoiceStateV1{
					HeadBlockHash:      head.Hash(),
					SafeBlockHash:      safe.Hash(),
					FinalizedBlockHash: finalized.Hash(),
				}
				var err error
				var res engine.ForkChoiceResponse
				if isCancun {
					res, err = f.engineAPI.ForkchoiceUpdatedV3(fcState, attrs)
				} else {
					res, err = f.engineAPI.ForkchoiceUpdatedV2(fcState, attrs)
				}
				if err != nil {
					f.log.Error("failed to start building L1 block", "err", err)
					continue
				}
				if res.PayloadID == nil {
					f.log.Error("failed to start block building", "res", res)
					continue
				}
				// wait with sealing, if we are not behind already
				delay := time.Unix(int64(newBlockTime), 0).Sub(f.clock.Now())
				tim := f.clock.NewTimer(delay)
				select {
				case <-tim.Ch():
					// no-op
				case <-quit:
					tim.Stop()
					return nil
				}
				var envelope *engine.ExecutionPayloadEnvelope
				if isPrague {
					envelope, err = f.engineAPI.GetPayloadV4(*res.PayloadID)
				} else if isCancun {
					envelope, err = f.engineAPI.GetPayloadV3(*res.PayloadID)
				} else {
					envelope, err = f.engineAPI.GetPayloadV2(*res.PayloadID)
				}
				if err != nil {
					f.log.Error("failed to finish building L1 block", "err", err)
					continue
				}

				blobHashes := make([]common.Hash, 0) // must be non-nil even when empty, due to geth engine API checks
				if envelope.BlobsBundle != nil {
					for _, commitment := range envelope.BlobsBundle.Commitments {
						if len(commitment) != 48 {
							f.log.Error("got malformed kzg commitment from engine", "commitment", commitment)
							break
						}
						blobHashes = append(blobHashes, opeth.KZGToVersionedHash(*(*[48]byte)(commitment)))
					}
					if len(blobHashes) != len(envelope.BlobsBundle.Commitments) {
						f.log.Error("invalid or incomplete blob data", "collected", len(blobHashes), "engine", len(envelope.BlobsBundle.Commitments))
						continue
					}
				}

				if isPrague {
					_, err = f.engineAPI.NewPayloadV4(*envelope.ExecutionPayload, blobHashes, &parentBeaconBlockRoot, make([]hexutil.Bytes, 0))
				} else if isCancun {
					_, err = f.engineAPI.NewPayloadV3(*envelope.ExecutionPayload, blobHashes, &parentBeaconBlockRoot)
				} else {
					_, err = f.engineAPI.NewPayloadV2(*envelope.ExecutionPayload)
				}
				if err != nil {
					f.log.Error("failed to insert built L1 block", "err", err)
					continue
				}

				if envelope.BlobsBundle != nil {
					slot := (envelope.ExecutionPayload.Timestamp - f.eth.BlockChain().Genesis().Time()) / f.blockTime
					if f.beacon == nil {
						f.log.Error("no blobs storage available")
						continue
					}
					if err := f.beacon.StoreBlobsBundle(slot, envelope.BlobsBundle); err != nil {
						f.log.Error("failed to persist blobs-bundle of block, not making block canonical now", "err", err)
						continue
					}
				}
				if _, err := f.engineAPI.ForkchoiceUpdatedV3(engine.ForkchoiceStateV1{
					HeadBlockHash:      envelope.ExecutionPayload.BlockHash,
					SafeBlockHash:      safe.Hash(),
					FinalizedBlockHash: finalized.Hash(),
				}, nil); err != nil {
					f.log.Error("failed to make built L1 block canonical", "err", err)
					continue
				}
				// Increment global withdrawals index in the CL.
				// The EL doesn't really care about the value,
				// but it's nice to mock something consistent with the CL specs.
				f.withdrawalsIndex += uint64(len(withdrawals))
			case <-quit:
				return nil
			}
		}
	})
	return nil
}

func (f *fakePoS) Stop() error {
	f.sub.Unsubscribe()
	if advancing, ok := f.clock.(*clock.AdvancingClock); ok {
		advancing.Stop()
	}
	return nil
}
