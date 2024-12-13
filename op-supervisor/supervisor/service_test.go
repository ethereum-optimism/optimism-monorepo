package supervisor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/dial"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/config"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

func TestSupervisorService(t *testing.T) {
	depSet, err := depset.NewStaticConfigDependencySet(make(map[types.ChainID]*depset.StaticConfigDependency))
	require.NoError(t, err)

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
		DependencySetSource: depSet,
		MockRun:             true,
	}
	logger := testlog.Logger(t, log.LevelError)
	supervisor, err := SupervisorFromConfig(context.Background(), cfg, logger)
	require.NoError(t, err)
	require.NoError(t, supervisor.Start(context.Background()), "start service")
	// run some RPC tests against the service with the mock backend
	{
		endpoint := "http://" + supervisor.rpcServer.Endpoint()
		t.Logf("dialing %s", endpoint)
		cl, err := dial.DialRPCClientWithTimeout(context.Background(), time.Second*5, logger, endpoint)
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		var dest types.SafetyLevel
		err = cl.CallContext(ctx, &dest, "supervisor_checkMessage",
			types.Identifier{
				Origin:      common.Address{0xaa},
				BlockNumber: 123,
				LogIndex:    42,
				Timestamp:   1234567,
				ChainID:     types.ChainID{0xbb},
			}, common.Hash{0xcc})
		cancel()
		require.NoError(t, err)
		require.Equal(t, types.CrossUnsafe, dest, "expecting mock to return cross-unsafe")
		cl.Close()
	}
	require.NoError(t, supervisor.Stop(context.Background()), "stop service")
}
