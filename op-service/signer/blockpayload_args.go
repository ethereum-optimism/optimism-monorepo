package signer

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// BlockPayloadArgs represents the arguments to sign a new block payload from the sequencer.
type BlockPayloadArgs struct {
	Domain        [32]byte `json:"domain"`
	ChainID       *big.Int `json:"chainId"`
	PayloadHash   []byte   `json:"payloadHash"`
	PayloadBytes  []byte
	SenderAddress *common.Address `json:"senderAddress"`
}

// NewBlockPayloadArgs creates a BlockPayloadArgs struct
func NewBlockPayloadArgs(domain [32]byte, chainId *big.Int, payloadBytes []byte, senderAddress *common.Address) *BlockPayloadArgs {
	payloadHash := crypto.Keccak256(payloadBytes)
	args := &BlockPayloadArgs{
		Domain:        domain,
		ChainID:       chainId,
		PayloadHash:   payloadHash,
		PayloadBytes:  payloadBytes,
		SenderAddress: senderAddress,
	}
	return args
}

func (args *BlockPayloadArgs) Check() error {
	if args.ChainID == nil {
		return errors.New("chainId not specified")
	}
	if len(args.PayloadHash) == 0 {
		return errors.New("payloadHash not specified")
	}
	return nil
}

// ToSigningHash creates a signingHash from the block payload args.
// Uses the hashing scheme from https://github.com/ethereum-optimism/specs/blob/main/specs/protocol/rollup-node-p2p.md#block-signatures
func (args *BlockPayloadArgs) ToSigningHash() (common.Hash, error) {
	if err := args.Check(); err != nil {
		return common.Hash{}, err
	}
	var msgInput [32 + 32 + 32]byte
	// domain: first 32 bytes
	copy(msgInput[:32], args.Domain[:])
	// chain_id: second 32 bytes
	if args.ChainID.BitLen() > 256 {
		return common.Hash{}, errors.New("chain_id is too large")
	}
	args.ChainID.FillBytes(msgInput[32:64])

	// payload_hash: third 32 bytes, hash of encoded payload
	copy(msgInput[64:], args.PayloadHash[:])

	return crypto.Keccak256Hash(msgInput[:]), nil
}
