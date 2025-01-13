package l2

import (
	"fmt"

	interopTypes "github.com/ethereum-optimism/optimism/op-program/client/interop/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"

	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum-optimism/optimism/op-program/client/mpt"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// StateOracle defines the high-level API used to retrieve L2 state data pre-images
// The returned data is always the preimage of the requested hash.
type StateOracle interface {
	// NodeByHash retrieves the merkle-patricia trie node pre-image for a given hash.
	// Trie nodes may be from the world state trie or any account storage trie.
	// Contract code is not stored as part of the trie and must be retrieved via CodeByHash
	NodeByHash(nodeHash common.Hash) []byte

	// CodeByHash retrieves the contract code pre-image for a given hash.
	// codeHash should be retrieved from the world state account for a contract.
	CodeByHash(codeHash common.Hash) []byte
}

// Oracle defines the high-level API used to retrieve L2 data.
// The returned data is always the preimage of the requested hash.
type Oracle interface {
	StateOracle

	// BlockByHash retrieves the block with the given hash.
	BlockByHash(blockHash common.Hash) *types.Block

	OutputByRoot(root common.Hash) eth.Output

	// BlockDataByHash retrieves the block, including all data used to construct it.
	BlockDataByHash(agreedBlockHash, blockHash common.Hash, chainID uint64) *types.Block

	TransitionStateByRoot(root common.Hash) *interopTypes.TransitionState
}

// PreimageOracle implements Oracle using by interfacing with the pure preimage.Oracle
// to fetch pre-images to decode into the requested data.
type PreimageOracle struct {
	oracle preimage.Oracle
	hint   preimage.Hinter
}

var _ Oracle = (*PreimageOracle)(nil)

func NewPreimageOracle(raw preimage.Oracle, hint preimage.Hinter) *PreimageOracle {
	return &PreimageOracle{
		oracle: raw,
		hint:   hint,
	}
}

func (p *PreimageOracle) headerByBlockHash(blockHash common.Hash) *types.Header {
	p.hint.Hint(BlockHeaderHint(blockHash))
	headerRlp := p.oracle.Get(preimage.Keccak256Key(blockHash))
	var header types.Header
	if err := rlp.DecodeBytes(headerRlp, &header); err != nil {
		panic(fmt.Errorf("invalid block header %s: %w", blockHash, err))
	}
	return &header
}

func (p *PreimageOracle) BlockByHash(blockHash common.Hash) *types.Block {
	header := p.headerByBlockHash(blockHash)
	txs := p.LoadTransactions(blockHash, header.TxHash)

	return types.NewBlockWithHeader(header).WithBody(types.Body{Transactions: txs})
}

func (p *PreimageOracle) LoadTransactions(blockHash common.Hash, txHash common.Hash) []*types.Transaction {
	p.hint.Hint(TransactionsHint(blockHash))

	opaqueTxs := mpt.ReadTrie(txHash, func(key common.Hash) []byte {
		return p.oracle.Get(preimage.Keccak256Key(key))
	})

	txs, err := eth.DecodeTransactions(opaqueTxs)
	if err != nil {
		panic(fmt.Errorf("failed to decode list of txs: %w", err))
	}
	return txs
}

func (p *PreimageOracle) NodeByHash(nodeHash common.Hash) []byte {
	p.hint.Hint(StateNodeHint(nodeHash))
	return p.oracle.Get(preimage.Keccak256Key(nodeHash))
}

func (p *PreimageOracle) CodeByHash(codeHash common.Hash) []byte {
	p.hint.Hint(CodeHint(codeHash))
	return p.oracle.Get(preimage.Keccak256Key(codeHash))
}

func (p *PreimageOracle) OutputByRoot(l2OutputRoot common.Hash) eth.Output {
	p.hint.Hint(L2OutputHint(l2OutputRoot))
	data := p.oracle.Get(preimage.Keccak256Key(l2OutputRoot))
	output, err := eth.UnmarshalOutput(data)
	if err != nil {
		panic(fmt.Errorf("invalid L2 output data for root %s: %w", l2OutputRoot, err))
	}
	return output
}

func (p *PreimageOracle) BlockDataByHash(agreedBlockHash, blockHash common.Hash, chainID uint64) *types.Block {
	hint := L2BlockDataHint{
		AgreedBlockHash: agreedBlockHash,
		BlockHash:       blockHash,
		ChainID:         chainID,
	}
	p.hint.Hint(hint)
	header := p.headerByBlockHash(blockHash)
	txs := p.LoadTransactions(blockHash, header.TxHash)
	return types.NewBlockWithHeader(header).WithBody(types.Body{Transactions: txs})
}

func (p *PreimageOracle) TransitionStateByRoot(root common.Hash) *interopTypes.TransitionState {
	p.hint.Hint(AgreedPrestateHint(root))
	data := p.oracle.Get(preimage.Keccak256Key(root))
	output, err := interopTypes.UnmarshalTransitionState(data)
	if err != nil {
		panic(fmt.Errorf("invalid agreed prestate data for root %s: %w", root, err))
	}
	return output
}
