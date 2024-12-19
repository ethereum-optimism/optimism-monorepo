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

type mockController struct {
	deriveFromL1Fn func(ref eth.BlockRef) error
}

func (m *mockController) DeriveFromL1(ref eth.BlockRef) error {
	if m.deriveFromL1Fn != nil {
		return m.deriveFromL1Fn(ref)
	}
	return nil
}

type mockChainsDB struct {
	recordNewL1Fn       func(ref eth.BlockRef) error
	lastCommonL1Fn      func() (types.BlockSeal, error)
	finalizedL1Fn       func() eth.BlockRef
	updateFinalizedL1Fn func(finalized eth.BlockRef) error
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

func (m *mockChainsDB) FinalizedL1() eth.BlockRef {
	if m.finalizedL1Fn != nil {
		return m.finalizedL1Fn()
	}
	return eth.BlockRef{}
}

func (m *mockChainsDB) UpdateFinalizedL1(finalized eth.BlockRef) error {
	if m.updateFinalizedL1Fn != nil {
		return m.updateFinalizedL1Fn(finalized)
	}
	return nil
}

type mockL1BlockRefByNumberFetcher struct {
	l1BlockByNumberFn func() (eth.L1BlockRef, error)
}

func (m *mockL1BlockRefByNumberFetcher) L1BlockRefByLabel(context.Context, eth.BlockLabel) (eth.L1BlockRef, error) {
	return eth.L1BlockRef{}, nil
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
			snc:           &mockController{},
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
	t.Run("Handles new L1", func(t *testing.T) {
		proc := processorForTesting()
		// return a new block number each time
		num := uint64(0)
		proc.client.(*mockL1BlockRefByNumberFetcher).l1BlockByNumberFn = func() (eth.L1BlockRef, error) {
			defer func() { num++ }()
			return eth.L1BlockRef{Number: num}, nil
		}
		// confirm that recordNewL1 is recordCalled for each block number received
		recordCalled := uint64(0)
		proc.db.(*mockChainsDB).recordNewL1Fn = func(ref eth.BlockRef) error {
			require.Equal(t, recordCalled, ref.Number)
			recordCalled++
			return nil
		}
		// confirm that deriveFromL1 is called for each block number received
		deriveCalled := uint64(0)
		proc.snc.(*mockController).deriveFromL1Fn = func(ref eth.BlockRef) error {
			require.Equal(t, deriveCalled, ref.Number)
			deriveCalled++
			return nil
		}
		proc.Start()
		defer proc.Stop()
		// the new L1 blocks should be recorded
		require.Eventually(t, func() bool {
			return recordCalled >= 1 && proc.currentNumber >= 1
		}, 10*time.Second, 100*time.Millisecond)

		// confirm that the db record and derive call counts match
		require.Equal(t, recordCalled, deriveCalled)
	})
	t.Run("Handles L1 record error", func(t *testing.T) {
		proc := processorForTesting()
		// return a new block number each time
		num := uint64(0)
		proc.client.(*mockL1BlockRefByNumberFetcher).l1BlockByNumberFn = func() (eth.L1BlockRef, error) {
			defer func() { num++ }()
			return eth.L1BlockRef{Number: num}, nil
		}
		// confirm that recordNewL1 is recordCalled for each block number received
		recordCalled := 0
		proc.db.(*mockChainsDB).recordNewL1Fn = func(ref eth.BlockRef) error {
			recordCalled++
			return fmt.Errorf("error")
		}
		// confirm that deriveFromL1 is called for each block number received
		deriveCalled := 0
		proc.snc.(*mockController).deriveFromL1Fn = func(ref eth.BlockRef) error {
			deriveCalled++
			return nil
		}
		proc.Start()
		defer proc.Stop()
		// because the record call fails, the current number should not be updated
		require.Never(t, func() bool {
			return recordCalled >= 1 && proc.currentNumber >= 1
		}, 10*time.Second, 100*time.Millisecond)
		// confirm derive was never called because the record call failed
		require.Equal(t, 0, deriveCalled)
	})
	t.Run("Handles L1 derive error", func(t *testing.T) {
		proc := processorForTesting()
		// return a new block number each time
		num := uint64(0)
		proc.client.(*mockL1BlockRefByNumberFetcher).l1BlockByNumberFn = func() (eth.L1BlockRef, error) {
			defer func() { num++ }()
			return eth.L1BlockRef{Number: num}, nil
		}
		// confirm that recordNewL1 is recordCalled for each block number received
		recordCalled := uint64(0)
		proc.db.(*mockChainsDB).recordNewL1Fn = func(ref eth.BlockRef) error {
			require.Equal(t, recordCalled, ref.Number)
			recordCalled++
			return nil
		}
		// confirm that deriveFromL1 is called for each block number received
		deriveCalled := uint64(0)
		proc.snc.(*mockController).deriveFromL1Fn = func(ref eth.BlockRef) error {
			deriveCalled++
			return fmt.Errorf("error")
		}
		proc.Start()
		defer proc.Stop()
		// because the derive call fails, the current number should not be updated
		require.Never(t, func() bool {
			return recordCalled >= 1 && proc.currentNumber >= 1
		}, 10*time.Second, 100*time.Millisecond)
		// confirm that the db record and derive call counts match
		// (because the derive call fails after the record call)
		require.Equal(t, recordCalled, deriveCalled)
	})
	t.Run("Updates L1 Finalized", func(t *testing.T) {
		proc := processorForTesting()
		proc.db.(*mockChainsDB).finalizedL1Fn = func() eth.BlockRef {
			return eth.BlockRef{Number: 0}
		}
		proc.db.(*mockChainsDB).updateFinalizedL1Fn = func(finalized eth.BlockRef) error {
			require.Equal(t, uint64(10), finalized.Number)
			return nil
		}
		proc.handleFinalized(context.Background(), eth.BlockRef{Number: 10})
	})
	t.Run("No L1 Finalized Update for Same Number", func(t *testing.T) {
		proc := processorForTesting()
		proc.db.(*mockChainsDB).finalizedL1Fn = func() eth.BlockRef {
			return eth.BlockRef{Number: 10}
		}
		proc.db.(*mockChainsDB).updateFinalizedL1Fn = func(finalized eth.BlockRef) error {
			require.Fail(t, "should not be called")
			return nil
		}
		proc.handleFinalized(context.Background(), eth.BlockRef{Number: 10})
	})
	t.Run("No L1 Finalized Update When Behind", func(t *testing.T) {
		proc := processorForTesting()
		proc.db.(*mockChainsDB).finalizedL1Fn = func() eth.BlockRef {
			return eth.BlockRef{Number: 20}
		}
		proc.db.(*mockChainsDB).updateFinalizedL1Fn = func(finalized eth.BlockRef) error {
			require.Fail(t, "should not be called")
			return nil
		}
		proc.handleFinalized(context.Background(), eth.BlockRef{Number: 10})
	})
}
