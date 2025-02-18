package prefetcher

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	clientTypes "github.com/ethereum-optimism/optimism/op-program/client/interop/types"
	"github.com/ethereum-optimism/optimism/op-program/client/l1"
	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	"github.com/ethereum-optimism/optimism/op-program/client/mpt"
	hostcommon "github.com/ethereum-optimism/optimism/op-program/host/common"
	"github.com/ethereum-optimism/optimism/op-program/host/kvstore"
	hosttypes "github.com/ethereum-optimism/optimism/op-program/host/types"
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

	ErrAgreedPrestateUnavailable = errors.New("agreed prestate unavailable")
)

var acceleratedPrecompiles = []common.Address{
	common.BytesToAddress([]byte{0x1}),  // ecrecover
	common.BytesToAddress([]byte{0x8}),  // bn256Pairing
	common.BytesToAddress([]byte{0x0a}), // KZG Point Evaluation
	common.BytesToAddress([]byte{0x0b}), // BLS12-381 G1 add
	common.BytesToAddress([]byte{0x0c}), // BLS12-381 G1 multi-scalar-multiply
	common.BytesToAddress([]byte{0x0d}), // BLS12-381 G2 add
	common.BytesToAddress([]byte{0x0e}), // BLS12-381 G2 multi-scalar-multiply
	common.BytesToAddress([]byte{0x0f}), // BLS12-381 pairing check
	common.BytesToAddress([]byte{0x10}), // BLS12-381 hash-to-g1
	common.BytesToAddress([]byte{0x11}), // BLS12-381 hash-to-g2
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

type Prefetcher struct {
	logger         log.Logger
	l1Fetcher      L1Source
	l1BlobFetcher  L1BlobSource
	defaultChainID eth.ChainID
	l2Sources      hosttypes.L2Sources
	lastHint       string
	lastBulkHint   string
	kvStore        kvstore.KV
	// l2Head is the L2 block hash to retrieve output root from if interop is disabled
	l2Head common.Hash

	// Used to run the program for native block execution
	executor       ProgramExecutor
	agreedPrestate []byte
}

func NewPrefetcher(
	logger log.Logger,
	l1Fetcher L1Source,
	l1BlobFetcher L1BlobSource,
	defaultChainID eth.ChainID,
	l2Sources hosttypes.L2Sources,
	kvStore kvstore.KV,
	executor ProgramExecutor,
	l2Head common.Hash,
	agreedPrestate []byte,
) *Prefetcher {
	return &Prefetcher{
		logger:         logger,
		l1Fetcher:      NewRetryingL1Source(logger, l1Fetcher),
		l1BlobFetcher:  NewRetryingL1BlobSource(logger, l1BlobFetcher),
		defaultChainID: defaultChainID,
		l2Sources:      l2Sources,
		kvStore:        kvStore,
		executor:       executor,
		l2Head:         l2Head,
		agreedPrestate: agreedPrestate,
	}
}

func (p *Prefetcher) Hint(hint string) error {
	p.logger.Trace("Received hint", "hint", hint)

	hintType, _, err := parseHint(hint)

	// This is a special case to force block execution in order to populate the cache with preimage data
	if hintType == l2.HintL2BlockData {
		return p.prefetch(context.Background(), hint)
	}

	// ignore parsing error, and assume non-bulk hint
	if err == nil && (hintType == l2.HintL2AccountProof || hintType == l2.HintL2PayloadWitness) {
		p.lastBulkHint = hint
	} else {
		p.lastHint = hint
	}
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
		// ignore ErrExperimentalPrefetchFailed as we will retry with the canonical source
		if err := p.prefetch(ctx, hint); err != nil && !errors.Is(err, hostcommon.ErrExperimentalPrefetchFailed) {
			return nil, fmt.Errorf("prefetch failed: %w", err)
		}
		pre, err = p.kvStore.Get(key)
		if err != nil {
			p.logger.Error("Fetched pre-images for last hint but did not find required key", "hint", hint, "key", key)
		}
	}
	return pre, err
}

// bulkPrefetch prefetches multiple preimages at once. This is used for execution witness and account proof hints.
func (p *Prefetcher) bulkPrefetch(ctx context.Context, hint string) error {
	hintType, hintBytes, err := parseHint(hint)
	if err != nil {
		return err
	}

	p.logger.Debug("Prefetching", "type", hintType, "bytes", hexutil.Bytes(hintBytes))
	switch hintType {
	case l2.HintL2AccountProof:
		// account proofs include the block hash and requested address in the hint
		if len(hintBytes) != 32+20+8 {
			return fmt.Errorf("invalid L2 account proof hint: %x", hintBytes)
		}

		blockHash := common.Hash(hintBytes[:32])
		address := common.Address(hintBytes[32:52])
		chainID := eth.ChainIDFromUInt64(binary.BigEndian.Uint64(hintBytes[52:]))

		cl, err := p.l2Sources.ForChainID(chainID)
		if err != nil {
			return err
		}

		// We request a proof of storage slot 0x00...00 because that will include the preimage of the storage root
		// which is actually all that's required.
		result, err := cl.GetProof(ctx, address, []common.Hash{{}}, blockHash.Hex())
		if err != nil {
			return fmt.Errorf("failed to fetch account proof for address %s: %w", address, err)
		}

		if len(result.StorageProof) == 0 {
			return fmt.Errorf("no storage proof found for address: %s", address)
		}

		if err := p.storeNodes(result.AccountProof); err != nil {
			return fmt.Errorf("failed to store account proof state: %w", err)
		}

		// set the preimage for the account storage root
		if err := p.storeNodes(result.StorageProof[0].Proof); err != nil {
			return fmt.Errorf("failed to store storage proof state: %w", err)
		}

		return nil

	case l2.HintL2PayloadWitness:
		var hint l2.PayloadWitnessHint
		if err := json.Unmarshal(hintBytes, &hint); err != nil {
			return fmt.Errorf("failed to unmarshal payload witness hint: %w", err)
		}

		chainID := p.defaultChainID
		if hint.ChainID != nil {
			chainID = *hint.ChainID
		}

		cl, err := p.l2Sources.ForChainID(chainID)
		if err != nil {
			return err
		}

		result, err := cl.PayloadExecutionWitness(ctx, hint.ParentBlockHash, *hint.PayloadAttributes)
		if err != nil {
			return fmt.Errorf("failed to fetch L2 execution witness for block with parent %v: %w", hint.ParentBlockHash, err)
		}

		// ignore keys because we want to rehash all of the values for safety
		values := make([]hexutil.Bytes, 0, len(result.State)+len(result.Codes))
		for _, value := range result.State {
			values = append(values, value)
		}

		for _, value := range result.Codes {
			values = append(values, value)
		}

		return p.storeNodes(values)
	default:
	}
	return fmt.Errorf("unknown bulk hint type: %v", hintType)
}

func (p *Prefetcher) prefetchState(ctx context.Context, hint string) error {
	hintType, hintBytes, err := parseHint(hint)
	if err != nil {
		return err
	}

	_, chainID, err := p.parseHashAndChainID("L2 state/code", hintBytes)
	if err != nil {
		return err
	}

	cl, err := p.l2Sources.ForChainID(chainID)
	if err != nil {
		return err
	}

	// some L2 state data can be fetched in bulk from block execution witnesses instead of direction from the MPT
	// if we have a bulk hint, we should use it instead of the last hint (will fallback to last hint after bulk hint is cleared and request is retried)
	if p.lastBulkHint != "" && cl.ExperimentalEnabled() {
		bulkHint := p.lastBulkHint
		p.lastBulkHint = ""
		return p.bulkPrefetch(ctx, bulkHint)
	}

	// if we don't have a bulk hint, we should just fetch state normally by MPT hash
	switch hintType {
	case l2.HintL2StateNode:
		hash, chainID, err := p.parseHashAndChainID("L2 state node", hintBytes)
		if err != nil {
			return err
		}
		source, err := p.l2Sources.ForChainID(chainID)
		if err != nil {
			return err
		}
		node, err := source.NodeByHash(ctx, hash)
		if err != nil {
			return fmt.Errorf("failed to fetch L2 state node %s: %w", hash, err)
		}
		return p.kvStore.Put(preimage.Keccak256Key(hash).PreimageKey(), node)
	case l2.HintL2Code:
		hash, chainID, err := p.parseHashAndChainID("L2 code", hintBytes)
		if err != nil {
			return err
		}
		source, err := p.l2Sources.ForChainID(chainID)
		if err != nil {
			return err
		}
		code, err := source.CodeByHash(ctx, hash)
		if err != nil {
			return fmt.Errorf("failed to fetch L2 contract code %s: %w", hash, err)
		}
		return p.kvStore.Put(preimage.Keccak256Key(hash).PreimageKey(), code)
	}
	return fmt.Errorf("unknown state hint type: %v", hintType)
}

// prefetch fetches the lastHint and stores the preimages in the kv store. This may call bulkPrefetch if available which will store multiple preimages at a time.
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
		hash, chainID, err := p.parseHashAndChainID("L2 header/tx", hintBytes)
		if err != nil {
			return err
		}
		source, err := p.l2Sources.ForChainID(chainID)
		if err != nil {
			return err
		}
		header, txs, err := source.InfoAndTxsByHash(ctx, hash)
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
	case l2.HintL2Receipts:
		hash, chainID, err := p.parseHashAndChainID("L2 receipts", hintBytes)
		if err != nil {
			return err
		}
		source, err := p.l2Sources.ForChainID(chainID)
		if err != nil {
			return err
		}
		_, receipts, err := source.FetchReceipts(ctx, hash)
		if err != nil {
			return fmt.Errorf("failed to fetch L1 block %s receipts: %w", hash, err)
		}
		return p.storeReceipts(receipts)
	case l2.HintL2Output:
		requestedHash, chainID, err := p.parseHashAndChainID("L2 output", hintBytes)
		if err != nil {
			return err
		}
		source, err := p.l2Sources.ForChainID(chainID)
		if err != nil {
			return err
		}
		if len(p.agreedPrestate) == 0 {
			output, err := source.OutputByRoot(ctx, p.l2Head)
			if err != nil {
				return fmt.Errorf("failed to fetch L2 output root for block %s: %w", p.l2Head, err)
			}
			hash := common.Hash(eth.OutputRoot(output))
			if requestedHash != hash {
				return fmt.Errorf("output root %v from block %v does not match requested root: %v", hash, p.l2Head, requestedHash)
			}
			return p.kvStore.Put(preimage.Keccak256Key(hash).PreimageKey(), output.Marshal())
		} else {
			prestate, err := clientTypes.UnmarshalTransitionState(p.agreedPrestate)
			if err != nil {
				return fmt.Errorf("cannot fetch output root, invalid agreed prestate: %w", err)
			}
			superRoot, err := eth.UnmarshalSuperRoot(prestate.SuperRoot)
			if err != nil {
				return fmt.Errorf("cannot fetch output root, invalid super root in prestate: %w", err)
			}
			superV1, ok := superRoot.(*eth.SuperV1)
			if !ok {
				return fmt.Errorf("cannot fetch output root, unsupported super root version in prestate: %v", superRoot.Version())
			}
			blockNum, err := source.RollupConfig().TargetBlockNumber(superV1.Timestamp)
			if err != nil {
				return fmt.Errorf("cannot fetch output root, failed to calculate block number at timestamp %v: %w", superV1.Timestamp, err)
			}
			output, err := source.OutputByNumber(ctx, blockNum)
			if err != nil {
				return fmt.Errorf("failed to fetch L2 output root for block %v: %w", blockNum, err)
			}
			return p.kvStore.Put(preimage.Keccak256Key(eth.OutputRoot(output)).PreimageKey(), output.Marshal())
		}
	case l2.HintL2BlockData:
		if p.executor == nil {
			return fmt.Errorf("this prefetcher does not support native block execution")
		}
		if len(hintBytes) != 32+32+8 {
			return fmt.Errorf("invalid L2 block data hint: %x", hint)
		}
		agreedBlockHash := common.Hash(hintBytes[:32])
		blockHash := common.Hash(hintBytes[32:64])
		chainID := eth.ChainIDFromUInt64(binary.BigEndian.Uint64(hintBytes[64:72]))
		key := BlockDataKey(blockHash)
		if _, err := p.kvStore.Get(key.Key()); err == nil {
			return nil
		}
		if err := p.nativeReExecuteBlock(ctx, agreedBlockHash, blockHash, chainID); err != nil {
			return fmt.Errorf("failed to re-execute block: %w", err)
		}
		return p.kvStore.Put(BlockDataKey(blockHash).Key(), []byte{1})
	case l2.HintAgreedPrestate:
		if len(p.agreedPrestate) == 0 {
			return ErrAgreedPrestateUnavailable
		}
		hash := crypto.Keccak256Hash(p.agreedPrestate)
		return p.kvStore.Put(preimage.Keccak256Key(hash).PreimageKey(), p.agreedPrestate)
	case l2.HintL2StateNode, l2.HintL2Code:
		// handle state access hints separately to allow for bulk fetching
		return p.prefetchState(ctx, hint)
	}
	return fmt.Errorf("unknown hint type: %v", hintType)
}

func (p *Prefetcher) parseHashAndChainID(hintType string, hintBytes []byte) (common.Hash, eth.ChainID, error) {
	switch len(hintBytes) {
	case 32:
		return common.Hash(hintBytes), p.defaultChainID, nil
	case 40:
		return common.Hash(hintBytes[0:32]), eth.ChainIDFromUInt64(binary.BigEndian.Uint64(hintBytes[32:])), nil
	default:
		return common.Hash{}, eth.ChainID{}, fmt.Errorf("invalid %s hint: %x", hintType, hintBytes)
	}
}

type BlockDataKey [32]byte

func (p BlockDataKey) Key() [32]byte {
	return crypto.Keccak256Hash([]byte("block_data"), p[:])
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
	return p.storeNodes(nodes)
}

func (p *Prefetcher) storeNodes(nodes []hexutil.Bytes) error {
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
	return vm.PrecompiledContractsPrague[address]
}
