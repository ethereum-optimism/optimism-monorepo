package l2

import (
	"encoding/binary"
	"fmt"

	"encoding/json"

	"github.com/ethereum-optimism/optimism/op-service/eth"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	preimage "github.com/ethereum-optimism/optimism/op-preimage"
)

const (
	HintL2BlockHeader    = "l2-block-header"
	HintL2Transactions   = "l2-transactions"
	HintL2Receipts       = "l2-receipts"
	HintL2Code           = "l2-code"
	HintL2StateNode      = "l2-state-node"
	HintL2Output         = "l2-output"
	HintL2BlockData      = "l2-block-data"
	HintAgreedPrestate   = "agreed-pre-state"
	HintL2AccountProof   = "l2-account-proof"
	HintL2PayloadWitness = "l2-payload-witness"
)

type LegacyBlockHeaderHint common.Hash

var _ preimage.Hint = LegacyBlockHeaderHint{}

func (l LegacyBlockHeaderHint) Hint() string {
	return HintL2BlockHeader + " " + (common.Hash)(l).String()
}

type HashAndChainID struct {
	Hash    common.Hash
	ChainID eth.ChainID
}

func (h HashAndChainID) Marshal() []byte {
	d := make([]byte, 32+8)
	copy(d[:32], h.Hash[:])
	binary.BigEndian.PutUint64(d[32:], eth.EvilChainIDToUInt64(h.ChainID))
	return d
}

type BlockHeaderHint HashAndChainID

var _ preimage.Hint = BlockHeaderHint{}

func (l BlockHeaderHint) Hint() string {
	return HintL2BlockHeader + " " + hexutil.Encode(HashAndChainID(l).Marshal())
}

type LegacyTransactionsHint common.Hash

var _ preimage.Hint = LegacyTransactionsHint{}

func (l LegacyTransactionsHint) Hint() string {
	return HintL2Transactions + " " + (common.Hash)(l).String()
}

type TransactionsHint HashAndChainID

var _ preimage.Hint = TransactionsHint{}

func (l TransactionsHint) Hint() string {
	return HintL2Transactions + " " + hexutil.Encode(HashAndChainID(l).Marshal())
}

type ReceiptsHint HashAndChainID

var _ preimage.Hint = ReceiptsHint{}

func (l ReceiptsHint) Hint() string {
	return HintL2Receipts + " " + hexutil.Encode(HashAndChainID(l).Marshal())
}

type CodeHint HashAndChainID

var _ preimage.Hint = CodeHint{}

func (l CodeHint) Hint() string {
	return HintL2Code + " " + hexutil.Encode(HashAndChainID(l).Marshal())
}

type LegacyCodeHint common.Hash

var _ preimage.Hint = LegacyCodeHint{}

func (l LegacyCodeHint) Hint() string {
	return HintL2Code + " " + (common.Hash)(l).String()
}

type StateNodeHint HashAndChainID

var _ preimage.Hint = StateNodeHint{}

func (l StateNodeHint) Hint() string {
	return HintL2StateNode + " " + hexutil.Encode(HashAndChainID(l).Marshal())
}

type LegacyStateNodeHint common.Hash

var _ preimage.Hint = LegacyStateNodeHint{}

func (l LegacyStateNodeHint) Hint() string {
	return HintL2StateNode + " " + (common.Hash)(l).String()
}

type L2OutputHint HashAndChainID

var _ preimage.Hint = L2OutputHint{}

func (l L2OutputHint) Hint() string {
	return HintL2Output + " " + hexutil.Encode(HashAndChainID(l).Marshal())
}

type LegacyL2OutputHint common.Hash

var _ preimage.Hint = LegacyL2OutputHint{}

func (l LegacyL2OutputHint) Hint() string {
	return HintL2Output + " " + (common.Hash)(l).String()
}

type L2BlockDataHint struct {
	AgreedBlockHash common.Hash
	BlockHash       common.Hash
	ChainID         eth.ChainID
}

var _ preimage.Hint = L2BlockDataHint{}

func (l L2BlockDataHint) Hint() string {
	hintBytes := make([]byte, 32+32+8)
	copy(hintBytes[:32], (common.Hash)(l.AgreedBlockHash).Bytes())
	copy(hintBytes[32:64], (common.Hash)(l.BlockHash).Bytes())
	binary.BigEndian.PutUint64(hintBytes[64:], eth.EvilChainIDToUInt64(l.ChainID))
	return fmt.Sprintf("%s 0x%s", HintL2BlockData, common.Bytes2Hex(hintBytes))
}

type AgreedPrestateHint common.Hash

var _ preimage.Hint = AgreedPrestateHint{}

func (l AgreedPrestateHint) Hint() string {
	return HintAgreedPrestate + " " + (common.Hash)(l).String()
}

type AccountProofHint struct {
	BlockHash common.Hash
	Address   common.Address
	ChainID   eth.ChainID
}

var _ preimage.Hint = AccountProofHint{}

func (l AccountProofHint) Hint() string {
	hintBytes := make([]byte, 32+20+8)
	copy(hintBytes[:32], l.BlockHash.Bytes())
	copy(hintBytes[32:52], l.Address.Bytes())
	binary.BigEndian.PutUint64(hintBytes[52:], eth.EvilChainIDToUInt64(l.ChainID))

	return HintL2AccountProof + " " + hexutil.Encode(hintBytes)
}

type PayloadWitnessHint struct {
	ParentBlockHash   common.Hash            `json:"parentBlockHash"`
	PayloadAttributes *eth.PayloadAttributes `json:"payloadAttributes"`
	ChainID           *eth.ChainID           `json:"chainID,omitempty"`
}

var _ preimage.Hint = PayloadWitnessHint{}

func (l PayloadWitnessHint) Hint() string {
	marshaled, err := json.Marshal(l)
	if err != nil {
		// should only happen if the struct is misconfigured
		panic(err)
	}

	return HintL2PayloadWitness + " " + hexutil.Encode(marshaled)
}
