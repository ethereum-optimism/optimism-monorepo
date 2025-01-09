package tasks

import (
	"context"
	"errors"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/stretchr/testify/require"
)

func TestLoadOutputRoot(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		expected := eth.Bytes32{0x11}
		l2 := &mockL2{
			outputRoot: expected,
			safeL2:     eth.L2BlockRef{Number: 65},
		}
		safeHead, outputRoot, err := loadOutputRoot(uint64(0), l2)
		require.NoError(t, err)
		require.Equal(t, l2.safeL2, safeHead)
		require.Equal(t, expected, outputRoot)
	})

	t.Run("Success-PriorToSafeHead", func(t *testing.T) {
		expected := eth.Bytes32{0x11}
		l2 := &mockL2{
			outputRoot: expected,
			safeL2: eth.L2BlockRef{
				Number: 10,
			},
		}
		safeHead, outputRoot, err := loadOutputRoot(uint64(20), l2)
		require.NoError(t, err)
		require.Equal(t, uint64(10), l2.requestedOutputRoot)
		require.Equal(t, l2.safeL2, safeHead)
		require.Equal(t, expected, outputRoot)
	})

	t.Run("Error-SafeHead", func(t *testing.T) {
		expectedErr := errors.New("boom")
		l2 := &mockL2{
			outputRoot: eth.Bytes32{0x11},
			safeL2:     eth.L2BlockRef{Number: 10},
			safeL2Err:  expectedErr,
		}
		_, _, err := loadOutputRoot(uint64(0), l2)
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("Error-OutputRoot", func(t *testing.T) {
		expectedErr := errors.New("boom")
		l2 := &mockL2{
			outputRoot:    eth.Bytes32{0x11},
			outputRootErr: expectedErr,
			safeL2:        eth.L2BlockRef{Number: 10},
		}
		_, _, err := loadOutputRoot(uint64(0), l2)
		require.ErrorIs(t, err, expectedErr)
	})
}

type mockL2 struct {
	safeL2    eth.L2BlockRef
	safeL2Err error

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

func (m *mockL2) L2OutputRoot(u uint64) (eth.Bytes32, error) {
	m.requestedOutputRoot = u
	if m.outputRootErr != nil {
		return eth.Bytes32{}, m.outputRootErr
	}
	return m.outputRoot, nil
}

var _ L2Source = (*mockL2)(nil)
