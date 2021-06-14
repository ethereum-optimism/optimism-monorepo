package rollup

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/crypto/sha3"
)

type RollupEngine struct{}

func NewRollupEngine() *RollupEngine {
	return &RollupEngine{}
}

// Author retrieves the Ethereum address of the account that minted the given
// block, which may be different from the header's coinbase if a consensus
// engine is based on signatures.
func (e *RollupEngine) Author(header *types.Header) (common.Address, error) {
	return header.Coinbase, nil
}

// VerifyHeader checks whether a header conforms to the consensus rules of a
// given engine. Verifying the seal may be done optionally here, or explicitly
// via the VerifySeal method.
func (e *RollupEngine) VerifyHeader(chain consensus.ChainReader, header *types.Header, seal bool) error {
	number := header.Number.Uint64()
	if chain.GetHeader(header.Hash(), number) != nil {
		return nil
	}
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	return nil
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (e *RollupEngine) VerifyHeaders(chain consensus.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	return nil, nil
}

// VerifyUncles verifies that the given block's uncles conform to the consensus
// rules of a given engine.
func (e *RollupEngine) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	return nil
}

// VerifySeal checks whether the crypto seal on a header is valid according to
// the consensus rules of the given engine.
func (e *RollupEngine) VerifySeal(chain consensus.ChainReader, header *types.Header) error {
	return nil
}

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (e *RollupEngine) Prepare(chain consensus.ChainReader, header *types.Header) error {
	parent := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	return nil
}

// Finalize runs any post-transaction state modifications (e.g. block rewards)
// but does not assemble the block.
//
// Note: The block header and state database might be updated to reflect any
// consensus rules that happen at finalization (e.g. block rewards).
func (e *RollupEngine) Finalize(chain consensus.ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header) {
	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
}

// FinalizeAndAssemble runs any post-transaction state modifications (e.g. block
// rewards) and assembles the final block.
//
// Note: The block header and state database might be updated to reflect any
// consensus rules that happen at finalization (e.g. block rewards).
func (e *RollupEngine) FinalizeAndAssemble(chain consensus.ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {
	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
	// Header seems complete, assemble into a block and return
	return types.NewBlock(header, txs, uncles, receipts), nil
}

// Seal generates a new sealing request for the given input block and pushes
// the result into the given channel.
//
// Note, the method returns immediately and will send the result async. More
// than one result may also be returned depending on the consensus algorithm.
func (e *RollupEngine) Seal(chain consensus.ChainReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	return nil
}

// SealHash returns the hash of a block prior to it being sealed.
func (e *RollupEngine) SealHash(header *types.Header) common.Hash {
	hasher := sha3.NewLegacyKeccak256()

	hash := common.Hash{}
	rlp.Encode(hasher, []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.Extra,
	})
	hasher.Sum(hash[:0])
	return hash
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
// that a new block should have.
func (e *RollupEngine) CalcDifficulty(chain consensus.ChainReader, time uint64, parent *types.Header) *big.Int {
	return common.Big1
}

// APIs returns the RPC APIs this consensus engine provides.
func (e *RollupEngine) APIs(chain consensus.ChainReader) []rpc.API {
	return []rpc.API{}
}

// Close terminates any background threads maintained by the consensus engine.
func (e *RollupEngine) Close() error {
	return nil
}
