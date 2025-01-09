package tasks

import (
	"context"
	"errors"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestLoadOutputRoot(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		l2 := &mockL2{
			blockHash:  common.Hash{0x24},
			outputRoot: eth.Bytes32{0x11},
			safeL2:     eth.L2BlockRef{Number: 65},
		}
		result, err := loadOutputRoot(uint64(0), l2)
		require.NoError(t, err)
		assertDerivationResult(t, result, l2.safeL2, l2.blockHash, l2.outputRoot)
	})

	t.Run("Success-PriorToSafeHead", func(t *testing.T) {
		expected := eth.Bytes32{0x11}
		l2 := &mockL2{
			blockHash:  common.Hash{0x24},
			outputRoot: expected,
			safeL2: eth.L2BlockRef{
				Number: 10,
			},
		}
		result, err := loadOutputRoot(uint64(20), l2)
		require.NoError(t, err)
		require.Equal(t, uint64(10), l2.requestedOutputRoot)
		assertDerivationResult(t, result, l2.safeL2, l2.blockHash, l2.outputRoot)
	})

	t.Run("Error-SafeHead", func(t *testing.T) {
		expectedErr := errors.New("boom")
		l2 := &mockL2{
			blockHash:  common.Hash{0x24},
			outputRoot: eth.Bytes32{0x11},
			safeL2:     eth.L2BlockRef{Number: 10},
			safeL2Err:  expectedErr,
		}
		_, err := loadOutputRoot(uint64(0), l2)
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("Error-OutputRoot", func(t *testing.T) {
		expectedErr := errors.New("boom")
		l2 := &mockL2{
			blockHash:     common.Hash{0x24},
			outputRoot:    eth.Bytes32{0x11},
			outputRootErr: expectedErr,
			safeL2:        eth.L2BlockRef{Number: 10},
		}
		_, err := loadOutputRoot(uint64(0), l2)
		require.ErrorIs(t, err, expectedErr)
	})
}

func assertDerivationResult(t *testing.T, actual DerivationResult, safeHead eth.L2BlockRef, blockHash common.Hash, outputRoot eth.Bytes32) {
	require.Equal(t, safeHead, actual.SafeHead)
	require.Equal(t, blockHash, actual.BlockHash)
	require.Equal(t, outputRoot, actual.OutputRoot)
}

type mockL2 struct {
	safeL2    eth.L2BlockRef
	safeL2Err error

	blockHash     common.Hash
	outputRoot    eth.Bytes32
	outputRootErr error

	requestedOutputRoot uint64
}

func (m *mockL2) L2BlockRefByLabel(ctx context.Context, label eth.BlockLabel) (eth.L2BlockRef, error) {
	if label != eth.Safe {
		panic("unexpected usage")
	}
	if m.safeL2Err != nil {
		return eth.L2BlockRef{}, m.safeL2Err
	}
	return m.safeL2, nil
}

func (m *mockL2) L2OutputRoot(u uint64) (common.Hash, eth.Bytes32, error) {
	m.requestedOutputRoot = u
	if m.outputRootErr != nil {
		return common.Hash{}, eth.Bytes32{}, m.outputRootErr
	}
	return m.blockHash, m.outputRoot, nil
}

var _ L2Source = (*mockL2)(nil)
