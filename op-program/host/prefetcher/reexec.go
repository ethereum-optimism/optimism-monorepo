package prefetcher

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	"github.com/ethereum-optimism/optimism/op-program/client/l2/engineapi"
	hostcommon "github.com/ethereum-optimism/optimism/op-program/host/common"
	"github.com/ethereum-optimism/optimism/op-program/host/config"
	"github.com/ethereum-optimism/optimism/op-program/host/kvstore"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
)

func NativeReExecuteBlock(
	ctx context.Context, prefetcher *Prefetcher, blockHash common.Hash) error {
	header, _, err := prefetcher.l2Fetcher.InfoAndTxsByHash(ctx, blockHash)
	if err != nil {
		return err
	}
	newCfg := *prefetcher.hostConfig
	newCfg.L2ClaimBlockNumber = header.NumberU64()
	// No need to set a L2CLaim. The program will derive the blockHash even for an invalid claim.
	// Thus, the kv store is populated with the data we need

	withPrefetcher := hostcommon.WithPrefetcher(
		func(context.Context, log.Logger, kvstore.KV, *config.Config) (hostcommon.Prefetcher, error) {
			return NewPrefetcher(
				prefetcher.logger,
				prefetcher.l1Fetcher,
				prefetcher.l1BlobFetcher,
				prefetcher.l2Fetcher,
				prefetcher.kvStore,
				prefetcher.chainConfig,
				nil, // disable recursive block execution
			), nil
		})
	return hostcommon.FaultProofProgram(ctx, prefetcher.logger, &newCfg, withPrefetcher)
}

func ReExecuteBlock(
	ctx context.Context, logger log.Logger, l2Source L2Source, chainCfg *params.ChainConfig, blockHash common.Hash) (*types.Block, []*types.Receipt, error) {
	headerInfo, txs, err := l2Source.InfoAndTxsByHash(ctx, blockHash)
	if err != nil {
		return nil, nil, err
	}
	headerRLP, err := headerInfo.HeaderRLP()
	if err != nil {
		return nil, nil, err
	}
	var header types.Header
	if err := rlp.DecodeBytes(headerRLP, &header); err != nil {
		return nil, nil, fmt.Errorf("invalid block header %s: %w", blockHash, err)
	}

	l2Oracle := &l2Oracle{L2Source: l2Source}
	provider, err := l2.NewOracleBackedL2ChainFromHeader(logger, l2Oracle, nil, chainCfg, &header)
	if err != nil {
		return nil, nil, err
	}
	processor, err := engineapi.NewBlockProcessorFromHeader(provider, &header)
	if err != nil {
		return nil, nil, err
	}
	for _, tx := range txs {
		err := processor.AddTx(tx)
		if err != nil {
			return nil, nil, err
		}
	}
	block, err := processor.Assemble()
	if err != nil {
		return nil, nil, err
	}
	return block, processor.Receipts(), nil
}

type l2Oracle struct {
	L2Source
}

func (o *l2Oracle) BlockByHash(hash common.Hash) *types.Block {
	headerInfo, txs, err := o.L2Source.InfoAndTxsByHash(context.Background(), hash)
	if err != nil {
		panic(err)
	}
	headerRLP, err := headerInfo.HeaderRLP()
	if err != nil {
		panic(err)
	}
	var header types.Header
	if err := rlp.DecodeBytes(headerRLP, &header); err != nil {
		panic(fmt.Errorf("invalid block header %s: %w", hash, err))
	}

	var depositTxs []*types.Transaction
	for _, tx := range txs {
		if tx.Type() == types.DepositTxType {
			depositTxs = append(depositTxs, tx)
		}
	}
	return types.NewBlockWithHeader(&header).WithBody(types.Body{Transactions: depositTxs})
}

func (o *l2Oracle) CodeByHash(hash common.Hash) []byte {
	code, err := o.L2Source.CodeByHash(context.Background(), hash)
	if err != nil {
		panic(err)
	}
	return code
}

func (o *l2Oracle) NodeByHash(hash common.Hash) []byte {
	node, err := o.L2Source.NodeByHash(context.Background(), hash)
	if err != nil {
		panic(err)
	}
	return node
}

func (o *l2Oracle) OutputByRoot(root common.Hash) eth.Output {
	output, err := o.L2Source.OutputByRoot(context.Background(), root)
	if err != nil {
		panic(err)
	}
	return output
}
