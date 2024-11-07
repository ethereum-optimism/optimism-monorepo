package logs

import (
	"encoding/binary"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/entrydb"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

func createHash(i int) common.Hash {
	if i == -1 { // parent-hash of genesis is zero
		return common.Hash{}
	}
	var data [9]byte
	data[0] = 0xff
	binary.BigEndian.PutUint64(data[1:], uint64(i))
	return crypto.Keccak256Hash(data[:])
}

func TestErrorOpeningDatabase(t *testing.T) {
	dir := t.TempDir()
	_, err := NewFromFile(testlog.Logger(t, log.LvlInfo), &stubMetrics{}, filepath.Join(dir, "missing-dir", "file.db"), false)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func runDBTest(t *testing.T, setup func(t *testing.T, db *DB, m *stubMetrics), assert func(t *testing.T, db *DB, m *stubMetrics)) {
	createDb := func(t *testing.T, dir string) (*DB, *stubMetrics, string) {
		logger := testlog.Logger(t, log.LvlTrace)
		path := filepath.Join(dir, "test.db")
		m := &stubMetrics{}
		db, err := NewFromFile(logger, m, path, false)
		require.NoError(t, err, "Failed to create database")
		t.Cleanup(func() {
			err := db.Close()
			if err != nil {
				require.ErrorIs(t, err, fs.ErrClosed)
			}
		})
		return db, m, path
	}

	t.Run("New", func(t *testing.T) {
		db, m, _ := createDb(t, t.TempDir())
		setup(t, db, m)
		assert(t, db, m)
	})

	t.Run("Existing", func(t *testing.T) {
		dir := t.TempDir()
		db, m, path := createDb(t, dir)
		setup(t, db, m)
		// Close and recreate the database
		require.NoError(t, db.Close())
		checkDBInvariants(t, path, m)

		db2, m, path := createDb(t, dir)
		assert(t, db2, m)
		checkDBInvariants(t, path, m)
	})
}

func TestEmptyDbDoesNotFindEntry(t *testing.T) {
	runDBTest(t,
		func(t *testing.T, db *DB, m *stubMetrics) {},
		func(t *testing.T, db *DB, m *stubMetrics) {
			requireFuture(t, db, 1, 0, createHash(1))
			requireFuture(t, db, 1, 0, common.Hash{})
		})
}

func TestLatestSealedBlockNum(t *testing.T) {
	t.Run("Empty case", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {},
			func(t *testing.T, db *DB, m *stubMetrics) {
				n, ok := db.LatestSealedBlockNum()
				require.False(t, ok, "empty db expected")
				require.Zero(t, n)
				idx, err := db.searchCheckpoint(0, 0)
				require.ErrorIs(t, err, types.ErrFuture, "no checkpoint in empty db")
				require.ErrorIs(t, err, types.ErrFuture, "no checkpoint in empty db")
				require.Zero(t, idx)
			})
	})
	t.Run("Zero case", func(t *testing.T) {
		genesis := eth.BlockID{Hash: createHash(0), Number: 0}
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				require.NoError(t, db.SealBlock(common.Hash{}, genesis, 5000), "seal genesis")
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				n, ok := db.LatestSealedBlockNum()
				require.True(t, ok, "genesis block expected")
				require.Equal(t, genesis.Number, n)
				idx, err := db.searchCheckpoint(0, 0)
				require.NoError(t, err)
				require.Zero(t, idx, "genesis block as checkpoint 0")

				// Test if we can open the genesis block
				ref, logCount, execMsgs, err := db.OpenBlock(0)
				require.NoError(t, err)
				require.Empty(t, execMsgs)
				require.Zero(t, logCount)
				require.Equal(t, genesis, ref.ID())
				require.Equal(t, uint64(5000), ref.Time)
			})
	})
	t.Run("Later genesis case", func(t *testing.T) {
		genesis := eth.BlockID{Hash: createHash(10), Number: 10}
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				require.NoError(t, db.SealBlock(common.Hash{}, genesis, 5000), "seal genesis")
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				n, ok := db.LatestSealedBlockNum()
				require.True(t, ok, "genesis block expected")
				require.Equal(t, genesis.Number, n)
				idx, err := db.searchCheckpoint(genesis.Number, 0)
				require.NoError(t, err)
				require.Zero(t, idx, "anchor block as checkpoint 0")
				_, err = db.searchCheckpoint(0, 0)
				require.ErrorIs(t, err, types.ErrSkipped, "no checkpoint before genesis")
				require.ErrorIs(t, err, types.ErrSkipped, "no checkpoint before genesis")

				// Test if we can open the starting block
				_, _, _, err = db.OpenBlock(genesis.Number)
				// no data to find the parent-hash.
				// OpenBlock cannot start from the first entry, when not 0.
				// To start at a non-zero block, index the seal of the parent-block block before it,
				// and then that parent-hash will be available.
				require.ErrorIs(t, err, types.ErrSkipped)
			})
	})
	t.Run("Block 1 case", func(t *testing.T) {
		genesis := eth.BlockID{Hash: createHash(0), Number: 0}
		block1 := eth.BlockID{Hash: createHash(1), Number: 1}
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				require.NoError(t, db.SealBlock(common.Hash{}, genesis, 5000), "seal genesis")
				require.NoError(t, db.SealBlock(genesis.Hash, block1, 5001), "seal block 1")
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				n, ok := db.LatestSealedBlockNum()
				require.True(t, ok, "block 1 expected")
				require.Equal(t, block1.Number, n)
				idx, err := db.searchCheckpoint(block1.Number, 0)
				require.NoError(t, err)
				require.Equal(t, entrydb.EntryIdx(0), idx, "checkpoint 0 still for block 1")

				// Test if we can open the starting block
				ref, logCount, execMsgs, err := db.OpenBlock(genesis.Number)
				require.NoError(t, err)
				require.Empty(t, execMsgs)
				require.Zero(t, logCount)
				require.Equal(t, genesis, ref.ID())
				require.Equal(t, uint64(5000), ref.Time)

				// Test if we can open the first block after genesis
				ref, logCount, execMsgs, err = db.OpenBlock(block1.Number)
				require.NoError(t, err)
				require.Empty(t, execMsgs)
				require.Zero(t, logCount)
				require.Equal(t, block1, ref.ID())
				require.Equal(t, uint64(5001), ref.Time)
			})
	})
	t.Run("Using checkpoint case", func(t *testing.T) {
		genesis := eth.BlockID{Hash: createHash(0), Number: 0}
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				require.NoError(t, db.SealBlock(common.Hash{}, genesis, 5000), "seal genesis")
				for i := 1; i <= 260; i++ {
					id := eth.BlockID{Hash: createHash(i), Number: uint64(i)}
					require.NoError(t, db.SealBlock(createHash(i-1), id, 5000+uint64(i)), "seal block %d", i)
				}
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				n, ok := db.LatestSealedBlockNum()
				require.True(t, ok, "latest block expected")
				expected := uint64(260)
				require.Equal(t, expected, n)
				idx, err := db.searchCheckpoint(expected, 0)
				require.NoError(t, err)
				// It costs 2 entries per block, so if we add more than 1 checkpoint worth of blocks,
				// then we get to checkpoint 2
				require.Equal(t, entrydb.EntryIdx(searchCheckpointFrequency*2), idx, "checkpoint 1 reached")

				// Test if we can open the block
				ref, logCount, execMsgs, err := db.OpenBlock(n)
				require.NoError(t, err)
				require.Empty(t, execMsgs)
				require.Zero(t, logCount)
				require.Equal(t, createHash(int(n)), ref.Hash)
				require.Equal(t, uint64(5000)+n, ref.Time)
			})
	})
}

func TestAddLog(t *testing.T) {
	t.Run("BlockZero", func(t *testing.T) {
		// There are no logs in the genesis block so recording an entry for block 0 should be rejected.
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {},
			func(t *testing.T, db *DB, m *stubMetrics) {
				genesis := eth.BlockID{Hash: createHash(15), Number: 0}
				err := db.AddLog(createHash(1), genesis, 0, nil)
				require.ErrorIs(t, err, types.ErrOutOfOrder)
				require.ErrorIs(t, err, types.ErrOutOfOrder)
			})
	})

	t.Run("FirstEntries", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				genesis := eth.BlockID{Hash: createHash(15), Number: 15}
				require.NoError(t, db.SealBlock(common.Hash{}, genesis, 5000), "seal genesis")
				err := db.AddLog(createHash(1), genesis, 0, nil)
				require.NoError(t, err, "first log after genesis")
				require.NoError(t, db.SealBlock(genesis.Hash, eth.BlockID{Hash: createHash(16), Number: 16}, 5001))
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				requireContains(t, db, 16, 0, createHash(1))

				ref, logCount, execMsgs, err := db.OpenBlock(16)
				require.NoError(t, err)
				require.Empty(t, execMsgs)
				require.Equal(t, uint32(1), logCount)
				require.Equal(t, eth.BlockRef{Hash: createHash(16), Number: 16, ParentHash: createHash(15), Time: 5001}, ref)
			})
	})

	t.Run("MultipleEntriesFromSameBlock", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				// create 15 empty blocks
				for i := 0; i <= 15; i++ {
					bl := eth.BlockID{Hash: createHash(i), Number: uint64(i)}
					require.NoError(t, db.SealBlock(createHash(i-1), bl, 5000+uint64(i)), "seal blocks")
				}
				// Now apply 3 logs on top of that, contents for block 16
				bl15 := eth.BlockID{Hash: createHash(15), Number: 15}
				err := db.AddLog(createHash(1), bl15, 0, nil)
				require.NoError(t, err)
				err = db.AddLog(createHash(2), bl15, 1, nil)
				require.NoError(t, err)
				err = db.AddLog(createHash(3), bl15, 2, nil)
				require.NoError(t, err)
				// Now seal block 16
				bl16 := eth.BlockID{Hash: createHash(16), Number: 16}
				err = db.SealBlock(bl15.Hash, bl16, 5016)
				require.NoError(t, err)
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				require.EqualValues(t, 16*2+3+2, m.entryCount, "empty blocks have logs")
				requireContains(t, db, 16, 0, createHash(1))
				requireContains(t, db, 16, 1, createHash(2))
				requireContains(t, db, 16, 2, createHash(3))

				ref, logCount, execMsgs, err := db.OpenBlock(13)
				require.NoError(t, err)
				require.Empty(t, execMsgs)
				require.Equal(t, uint32(0), logCount)
				require.Equal(t, eth.BlockRef{Hash: createHash(13), Number: 13, ParentHash: createHash(12), Time: 5013}, ref)

				ref, logCount, execMsgs, err = db.OpenBlock(16)
				require.NoError(t, err)
				require.Empty(t, execMsgs)
				require.Equal(t, uint32(3), logCount)
				require.Equal(t, eth.BlockRef{Hash: createHash(16), Number: 16, ParentHash: createHash(15), Time: 5016}, ref)
			})
	})

	t.Run("MultipleEntriesFromMultipleBlocks", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl14 := eth.BlockID{Hash: createHash(14), Number: 14}
				err := db.SealBlock(createHash(13), bl14, 5000)
				require.NoError(t, err)
				bl15 := eth.BlockID{Hash: createHash(15), Number: 15}
				err = db.SealBlock(createHash(14), bl15, 5001)
				require.NoError(t, err)
				err = db.AddLog(createHash(1), bl15, 0, nil)
				require.NoError(t, err)
				err = db.AddLog(createHash(2), bl15, 1, nil)
				require.NoError(t, err)
				bl16 := eth.BlockID{Hash: createHash(16), Number: 16}
				err = db.SealBlock(bl15.Hash, bl16, 5003)
				require.NoError(t, err)
				err = db.AddLog(createHash(3), bl16, 0, nil)
				require.NoError(t, err)
				err = db.AddLog(createHash(4), bl16, 1, nil)
				require.NoError(t, err)
				bl17 := eth.BlockID{Hash: createHash(17), Number: 17}
				err = db.SealBlock(bl16.Hash, bl17, 5003)
				require.NoError(t, err)
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				require.EqualValues(t, 2+2+1+1+2+1+1+2, m.entryCount, "should not output new searchCheckpoint for every block")
				requireContains(t, db, 16, 0, createHash(1))
				requireContains(t, db, 16, 1, createHash(2))
				requireContains(t, db, 17, 0, createHash(3))
				requireContains(t, db, 17, 1, createHash(4))
			})
	})

	t.Run("ErrorWhenBeforeCurrentBlock", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl15 := eth.BlockID{Hash: createHash(15), Number: 15}
				err := db.SealBlock(common.Hash{}, bl15, 5001)
				require.NoError(t, err)
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl14 := eth.BlockID{Hash: createHash(14), Number: 14}
				err := db.SealBlock(createHash(13), bl14, 5000)
				require.ErrorIs(t, err, types.ErrConflict)
				require.ErrorIs(t, err, types.ErrConflict)
			})
	})

	t.Run("ErrorWhenBeforeCurrentBlockButAfterLastCheckpoint", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				err := db.lastEntryContext.forceBlock(eth.BlockID{Hash: createHash(13), Number: 13}, 5000)
				require.NoError(t, err)
				err = db.SealBlock(createHash(13), eth.BlockID{Hash: createHash(14), Number: 14}, 5001)
				require.NoError(t, err)
				err = db.SealBlock(createHash(14), eth.BlockID{Hash: createHash(15), Number: 15}, 5002)
				require.NoError(t, err)
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				onto := eth.BlockID{Hash: createHash(14), Number: 14}
				err := db.AddLog(createHash(1), onto, 0, nil)
				require.ErrorIs(t, err, types.ErrOutOfOrder, "cannot build logs on 14 when 15 is already sealed")
				require.ErrorIs(t, err, types.ErrOutOfOrder, "cannot build logs on 14 when 15 is already sealed")
			})
	})

	t.Run("ErrorWhenBeforeCurrentLogEvent", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl15 := eth.BlockID{Hash: createHash(15), Number: 15}
				err := db.lastEntryContext.forceBlock(bl15, 5000)
				require.NoError(t, err)
				require.NoError(t, db.AddLog(createHash(1), bl15, 0, nil))
				require.NoError(t, db.AddLog(createHash(1), bl15, 1, nil))
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl15 := eth.BlockID{Hash: createHash(15), Number: 15}
				err := db.AddLog(createHash(1), bl15, 0, nil)
				require.ErrorIs(t, err, types.ErrOutOfOrder, "already at log index 2")
				require.ErrorIs(t, err, types.ErrOutOfOrder, "already at log index 2")
			})
	})

	t.Run("ErrorWhenBeforeBlockSeal", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl15 := eth.BlockID{Hash: createHash(15), Number: 15}
				err := db.lastEntryContext.forceBlock(bl15, 5000)
				require.NoError(t, err)
				require.NoError(t, db.AddLog(createHash(1), bl15, 0, nil))
				require.NoError(t, db.AddLog(createHash(1), bl15, 1, nil))
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				err := db.AddLog(createHash(1), eth.BlockID{Hash: createHash(16), Number: 16}, 0, nil)
				require.ErrorIs(t, err, types.ErrOutOfOrder)
				require.ErrorIs(t, err, types.ErrOutOfOrder)
			})
	})

	t.Run("ErrorWhenAtCurrentLogEvent", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl15 := eth.BlockID{Hash: createHash(15), Number: 15}
				err := db.lastEntryContext.forceBlock(bl15, 5000)
				require.NoError(t, err)
				require.NoError(t, db.AddLog(createHash(1), bl15, 0, nil))
				require.NoError(t, db.AddLog(createHash(1), bl15, 1, nil))
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl15 := eth.BlockID{Hash: createHash(15), Number: 15}
				err := db.AddLog(createHash(1), bl15, 1, nil)
				require.ErrorIs(t, err, types.ErrOutOfOrder, "already at log index 2")
				require.ErrorIs(t, err, types.ErrOutOfOrder, "already at log index 2")
			})
	})

	t.Run("ErrorWhenAtCurrentLogEventButAfterLastCheckpoint", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl15 := eth.BlockID{Hash: createHash(15), Number: 15}
				err := db.lastEntryContext.forceBlock(bl15, 5000)
				require.NoError(t, err)
				require.NoError(t, db.AddLog(createHash(1), bl15, 0, nil))
				require.NoError(t, db.AddLog(createHash(1), bl15, 1, nil))
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl15 := eth.BlockID{Hash: createHash(16), Number: 16}
				err := db.AddLog(createHash(1), bl15, 2, nil)
				require.ErrorIs(t, err, types.ErrOutOfOrder)
				require.ErrorIs(t, err, types.ErrOutOfOrder)
			})
	})

	t.Run("ErrorWhenSkippingLogEvent", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl15 := eth.BlockID{Hash: createHash(15), Number: 15}
				err := db.lastEntryContext.forceBlock(bl15, 5000)
				require.NoError(t, err)
				require.NoError(t, db.AddLog(createHash(1), bl15, 0, nil))
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl15 := eth.BlockID{Hash: createHash(15), Number: 15}
				err := db.AddLog(createHash(1), bl15, 2, nil)
				require.ErrorIs(t, err, types.ErrOutOfOrder)
				require.ErrorIs(t, err, types.ErrOutOfOrder)
			})
	})

	t.Run("ErrorWhenFirstLogIsNotLogIdxZero", func(t *testing.T) {
		runDBTest(t, func(t *testing.T, db *DB, m *stubMetrics) {
			bl15 := eth.BlockID{Hash: createHash(15), Number: 15}
			err := db.lastEntryContext.forceBlock(bl15, 5000)
			require.NoError(t, err)
		},
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl15 := eth.BlockID{Hash: createHash(15), Number: 15}
				err := db.AddLog(createHash(1), bl15, 5, nil)
				require.ErrorIs(t, err, types.ErrOutOfOrder)
				require.ErrorIs(t, err, types.ErrOutOfOrder)
			})
	})

	t.Run("ErrorWhenFirstLogOfNewBlockIsNotLogIdxZero", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl15 := eth.BlockID{Hash: createHash(15), Number: 15}
				err := db.lastEntryContext.forceBlock(bl15, 5000)
				require.NoError(t, err)
				err = db.AddLog(createHash(1), bl15, 0, nil)
				require.NoError(t, err)
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl15 := eth.BlockID{Hash: createHash(15), Number: 15}
				err := db.AddLog(createHash(1), bl15, 1, nil)
				require.NoError(t, err)
				bl16 := eth.BlockID{Hash: createHash(16), Number: 16}
				err = db.SealBlock(bl15.Hash, bl16, 5001)
				require.NoError(t, err)
				err = db.AddLog(createHash(1), bl16, 1, nil)
				require.ErrorIs(t, err, types.ErrOutOfOrder)
				require.ErrorIs(t, err, types.ErrOutOfOrder)
			})
	})

	t.Run("MultipleSearchCheckpoints", func(t *testing.T) {
		block0 := eth.BlockID{Hash: createHash(10), Number: 10}
		block1 := eth.BlockID{Hash: createHash(11), Number: 11}
		block2 := eth.BlockID{Hash: createHash(12), Number: 12}
		block3 := eth.BlockID{Hash: createHash(13), Number: 13}
		block4 := eth.BlockID{Hash: createHash(14), Number: 14}
		// Ignoring seal-checkpoints in checkpoint counting comments here;
		// First search-checkpoint is at entry idx 0
		// Block 1 logs don't reach the second search-checkpoint
		block1LogCount := searchCheckpointFrequency - 10
		// Block 2 logs extend to just after the third search-checkpoint
		block2LogCount := searchCheckpointFrequency + 16
		// Block 3 logs extend to immediately before the fourth search-checkpoint
		block3LogCount := searchCheckpointFrequency - 19
		block4LogCount := 2
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				// force in block 0
				require.NoError(t, db.lastEntryContext.forceBlock(block0, 3000))
				expectedIndex := entrydb.EntryIdx(2)
				t.Logf("block 0 complete, at entry %d", db.lastEntryContext.NextIndex())
				require.Equal(t, expectedIndex, db.lastEntryContext.NextIndex())
				{ // create block 1
					for i := 0; i < block1LogCount; i++ {
						err := db.AddLog(createHash(i), block0, uint32(i), nil)
						require.NoError(t, err)
					}
					err := db.SealBlock(block0.Hash, block1, 3001) // second seal-checkpoint
					require.NoError(t, err)
				}
				expectedIndex += entrydb.EntryIdx(block1LogCount) + 2
				t.Logf("block 1 complete, at entry %d", db.lastEntryContext.NextIndex())
				require.Equal(t, expectedIndex, db.lastEntryContext.NextIndex(), "added logs and a seal checkpoint")
				{ // create block 2
					for i := 0; i < block2LogCount; i++ {
						// two of these imply a search checkpoint, the second and third search-checkpoint
						err := db.AddLog(createHash(i), block1, uint32(i), nil)
						require.NoError(t, err)
					}
					err := db.SealBlock(block1.Hash, block2, 3002) // third seal-checkpoint
					require.NoError(t, err)
				}
				expectedIndex += entrydb.EntryIdx(block2LogCount) + 2 + 2 + 2
				t.Logf("block 2 complete, at entry %d", db.lastEntryContext.NextIndex())
				require.Equal(t, expectedIndex, db.lastEntryContext.NextIndex(), "added logs, two search checkpoints, and a seal checkpoint")
				{ // create block 3
					for i := 0; i < block3LogCount; i++ {
						err := db.AddLog(createHash(i), block2, uint32(i), nil)
						require.NoError(t, err)
					}
					err := db.SealBlock(block2.Hash, block3, 3003)
					require.NoError(t, err)
				}
				expectedIndex += entrydb.EntryIdx(block3LogCount) + 2
				t.Logf("block 3 complete, at entry %d", db.lastEntryContext.NextIndex())
				require.Equal(t, expectedIndex, db.lastEntryContext.NextIndex(), "added logs, and a seal checkpoint")

				// Verify that we're right before the fourth search-checkpoint will be written.
				// entryCount is the number of entries, so given 0 based indexing is the index of the next entry
				// the first checkpoint is at entry 0, the second at entry searchCheckpointFrequency etc
				// so the fourth is at entry 3*searchCheckpointFrequency.
				require.EqualValues(t, 3*searchCheckpointFrequency-1, m.entryCount)
				{ // create block 4
					for i := 0; i < block4LogCount; i++ {
						// includes a fourth search checkpoint
						err := db.AddLog(createHash(i), block3, uint32(i), nil)
						require.NoError(t, err)
					}
					err := db.SealBlock(block3.Hash, block4, 3003) // fourth seal checkpoint
					require.NoError(t, err)
				}
				expectedIndex += entrydb.EntryIdx(block4LogCount) + 2 + 2
				require.Equal(t, expectedIndex, db.lastEntryContext.NextIndex(), "added logs, a search checkpoint, and a seal checkpoint")
				t.Logf("block 4 complete, at entry %d", db.lastEntryContext.NextIndex())
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				// Check that we wrote additional search checkpoints and seal checkpoints
				expectedCheckpointCount := 4 + 4
				expectedEntryCount := block1LogCount + block2LogCount + block3LogCount + block4LogCount + (2 * expectedCheckpointCount)
				require.EqualValues(t, expectedEntryCount, m.entryCount)
				// Check we can find all the logs.
				for i := 0; i < block1LogCount; i++ {
					requireContains(t, db, block1.Number, uint32(i), createHash(i))
				}
				// Block 2 logs extend to just after the third checkpoint
				for i := 0; i < block2LogCount; i++ {
					requireContains(t, db, block2.Number, uint32(i), createHash(i))
				}
				// Block 3 logs extend to immediately before the fourth checkpoint
				for i := 0; i < block3LogCount; i++ {
					requireContains(t, db, block3.Number, uint32(i), createHash(i))
				}
				// Block 4 logs start immediately after the fourth checkpoint
				for i := 0; i < block4LogCount; i++ {
					requireContains(t, db, block4.Number, uint32(i), createHash(i))
				}
			})
	})
}

func TestAddDependentLog(t *testing.T) {
	execMsg := types.ExecutingMessage{
		Chain:     3,
		BlockNum:  42894,
		LogIdx:    42,
		Timestamp: 8742482,
		Hash:      createHash(8844),
	}
	t.Run("FirstEntry", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl15 := eth.BlockID{Hash: createHash(15), Number: 15}
				require.NoError(t, db.lastEntryContext.forceBlock(bl15, 5000))
				err := db.AddLog(createHash(1), bl15, 0, &execMsg)
				require.NoError(t, err)
				bl16 := eth.BlockID{Hash: createHash(16), Number: 16}
				require.NoError(t, db.SealBlock(bl15.Hash, bl16, 5002))
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				requireContains(t, db, 16, 0, createHash(1), execMsg)
			})
	})

	t.Run("BlockSealSearchCheckpointOverlap", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl15 := eth.BlockID{Hash: createHash(15), Number: 15}
				require.NoError(t, db.lastEntryContext.forceBlock(bl15, 5000))
				for i := uint32(0); m.entryCount < searchCheckpointFrequency-1; i++ {
					require.NoError(t, db.AddLog(createHash(9), bl15, i, nil))
				}
				bl16 := eth.BlockID{Hash: createHash(16), Number: 16}
				require.NoError(t, db.SealBlock(bl15.Hash, bl16, 5001))
				// added 3 entries: seal-checkpoint, then a search-checkpoint, then the canonical hash
				require.Equal(t, m.entryCount, int64(searchCheckpointFrequency+2))
				err := db.AddLog(createHash(1), bl16, 0, &execMsg)
				require.NoError(t, err)
				bl17 := eth.BlockID{Hash: createHash(17), Number: 17}
				require.NoError(t, db.SealBlock(bl16.Hash, bl17, 5002))
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				requireContains(t, db, 16, 0, createHash(9))
				requireContains(t, db, 17, 0, createHash(1), execMsg)
			})
	})

	t.Run("AvoidCheckpointOverlapWithExecutingCheck", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl15 := eth.BlockID{Hash: createHash(15), Number: 15}
				require.NoError(t, db.lastEntryContext.forceBlock(bl15, 5000))
				// we add 256 - 2 (start) - 2 (init msg, exec link) = 252 entries
				for i := uint32(0); i < 252; i++ {
					require.NoError(t, db.AddLog(createHash(9), bl15, i, nil))
				}
				// add an executing message
				err := db.AddLog(createHash(1), bl15, 252, &execMsg)
				require.NoError(t, err)
				// 0,1: start
				// 2..252+2: initiating logs without exec message
				// 254 = inferred padding - 3 entries for exec msg would overlap with checkpoint
				// 255 = inferred padding
				// 256 = search checkpoint - what would be the exec check without padding
				// 257 = canonical hash
				// 258 = initiating message
				// 259 = executing message link
				// 260 = executing message check
				require.Equal(t, int64(261), m.entryCount)
				db.debugTip()
				bl16 := eth.BlockID{Hash: createHash(16), Number: 16}
				require.NoError(t, db.SealBlock(bl15.Hash, bl16, 5001))
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				requireContains(t, db, 16, 251, createHash(9))
				requireContains(t, db, 16, 252, createHash(1), execMsg)
			})
	})

	t.Run("AvoidCheckpointOverlapWithExecutingLink", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl15 := eth.BlockID{Hash: createHash(15), Number: 15}
				require.NoError(t, db.lastEntryContext.forceBlock(bl15, 5000))
				// we add 256 - 2 (start) - 1 (init msg) = 253 entries
				for i := uint32(0); i < 253; i++ {
					require.NoError(t, db.AddLog(createHash(9), bl15, i, nil))
				}
				// add an executing message
				err := db.AddLog(createHash(1), bl15, 253, &execMsg)
				require.NoError(t, err)
				// 0,1: start
				// 2..253+2: initiating logs without exec message
				// 255 = inferred padding - 3 entries for exec msg would overlap with checkpoint
				// 256 = search checkpoint - what would be the exec link without padding
				// 257 = canonical hash
				// 258 = initiating message
				// 259 = executing message link
				// 260 = executing message check
				db.debugTip()
				require.Equal(t, int64(261), m.entryCount)
				bl16 := eth.BlockID{Hash: createHash(16), Number: 16}
				require.NoError(t, db.SealBlock(bl15.Hash, bl16, 5001))
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				requireContains(t, db, 16, 252, createHash(9))
				requireContains(t, db, 16, 253, createHash(1), execMsg)
			})
	})
}

func TestContains(t *testing.T) {
	runDBTest(t,
		func(t *testing.T, db *DB, m *stubMetrics) {
			bl50 := eth.BlockID{Hash: createHash(50), Number: 50}
			require.NoError(t, db.lastEntryContext.forceBlock(bl50, 5000))
			require.NoError(t, db.AddLog(createHash(1), bl50, 0, nil))
			require.NoError(t, db.AddLog(createHash(3), bl50, 1, nil))
			require.NoError(t, db.AddLog(createHash(2), bl50, 2, nil))
			bl51 := eth.BlockID{Hash: createHash(51), Number: 51}
			require.NoError(t, db.SealBlock(bl50.Hash, bl51, 5001))
			bl52 := eth.BlockID{Hash: createHash(52), Number: 52}
			require.NoError(t, db.SealBlock(bl51.Hash, bl52, 5001))
			require.NoError(t, db.AddLog(createHash(1), bl52, 0, nil))
			require.NoError(t, db.AddLog(createHash(3), bl52, 1, nil))
		},
		func(t *testing.T, db *DB, m *stubMetrics) {
			// Should find added logs
			requireContains(t, db, 51, 0, createHash(1))
			requireContains(t, db, 51, 1, createHash(3))
			requireContains(t, db, 51, 2, createHash(2))
			requireFuture(t, db, 53, 0, createHash(1))
			requireFuture(t, db, 53, 1, createHash(3))

			// 52 was sealed as empty
			requireConflicts(t, db, 52, 0, createHash(1))

			// 53 only contained 2 logs, not 3, and is not sealed yet
			requireFuture(t, db, 53, 2, createHash(3))
			// 54 doesn't exist yet
			requireFuture(t, db, 54, 0, createHash(3))

			// 51 only contained 3 logs, not 4
			requireConflicts(t, db, 51, 3, createHash(2))
		})
}

func TestExecutes(t *testing.T) {
	execMsg1 := types.ExecutingMessage{
		Chain:     33,
		BlockNum:  22,
		LogIdx:    99,
		Timestamp: 948294,
		Hash:      createHash(332299),
	}
	execMsg2 := types.ExecutingMessage{
		Chain:     44,
		BlockNum:  55,
		LogIdx:    66,
		Timestamp: 77777,
		Hash:      createHash(445566),
	}
	execMsg3 := types.ExecutingMessage{
		Chain:     77,
		BlockNum:  88,
		LogIdx:    89,
		Timestamp: 6578567,
		Hash:      createHash(778889),
	}
	runDBTest(t,
		func(t *testing.T, db *DB, m *stubMetrics) {
			bl50 := eth.BlockID{Hash: createHash(50), Number: 50}
			require.NoError(t, db.lastEntryContext.forceBlock(bl50, 500))
			require.NoError(t, db.AddLog(createHash(1), bl50, 0, nil))
			require.NoError(t, db.AddLog(createHash(3), bl50, 1, &execMsg1))
			require.NoError(t, db.AddLog(createHash(2), bl50, 2, nil))
			bl51 := eth.BlockID{Hash: createHash(51), Number: 51}
			require.NoError(t, db.SealBlock(bl50.Hash, bl51, 5001))
			bl52 := eth.BlockID{Hash: createHash(52), Number: 52}
			require.NoError(t, db.SealBlock(bl51.Hash, bl52, 5001))
			require.NoError(t, db.AddLog(createHash(1), bl52, 0, &execMsg2))
			require.NoError(t, db.AddLog(createHash(3), bl52, 1, &execMsg3))
		},
		func(t *testing.T, db *DB, m *stubMetrics) {
			// Should find added logs
			requireExecutingMessage(t, db, 51, 0, types.ExecutingMessage{})
			requireExecutingMessage(t, db, 51, 1, execMsg1)
			requireExecutingMessage(t, db, 51, 2, types.ExecutingMessage{})
			requireExecutingMessage(t, db, 53, 0, execMsg2)
			requireExecutingMessage(t, db, 53, 1, execMsg3)

			// 52 was sealed without logs
			requireConflicts(t, db, 52, 0, createHash(1))

			// 53 only contained 2 logs, not 3, and is not sealed yet
			requireFuture(t, db, 53, 2, createHash(3))
			// 54 doesn't exist yet
			requireFuture(t, db, 54, 0, createHash(3))

			// 51 only contained 3 logs, not 4
			requireConflicts(t, db, 51, 3, createHash(2))

			// 51 contains an executing message, and 2 other non-executing logs
			ref, logCount, execMsgs, err := db.OpenBlock(51)
			require.NoError(t, err)
			require.Len(t, execMsgs, 1)
			require.Equal(t, &execMsg1, execMsgs[1])
			require.Equal(t, uint32(3), logCount)
			require.Equal(t, eth.BlockRef{Hash: createHash(51), Number: 51, ParentHash: createHash(50), Time: 5001}, ref)
		})
}

func TestGetBlockInfo(t *testing.T) {
	t.Run("ReturnsErrFutureWhenEmpty", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {},
			func(t *testing.T, db *DB, m *stubMetrics) {
				_, err := db.FindSealedBlock(10)
				require.ErrorIs(t, err, types.ErrFuture)
				_, err = db.FindSealedBlock(10)
				require.ErrorIs(t, err, types.ErrFuture)
			})
	})

	t.Run("ReturnsErrFutureWhenRequestedBlockBeforeFirstSearchCheckpoint", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl11 := eth.BlockID{Hash: createHash(11), Number: 11}
				require.NoError(t, db.lastEntryContext.forceBlock(bl11, 500))
				err := db.AddLog(createHash(1), bl11, 0, nil)
				require.NoError(t, err)
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				// if the DB starts at 11, then shouldn't find 10
				_, err := db.FindSealedBlock(10)
				require.ErrorIs(t, err, types.ErrSkipped)
				_, err = db.FindSealedBlock(10)
				require.ErrorIs(t, err, types.ErrSkipped)
			})
	})

	t.Run("ReturnFirstBlockInfo", func(t *testing.T) {
		block := eth.BlockID{Hash: createHash(11), Number: 11}
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				require.NoError(t, db.SealBlock(common.Hash{}, block, 500))
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				_, err := db.FindSealedBlock(block.Number)
				require.NoError(t, err)
				seal, err := db.FindSealedBlock(block.Number)
				require.NoError(t, err)
				require.Equal(t, block, seal.ID())
				require.Equal(t, uint64(500), seal.Timestamp)
				require.Equal(t, block, seal.ID())
				require.Equal(t, uint64(500), seal.Timestamp)
			})
	})
}

func requireContains(t *testing.T, db *DB, blockNum uint64, logIdx uint32, logHash common.Hash, execMsg ...types.ExecutingMessage) {
	require.LessOrEqual(t, len(execMsg), 1, "cannot have multiple executing messages for a single log")
	m, ok := db.m.(*stubMetrics)
	require.True(t, ok, "Did not get the expected metrics type")
	_, err := db.Contains(blockNum, logIdx, logHash)
	require.NoErrorf(t, err, "Error searching for log %v in block %v", logIdx, blockNum)
	require.LessOrEqual(t, m.entriesReadForSearch, int64(searchCheckpointFrequency*2), "Should not need to read more than between two checkpoints")
	require.NotZero(t, m.entriesReadForSearch, "Must read at least some entries to find the log")

	var expectedExecMsg types.ExecutingMessage
	if len(execMsg) == 1 {
		expectedExecMsg = execMsg[0]
	}
	requireExecutingMessage(t, db, blockNum, logIdx, expectedExecMsg)
}

func requireConflicts(t *testing.T, db *DB, blockNum uint64, logIdx uint32, logHash common.Hash) {
	m, ok := db.m.(*stubMetrics)
	require.True(t, ok, "Did not get the expected metrics type")
	_, err := db.Contains(blockNum, logIdx, logHash)
	require.ErrorIs(t, err, types.ErrConflict, "canonical chain must not include this log")
	require.ErrorIs(t, err, types.ErrConflict, "canonical chain must not include this log")
	require.LessOrEqual(t, m.entriesReadForSearch, int64(searchCheckpointFrequency*2), "Should not need to read more than between two checkpoints")
}

func requireFuture(t *testing.T, db *DB, blockNum uint64, logIdx uint32, logHash common.Hash) {
	m, ok := db.m.(*stubMetrics)
	require.True(t, ok, "Did not get the expected metrics type")
	_, err := db.Contains(blockNum, logIdx, logHash)
	require.ErrorIs(t, err, types.ErrFuture, "canonical chain does not yet include this log")
	require.ErrorIs(t, err, types.ErrFuture, "canonical chain does not yet include this log")
	require.LessOrEqual(t, m.entriesReadForSearch, int64(searchCheckpointFrequency*2), "Should not need to read more than between two checkpoints")
}

func requireExecutingMessage(t *testing.T, db *DB, blockNum uint64, logIdx uint32, execMsg types.ExecutingMessage) {
	m, ok := db.m.(*stubMetrics)
	require.True(t, ok, "Did not get the expected metrics type")
	_, iter, err := db.findLogInfo(blockNum, logIdx)
	require.NoError(t, err, "Error when searching for executing message")
	actualExecMsg := iter.ExecMessage() // non-nil if not just an initiating message, but also an executing message
	if execMsg == (types.ExecutingMessage{}) {
		require.Nil(t, actualExecMsg)
	} else {
		require.NotNil(t, actualExecMsg)
		require.Equal(t, execMsg, *actualExecMsg, "Should return matching executing message")
	}
	require.LessOrEqual(t, m.entriesReadForSearch, int64(searchCheckpointFrequency*2), "Should not need to read more than between two checkpoints")
	require.NotZero(t, m.entriesReadForSearch, "Must read at least some entries to find the log")
}

func TestRecoverOnCreate(t *testing.T) {
	createDb := func(t *testing.T, store *entrydb.MemEntryStore[EntryType, Entry]) (*DB, *stubMetrics, error) {
		logger := testlog.Logger(t, log.LvlInfo)
		m := &stubMetrics{}
		db, err := NewFromEntryStore(logger, m, store, true)
		return db, m, err
	}

	storeWithEvents := func(evts ...Entry) *entrydb.MemEntryStore[EntryType, Entry] {
		store := &entrydb.MemEntryStore[EntryType, Entry]{}
		_ = store.Append(evts...)
		return store
	}
	t.Run("NoTruncateWhenLastEntryIsLogWithNoExecMessageSealed", func(t *testing.T) {
		store := storeWithEvents(
			// seal 0, 1, 2, 3
			newSearchCheckpoint(0, 0, 100).encode(),
			newCanonicalHash(createHash(300)).encode(),
			newSearchCheckpoint(1, 0, 101).encode(),
			newCanonicalHash(createHash(301)).encode(),
			newSearchCheckpoint(2, 0, 102).encode(),
			newCanonicalHash(createHash(302)).encode(),
			newSearchCheckpoint(3, 0, 103).encode(),
			newCanonicalHash(createHash(303)).encode(),
			// open and seal 4
			newInitiatingEvent(createHash(1), false).encode(),
			newSearchCheckpoint(4, 0, 104).encode(),
			newCanonicalHash(createHash(304)).encode(),
		)
		db, m, err := createDb(t, store)
		require.NoError(t, err)
		require.EqualValues(t, int64(4*2+3), m.entryCount)
		requireContains(t, db, 4, 0, createHash(1))
	})

	t.Run("NoTruncateWhenLastEntryIsExecutingCheckSealed", func(t *testing.T) {
		execMsg := types.ExecutingMessage{
			Chain:     4,
			BlockNum:  10,
			LogIdx:    4,
			Timestamp: 1288,
			Hash:      createHash(4),
		}
		linkEvt, err := newExecutingLink(execMsg)
		require.NoError(t, err)
		store := storeWithEvents(
			newSearchCheckpoint(0, 0, 100).encode(),
			newCanonicalHash(createHash(300)).encode(),
			newSearchCheckpoint(1, 0, 101).encode(),
			newCanonicalHash(createHash(301)).encode(),
			newSearchCheckpoint(2, 0, 102).encode(),
			newCanonicalHash(createHash(302)).encode(),
			newInitiatingEvent(createHash(1111), true).encode(),
			linkEvt.encode(),
			newExecutingCheck(execMsg.Hash).encode(),
			newSearchCheckpoint(3, 0, 103).encode(),
			newCanonicalHash(createHash(303)).encode(),
		)
		db, m, err := createDb(t, store)
		require.NoError(t, err)
		require.EqualValues(t, int64(3*2+5), m.entryCount)
		requireContains(t, db, 3, 0, createHash(1111), execMsg)
	})

	t.Run("TruncateWhenLastEntrySearchCheckpoint", func(t *testing.T) {
		// A checkpoint, without a canonical blockhash, is useless, and thus truncated.
		store := storeWithEvents(
			newSearchCheckpoint(0, 0, 100).encode())
		_, m, err := createDb(t, store)
		require.NoError(t, err)
		require.EqualValues(t, int64(0), m.entryCount)
	})

	t.Run("NoTruncateWhenLastEntryCanonicalHash", func(t *testing.T) {
		// A completed seal is fine to have as last entry.
		store := storeWithEvents(
			newSearchCheckpoint(0, 0, 100).encode(),
			newCanonicalHash(createHash(344)).encode(),
		)
		_, m, err := createDb(t, store)
		require.NoError(t, err)
		require.EqualValues(t, int64(2), m.entryCount)
	})

	t.Run("TruncateWhenLastEntryInitEventWithExecMsg", func(t *testing.T) {
		// An initiating event that claims an executing message,
		// without said executing message, is dropped.
		store := storeWithEvents(
			newSearchCheckpoint(0, 0, 100).encode(),
			newCanonicalHash(createHash(344)).encode(),
			// both pruned because we go back to a seal
			newInitiatingEvent(createHash(0), false).encode(),
			newInitiatingEvent(createHash(1), true).encode(),
		)
		_, m, err := createDb(t, store)
		require.NoError(t, err)
		require.EqualValues(t, int64(2), m.entryCount)
	})

	t.Run("NoTruncateWhenLastEntrySealed", func(t *testing.T) {
		// An initiating event that claims an executing message,
		// without said executing message, is dropped.
		store := storeWithEvents(
			newSearchCheckpoint(0, 0, 100).encode(),
			newCanonicalHash(createHash(300)).encode(),
			// pruned because we go back to a seal
			newInitiatingEvent(createHash(0), false).encode(),
			newSearchCheckpoint(1, 0, 100).encode(),
			newCanonicalHash(createHash(301)).encode(),
		)
		_, m, err := createDb(t, store)
		require.NoError(t, err)
		require.EqualValues(t, int64(5), m.entryCount)
	})

	t.Run("TruncateWhenLastEntryInitEventWithExecLink", func(t *testing.T) {
		execMsg := types.ExecutingMessage{
			Chain:     4,
			BlockNum:  10,
			LogIdx:    4,
			Timestamp: 1288,
			Hash:      createHash(4),
		}
		linkEvt, err := newExecutingLink(execMsg)
		require.NoError(t, err)
		store := storeWithEvents(
			newSearchCheckpoint(3, 0, 100).encode(),
			newCanonicalHash(createHash(344)).encode(),
			newInitiatingEvent(createHash(1), true).encode(),
			linkEvt.encode(),
		)
		_, m, err := createDb(t, store)
		require.NoError(t, err)
		require.EqualValues(t, int64(2), m.entryCount)
	})
}

func TestRewind(t *testing.T) {
	t.Run("WhenEmpty", func(t *testing.T) {
		runDBTest(t, func(t *testing.T, db *DB, m *stubMetrics) {},
			func(t *testing.T, db *DB, m *stubMetrics) {
				require.ErrorIs(t, db.Rewind(100), types.ErrFuture)
				require.ErrorIs(t, db.Rewind(100), types.ErrFuture)
				// Genesis is a block to, not present in an empty DB
				require.ErrorIs(t, db.Rewind(0), types.ErrFuture)
				require.ErrorIs(t, db.Rewind(0), types.ErrFuture)
			})
	})

	t.Run("AfterLastBlock", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl50 := eth.BlockID{Hash: createHash(50), Number: 50}
				require.NoError(t, db.SealBlock(createHash(49), bl50, 500))
				require.NoError(t, db.AddLog(createHash(1), bl50, 0, nil))
				require.NoError(t, db.AddLog(createHash(2), bl50, 1, nil))
				bl51 := eth.BlockID{Hash: createHash(51), Number: 51}
				require.NoError(t, db.SealBlock(bl50.Hash, bl51, 502))
				require.NoError(t, db.AddLog(createHash(3), bl51, 0, nil))
				bl52 := eth.BlockID{Hash: createHash(52), Number: 52}
				require.NoError(t, db.SealBlock(bl51.Hash, bl52, 504))
				require.NoError(t, db.AddLog(createHash(4), bl52, 0, nil))
				// cannot rewind to a block that is not sealed yet
				require.ErrorIs(t, db.Rewind(53), types.ErrFuture)
				require.ErrorIs(t, db.Rewind(53), types.ErrFuture)
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				requireContains(t, db, 51, 0, createHash(1))
				requireContains(t, db, 51, 1, createHash(2))
				requireContains(t, db, 52, 0, createHash(3))
				// Still have the pending log of unsealed block if the rewind to unknown sealed block fails
				requireFuture(t, db, 53, 0, createHash(4))
			})
	})

	t.Run("BeforeFirstBlock", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl50 := eth.BlockID{Hash: createHash(50), Number: 50}
				require.NoError(t, db.SealBlock(createHash(49), bl50, 500))
				require.NoError(t, db.AddLog(createHash(1), bl50, 0, nil))
				require.NoError(t, db.AddLog(createHash(2), bl50, 1, nil))
				// cannot go back to an unknown block
				require.ErrorIs(t, db.Rewind(25), types.ErrSkipped)
				require.ErrorIs(t, db.Rewind(25), types.ErrSkipped)
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				// block 51 is not sealed yet
				requireFuture(t, db, 51, 0, createHash(1))
				requireFuture(t, db, 51, 0, createHash(1))
			})
	})

	t.Run("AtFirstBlock", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl50 := eth.BlockID{Hash: createHash(50), Number: 50}
				require.NoError(t, db.SealBlock(createHash(49), bl50, 500))
				require.NoError(t, db.AddLog(createHash(1), bl50, 0, nil))
				require.NoError(t, db.AddLog(createHash(2), bl50, 1, nil))
				bl51 := eth.BlockID{Hash: createHash(51), Number: 51}
				require.NoError(t, db.SealBlock(bl50.Hash, bl51, 502))
				require.NoError(t, db.AddLog(createHash(1), bl51, 0, nil))
				require.NoError(t, db.AddLog(createHash(2), bl51, 1, nil))
				bl52 := eth.BlockID{Hash: createHash(52), Number: 52}
				require.NoError(t, db.SealBlock(bl51.Hash, bl52, 504))
				require.NoError(t, db.Rewind(51))
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				requireContains(t, db, 51, 0, createHash(1))
				requireContains(t, db, 51, 1, createHash(2))
				requireFuture(t, db, 52, 0, createHash(1))
				requireFuture(t, db, 52, 1, createHash(2))
			})
	})

	t.Run("AfterSecondCheckpoint", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl50 := eth.BlockID{Hash: createHash(50), Number: 50}
				require.NoError(t, db.SealBlock(createHash(49), bl50, 500))
				for i := uint32(0); m.entryCount < searchCheckpointFrequency; i++ {
					require.NoError(t, db.AddLog(createHash(1), bl50, i, nil))
				}
				// The checkpoint is added automatically,
				// it will be there as soon as it reaches 255 with log events.
				// Thus add 2 for the checkpoint.
				require.EqualValues(t, searchCheckpointFrequency+2, m.entryCount)
				bl51 := eth.BlockID{Hash: createHash(51), Number: 51}
				require.NoError(t, db.SealBlock(bl50.Hash, bl51, 502))
				require.NoError(t, db.AddLog(createHash(1), bl51, 0, nil))
				require.EqualValues(t, searchCheckpointFrequency+2+3, m.entryCount, "Should have inserted new checkpoint and extra log")
				require.NoError(t, db.AddLog(createHash(2), bl51, 1, nil))
				bl52 := eth.BlockID{Hash: createHash(52), Number: 52}
				require.NoError(t, db.SealBlock(bl51.Hash, bl52, 504))
				require.NoError(t, db.Rewind(51))
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				require.EqualValues(t, searchCheckpointFrequency+2+2, m.entryCount, "Should have deleted second checkpoint")
				requireContains(t, db, 51, 0, createHash(1))
				requireContains(t, db, 51, 1, createHash(1))
				requireFuture(t, db, 52, 0, createHash(1))
				requireFuture(t, db, 52, 1, createHash(2))
			})
	})

	t.Run("BetweenBlockEntries", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				// create many blocks, and all the odd blocks get 2 logs
				for i := uint32(0); i < 30; i++ {
					bl := eth.BlockID{Hash: createHash(int(i)), Number: uint64(i)}
					require.NoError(t, db.SealBlock(createHash(int(i)-1), bl, 500+uint64(i)))
					if i%2 == 0 {
						require.NoError(t, db.AddLog(createHash(1), bl, 0, nil))
						require.NoError(t, db.AddLog(createHash(2), bl, 1, nil))
					}
				}
				require.NoError(t, db.Rewind(15))
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				requireContains(t, db, 15, 0, createHash(1))
				requireContains(t, db, 15, 1, createHash(2))
				requireFuture(t, db, 16, 0, createHash(1))
				requireFuture(t, db, 16, 1, createHash(2))
			})
	})

	t.Run("AtLastEntry", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				// create many blocks, and all the even blocks get 2 logs
				for i := uint32(0); i <= 30; i++ {
					bl := eth.BlockID{Hash: createHash(int(i)), Number: uint64(i)}
					require.NoError(t, db.SealBlock(createHash(int(i)-1), bl, 500+uint64(i)))
					if i%2 == 1 {
						require.NoError(t, db.AddLog(createHash(1), bl, 0, nil))
						require.NoError(t, db.AddLog(createHash(2), bl, 1, nil))
					}
				}
				// We ended at 30, and sealed it, nothing left to prune
				require.NoError(t, db.Rewind(30))
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				requireContains(t, db, 20, 0, createHash(1))
				requireContains(t, db, 20, 1, createHash(2))
				// built on top of 29, these are in sealed block 30, still around
				requireContains(t, db, 30, 0, createHash(1))
				requireContains(t, db, 30, 1, createHash(2))
			})
	})

	t.Run("ReadDeletedBlocks", func(t *testing.T) {
		runDBTest(t,
			func(t *testing.T, db *DB, m *stubMetrics) {
				// create many blocks, and all the odd blocks get 2 logs
				for i := uint32(0); i < 30; i++ {
					bl := eth.BlockID{Hash: createHash(int(i)), Number: uint64(i)}
					require.NoError(t, db.SealBlock(createHash(int(i)-1), bl, 500+uint64(i)))
					if i%2 == 0 {
						require.NoError(t, db.AddLog(createHash(1), bl, 0, nil))
						require.NoError(t, db.AddLog(createHash(2), bl, 1, nil))
					}
				}
				require.NoError(t, db.Rewind(16))
			},
			func(t *testing.T, db *DB, m *stubMetrics) {
				bl29 := eth.BlockID{Hash: createHash(29), Number: 29}
				// 29 was deleted
				err := db.AddLog(createHash(2), bl29, 1, nil)
				require.ErrorIs(t, err, types.ErrOutOfOrder, "Cannot add log on removed block")
				require.ErrorIs(t, err, types.ErrOutOfOrder, "Cannot add log on removed block")
				// 15 is older, we have up to 16
				bl15 := eth.BlockID{Hash: createHash(15), Number: 15}
				// try to add a third log to 15
				err = db.AddLog(createHash(10), bl15, 2, nil)
				require.ErrorIs(t, err, types.ErrOutOfOrder)
				require.ErrorIs(t, err, types.ErrOutOfOrder)
				bl16 := eth.BlockID{Hash: createHash(16), Number: 16}
				// try to add a log to 17, on top of 16
				err = db.AddLog(createHash(42), bl16, 0, nil)
				require.NoError(t, err)
				// not sealed yet
				requireFuture(t, db, 17, 0, createHash(42))
			})
	})
}

type stubMetrics struct {
	entryCount           int64
	entriesReadForSearch int64
}

func (s *stubMetrics) RecordDBEntryCount(kind string, count int64) {
	s.entryCount = count
}

func (s *stubMetrics) RecordDBSearchEntriesRead(count int64) {
	s.entriesReadForSearch = count
}

var _ Metrics = (*stubMetrics)(nil)

var _ entrydb.EntryStore[EntryType, Entry] = (*entrydb.MemEntryStore[EntryType, Entry])(nil)
