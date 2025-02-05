package common

import (
	"context"
	"errors"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	hosttypes "github.com/ethereum-optimism/optimism/op-program/host/types"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

var (
	ErrExperimentalPrefetchFailed   = errors.New("experimental prefetch failed")
	ErrExperimentalPrefetchDisabled = errors.New("experimental prefetch disabled")
)

// L2Source is a source of L2 data, it abstracts away the details of how to fetch L2 data between canonical and experimental sources.
// It also tracks metrics for each of the apis. Once experimental sources are stable, this will only route to the "experimental" source.
type L2Source struct {
	logger log.Logger

	// canonical source, used as a fallback if experimental source is enabled but fails
	// the underlying node should be a geth hash scheme archival node.
	canonicalEthClient   *L2Client
	canonicalDebugClient *sources.DebugClient

	// experimental source, used as the primary source if enabled
	experimentalClient *L2Client
}

var _ hosttypes.L2Source = &L2Source{}

// NewL2SourceWithClient creates a new L2 source with the given client as the canonical client.
// This doesn't configure the experimental source, but is useful for testing.
func NewL2SourceWithClient(logger log.Logger, canonicalL2Client *L2Client, canonicalDebugClient *sources.DebugClient) *L2Source {
	source := &L2Source{
		logger:               logger,
		canonicalEthClient:   canonicalL2Client,
		canonicalDebugClient: canonicalDebugClient,
	}

	return source
}

func NewL2SourceFromRPC(logger log.Logger, rollupCfg *rollup.Config, canonicalL2RPC client.RPC, experimentalRPC client.RPC) (*L2Source, error) {
	canonicalDebugClient := sources.NewDebugClient(canonicalL2RPC.CallContext)

	canonicalL2ClientCfg := sources.L2ClientDefaultConfig(rollupCfg, true)
	canonicalL2Client, err := NewL2Client(canonicalL2RPC, logger, nil, &L2ClientConfig{L2ClientConfig: canonicalL2ClientCfg})
	if err != nil {
		return nil, err
	}

	source := NewL2SourceWithClient(logger, canonicalL2Client, canonicalDebugClient)

	if experimentalRPC == nil {
		return source, nil
	}

	experimentalL2ClientCfg := sources.L2ClientDefaultConfig(rollupCfg, true)
	experimentalL2Client, err := NewL2Client(experimentalRPC, logger, nil, &L2ClientConfig{L2ClientConfig: experimentalL2ClientCfg})
	if err != nil {
		return nil, err
	}

	source.experimentalClient = experimentalL2Client

	return source, nil
}

func (s *L2Source) RollupConfig() *rollup.Config {
	return s.canonicalEthClient.RollupConfig()
}

func (l *L2Source) ExperimentalEnabled() bool {
	return l.experimentalClient != nil
}

// CodeByHash implements prefetcher.L2Source.
func (l *L2Source) CodeByHash(ctx context.Context, hash common.Hash) ([]byte, error) {
	if l.ExperimentalEnabled() {
		// This means experimental source was not able to retrieve relevant information from eth_getProof or debug_executePayload
		// We should fall back to the canonical source, and log a warning, and record a metric
		l.logger.Warn("Experimental source failed to retrieve code by hash, falling back to canonical source", "hash", hash)
	}
	return l.canonicalDebugClient.CodeByHash(ctx, hash)
}

// FetchReceipts implements prefetcher.L2Source.
func (l *L2Source) FetchReceipts(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Receipts, error) {
	if l.ExperimentalEnabled() {
		return l.experimentalClient.FetchReceipts(ctx, blockHash)
	}
	return l.canonicalEthClient.FetchReceipts(ctx, blockHash)
}

// NodeByHash implements prefetcher.L2Source.
func (l *L2Source) NodeByHash(ctx context.Context, hash common.Hash) ([]byte, error) {
	if l.ExperimentalEnabled() {
		// This means experimental source was not able to retrieve relevant information from eth_getProof or debug_executePayload
		// We should fall back to the canonical source, and log a warning, and record a metric
		l.logger.Warn("Experimental source failed to retrieve node by hash, falling back to canonical source", "hash", hash)
	}
	return l.canonicalDebugClient.NodeByHash(ctx, hash)
}

// InfoAndTxsByHash implements prefetcher.L2Source.
func (l *L2Source) InfoAndTxsByHash(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Transactions, error) {
	if l.ExperimentalEnabled() {
		return l.experimentalClient.InfoAndTxsByHash(ctx, blockHash)
	}
	return l.canonicalEthClient.InfoAndTxsByHash(ctx, blockHash)
}

// OutputByRoot implements prefetcher.L2Source.
func (l *L2Source) OutputByRoot(ctx context.Context, blockRoot common.Hash) (eth.Output, error) {
	if l.ExperimentalEnabled() {
		return l.experimentalClient.OutputByRoot(ctx, blockRoot)
	}
	return l.canonicalEthClient.OutputByRoot(ctx, blockRoot)
}

// OutputByBlockNumber implements prefetcher.L2Source.
func (l *L2Source) OutputByNumber(ctx context.Context, blockNum uint64) (eth.Output, error) {
	if l.ExperimentalEnabled() {
		return l.experimentalClient.OutputByNumber(ctx, blockNum)
	}
	return l.canonicalEthClient.OutputByNumber(ctx, blockNum)
}

// PayloadExecutionWitness implements prefetcher.L2Source.
func (l *L2Source) PayloadExecutionWitness(ctx context.Context, parentHash common.Hash, payloadAttributes eth.PayloadAttributes) (*eth.ExecutionWitness, error) {
	if !l.ExperimentalEnabled() {
		l.logger.Error("Experimental source is not enabled, cannot fetch execution witness", "parentHash", parentHash)
		return nil, ErrExperimentalPrefetchDisabled
	}

	// log errors, but return standard error so we know to retry with legacy source
	witness, err := l.experimentalClient.PayloadExecutionWitness(ctx, parentHash, payloadAttributes)
	if err != nil {
		l.logger.Error("Failed to fetch execution witness from experimental source", "parentHash", parentHash, "err", err)
		return nil, ErrExperimentalPrefetchFailed
	}
	return witness, nil
}

// GetProof implements prefetcher.L2Source.
func (l *L2Source) GetProof(ctx context.Context, address common.Address, storage []common.Hash, blockTag string) (*eth.AccountResult, error) {
	if l.ExperimentalEnabled() {
		return l.experimentalClient.GetProof(ctx, address, storage, blockTag)
	}
	proof, err := l.canonicalEthClient.GetProof(ctx, address, storage, blockTag)
	if err != nil {
		l.logger.Error("Failed to fetch proof from canonical source", "address", address, "storage", storage, "blockTag", blockTag, "err", err)
		return nil, ErrExperimentalPrefetchFailed
	}
	return proof, nil
}
