package signer

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// BlockPayloadArgs represents the arguments to sign a new block payload from the sequencer.
// This is maintained until the V1 signing API in the op-signer server is no longer served.
type BlockPayloadArgs struct {
	Domain        [32]byte        `json:"domain"`
	ChainID       *big.Int        `json:"chainId"`
	PayloadHash   []byte          `json:"payloadHash"`
	SenderAddress *common.Address `json:"senderAddress"`

	// note: older versions of this included a `PayloadBytes` value,
	// not JSON-named, but JSON-encoded anyway.
	// Since this was unused, it is no longer included.
}

// NewBlockPayloadArgs creates a BlockPayloadArgs struct
func NewBlockPayloadArgs(domain [32]byte, chainId *big.Int, payloadBytes []byte, senderAddress *common.Address) *BlockPayloadArgs {
	payloadHash := PayloadHash(payloadBytes)
	args := &BlockPayloadArgs{
		Domain:        domain,
		ChainID:       chainId,
		PayloadHash:   payloadHash[:],
		SenderAddress: senderAddress,
	}
	return args
}

// Check checks that the attributes are set and conform to type assumptions.
func (args *BlockPayloadArgs) Check() error {
	if args.ChainID == nil {
		return errors.New("chainId not specified")
	}
	if args.ChainID.BitLen() > 256 {
		return errors.New("chain_id is too large")
	}
	if len(args.PayloadHash) == 0 {
		return errors.New("payloadHash not specified")
	}
	if len(args.PayloadHash) != 32 {
		return errors.New("payloadHash has unexpected length")
	}
	return nil
}

func (args *BlockPayloadArgs) Message() (*BlockSigningMessage, error) {
	if err := args.Check(); err != nil {
		return nil, err
	}
	return &BlockSigningMessage{
		Domain:      args.Domain,
		ChainID:     eth.ChainIDFromBig(args.ChainID),
		PayloadHash: common.BytesToHash(args.PayloadHash),
	}, nil
}

// BlockPayloadArgsV2 represents the arguments to sign a new block payload from the sequencer.
// This replaces BlockPayloadArgs, to fix JSON encoding.
type BlockPayloadArgsV2 struct {
	Domain      eth.Bytes32 `json:"domain"`
	ChainID     eth.ChainID `json:"chainId"`
	PayloadHash common.Hash `json:"payloadHash"`

	// SenderAddress is optional, it helps determine which account to sign with,
	// if multiple accounts are available.
	SenderAddress *common.Address `json:"senderAddress"`
}

// PayloadHash computes the hash of the payload, an attribute of the signing message.
// The payload-hash is NOT the signing-hash.
func PayloadHash(payload []byte) common.Hash {
	return crypto.Keccak256Hash(payload)
}

func (args *BlockPayloadArgsV2) Message() (*BlockSigningMessage, error) {
	// A zero Domain is valid, and thus not checked.
	if args.ChainID == (eth.ChainID{}) {
		return nil, errors.New("chainId not specified")
	}
	if args.PayloadHash == (common.Hash{}) {
		return nil, errors.New("payloadHash not specified")
	}
	return &BlockSigningMessage{
		Domain:      args.Domain,
		ChainID:     args.ChainID,
		PayloadHash: args.PayloadHash,
	}, nil
}

// BlockSigningMessage is the message representing a block. For signing-message construction.
// NOT FOR API USAGE. See BlockPayloadArgsV2 instead.
type BlockSigningMessage struct {
	Domain      eth.Bytes32
	ChainID     eth.ChainID
	PayloadHash common.Hash
}

// ToSigningHash creates a signingHash from the block payload args.
// Uses the hashing scheme from https://github.com/ethereum-optimism/specs/blob/main/specs/protocol/rollup-node-p2p.md#block-signatures
func (msg *BlockSigningMessage) ToSigningHash() common.Hash {
	var msgInput [32 + 32 + 32]byte
	// domain: first 32 bytes
	copy(msgInput[:32], msg.Domain[:])
	// chain_id: second 32 bytes
	chainID := msg.ChainID.Bytes32()
	copy(msgInput[32:64], chainID[:])
	// payload_hash: third 32 bytes, hash of encoded payload
	copy(msgInput[64:], msg.PayloadHash[:])

	return crypto.Keccak256Hash(msgInput[:])
}
