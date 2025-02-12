package db

import (
	"fmt"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/metrics"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/logs"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

type mockDerivationStorage struct {
	lastFn func() (pair types.DerivedBlockSealPair, err error)
}

func (m *mockDerivationStorage) First() (pair types.DerivedBlockSealPair, err error) {
	return types.DerivedBlockSealPair{}, nil
}
func (m *mockDerivationStorage) Last() (pair types.DerivedBlockSealPair, err error) {
	if m.lastFn != nil {
		return m.lastFn()
	}
	return types.DerivedBlockSealPair{}, nil
}
func (m *mockDerivationStorage) Invalidated() (pair types.DerivedBlockSealPair, err error) {
	return types.DerivedBlockSealPair{}, nil
}
func (m *mockDerivationStorage) AddDerived(derivedFrom eth.BlockRef, derived eth.BlockRef) error {
	return nil
}
func (m *mockDerivationStorage) ReplaceInvalidatedBlock(replacementDerived eth.BlockRef, invalidated common.Hash) (types.DerivedBlockSealPair, error) {
	return types.DerivedBlockSealPair{}, nil
}
func (m *mockDerivationStorage) RewindAndInvalidate(invalidated types.DerivedBlockRefPair) error {
	return nil
}
func (m *mockDerivationStorage) SourceToLastDerived(source eth.BlockID) (derived types.BlockSeal, err error) {
	return types.BlockSeal{}, nil
}
func (m *mockDerivationStorage) ContainsDerived(derived eth.BlockID) error {
	return nil
}
func (m *mockDerivationStorage) DerivedToFirstSource(derived eth.BlockID) (source types.BlockSeal, err error) {
	return types.BlockSeal{}, nil
}
func (m *mockDerivationStorage) Next(pair types.DerivedIDPair) (next types.DerivedBlockSealPair, err error) {
	return types.DerivedBlockSealPair{}, nil
}
func (m *mockDerivationStorage) NextSource(source eth.BlockID) (nextSource types.BlockSeal, err error) {
	return types.BlockSeal{}, nil
}
func (m *mockDerivationStorage) NextDerived(derived eth.BlockID) (next types.DerivedBlockSealPair, err error) {
	return types.DerivedBlockSealPair{}, nil
}
func (m *mockDerivationStorage) PreviousSource(source eth.BlockID) (prevSource types.BlockSeal, err error) {
	return types.BlockSeal{}, nil
}
func (m *mockDerivationStorage) PreviousDerived(derived eth.BlockID) (prevDerived types.BlockSeal, err error) {
	return types.BlockSeal{}, nil
}
func (m *mockDerivationStorage) RewindToScope(scope eth.BlockID) error {
	return nil
}
func (m *mockDerivationStorage) RewindToFirstDerived(derived eth.BlockID) error {
	return nil
}

type mockLogStorage struct {
	latestBlock eth.BlockID
	blocks      map[uint64]mockBlock
}

func (m *mockLogStorage) LatestSealedBlock() (id eth.BlockID, ok bool) {
	if m.latestBlock.Number == 0 {
		return eth.BlockID{}, false
	}
	return m.latestBlock, true
}

func (m *mockLogStorage) OpenBlock(blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
	if blockNum > m.latestBlock.Number {
		return eth.BlockRef{}, 0, nil, fmt.Errorf("block not found: number too high")
	}

	if block, ok := m.blocks[blockNum]; ok {
		return block.ref, uint32(len(block.execMsgs)), block.execMsgs, nil
	}

	return eth.BlockRef{
		Number: blockNum,
		Time:   1000,
	}, 0, make(map[uint32]*types.ExecutingMessage), nil
}

func (m *mockLogStorage) Close() error { return nil }
func (m *mockLogStorage) AddLog(logHash common.Hash, parentBlock eth.BlockID, logIdx uint32, execMsg *types.ExecutingMessage) error {
	return nil
}
func (m *mockLogStorage) SealBlock(parentHash common.Hash, block eth.BlockID, timestamp uint64) error {
	return nil
}
func (m *mockLogStorage) Rewind(newHead eth.BlockID) error { return nil }
func (m *mockLogStorage) FindSealedBlock(number uint64) (block types.BlockSeal, err error) {
	return types.BlockSeal{}, nil
}
func (m *mockLogStorage) IteratorStartingAt(sealedNum uint64, logsSince uint32) (logs.Iterator, error) {
	return nil, nil
}
func (m *mockLogStorage) Contains(types.ContainsQuery) (includedIn types.BlockSeal, err error) {
	return types.BlockSeal{}, nil
}

type mockBlock struct {
	ref      eth.BlockRef
	execMsgs map[uint32]*types.ExecutingMessage
}

func sampleDepSet(t *testing.T) depset.DependencySet {
	depSet, err := depset.NewStaticConfigDependencySet(
		map[eth.ChainID]*depset.StaticConfigDependency{
			eth.ChainIDFromUInt64(900): {
				ChainIndex:     900,
				ActivationTime: 42,
				HistoryMinTime: 100,
			},
			eth.ChainIDFromUInt64(901): {
				ChainIndex:     901,
				ActivationTime: 30,
				HistoryMinTime: 20,
			},
			eth.ChainIDFromUInt64(902): {
				ChainIndex:     902,
				ActivationTime: 30,
				HistoryMinTime: 20,
			},
		})
	require.NoError(t, err)
	return depSet
}

func TestCommonL1UnknownChain(t *testing.T) {
	m1 := &mockDerivationStorage{}
	m2 := &mockDerivationStorage{}
	logger := testlog.Logger(t, log.LevelDebug)
	chainDB := NewChainsDB(logger, sampleDepSet(t), metrics.NoopMetrics)

	// add a mock local derived-from storage to drive the test
	chainDB.AddLocalDerivationDB(eth.ChainIDFromUInt64(900), m1)
	chainDB.AddLocalDerivationDB(eth.ChainIDFromUInt64(901), m2)
	// don't attach a mock for chain 902

	_, err := chainDB.LastCommonL1()
	require.ErrorIs(t, err, types.ErrUnknownChain)
}

func TestCommonL1(t *testing.T) {
	m1 := &mockDerivationStorage{}
	m2 := &mockDerivationStorage{}
	m3 := &mockDerivationStorage{}
	logger := testlog.Logger(t, log.LevelDebug)
	chainDB := NewChainsDB(logger, sampleDepSet(t), metrics.NoopMetrics)

	// add a mock local derived-from storage to drive the test
	chainDB.AddLocalDerivationDB(eth.ChainIDFromUInt64(900), m1)
	chainDB.AddLocalDerivationDB(eth.ChainIDFromUInt64(901), m2)
	chainDB.AddLocalDerivationDB(eth.ChainIDFromUInt64(902), m3)

	// returnN is a helper function which creates a Latest Function for the test
	returnN := func(n uint64) func() (pair types.DerivedBlockSealPair, err error) {
		return func() (pair types.DerivedBlockSealPair, err error) {
			return types.DerivedBlockSealPair{
				Source: types.BlockSeal{
					Number: n,
				},
			}, nil
		}
	}
	t.Run("pattern 1", func(t *testing.T) {
		m1.lastFn = returnN(1)
		m2.lastFn = returnN(2)
		m3.lastFn = returnN(3)

		latest, err := chainDB.LastCommonL1()
		require.NoError(t, err)
		require.Equal(t, uint64(1), latest.Number)
	})
	t.Run("pattern 2", func(t *testing.T) {
		m1.lastFn = returnN(3)
		m2.lastFn = returnN(2)
		m3.lastFn = returnN(1)

		latest, err := chainDB.LastCommonL1()
		require.NoError(t, err)
		require.Equal(t, uint64(1), latest.Number)
	})
	t.Run("pattern 3", func(t *testing.T) {
		m1.lastFn = returnN(99)
		m2.lastFn = returnN(1)
		m3.lastFn = returnN(98)

		latest, err := chainDB.LastCommonL1()
		require.NoError(t, err)
		require.Equal(t, uint64(1), latest.Number)
	})
	t.Run("error", func(t *testing.T) {
		m1.lastFn = returnN(99)
		m2.lastFn = returnN(1)
		m3.lastFn = func() (pair types.DerivedBlockSealPair, err error) {
			return types.DerivedBlockSealPair{}, fmt.Errorf("error")
		}
		latest, err := chainDB.LastCommonL1()
		require.Error(t, err)
		require.Equal(t, types.BlockSeal{}, latest)
	})
}

func TestFindFirstBlockReferencingLogs(t *testing.T) {
	sourceBlock := types.BlockSeal{
		Hash:      common.HexToHash("0x1234"),
		Number:    100,
		Timestamp: 1000,
	}
	sourceChainIndex := types.ChainIndex(1)
	foreignChainID := eth.ChainIDFromUInt64(2)

	tests := []struct {
		name          string
		blocks        map[uint64]mockBlock
		latestBlock   eth.BlockID
		expectedBlock eth.BlockRef
		expectedFound bool
		expectedError error
	}{
		{
			name:          "no blocks in foreign chain",
			blocks:        map[uint64]mockBlock{},
			expectedBlock: eth.BlockRef{},
			expectedFound: false,
			expectedError: nil,
		},
		{
			name: "no dependent blocks",
			blocks: map[uint64]mockBlock{
				200: {
					ref: eth.BlockRef{
						Hash:   common.HexToHash("0x2000"),
						Number: 200,
						Time:   1100,
					},
					execMsgs: map[uint32]*types.ExecutingMessage{
						0: {
							Chain:     2, // Different chain
							BlockNum:  100,
							LogIdx:    0,
							Timestamp: 1100,
						},
					},
				},
			},
			latestBlock: eth.BlockID{
				Hash:   common.HexToHash("0x2000"),
				Number: 200,
			},
			expectedBlock: eth.BlockRef{},
			expectedFound: false,
			expectedError: nil,
		},
		{
			name: "finds block referencing source block",
			blocks: map[uint64]mockBlock{
				200: {
					ref: eth.BlockRef{
						Hash:   common.HexToHash("0x2000"),
						Number: 200,
						Time:   1100,
					},
					execMsgs: map[uint32]*types.ExecutingMessage{
						0: {
							Chain:     sourceChainIndex,
							BlockNum:  sourceBlock.Number,
							LogIdx:    0,
							Timestamp: 1100,
						},
					},
				},
			},
			latestBlock: eth.BlockID{
				Hash:   common.HexToHash("0x2000"),
				Number: 200,
			},
			expectedBlock: eth.BlockRef{
				Hash:   common.HexToHash("0x2000"),
				Number: 200,
				Time:   1100,
			},
			expectedFound: true,
			expectedError: nil,
		},
		{
			name: "finds block referencing later block from source chain",
			blocks: map[uint64]mockBlock{
				200: {
					ref: eth.BlockRef{
						Hash:   common.HexToHash("0x2000"),
						Number: 200,
						Time:   1100,
					},
					execMsgs: map[uint32]*types.ExecutingMessage{
						0: {
							Chain:     sourceChainIndex,
							BlockNum:  sourceBlock.Number + 5, // References a later block
							LogIdx:    0,
							Timestamp: 1100,
						},
					},
				},
			},
			latestBlock: eth.BlockID{
				Hash:   common.HexToHash("0x2000"),
				Number: 200,
			},
			expectedBlock: eth.BlockRef{
				Hash:   common.HexToHash("0x2000"),
				Number: 200,
				Time:   1100,
			},
			expectedFound: true,
			expectedError: nil,
		},
		{
			name: "ignores block referencing earlier block from source chain",
			blocks: map[uint64]mockBlock{
				200: {
					ref: eth.BlockRef{
						Hash:   common.HexToHash("0x2000"),
						Number: 200,
						Time:   1100,
					},
					execMsgs: map[uint32]*types.ExecutingMessage{
						0: {
							Chain:     sourceChainIndex,
							BlockNum:  sourceBlock.Number - 1, // References an earlier block
							LogIdx:    0,
							Timestamp: 1100,
						},
					},
				},
			},
			latestBlock: eth.BlockID{
				Hash:   common.HexToHash("0x2000"),
				Number: 200,
			},
			expectedBlock: eth.BlockRef{},
			expectedFound: false,
			expectedError: nil,
		},
		{
			name: "stops at block with timestamp < source block",
			blocks: map[uint64]mockBlock{
				200: {
					ref: eth.BlockRef{
						Hash:   common.HexToHash("0x2000"),
						Number: 200,
						Time:   1100,
					},
					execMsgs: map[uint32]*types.ExecutingMessage{
						0: {
							Chain:     sourceChainIndex,
							BlockNum:  sourceBlock.Number,
							LogIdx:    0,
							Timestamp: 1100,
						},
					},
				},
				199: {
					ref: eth.BlockRef{
						Hash:   common.HexToHash("0x1990"),
						Number: 199,
						Time:   900, // Before source block timestamp
					},
					execMsgs: map[uint32]*types.ExecutingMessage{
						0: {
							Chain:     sourceChainIndex,
							BlockNum:  sourceBlock.Number + 1, // Even though it references a later block
							LogIdx:    0,
							Timestamp: 900,
						},
					},
				},
			},
			latestBlock: eth.BlockID{
				Hash:   common.HexToHash("0x2000"),
				Number: 200,
			},
			expectedBlock: eth.BlockRef{
				Hash:   common.HexToHash("0x2000"),
				Number: 200,
				Time:   1100,
			},
			expectedFound: true,
			expectedError: nil,
		},
		{
			name: "returns first dependent block",
			blocks: map[uint64]mockBlock{
				200: {
					ref: eth.BlockRef{
						Hash:   common.HexToHash("0x2000"),
						Number: 200,
						Time:   1100,
					},
					execMsgs: map[uint32]*types.ExecutingMessage{
						0: {
							Chain:     sourceChainIndex,
							BlockNum:  sourceBlock.Number + 2, // References later block
							LogIdx:    0,
							Timestamp: 1100,
						},
					},
				},
				199: {
					ref: eth.BlockRef{
						Hash:   common.HexToHash("0x1990"),
						Number: 199,
						Time:   1050,
					},
					execMsgs: map[uint32]*types.ExecutingMessage{
						0: {
							Chain:     sourceChainIndex,
							BlockNum:  sourceBlock.Number, // References source block
							LogIdx:    1,
							Timestamp: 1050,
						},
					},
				},
			},
			latestBlock: eth.BlockID{
				Hash:   common.HexToHash("0x2000"),
				Number: 200,
			},
			expectedBlock: eth.BlockRef{
				Hash:   common.HexToHash("0x1990"),
				Number: 199,
				Time:   1050,
			},
			expectedFound: true,
			expectedError: nil,
		},
		{
			name:          "error on unknown chain",
			blocks:        map[uint64]mockBlock{},
			expectedBlock: eth.BlockRef{},
			expectedFound: false,
			expectedError: types.ErrUnknownChain,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger := testlog.Logger(t, log.LevelDebug)
			chainDB := NewChainsDB(logger, sampleDepSet(t), metrics.NoopMetrics)

			mockLogDB := &mockLogStorage{
				latestBlock: test.latestBlock,
				blocks:      test.blocks,
			}
			if test.expectedError != types.ErrUnknownChain {
				chainDB.AddLogDB(foreignChainID, mockLogDB)
			}

			block, found, err := chainDB.FindFirstBlockReferencingLogs(sourceBlock, sourceChainIndex, foreignChainID)
			if test.expectedError != nil {
				require.ErrorIs(t, err, test.expectedError)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectedFound, found)
				require.Equal(t, test.expectedBlock, block)
			}
		})
	}
}
