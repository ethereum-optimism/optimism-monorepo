package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-chain-ops/crossdomain"
	"github.com/ethereum-optimism/optimism/op-node/bindings"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type OutputRootWithChainId struct {
	ChainId *big.Int
	Root    common.Hash
}

// Define a proper type for SuperRootProof
type SuperRootProof struct {
	Version     uint8
	Timestamp   uint64
	OutputRoots []OutputRootWithChainId
}

var UnknownNonceVersion = errors.New("Unknown nonce version")

// checkOk checks if ok is false, and panics if so.
// Shorthand to ease go's god awful error handling
func checkOk(ok bool) {
	if !ok {
		panic(fmt.Errorf("checkOk failed"))
	}
}

// checkErr checks if err is not nil, and throws if so.
// Shorthand to ease go's god awful error handling
func checkErr(err error, failReason string) {
	if err != nil {
		panic(fmt.Errorf("%s: %w", failReason, err))
	}
}

// encodeCrossDomainMessage encodes a versioned cross domain message into a byte array.
func encodeCrossDomainMessage(nonce *big.Int, sender common.Address, target common.Address, value *big.Int, gasLimit *big.Int, data []byte) ([]byte, error) {
	_, version := crossdomain.DecodeVersionedNonce(nonce)

	var encoded []byte
	var err error
	if version.Cmp(big.NewInt(0)) == 0 {
		// Encode cross domain message V0
		encoded, err = crossdomain.EncodeCrossDomainMessageV0(target, sender, data, nonce)
	} else if version.Cmp(big.NewInt(1)) == 0 {
		// Encode cross domain message V1
		encoded, err = crossdomain.EncodeCrossDomainMessageV1(nonce, sender, target, value, gasLimit, data)
	} else {
		return nil, UnknownNonceVersion
	}

	return encoded, err
}

// parseSuperRootProof parses an abi encoded super root proof into a SuperRootProof struct.
func parseSuperRootProof(abiEncodedProof []byte) (*SuperRootProof, error) {
	// Parse the input as hex data
	unpacked, err := superRootProofArgs.Unpack(abiEncodedProof)
	if err != nil {
		return nil, err
	}

	// The Unpack method returns a slice of interface{}, so we need to get the first element
	if len(unpacked) != 1 {
		return nil, errors.New("unexpected number of values after unpacking super root proof")
	}

	// Use an anonymous struct matching the tuple’s layout.
	tmp := unpacked[0].(struct {
		Version     [1]uint8 `json:"version"`
		Timestamp   uint64   `json:"timestamp"`
		OutputRoots []struct {
			ChainId *big.Int `json:"chainId"`
			Root    [32]byte `json:"root"`
		} `json:"outputRoots"`
	})

	// Convert into our desired SuperRootProof type.
	proof := SuperRootProof{
		Version:   tmp.Version[0],
		Timestamp: tmp.Timestamp,
	}
	for _, o := range tmp.OutputRoots {
		proof.OutputRoots = append(proof.OutputRoots, OutputRootWithChainId{
			ChainId: o.ChainId,
			Root:    common.BytesToHash(o.Root[:]),
		})
	}

	return &proof, nil
}

// encodeSuperRootProof encodes a super root proof into a byte array.
func encodeSuperRootProof(superRootProof *SuperRootProof) ([]byte, error) {
	// Version must match the expected version (0x01)
	if superRootProof.Version != 0x01 {
		return nil, errors.New("invalid super root version")
	}

	// Output roots must not be empty
	if len(superRootProof.OutputRoots) == 0 {
		return nil, errors.New("empty super root")
	}

	// Start with version byte and timestamp
	encoded := []byte{superRootProof.Version}

	// Add timestamp as bytes8 (uint64)
	timestampBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(timestampBytes, superRootProof.Timestamp)
	encoded = append(encoded, timestampBytes...)

	// Add each output root (chainId + root)
	for _, outputRoot := range superRootProof.OutputRoots {
		// Append chainId bytes (padded to 32 bytes)
		chainIdBytes := make([]byte, 32)
		outputRoot.ChainId.FillBytes(chainIdBytes)
		encoded = append(encoded, chainIdBytes...)

		// Append root hash (already 32 bytes)
		encoded = append(encoded, outputRoot.Root.Bytes()...)
	}

	return encoded, nil
}

// hashWithdrawal hashes a withdrawal transaction.
func hashWithdrawal(nonce *big.Int, sender common.Address, target common.Address, value *big.Int, gasLimit *big.Int, data []byte) (common.Hash, error) {
	wd := crossdomain.Withdrawal{
		Nonce:    nonce,
		Sender:   &sender,
		Target:   &target,
		Value:    value,
		GasLimit: gasLimit,
		Data:     data,
	}
	return wd.Hash()
}

// hashOutputRootProof hashes an output root proof.
func hashOutputRootProof(version common.Hash, stateRoot common.Hash, messagePasserStorageRoot common.Hash, latestBlockHash common.Hash) (common.Hash, error) {
	hash, err := rollup.ComputeL2OutputRoot(&bindings.TypesOutputRootProof{
		Version:                  version,
		StateRoot:                stateRoot,
		MessagePasserStorageRoot: messagePasserStorageRoot,
		LatestBlockhash:          latestBlockHash,
	})
	if err != nil {
		return common.Hash{}, err
	}
	return common.Hash(hash), nil
}

// makeDepositTx creates a deposit transaction type.
func makeDepositTx(
	from common.Address,
	to common.Address,
	value *big.Int,
	mint *big.Int,
	gasLimit *big.Int,
	isCreate bool,
	data []byte,
	l1BlockHash common.Hash,
	logIndex *big.Int,
) types.DepositTx {
	// Create deposit transaction source
	udp := derive.UserDepositSource{
		L1BlockHash: l1BlockHash,
		LogIndex:    logIndex.Uint64(),
	}

	// Create deposit transaction
	depositTx := types.DepositTx{
		SourceHash:          udp.SourceHash(),
		From:                from,
		Value:               value,
		Gas:                 gasLimit.Uint64(),
		IsSystemTransaction: false, // This will never be a system transaction in the tests.
		Data:                data,
	}

	// Fill optional fields
	if mint.Cmp(big.NewInt(0)) == 1 {
		depositTx.Mint = mint
	}
	if !isCreate {
		depositTx.To = &to
	}

	return depositTx
}

// Custom type to write the generated proof to
type proofList [][]byte

func (n *proofList) Put(key []byte, value []byte) error {
	*n = append(*n, value)
	return nil
}

func (n *proofList) Delete(key []byte) error {
	panic("not supported")
}
