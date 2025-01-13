package tasks

import (
	"errors"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestLoadOutputRoot(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		safeHead := eth.L2BlockRef{Number: 65}
		l2 := &mockL2{
			blockHash:  common.Hash{0x24},
			outputRoot: eth.Bytes32{0x11},
		}
		result, err := loadOutputRoot(uint64(0), safeHead, l2)
		require.NoError(t, err)
		assertDerivationResult(t, result, safeHead, l2.blockHash, l2.outputRoot)
	})

	t.Run("Success-PriorToSafeHead", func(t *testing.T) {
		expected := eth.Bytes32{0x11}
		safeHead := eth.L2BlockRef{
			Number: 10,
		}
		l2 := &mockL2{
			blockHash:  common.Hash{0x24},
			outputRoot: expected,
		}
		result, err := loadOutputRoot(uint64(20), safeHead, l2)
		require.NoError(t, err)
		require.Equal(t, uint64(10), l2.requestedOutputRoot)
		assertDerivationResult(t, result, safeHead, l2.blockHash, l2.outputRoot)
	})

	t.Run("Error-OutputRoot", func(t *testing.T) {
		expectedErr := errors.New("boom")
		safeHead := eth.L2BlockRef{Number: 10}
		l2 := &mockL2{
			blockHash:     common.Hash{0x24},
			outputRoot:    eth.Bytes32{0x11},
			outputRootErr: expectedErr,
		}
		_, err := loadOutputRoot(uint64(0), safeHead, l2)
		require.ErrorIs(t, err, expectedErr)
	})
}

func assertDerivationResult(t *testing.T, actual DerivationResult, safeHead eth.L2BlockRef, blockHash common.Hash, outputRoot eth.Bytes32) {
	require.Equal(t, safeHead, actual.Head)
	require.Equal(t, blockHash, actual.BlockHash)
	require.Equal(t, outputRoot, actual.OutputRoot)
}

type mockL2 struct {
	blockHash     common.Hash
	outputRoot    eth.Bytes32
	outputRootErr error

	requestedOutputRoot uint64
}

func (m *mockL2) L2OutputRoot(u uint64) (common.Hash, eth.Bytes32, error) {
	m.requestedOutputRoot = u
	if m.outputRootErr != nil {
		return common.Hash{}, eth.Bytes32{}, m.outputRootErr
	}
	return m.blockHash, m.outputRoot, nil
}

var _ L2Source = (*mockL2)(nil)
