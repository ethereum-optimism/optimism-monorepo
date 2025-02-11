package sequencer

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/log"
	gn "github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-service/client"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/config"
)

func TestService(t *testing.T) {
	cfg := &config.Config{
		Version: "",
		LogConfig: oplog.CLIConfig{
			Level:  log.LevelError,
			Color:  false,
			Format: oplog.FormatLogFmt,
		},
		MetricsConfig: opmetrics.CLIConfig{
			Enabled:    true,
			ListenAddr: "127.0.0.1",
			ListenPort: 0, // pick a port automatically
		},
		PprofConfig: oppprof.CLIConfig{
			ListenEnabled:   true,
			ListenAddr:      "127.0.0.1",
			ListenPort:      0, // pick a port automatically
			ProfileType:     "",
			ProfileDir:      "",
			ProfileFilename: "",
		},
		RPC: oprpc.CLIConfig{
			ListenAddr:  "127.0.0.1",
			ListenPort:  0, // pick a port automatically
			EnableAdmin: true,
		},
		MockRun:       true,
		JWTSecretPath: filepath.Join(t.TempDir(), "test_jwt_secret.txt"),
	}
	logger := testlog.Logger(t, log.LevelError)

	s, err := FromConfig(context.Background(), cfg, logger)
	require.NoError(t, err)
	require.NoError(t, s.Start(context.Background()), "start service")
	// run some RPC tests against the service with the mock backend
	{
		endpoint := s.httpServer.HTTPEndpoint()
		t.Logf("dialing %s", endpoint)
		opts := []client.RPCOption{
			client.WithFixedDialBackoff(time.Second * 5),
			client.WithDialAttempts(4),
			client.WithGethRPCOptions(rpc.WithHTTPAuth(gn.NewJWTAuth(s.jwtSecret))),
		}
		cl, err := client.NewRPC(context.Background(), logger, endpoint, opts...)
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		var dest string
		err = cl.CallContext(ctx, &dest, "admin_hello", "foobar")
		cancel()
		require.NoError(t, err)
		require.Equal(t, "hello foobar!", dest, "expecting mock result")
		cl.Close()
	}
	require.NoError(t, s.Stop(context.Background()), "stop service")
}
