package types

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type DataFormat string

const (
	DataFormatFile      DataFormat = "file"
	DataFormatDirectory DataFormat = "directory"
	DataFormatPebble    DataFormat = "pebble"
)

var SupportedDataFormats = []DataFormat{DataFormatFile, DataFormatDirectory, DataFormatPebble}

type L2Source interface {
	InfoAndTxsByHash(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Transactions, error)
	NodeByHash(ctx context.Context, hash common.Hash) ([]byte, error)
	CodeByHash(ctx context.Context, hash common.Hash) ([]byte, error)
	OutputByRoot(ctx context.Context, blockRoot common.Hash) (eth.Output, error)
}
