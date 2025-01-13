package l2

import (
	"encoding/binary"
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	preimage "github.com/ethereum-optimism/optimism/op-preimage"
)

const (
	HintL2BlockHeader  = "l2-block-header"
	HintL2Transactions = "l2-transactions"
	HintL2Code         = "l2-code"
	HintL2StateNode    = "l2-state-node"
	HintL2Output       = "l2-output"
	HintL2BlockData    = "l2-block-data"
	HintAgreedPrestate = "agreed-pre-state"
)

type BlockHeaderHint common.Hash

var _ preimage.Hint = BlockHeaderHint{}

func (l BlockHeaderHint) Hint() string {
	return HintL2BlockHeader + " " + (common.Hash)(l).String()
}

type TransactionsHint common.Hash

var _ preimage.Hint = TransactionsHint{}

func (l TransactionsHint) Hint() string {
	return HintL2Transactions + " " + (common.Hash)(l).String()
}

type CodeHint common.Hash

var _ preimage.Hint = CodeHint{}

func (l CodeHint) Hint() string {
	return HintL2Code + " " + (common.Hash)(l).String()
}

type StateNodeHint common.Hash

var _ preimage.Hint = StateNodeHint{}

func (l StateNodeHint) Hint() string {
	return HintL2StateNode + " " + (common.Hash)(l).String()
}

type L2OutputHint common.Hash

var _ preimage.Hint = L2OutputHint{}

func (l L2OutputHint) Hint() string {
	return HintL2Output + " " + (common.Hash)(l).String()
}

type L2BlockDataHint struct {
	AgreedBlockHash common.Hash
	BlockHash       common.Hash
	ChainID         uint64
}

var _ preimage.Hint = L2BlockDataHint{}

func (l L2BlockDataHint) Hint() string {
	hintBytes := make([]byte, 32+32+8)
	copy(hintBytes[:32], (common.Hash)(l.AgreedBlockHash).Bytes())
	copy(hintBytes[32:64], (common.Hash)(l.BlockHash).Bytes())
	binary.BigEndian.PutUint64(hintBytes[64:], l.ChainID)
	return fmt.Sprintf("%s 0x%s", HintL2BlockData, common.Bytes2Hex(hintBytes))
}

type AgreedPrestateHint common.Hash

var _ preimage.Hint = AgreedPrestateHint{}

func (l AgreedPrestateHint) Hint() string {
	return HintAgreedPrestate + " " + (common.Hash)(l).String()
}
