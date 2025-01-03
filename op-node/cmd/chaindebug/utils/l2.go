package utils

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources"
)

type L2Entry struct {
	Block            *sources.RPCBlock `json:"block"`
	L1Origin         eth.BlockID       `json:"l1origin"`
	SequenceNumber   uint64            `json:"sequenceNumber"` // distance to first block of epoch
	L1OriginTime     uint64            `json:"l1OriginTime"`
	DepositTxs       []common.Hash     `json:"depositTxs"`
	UserTransactions []common.Hash     `json:"userTxs"`
}

func OnL2Block(cfg *rollup.Config, logger log.Logger, outDir string) (func(ctx context.Context, bl *sources.RPCBlock) error, error) {

	blocksDir := filepath.Join(outDir, "l2-blocks")
	if err := os.MkdirAll(blocksDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to make blocks dir: %w", err)
	}

	return func(ctx context.Context, bl *sources.RPCBlock) error {
		entry := &L2Entry{
			Block: bl,
		}
		if len(bl.Transactions) > 0 {
			l1Info, err := derive.L1BlockInfoFromBytes(cfg, cfg.BlockTime, bl.Transactions[0].Data())
			if err != nil {
				return err
			}
			entry.L1Origin = eth.BlockID{
				Hash:   l1Info.BlockHash,
				Number: l1Info.Number,
			}
			entry.SequenceNumber = l1Info.SequenceNumber
			entry.L1OriginTime = l1Info.Time

			for _, tx := range bl.Transactions {
				switch tx.Type() {
				case types.DepositTxType:
					entry.DepositTxs = append(entry.DepositTxs, tx.Hash())
				default:
					entry.UserTransactions = append(entry.UserTransactions, tx.Hash())
				}
			}
		}
		filename := filepath.Join(blocksDir, fmt.Sprintf("%08d_%s.json", uint64(bl.Number), bl.Hash))
		if err := writeJSON(filename, bl); err != nil {
			return fmt.Errorf("failed to write block json %q: %w", filename, err)
		}

		logger.Info("Processed L2 block", "block", entry.Block.BlockID())
		return nil
	}, nil
}
