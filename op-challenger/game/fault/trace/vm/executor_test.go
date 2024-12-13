package vm

import (
	"context"
	"math"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

func TestGenerateProof(t *testing.T) {
	input := "starting.json"
	tempDir := t.TempDir()
	dir := filepath.Join(tempDir, "gameDir")
	cfg := Config{
		VmType:       "test",
		L1:           "http://localhost:8888",
		L1Beacon:     "http://localhost:9000",
		L2:           "http://localhost:9999",
		VmBin:        "./bin/testvm",
		Server:       "./bin/testserver",
		Network:      "op-test",
		SnapshotFreq: 500,
		InfoFreq:     900,
	}
	prestate := "pre.json"

	inputs := utils.LocalGameInputs{
		L1Head:        common.Hash{0x11},
		L2Head:        common.Hash{0x22},
		L2OutputRoot:  common.Hash{0x33},
		L2Claim:       common.Hash{0x44},
		L2BlockNumber: big.NewInt(3333),
	}

	info := &mipsevm.DebugInfo{
		MemoryUsed:                   11,
		TotalSteps:                   123455,
		RmwSuccessCount:              12,
		RmwFailCount:                 34,
		MaxStepsBetweenLLAndSC:       56,
		ReservationInvalidationCount: 78,
		ForcedPreemptionCount:        910,
		FailedWakeupCount:            1112,
		IdleStepCountThread0:         1314,
	}

	captureExec := func(t *testing.T, cfg Config, proofAt uint64, m Metricer) (string, string, map[string]string) {
		executor := NewExecutor(testlog.Logger(t, log.LevelInfo), m, cfg, NewOpProgramServerExecutor(testlog.Logger(t, log.LvlInfo)), prestate, inputs)
		executor.selectSnapshot = func(logger log.Logger, dir string, absolutePreState string, i uint64, binary bool) (string, error) {
			return input, nil
		}
		var binary string
		var subcommand string
		args := make(map[string]string)
		executor.cmdExecutor = func(ctx context.Context, l log.Logger, b string, a ...string) error {
			binary = b
			subcommand = a[0]
			for i := 1; i < len(a); {
				if a[i] == "--" {
					// Skip over the divider between vm and server program
					i += 1
					continue
				}
				args[a[i]] = a[i+1]
				i += 2
			}

			// Write debuginfo file
			debugPath := args["--debug-info"]
			err := jsonutil.WriteJSON(info, ioutil.ToStdOutOrFileOrNoop(debugPath, 0o755))
			require.NoError(t, err)
			return nil
		}
		err := executor.GenerateProof(context.Background(), dir, proofAt)
		require.NoError(t, err)
		return binary, subcommand, args
	}

	t.Run("Network", func(t *testing.T) {
		m := newMetrics()
		cfg.Network = "mainnet"
		cfg.RollupConfigPath = ""
		cfg.L2GenesisPath = ""
		cfg.DebugInfo = true
		binary, subcommand, args := captureExec(t, cfg, 150_000_000, m)
		require.DirExists(t, filepath.Join(dir, PreimagesDir))
		require.DirExists(t, filepath.Join(dir, utils.ProofsDir))
		require.DirExists(t, filepath.Join(dir, SnapsDir))
		require.Equal(t, cfg.VmBin, binary)
		require.Equal(t, "run", subcommand)
		require.Equal(t, input, args["--input"])
		require.Contains(t, args, "--meta")
		require.Equal(t, "", args["--meta"])
		require.Equal(t, FinalStatePath(dir, cfg.BinarySnapshots), args["--output"])
		require.Equal(t, "=150000000", args["--proof-at"])
		require.Equal(t, "=150000001", args["--stop-at"])
		require.Equal(t, "%500", args["--snapshot-at"])
		require.Equal(t, "%900", args["--info-at"])
		// Slight quirk of how we pair off args
		// The server binary winds up as the key and the first arg --server as the value which has no value
		// Then everything else pairs off correctly again
		require.Equal(t, "--server", args[cfg.Server])
		require.Equal(t, cfg.L1, args["--l1"])
		require.Equal(t, cfg.L1Beacon, args["--l1.beacon"])
		require.Equal(t, cfg.L2, args["--l2"])
		require.Equal(t, filepath.Join(dir, PreimagesDir), args["--datadir"])
		require.Equal(t, filepath.Join(dir, utils.ProofsDir, "%d.json.gz"), args["--proof-fmt"])
		require.Equal(t, filepath.Join(dir, SnapsDir, "%d.json.gz"), args["--snapshot-fmt"])
		require.Equal(t, cfg.Network, args["--network"])
		require.NotContains(t, args, "--rollup.config")
		require.NotContains(t, args, "--l2.genesis")

		// Local game inputs
		require.Equal(t, inputs.L1Head.Hex(), args["--l1.head"])
		require.Equal(t, inputs.L2Head.Hex(), args["--l2.head"])
		require.Equal(t, inputs.L2OutputRoot.Hex(), args["--l2.outputroot"])
		require.Equal(t, inputs.L2Claim.Hex(), args["--l2.claim"])
		require.Equal(t, "3333", args["--l2.blocknumber"])

		// Check metrics
		validateMetrics(t, m, info, cfg)
	})

	t.Run("RollupAndGenesis", func(t *testing.T) {
		m := newMetrics()
		cfg.Network = ""
		cfg.RollupConfigPath = "rollup.json"
		cfg.L2GenesisPath = "genesis.json"
		cfg.DebugInfo = false
		_, _, args := captureExec(t, cfg, 150_000_000, m)
		require.NotContains(t, args, "--network")
		require.Equal(t, cfg.RollupConfigPath, args["--rollup.config"])
		require.Equal(t, cfg.L2GenesisPath, args["--l2.genesis"])
		validateMetrics(t, m, info, cfg)
	})

	t.Run("NoStopAtWhenProofIsMaxUInt", func(t *testing.T) {
		m := newMetrics()
		cfg.Network = "mainnet"
		cfg.RollupConfigPath = "rollup.json"
		cfg.L2GenesisPath = "genesis.json"
		cfg.DebugInfo = true
		_, _, args := captureExec(t, cfg, math.MaxUint64, m)
		// stop-at would need to be one more than the proof step which would overflow back to 0
		// so expect that it will be omitted. We'll ultimately want asterisc to execute until the program exits.
		require.NotContains(t, args, "--stop-at")
		validateMetrics(t, m, info, cfg)
	})

	t.Run("BinarySnapshots", func(t *testing.T) {
		m := newMetrics()
		cfg.Network = "mainnet"
		cfg.BinarySnapshots = true
		_, _, args := captureExec(t, cfg, 100, m)
		require.Equal(t, filepath.Join(dir, SnapsDir, "%d.bin.gz"), args["--snapshot-fmt"])
		validateMetrics(t, m, info, cfg)
	})

	t.Run("JsonSnapshots", func(t *testing.T) {
		m := newMetrics()
		cfg.Network = "mainnet"
		cfg.BinarySnapshots = false
		_, _, args := captureExec(t, cfg, 100, m)
		require.Equal(t, filepath.Join(dir, SnapsDir, "%d.json.gz"), args["--snapshot-fmt"])
		validateMetrics(t, m, info, cfg)
	})
}

func validateMetrics(t require.TestingT, m *capturingVmMetrics, expected *mipsevm.DebugInfo, cfg Config) {
	require.Equal(t, 1, m.executionTimeRecordCount, "Should record vm execution time")

	// Check metrics sourced from cannon.mipsevm.DebugInfo json file
	if cfg.DebugInfo {
		require.Equal(t, expected.MemoryUsed, m.memoryUsed)
		require.Equal(t, expected.TotalSteps, m.steps)
		require.Equal(t, expected.RmwSuccessCount, m.rmwSuccessCount)
		require.Equal(t, expected.RmwFailCount, m.rmwFailCount)
		require.Equal(t, expected.MaxStepsBetweenLLAndSC, m.maxStepsBetweenLLAndSC)
		require.Equal(t, expected.ReservationInvalidationCount, m.reservationInvalidations)
		require.Equal(t, expected.ForcedPreemptionCount, m.forcedPreemptions)
		require.Equal(t, expected.FailedWakeupCount, m.failedWakeup)
		require.Equal(t, expected.IdleStepCountThread0, m.idleStepsThread0)
	} else {
		// If debugInfo is disabled, json file should not be written and metrics should be zeroed out
		require.Equal(t, hexutil.Uint64(0), m.memoryUsed)
		require.Equal(t, uint64(0), m.steps)
		require.Equal(t, uint64(0), m.rmwSuccessCount)
		require.Equal(t, uint64(0), m.rmwFailCount)
		require.Equal(t, uint64(0), m.maxStepsBetweenLLAndSC)
		require.Equal(t, uint64(0), m.reservationInvalidations)
		require.Equal(t, uint64(0), m.forcedPreemptions)
		require.Equal(t, uint64(0), m.failedWakeup)
		require.Equal(t, uint64(0), m.idleStepsThread0)
	}
}

func newMetrics() *capturingVmMetrics {
	return &capturingVmMetrics{}
}

type capturingVmMetrics struct {
	executionTimeRecordCount int
	memoryUsed               hexutil.Uint64
	steps                    uint64
	rmwSuccessCount          uint64
	rmwFailCount             uint64
	maxStepsBetweenLLAndSC   uint64
	reservationInvalidations uint64
	forcedPreemptions        uint64
	failedWakeup             uint64
	idleStepsThread0         uint64
}

func (c *capturingVmMetrics) RecordSteps(val uint64) {
	c.steps = val
}

func (c *capturingVmMetrics) RecordExecutionTime(t time.Duration) {
	c.executionTimeRecordCount += 1
}

func (c *capturingVmMetrics) RecordMemoryUsed(memoryUsed uint64) {
	c.memoryUsed = hexutil.Uint64(memoryUsed)
}

func (c *capturingVmMetrics) RecordRmwSuccessCount(val uint64) {
	c.rmwSuccessCount = val
}

func (c *capturingVmMetrics) RecordRmwFailCount(val uint64) {
	c.rmwFailCount = val
}

func (c *capturingVmMetrics) RecordMaxStepsBetweenLLAndSC(val uint64) {
	c.maxStepsBetweenLLAndSC = val
}

func (c *capturingVmMetrics) RecordReservationInvalidationCount(val uint64) {
	c.reservationInvalidations = val
}

func (c *capturingVmMetrics) RecordForcedPreemptionCount(val uint64) {
	c.forcedPreemptions = val
}

func (c *capturingVmMetrics) RecordFailedWakeupCount(val uint64) {
	c.failedWakeup = val
}

func (c *capturingVmMetrics) RecordIdleStepCountThread0(val uint64) {
	c.idleStepsThread0 = val
}

var _ Metricer = (*capturingVmMetrics)(nil)
