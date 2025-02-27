package proposer

import (
	"testing"

	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestValidConfigIsValid(t *testing.T) {
	cfg := validConfig()
	require.NoError(t, cfg.Check())
}

func TestRollupRpc(t *testing.T) {
	t.Run("RequiredWithL2OO", func(t *testing.T) {
		cfg := validConfig()
		cfg.DGFAddress = ""
		cfg.L2OOAddress = common.Address{0xaa}.Hex()
		cfg.ProposalInterval = 0
		cfg.RollupRpc = ""
		cfg.SupervisorRpcs = []string{"http://localhost:8882/supervisor"}
		require.ErrorIs(t, cfg.Check(), ErrMissingRollupRpc)
	})

	for _, gameType := range preInteropGameTypes {
		t.Run("RequiredWithPreInteropGame", func(t *testing.T) {
			cfg := validConfig()
			cfg.DGFAddress = common.Address{0xaa}.Hex()
			cfg.ProposalInterval = 20
			cfg.RollupRpc = ""
			cfg.SupervisorRpcs = []string{"http://localhost:8882/supervisor"}
			cfg.DisputeGameType = gameType
			require.ErrorIs(t, cfg.Check(), ErrMissingRollupRpc)
		})
	}

	t.Run("NotRequiredForOtherGameTypes", func(t *testing.T) {
		cfg := validConfig()
		cfg.DGFAddress = common.Address{0xaa}.Hex()
		cfg.ProposalInterval = 20
		cfg.RollupRpc = ""
		cfg.SupervisorRpcs = []string{"http://localhost:8882/supervisor"}
		cfg.DisputeGameType = 492743
		require.NoError(t, cfg.Check())
	})
}

func TestSupervisorRpc(t *testing.T) {
	t.Run("NotRequiredWithL2OO", func(t *testing.T) {
		cfg := validConfig()
		cfg.DGFAddress = ""
		cfg.L2OOAddress = common.Address{0xaa}.Hex()
		cfg.ProposalInterval = 0
		cfg.RollupRpc = "http://localhost/rollup"
		cfg.SupervisorRpcs = nil
		require.NoError(t, cfg.Check())
	})

	for _, gameType := range postInteropGameTypes {
		t.Run("RequiredWithPostInteropGame", func(t *testing.T) {
			cfg := validConfig()
			cfg.DGFAddress = common.Address{0xaa}.Hex()
			cfg.ProposalInterval = 20
			cfg.RollupRpc = "http://localhost:8882/rollup"
			cfg.SupervisorRpcs = nil
			cfg.DisputeGameType = gameType
			require.ErrorIs(t, cfg.Check(), ErrMissingSupervisorRpc)
		})

		t.Run("NotRequiredForOtherGameTypes", func(t *testing.T) {
			cfg := validConfig()
			cfg.DGFAddress = common.Address{0xaa}.Hex()
			cfg.ProposalInterval = 20
			cfg.RollupRpc = "http://localhost:8882/rollup"
			cfg.SupervisorRpcs = nil
			cfg.DisputeGameType = 492743
			require.NoError(t, cfg.Check())
		})
	}
}

func TestDisallowRollupAndSupervisorRPC(t *testing.T) {
	cfg := validConfig()
	cfg.ProposalInterval = 20
	cfg.RollupRpc = "http://localhost:8882/rollup"
	cfg.SupervisorRpcs = []string{"http://localhost:8882/supervisor"}
	cfg.DisputeGameType = 492743
	require.ErrorIs(t, cfg.Check(), ErrConflictingSource)
}

func validConfig() *CLIConfig {
	return &CLIConfig{
		L1EthRpc:                     "http://localhost:8888/l1",
		RollupRpc:                    "http://localhost:8888/l2",
		SupervisorRpcs:               nil,
		L2OOAddress:                  "",
		PollInterval:                 100,
		AllowNonFinalized:            false,
		TxMgrConfig:                  txmgr.NewCLIConfig("http://localhost:8888/l1", txmgr.DefaultBatcherFlagValues),
		RPCConfig:                    oprpc.DefaultCLIConfig(),
		LogConfig:                    oplog.DefaultCLIConfig(),
		MetricsConfig:                opmetrics.DefaultCLIConfig(),
		PprofConfig:                  oppprof.DefaultCLIConfig(),
		DGFAddress:                   common.Address{0xaa, 0xbb, 0xcc}.Hex(),
		ProposalInterval:             50,
		DisputeGameType:              0,
		ActiveSequencerCheckDuration: 0,
		WaitNodeSync:                 false,
	}
}
