package tasks

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	cldr "github.com/ethereum-optimism/optimism/op-program/client/driver"
	"github.com/ethereum-optimism/optimism/op-program/client/l1"
	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

type L2Source interface {
	L2BlockRefByLabel(ctx context.Context, label eth.BlockLabel) (eth.L2BlockRef, error)
	L2OutputRoot(uint64) (common.Hash, eth.Bytes32, error)
}

type DerivationResult struct {
	SafeHead   eth.L2BlockRef
	BlockHash  common.Hash
	OutputRoot eth.Bytes32
}

// RunDerivation executes the L2 state transition, given a minimal interface to retrieve data.
// Returns the L2BlockRef of the safe head reached and the output root at l2ClaimBlockNum or
// the final safe head when l1Head is reached if l2ClaimBlockNum is not reached.
// Derivation may stop prior to l1Head if the l2ClaimBlockNum has already been reached though
// this is not guaranteed.
func RunDerivation(
	logger log.Logger,
	cfg *rollup.Config,
	l2Cfg *params.ChainConfig,
	l1Head common.Hash,
	l2OutputRoot common.Hash,
	l2ClaimBlockNum uint64,
	l1Oracle l1.Oracle,
	l2Oracle l2.Oracle) (DerivationResult, error) {
	l1Source := l1.NewOracleL1Client(logger, l1Oracle, l1Head)
	l1BlobsSource := l1.NewBlobFetcher(logger, l1Oracle)
	engineBackend, err := l2.NewOracleBackedL2Chain(logger, l2Oracle, l1Oracle /* kzg oracle */, l2Cfg, l2OutputRoot)
	if err != nil {
		return DerivationResult{}, fmt.Errorf("failed to create oracle-backed L2 chain: %w", err)
	}
	l2Source := l2.NewOracleEngine(cfg, logger, engineBackend)

	logger.Info("Starting derivation")
	d := cldr.NewDriver(logger, cfg, l1Source, l1BlobsSource, l2Source, l2ClaimBlockNum)
	if err := d.RunComplete(); err != nil {
		return DerivationResult{}, fmt.Errorf("failed to run program to completion: %w", err)
	}
	return loadOutputRoot(l2ClaimBlockNum, l2Source)
}

func loadOutputRoot(l2ClaimBlockNum uint64, src L2Source) (DerivationResult, error) {
	l2Head, err := src.L2BlockRefByLabel(context.Background(), eth.Safe)
	if err != nil {
		return DerivationResult{}, fmt.Errorf("cannot retrieve safe head: %w", err)
	}
	blockHash, outputRoot, err := src.L2OutputRoot(min(l2ClaimBlockNum, l2Head.Number))
	if err != nil {
		return DerivationResult{}, fmt.Errorf("calculate L2 output root: %w", err)
	}
	return DerivationResult{
		SafeHead:   l2Head,
		BlockHash:  blockHash,
		OutputRoot: outputRoot,
	}, nil
}
