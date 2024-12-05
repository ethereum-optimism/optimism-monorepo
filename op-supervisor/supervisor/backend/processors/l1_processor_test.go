package processors

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

type mockChainsDB struct {
	recordNewL1Fn  func(ref eth.BlockRef) error
	lastCommonL1Fn func() (types.BlockSeal, error)
}

func (m *mockChainsDB) RecordNewL1(ref eth.BlockRef) error {
	if m.recordNewL1Fn != nil {
		return m.recordNewL1Fn(ref)
	}
	return nil
}

func (m *mockChainsDB) LastCommonL1() (types.BlockSeal, error) {
	if m.lastCommonL1Fn != nil {
		return m.lastCommonL1Fn()
	}
	return types.BlockSeal{}, nil
}

type mockL1BlockRefByNumberFetcher struct {
	l1BlockByNumberFn func() (eth.L1BlockRef, error)
}

func (m *mockL1BlockRefByNumberFetcher) L1BlockRefByNumber(context.Context, uint64) (eth.L1BlockRef, error) {
	if m.l1BlockByNumberFn != nil {
		return m.l1BlockByNumberFn()
	}
	return eth.L1BlockRef{}, nil
}

func TestL1Processor(t *testing.T) {
	processorForTesting := func() *L1Processor {
		ctx, cancel := context.WithCancel(context.Background())
		proc := &L1Processor{
			log:           testlog.Logger(t, log.LvlInfo),
			client:        &mockL1BlockRefByNumberFetcher{},
			currentNumber: 0,
			tickDuration:  1 * time.Second,
			db:            &mockChainsDB{},
			ctx:           ctx,
			cancel:        cancel,
		}
		return proc
	}
	t.Run("Initializes LastCommonL1", func(t *testing.T) {
		proc := processorForTesting()
		proc.db.(*mockChainsDB).lastCommonL1Fn = func() (types.BlockSeal, error) {
			return types.BlockSeal{Number: 10}, nil
		}
		// before starting, the current number should be 0
		require.Equal(t, uint64(0), proc.currentNumber)
		proc.Start()
		defer proc.Stop()
		// after starting, the current number should still be 0
		require.Equal(t, uint64(10), proc.currentNumber)
	})
	t.Run("Initializes LastCommonL1 at 0 if error", func(t *testing.T) {
		proc := processorForTesting()
		proc.db.(*mockChainsDB).lastCommonL1Fn = func() (types.BlockSeal, error) {
			return types.BlockSeal{Number: 10}, fmt.Errorf("error")
		}
		// before starting, the current number should be 0
		require.Equal(t, uint64(0), proc.currentNumber)
		proc.Start()
		defer proc.Stop()
		// the error means the current number should still be 0
		require.Equal(t, uint64(0), proc.currentNumber)
	})
	t.Run("Records new L1", func(t *testing.T) {
		proc := processorForTesting()
		// return a new block number each time
		num := uint64(0)
		proc.client.(*mockL1BlockRefByNumberFetcher).l1BlockByNumberFn = func() (eth.L1BlockRef, error) {
			defer func() { num++ }()
			return eth.L1BlockRef{Number: num}, nil
		}
		// confirm that recordNewL1 is called for each block number received
		called := uint64(0)
		proc.db.(*mockChainsDB).recordNewL1Fn = func(ref eth.BlockRef) error {
			require.Equal(t, called, ref.Number)
			called++
			return nil
		}
		proc.Start()
		defer proc.Stop()
		require.Eventually(t, func() bool {
			return called >= 1 && proc.currentNumber >= 1
		}, 10*time.Second, 100*time.Millisecond)

	})

}
