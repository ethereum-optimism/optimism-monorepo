package prefetcher

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"strings"

	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum-optimism/optimism/op-program/client/l1"
	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	"github.com/ethereum-optimism/optimism/op-program/client/mpt"
	"github.com/ethereum-optimism/optimism/op-program/host/kvstore"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

var (
	precompileSuccess = [1]byte{1}
	precompileFailure = [1]byte{0}
)

var (
	ErrExperimentalPrefetchFailed   = errors.New("experimental prefetch failed")
	ErrExperimentalPrefetchDisabled = errors.New("experimental prefetch disabled")
)

var acceleratedPrecompiles = []common.Address{
	common.BytesToAddress([]byte{0x1}),  // ecrecover
	common.BytesToAddress([]byte{0x8}),  // bn256Pairing
	common.BytesToAddress([]byte{0x0a}), // KZG Point Evaluation
}

type L1Source interface {
	InfoByHash(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, error)
	InfoAndTxsByHash(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Transactions, error)
	FetchReceipts(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Receipts, error)
}

type L1BlobSource interface {
	GetBlobSidecars(ctx context.Context, ref eth.L1BlockRef, hashes []eth.IndexedBlobHash) ([]*eth.BlobSidecar, error)
	GetBlobs(ctx context.Context, ref eth.L1BlockRef, hashes []eth.IndexedBlobHash) ([]*eth.Blob, error)
}

type L2Source interface {
	InfoAndTxsByHash(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Transactions, error)
	NodeByHash(ctx context.Context, hash common.Hash) ([]byte, error)
	CodeByHash(ctx context.Context, hash common.Hash) ([]byte, error)
	OutputByRoot(ctx context.Context, root common.Hash) (eth.Output, error)
}

type Prefetcher struct {
	logger        log.Logger
	l1Fetcher     L1Source
	l1BlobFetcher L1BlobSource
	l2Fetcher     L2Source
	lastHint      string
	kvStore       kvstore.KV
}

func NewPrefetcher(logger log.Logger, l1Fetcher L1Source, l1BlobFetcher L1BlobSource, l2Fetcher L2Source, kvStore kvstore.KV) *Prefetcher {
	return &Prefetcher{
		logger:        logger,
		l1Fetcher:     NewRetryingL1Source(logger, l1Fetcher),
		l1BlobFetcher: NewRetryingL1BlobSource(logger, l1BlobFetcher),
		l2Fetcher:     NewRetryingL2Source(logger, l2Fetcher),
		kvStore:       kvStore,
	}
}

func (p *Prefetcher) Hint(hint string) error {
	p.logger.Trace("Received hint", "hint", hint)
	p.lastHint = hint
	return nil
}

func (p *Prefetcher) GetPreimage(ctx context.Context, key common.Hash) ([]byte, error) {
	p.logger.Trace("Pre-image requested", "key", key)
	pre, err := p.kvStore.Get(key)
	// Use a loop to keep retrying the prefetch as long as the key is not found
	// This handles the case where the prefetch downloads a preimage, but it is then deleted unexpectedly
	// before we get to read it.
	for errors.Is(err, kvstore.ErrNotFound) && p.lastHint != "" {
		hint := p.lastHint
		if err := p.prefetch(ctx, hint); err != nil {
			return nil, fmt.Errorf("prefetch failed: %w", err)
		}
		pre, err = p.kvStore.Get(key)
		if err != nil {
			p.logger.Error("Fetched pre-images for last hint but did not find required key", "hint", hint, "key", key)
		}
	}
	return pre, err
}

func (p *Prefetcher) prefetch(ctx context.Context, hint string) error {
	hintType, hintBytes, err := parseHint(hint)
	if err != nil {
		return err
	}
	p.logger.Debug("Prefetching", "type", hintType, "bytes", hexutil.Bytes(hintBytes))
	switch hintType {
	case l1.HintL1BlockHeader:
		if len(hintBytes) != 32 {
			return fmt.Errorf("invalid L1 block hint: %x", hint)
		}
		hash := common.Hash(hintBytes)
		header, err := p.l1Fetcher.InfoByHash(ctx, hash)
		if err != nil {
			return fmt.Errorf("failed to fetch L1 block %s header: %w", hash, err)
		}
		data, err := header.HeaderRLP()
		if err != nil {
			return fmt.Errorf("marshall header: %w", err)
		}
		return p.kvStore.Put(preimage.Keccak256Key(hash).PreimageKey(), data)
	case l1.HintL1Transactions:
		if len(hintBytes) != 32 {
			return fmt.Errorf("invalid L1 transactions hint: %x", hint)
		}
		hash := common.Hash(hintBytes)
		_, txs, err := p.l1Fetcher.InfoAndTxsByHash(ctx, hash)
		if err != nil {
			return fmt.Errorf("failed to fetch L1 block %s txs: %w", hash, err)
		}
		return p.storeTransactions(txs)
	case l1.HintL1Receipts:
		if len(hintBytes) != 32 {
			return fmt.Errorf("invalid L1 receipts hint: %x", hint)
		}
		hash := common.Hash(hintBytes)
		_, receipts, err := p.l1Fetcher.FetchReceipts(ctx, hash)
		if err != nil {
			return fmt.Errorf("failed to fetch L1 block %s receipts: %w", hash, err)
		}
		return p.storeReceipts(receipts)
	case l1.HintL1Blob:
		if len(hintBytes) != 48 {
			return fmt.Errorf("invalid blob hint: %x", hint)
		}

		blobVersionHash := common.Hash(hintBytes[:32])
		blobHashIndex := binary.BigEndian.Uint64(hintBytes[32:40])
		refTimestamp := binary.BigEndian.Uint64(hintBytes[40:48])

		// Fetch the blob sidecar for the indexed blob hash passed in the hint.
		indexedBlobHash := eth.IndexedBlobHash{
			Hash:  blobVersionHash,
			Index: blobHashIndex,
		}
		// We pass an `eth.L1BlockRef`, but `GetBlobSidecars` only uses the timestamp, which we received in the hint.
		sidecars, err := p.l1BlobFetcher.GetBlobSidecars(ctx, eth.L1BlockRef{Time: refTimestamp}, []eth.IndexedBlobHash{indexedBlobHash})
		if err != nil || len(sidecars) != 1 {
			return fmt.Errorf("failed to fetch blob sidecars for %s %d: %w", blobVersionHash, blobHashIndex, err)
		}
		sidecar := sidecars[0]

		// Put the preimage for the versioned hash into the kv store
		if err = p.kvStore.Put(preimage.Sha256Key(blobVersionHash).PreimageKey(), sidecar.KZGCommitment[:]); err != nil {
			return err
		}

		// Put all of the blob's field elements into the kv store. There should be 4096. The preimage oracle key for
		// each field element is the keccak256 hash of `abi.encodePacked(sidecar.KZGCommitment, uint256(i))`
		blobKey := make([]byte, 80)
		copy(blobKey[:48], sidecar.KZGCommitment[:])
		for i := 0; i < params.BlobTxFieldElementsPerBlob; i++ {
			binary.BigEndian.PutUint64(blobKey[72:], uint64(i))
			blobKeyHash := crypto.Keccak256Hash(blobKey)
			if err := p.kvStore.Put(preimage.Keccak256Key(blobKeyHash).PreimageKey(), blobKey); err != nil {
				return err
			}
			if err = p.kvStore.Put(preimage.BlobKey(blobKeyHash).PreimageKey(), sidecar.Blob[i<<5:(i+1)<<5]); err != nil {
				return err
			}
		}
		return nil
	case l1.HintL1Precompile:
		if len(hintBytes) < 20 {
			return fmt.Errorf("invalid precompile hint: %x", hint)
		}
		precompileAddress := common.BytesToAddress(hintBytes[:20])
		// For extra safety, avoid accelerating unexpected precompiles
		if !slices.Contains(acceleratedPrecompiles, precompileAddress) {
			return fmt.Errorf("unsupported precompile address: %s", precompileAddress)
		}
		// NOTE: We use the precompiled contracts from Cancun because it's the only set that contains the addresses of all accelerated precompiles
		// We assume the precompile Run function behavior does not change across EVM upgrades.
		// As such, we must not rely on upgrade-specific behavior such as precompile.RequiredGas.
		precompile := getPrecompiledContract(precompileAddress)

		// KZG Point Evaluation precompile also verifies its input
		result, err := precompile.Run(hintBytes[20:])
		if err == nil {
			result = append(precompileSuccess[:], result...)
		} else {
			result = append(precompileFailure[:], result...)
		}
		inputHash := crypto.Keccak256Hash(hintBytes)
		// Put the input preimage so it can be loaded later
		if err := p.kvStore.Put(preimage.Keccak256Key(inputHash).PreimageKey(), hintBytes); err != nil {
			return err
		}
		return p.kvStore.Put(preimage.PrecompileKey(inputHash).PreimageKey(), result)
	case l1.HintL1PrecompileV2:
		if len(hintBytes) < 28 {
			return fmt.Errorf("invalid precompile hint: %x", hint)
		}
		precompileAddress := common.BytesToAddress(hintBytes[:20])
		// requiredGas := hintBytes[20:28] - unused by the host. Since the client already validates gas requirements.
		// The requiredGas is only used by the L1 PreimageOracle to enforce complete precompile execution.

		// For extra safety, avoid accelerating unexpected precompiles
		if !slices.Contains(acceleratedPrecompiles, precompileAddress) {
			return fmt.Errorf("unsupported precompile address: %s", precompileAddress)
		}
		// NOTE: We use the precompiled contracts from Cancun because it's the only set that contains the addresses of all accelerated precompiles
		// We assume the precompile Run function behavior does not change across EVM upgrades.
		// As such, we must not rely on upgrade-specific behavior such as precompile.RequiredGas.
		precompile := getPrecompiledContract(precompileAddress)

		// KZG Point Evaluation precompile also verifies its input
		result, err := precompile.Run(hintBytes[28:])
		if err == nil {
			result = append(precompileSuccess[:], result...)
		} else {
			result = append(precompileFailure[:], result...)
		}
		inputHash := crypto.Keccak256Hash(hintBytes)
		// Put the input preimage so it can be loaded later
		if err := p.kvStore.Put(preimage.Keccak256Key(inputHash).PreimageKey(), hintBytes); err != nil {
			return err
		}
		return p.kvStore.Put(preimage.PrecompileKey(inputHash).PreimageKey(), result)
	case l2.HintL2BlockHeader, l2.HintL2Transactions:
		if len(hintBytes) != 32 {
			return fmt.Errorf("invalid L2 header/tx hint: %x", hint)
		}
		hash := common.Hash(hintBytes)
		header, txs, err := p.l2Fetcher.InfoAndTxsByHash(ctx, hash)
		if err != nil {
			return fmt.Errorf("failed to fetch L2 block %s: %w", hash, err)
		}
		data, err := header.HeaderRLP()
		if err != nil {
			return fmt.Errorf("failed to encode header to RLP: %w", err)
		}
		err = p.kvStore.Put(preimage.Keccak256Key(hash).PreimageKey(), data)
		if err != nil {
			return err
		}
		return p.storeTransactions(txs)
	case l2.HintL2StateNode:
		if len(hintBytes) != 32 {
			return fmt.Errorf("invalid L2 state node hint: %x", hint)
		}
		hash := common.Hash(hintBytes)
		node, err := p.l2Fetcher.NodeByHash(ctx, hash)
		if err != nil {
			return fmt.Errorf("failed to fetch L2 state node %s: %w", hash, err)
		}
		return p.kvStore.Put(preimage.Keccak256Key(hash).PreimageKey(), node)
	case l2.HintL2Code:
		if len(hintBytes) != 32 {
			return fmt.Errorf("invalid L2 code hint: %x", hint)
		}
		hash := common.Hash(hintBytes)
		code, err := p.l2Fetcher.CodeByHash(ctx, hash)
		if err != nil {
			return fmt.Errorf("failed to fetch L2 contract code %s: %w", hash, err)
		}
		return p.kvStore.Put(preimage.Keccak256Key(hash).PreimageKey(), code)
	case l2.HintL2Output:
		if len(hintBytes) != 32 {
			return fmt.Errorf("invalid L2 output hint: %x", hint)
		}
		hash := common.Hash(hintBytes)
		output, err := p.l2Fetcher.OutputByRoot(ctx, hash)
		if err != nil {
			return fmt.Errorf("failed to fetch L2 output root %s: %w", hash, err)
		}
		return p.kvStore.Put(preimage.Keccak256Key(hash).PreimageKey(), output.Marshal())
	}
	return fmt.Errorf("unknown hint type: %v", hintType)
}

func (p *Prefetcher) storeReceipts(receipts types.Receipts) error {
	opaqueReceipts, err := eth.EncodeReceipts(receipts)
	if err != nil {
		return err
	}
	return p.storeTrieNodes(opaqueReceipts)
}

func (p *Prefetcher) storeTransactions(txs types.Transactions) error {
	opaqueTxs, err := eth.EncodeTransactions(txs)
	if err != nil {
		return err
	}
	return p.storeTrieNodes(opaqueTxs)
}

func (p *Prefetcher) storeTrieNodes(values []hexutil.Bytes) error {
	_, nodes := mpt.WriteTrie(values)
	for _, node := range nodes {
		key := preimage.Keccak256Key(crypto.Keccak256Hash(node)).PreimageKey()
		if err := p.kvStore.Put(key, node); err != nil {
			return fmt.Errorf("failed to store node: %w", err)
		}
	}
	return nil
}

// parseHint parses a hint string in wire protocol. Returns the hint type, requested hash and error (if any).
func parseHint(hint string) (string, []byte, error) {
	hintType, bytesStr, found := strings.Cut(hint, " ")
	if !found {
		return "", nil, fmt.Errorf("unsupported hint: %s", hint)
	}

	hintBytes, err := hexutil.Decode(bytesStr)
	if err != nil {
		return "", make([]byte, 0), fmt.Errorf("invalid bytes: %s", bytesStr)
	}
	return hintType, hintBytes, nil
}

func getPrecompiledContract(address common.Address) vm.PrecompiledContract {
	return vm.PrecompiledContractsCancun[address]
}
