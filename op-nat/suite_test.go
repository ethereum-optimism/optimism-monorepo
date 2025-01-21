package nat

import (
	"context"
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSuite(t *testing.T) {
	t.Run("runs all tests in order", func(t *testing.T) {
		executionOrder := []string{}

		suite := &Suite{
			Tests: []Test{
				{
					ID: "test1",
					Fn: func(ctx context.Context, log log.Logger, cfg Config, params interface{}) (bool, error) {
						executionOrder = append(executionOrder, "test1")
						return true, nil
					},
				},
				{
					ID: "test2",
					Fn: func(ctx context.Context, log log.Logger, cfg Config, params interface{}) (bool, error) {
						executionOrder = append(executionOrder, "test2")
						return true, nil
					},
				},
			},
		}

		result, err := suite.Run(context.Background(), log.New(), Config{}, nil)

		require.NoError(t, err)
		assert.True(t, result.Passed)
		assert.Equal(t, []string{"test1", "test2"}, executionOrder)
	})

	t.Run("fails if any test fails", func(t *testing.T) {
		suite := &Suite{
			Tests: []Test{
				{
					ID: "test1",
					Fn: func(ctx context.Context, log log.Logger, cfg Config, params interface{}) (bool, error) {
						return true, nil
					},
				},
				{
					ID: "test2",
					Fn: func(ctx context.Context, log log.Logger, cfg Config, params interface{}) (bool, error) {
						return false, nil
					},
				},
			},
		}

		result, err := suite.Run(context.Background(), log.New(), Config{}, nil)

		require.NoError(t, err)
		assert.False(t, result.Passed)
		assert.NoError(t, result.Error)
	})

	t.Run("errors if any test errors", func(t *testing.T) {
		suite := &Suite{
			Tests: []Test{
				{
					ID: "test1",
					Fn: func(ctx context.Context, log log.Logger, cfg Config, params interface{}) (bool, error) {
						return false, errors.New("test-error")
					},
				},
				{
					ID: "test2",
					Fn: func(ctx context.Context, log log.Logger, cfg Config, params interface{}) (bool, error) {
						return true, nil
					},
				},
			},
		}

		result, err := suite.Run(context.Background(), log.New(), Config{}, nil)

		require.NoError(t, err)
		assert.False(t, result.Passed)
		assert.Error(t, result.Error)
	})
}
