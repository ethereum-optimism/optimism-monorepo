package rpc

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

func TestObtainJWTSecret(t *testing.T) {
	testPath := t.TempDir()
	logger := testlog.Logger(t, log.LvlInfo)

	t.Run("no-generate", func(t *testing.T) {
		secret, err := ObtainJWTSecret(logger, filepath.Join(testPath, "non_existent.txt"), false)
		require.ErrorIs(t, err, fs.ErrNotExist, "secret does not exist")
		require.Equal(t, eth.Bytes32{}, secret)

		// Not generated, still not there
		againSecret, err := ObtainJWTSecret(logger, filepath.Join(testPath, "non_existent.txt"), false)
		require.ErrorIs(t, err, fs.ErrNotExist, "secret does not exist")
		require.Equal(t, eth.Bytes32{}, againSecret)
	})

	t.Run("yes-generate", func(t *testing.T) {
		secret, err := ObtainJWTSecret(logger, filepath.Join(testPath, "will_generate.txt"), true)
		require.NoError(t, err)
		require.NotEqual(t, eth.Bytes32{}, secret)

		// it was generated, and should be there now
		againSecret, err := ObtainJWTSecret(logger, filepath.Join(testPath, "will_generate.txt"), false)
		require.NoError(t, err)
		require.Equal(t, secret, againSecret, "read the secret that was persisted")

		// now read again, but suggest generating it if missing. It's not missing, so shouldn't override
		stillSameSecret, err := ObtainJWTSecret(logger, filepath.Join(testPath, "will_generate.txt"), true)
		require.NoError(t, err)
		require.Equal(t, secret, stillSameSecret)
	})
}
