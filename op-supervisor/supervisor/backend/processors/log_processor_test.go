package processors

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

var logProcessorChainID = types.ChainIDFromUInt64(4)

func TestLogProcessor(t *testing.T) {
	ctx := context.Background()
	block1 := eth.BlockRef{
		ParentHash: common.Hash{0x42},
		Number:     100,
		Hash:       common.Hash{0x11},
		Time:       1111,
	}
	t.Run("NoOutputWhenLogsAreEmpty", func(t *testing.T) {
		store := &stubLogStorage{}
		processor := NewLogProcessor(logProcessorChainID, store)

		err := processor.ProcessLogs(ctx, block1, ethTypes.Receipts{})
		require.NoError(t, err)
		require.Empty(t, store.logs)
	})

	t.Run("OutputLogs", func(t *testing.T) {
		rcpts := ethTypes.Receipts{
			{
				Logs: []*ethTypes.Log{
					{
						Address: common.Address{0x11},
						Topics:  []common.Hash{{0xaa}},
						Data:    []byte{0xbb},
					},
					{
						Address: common.Address{0x22},
						Topics:  []common.Hash{{0xcc}},
						Data:    []byte{0xdd},
					},
				},
			},
			{
				Logs: []*ethTypes.Log{
					{
						Address: common.Address{0x33},
						Topics:  []common.Hash{{0xee}},
						Data:    []byte{0xff},
					},
				},
			},
		}
		store := &stubLogStorage{}
		processor := NewLogProcessor(logProcessorChainID, store)

		err := processor.ProcessLogs(ctx, block1, rcpts)
		require.NoError(t, err)
		expectedLogs := []storedLog{
			{
				parent:  block1.ParentID(),
				logIdx:  0,
				logHash: logToLogHash(rcpts[0].Logs[0]),
				execMsg: nil,
			},
			{
				parent:  block1.ParentID(),
				logIdx:  0,
				logHash: logToLogHash(rcpts[0].Logs[1]),
				execMsg: nil,
			},
			{
				parent:  block1.ParentID(),
				logIdx:  0,
				logHash: logToLogHash(rcpts[1].Logs[0]),
				execMsg: nil,
			},
		}
		require.Equal(t, expectedLogs, store.logs)

		expectedBlocks := []storedSeal{
			{
				parent:    block1.ParentHash,
				block:     block1.ID(),
				timestamp: block1.Time,
			},
		}
		require.Equal(t, expectedBlocks, store.seals)
	})

	t.Run("IncludeExecutingMessage", func(t *testing.T) {
		rcpts := ethTypes.Receipts{
			{
				Logs: []*ethTypes.Log{
					{
						Address: predeploys.CrossL2InboxAddr,
						Topics:  []common.Hash{},
						Data:    []byte{0xff},
					},
				},
			},
		}
		execMsg := &types.ExecutingMessage{
			Chain:     4, // TODO(#11105): translate chain ID to chain index
			BlockNum:  6,
			LogIdx:    8,
			Timestamp: 10,
			Hash:      common.Hash{0xaa},
		}
		store := &stubLogStorage{}
		processor := NewLogProcessor(types.ChainID{4}, store).(*logProcessor)
		processor.eventDecoder = func(l *ethTypes.Log) (*types.ExecutingMessage, error) {
			require.Equal(t, rcpts[0].Logs[0], l)
			return execMsg, nil
		}

		err := processor.ProcessLogs(ctx, block1, rcpts)
		require.NoError(t, err)
		expected := []storedLog{
			{
				parent:  block1.ParentID(),
				logIdx:  0,
				logHash: logToLogHash(rcpts[0].Logs[0]),
				execMsg: execMsg,
			},
		}
		require.Equal(t, expected, store.logs)

		expectedBlocks := []storedSeal{
			{
				parent:    block1.ParentHash,
				block:     block1.ID(),
				timestamp: block1.Time,
			},
		}
		require.Equal(t, expectedBlocks, store.seals)
	})
}

func TestToLogHash(t *testing.T) {
	mkLog := func() *ethTypes.Log {
		return &ethTypes.Log{
			Address: common.Address{0xaa, 0xbb},
			Topics: []common.Hash{
				{0xcc},
				{0xdd},
			},
			Data:        []byte{0xee, 0xff, 0x00},
			BlockNumber: 12345,
			TxHash:      common.Hash{0x11, 0x22, 0x33},
			TxIndex:     4,
			BlockHash:   common.Hash{0x44, 0x55},
			Index:       8,
			Removed:     false,
		}
	}
	relevantMods := []func(l *ethTypes.Log){
		func(l *ethTypes.Log) { l.Address = common.Address{0xab, 0xcd} },
		func(l *ethTypes.Log) { l.Topics = append(l.Topics, common.Hash{0x12, 0x34}) },
		func(l *ethTypes.Log) { l.Topics = l.Topics[:len(l.Topics)-1] },
		func(l *ethTypes.Log) { l.Topics[0] = common.Hash{0x12, 0x34} },
		func(l *ethTypes.Log) { l.Data = append(l.Data, 0x56) },
		func(l *ethTypes.Log) { l.Data = l.Data[:len(l.Data)-1] },
		func(l *ethTypes.Log) { l.Data[0] = 0x45 },
	}
	irrelevantMods := []func(l *ethTypes.Log){
		func(l *ethTypes.Log) { l.BlockNumber = 987 },
		func(l *ethTypes.Log) { l.TxHash = common.Hash{0xab, 0xcd} },
		func(l *ethTypes.Log) { l.TxIndex = 99 },
		func(l *ethTypes.Log) { l.BlockHash = common.Hash{0xab, 0xcd} },
		func(l *ethTypes.Log) { l.Index = 98 },
		func(l *ethTypes.Log) { l.Removed = true },
	}
	refHash := logToLogHash(mkLog())
	// The log hash is stored in the database so test that it matches the actual value.
	// If this changes, compatibility with existing databases may be affected
	expectedRefHash := common.HexToHash("0x4e1dc08fddeb273275f787762cdfe945cf47bb4e80a1fabbc7a825801e81b73f")
	require.Equal(t, expectedRefHash, refHash, "reference hash changed, check that database compatibility is not broken")

	// Check that the hash is changed when any data it should include changes
	for i, mod := range relevantMods {
		l := mkLog()
		mod(l)
		hash := logToLogHash(l)
		require.NotEqualf(t, refHash, hash, "expected relevant modification %v to affect the hash but it did not", i)
	}
	// Check that the hash is not changed when any data it should not include changes
	for i, mod := range irrelevantMods {
		l := mkLog()
		mod(l)
		hash := logToLogHash(l)
		require.Equal(t, refHash, hash, "expected irrelevant modification %v to not affect the hash but it did", i)
	}
}

type stubLogStorage struct {
	logs  []storedLog
	seals []storedSeal
}

func (s *stubLogStorage) SealBlock(chainID types.ChainID, block eth.BlockRef) error {
	if logProcessorChainID != chainID {
		return fmt.Errorf("chain id mismatch, expected %v but got %v", logProcessorChainID, chainID)
	}
	s.seals = append(s.seals, storedSeal{
		parent:    block.ParentHash,
		block:     block.ID(),
		timestamp: block.Time,
	})
	return nil
}

func (s *stubLogStorage) AddLog(chainID types.ChainID, logHash common.Hash, parentBlock eth.BlockID, logIdx uint32, execMsg *types.ExecutingMessage) error {
	if logProcessorChainID != chainID {
		return fmt.Errorf("chain id mismatch, expected %v but got %v", logProcessorChainID, chainID)
	}
	s.logs = append(s.logs, storedLog{
		parent:  parentBlock,
		logIdx:  logIdx,
		logHash: logHash,
		execMsg: execMsg,
	})
	return nil
}

type storedSeal struct {
	parent    common.Hash
	block     eth.BlockID
	timestamp uint64
}

type storedLog struct {
	parent  eth.BlockID
	logIdx  uint32
	logHash common.Hash
	execMsg *types.ExecutingMessage
}
