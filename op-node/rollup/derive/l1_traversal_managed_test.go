package derive

import (
	"context"
	"io"
	"math/big"
	"math/rand" // nosemgrep
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

func TestL1TraversalManaged(t *testing.T) {
	rng := rand.New(rand.NewSource(1234))
	a := testutils.RandomBlockRef(rng)
	// Load up the initial state with a reset
	l1Cfg := eth.SystemConfig{
		BatcherAddr: testutils.RandomAddress(rng),
		Overhead:    [32]byte{42},
		Scalar:      [32]byte{69},
	}
	sysCfgAddr := testutils.RandomAddress(rng)
	cfg := &rollup.Config{
		Genesis:               rollup.Genesis{SystemConfig: l1Cfg},
		L1SystemConfigAddress: sysCfgAddr,
	}
	l1F := &testutils.MockL1Source{}
	tr := NewL1TraversalManaged(testlog.Logger(t, log.LevelError), cfg, l1F)

	_ = tr.Reset(context.Background(), a, l1Cfg)

	// First call will not succeed, we count the first block as consumed-already,
	// since other stages had it too.
	ref, err := tr.NextL1Block(context.Background())
	require.ErrorIs(t, err, io.EOF)
	require.Equal(t, eth.L1BlockRef{}, ref)

	// Advancing doesn't work either, we have no data to advance to.
	require.ErrorIs(t, tr.AdvanceL1Block(context.Background()), io.EOF)

	// again, EOF until we provide the block
	ref, err = tr.NextL1Block(context.Background())
	require.Equal(t, eth.L1BlockRef{}, ref)
	require.Equal(t, io.EOF, err)

	// Now provide the next L1 block
	b := testutils.NextRandomRef(rng, a)

	// L1 block info and receipts are fetched to update the system config.
	l1F.ExpectFetchReceipts(b.Hash, &testutils.MockBlockInfo{
		InfoHash:             b.Hash,
		InfoParentHash:       b.ParentHash,
		InfoCoinbase:         common.Address{},
		InfoRoot:             common.Hash{},
		InfoNum:              b.Number,
		InfoTime:             b.Time,
		InfoMixDigest:        [32]byte{},
		InfoBaseFee:          big.NewInt(10),
		InfoBlobBaseFee:      big.NewInt(10),
		InfoReceiptRoot:      common.Hash{},
		InfoGasUsed:          0,
		InfoGasLimit:         30_000_000,
		InfoHeaderRLP:        nil,
		InfoParentBeaconRoot: nil,
	}, nil, nil)
	require.NoError(t, tr.ProvideNextL1(context.Background(), b))
	l1F.AssertExpectations(t)

	// It should provide B now
	ref, err = tr.NextL1Block(context.Background())
	require.NoError(t, err)
	require.Equal(t, b, ref)

	// And EOF again after traversing
	ref, err = tr.NextL1Block(context.Background())
	require.Equal(t, eth.L1BlockRef{}, ref)
	require.Equal(t, io.EOF, err)

}
