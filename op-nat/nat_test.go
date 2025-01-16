package nat

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNATParameterization(t *testing.T) {
	// Create a test that records its received parameters
	var receivedParams interface{}
	testFn := func(ctx context.Context, log log.Logger, cfg Config, params interface{}) (bool, error) {
		receivedParams = params
		return true, nil
	}

	test := &Test{
		ID:            "test-with-params",
		DefaultParams: map[string]string{"value": "default"},
		Fn:            testFn,
	}

	// Create a basic config with our test
	cfg := &Config{
		Validators:          []Validator{test},
		SenderSecretKey:     "0x0",
		ReceiverPublicKeys:  []string{"0x0"},
		ReceiverPrivateKeys: []string{"0x0"},
	}
	logger := log.New()

	t.Run("uses default parameters when none provided", func(t *testing.T) {
		nat, err := New(context.Background(), cfg, logger, "test")
		require.NoError(t, err)
		t.Cleanup(func() {
			err := nat.Stop(context.Background())
			require.NoError(t, err)
		})

		err = nat.Start(context.Background())
		require.NoError(t, err)

		assert.Equal(t, test.DefaultParams, receivedParams)
	})

	t.Run("uses custom parameters when provided", func(t *testing.T) {
		nat, err := New(context.Background(), cfg, logger, "test")
		require.NoError(t, err)
		t.Cleanup(func() {
			err := nat.Stop(context.Background())
			require.NoError(t, err)
		})

		customParams := map[string]string{"value": "custom"}
		nat.params = map[string]interface{}{
			test.ID: customParams,
		}

		err = nat.Start(context.Background())
		require.NoError(t, err)

		assert.Equal(t, customParams, receivedParams)
	})

	t.Run("different test instances can have different parameters", func(t *testing.T) {
		// Create two instances with different parameters
		nat1, err := New(context.Background(), cfg, logger, "test1")
		require.NoError(t, err)
		nat1.params = map[string]interface{}{
			test.ID: map[string]string{"value": "instance1"},
		}

		nat2, err := New(context.Background(), cfg, logger, "test2")
		require.NoError(t, err)
		nat2.params = map[string]interface{}{
			test.ID: map[string]string{"value": "instance2"},
		}

		t.Cleanup(func() {
			err := nat1.Stop(context.Background())
			require.NoError(t, err)
			err = nat2.Stop(context.Background())
			require.NoError(t, err)
		})

		// Run first instance
		err = nat1.Start(context.Background())
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"value": "instance1"}, receivedParams)

		// Run second instance
		err = nat2.Start(context.Background())
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"value": "instance2"}, receivedParams)
	})

	t.Run("results are properly recorded", func(t *testing.T) {
		nat, err := New(context.Background(), cfg, logger, "test")
		require.NoError(t, err)
		t.Cleanup(func() {
			err := nat.Stop(context.Background())
			require.NoError(t, err)
		})
		nat.params = make(map[string]interface{})

		err = nat.Start(context.Background())
		require.NoError(t, err)

		require.Len(t, nat.results, 1)
		assert.Equal(t, "test-with-params", nat.results[0].ID)
		assert.Equal(t, "Test", nat.results[0].Type)
		assert.True(t, nat.results[0].Passed)
	})
}
