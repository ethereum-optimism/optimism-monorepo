package nat

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTest(t *testing.T) {
	t.Run("uses default parameters", func(t *testing.T) {
		defaultParams := map[string]string{"key": "value"}
		var receivedParams interface{}

		test := &Test{
			ID:            "test-default-params",
			DefaultParams: defaultParams,
			Fn: func(ctx context.Context, log log.Logger, cfg Config, params interface{}) (bool, error) {
				receivedParams = params
				return true, nil
			},
		}

		result, err := test.Run(context.Background(), log.New(), Config{}, nil)

		require.NoError(t, err)
		assert.True(t, result.Passed)
		assert.Equal(t, defaultParams, receivedParams)
	})

	t.Run("uses provided parameters", func(t *testing.T) {
		customParams := map[string]string{"custom": "param"}
		var receivedParams interface{}

		test := &Test{
			ID: "test-custom-params",
			Fn: func(ctx context.Context, log log.Logger, cfg Config, params interface{}) (bool, error) {
				receivedParams = params
				return true, nil
			},
		}

		result, err := test.Run(context.Background(), log.New(), Config{}, customParams)

		require.NoError(t, err)
		assert.True(t, result.Passed)
		assert.Equal(t, customParams, receivedParams)
	})

	t.Run("returns correct result based on Fn return value", func(t *testing.T) {
		testCases := []struct {
			name         string
			fnReturn     bool
			fnErr        error
			expectPassed bool
			expectErr    error
		}{
			{
				name:         "returns true when Fn returns true",
				fnReturn:     true,
				fnErr:        nil,
				expectPassed: true,
				expectErr:    nil,
			},
			{
				name:         "returns false when Fn returns false",
				fnReturn:     false,
				fnErr:        nil,
				expectPassed: false,
				expectErr:    nil,
			},
			{
				name:         "propagates error from Fn",
				fnReturn:     false,
				fnErr:        assert.AnError,
				expectPassed: false,
				expectErr:    assert.AnError,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				test := &Test{
					ID: "test-return-values",
					Fn: func(ctx context.Context, log log.Logger, cfg Config, params interface{}) (bool, error) {
						return tc.fnReturn, tc.fnErr
					},
				}

				result, err := test.Run(context.Background(), log.New(), Config{}, nil)

				if tc.expectErr != nil {
					assert.Equal(t, tc.expectErr, err)
				} else {
					require.NoError(t, err)
					assert.Equal(t, tc.expectPassed, result.Passed)
				}
			})
		}
	})
}
