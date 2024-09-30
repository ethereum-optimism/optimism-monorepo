package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	op_service "github.com/ethereum-optimism/optimism/op-service"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
)

// legacy geth log levels - the geth command line --verbosity flag wasn't
// migrated to use slog's numerical levels.
const (
	LegacyLevelCrit = iota
	LegacyLevelError
	LegacyLevelWarn
	LegacyLevelInfo
	LegacyLevelDebug
	LegacyLevelTrace
)

type AllocType string

const (
	AllocTypeStandard AllocType = "standard"
	AllocTypeAltDA    AllocType = "alt-da"
	AllocTypeL2OO     AllocType = "l2oo"
	AllocTypeMTCannon AllocType = "mt-cannon"

	DefaultAllocType = AllocTypeStandard
)

func (a AllocType) Check() error {
	switch a {
	case AllocTypeStandard, AllocTypeAltDA, AllocTypeL2OO, AllocTypeMTCannon:
		return nil
	default:
		return fmt.Errorf("unknown alloc type: %q", a)
	}
}

func (a AllocType) SupportsProofs() bool {
	switch a {
	case AllocTypeStandard, AllocTypeMTCannon:
		return true
	default:
		return false
	}
}

var allocTypes = []AllocType{AllocTypeStandard, AllocTypeAltDA, AllocTypeL2OO}

var (
	// All of the following variables are set in the init function
	// and read from JSON files on disk that are generated by the
	// foundry deploy script. These are globally exported to be used
	// in end to end tests.

	// L1Allocs represents the L1 genesis block state.
	l1AllocsByType = make(map[AllocType]*foundry.ForgeAllocs)
	// L1Deployments maps contract names to accounts in the L1
	// genesis block state.
	l1DeploymentsByType = make(map[AllocType]*genesis.L1Deployments)
	// l2Allocs represents the L2 allocs, by hardfork/mode (e.g. delta, ecotone, interop, other)
	l2AllocsByType = make(map[AllocType]genesis.L2AllocsModeMap)
	// DeployConfig represents the deploy config used by the system.
	deployConfigsByType = make(map[AllocType]*genesis.DeployConfig)
	// EthNodeVerbosity is the (legacy geth) level of verbosity to output
	EthNodeVerbosity int
)

func L1Allocs(allocType AllocType) *foundry.ForgeAllocs {
	allocs, ok := l1AllocsByType[allocType]
	if !ok {
		panic(fmt.Errorf("unknown L1 alloc type: %q", allocType))
	}
	return allocs.Copy()
}

func L1Deployments(allocType AllocType) *genesis.L1Deployments {
	deployments, ok := l1DeploymentsByType[allocType]
	if !ok {
		panic(fmt.Errorf("unknown L1 deployments type: %q", allocType))
	}
	return deployments.Copy()
}

func L2Allocs(allocType AllocType, mode genesis.L2AllocsMode) *foundry.ForgeAllocs {
	allocsByType, ok := l2AllocsByType[allocType]
	if !ok {
		panic(fmt.Errorf("unknown L2 alloc type: %q", allocType))
	}

	allocs, ok := allocsByType[mode]
	if !ok {
		panic(fmt.Errorf("unknown L2 allocs mode: %q", mode))
	}
	return allocs.Copy()
}

func DeployConfig(allocType AllocType) *genesis.DeployConfig {
	dc, ok := deployConfigsByType[allocType]
	if !ok {
		panic(fmt.Errorf("unknown deploy config type: %q", allocType))
	}
	return dc.Copy()
}

func init() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	root, err := op_service.FindMonorepoRoot(cwd)
	if err != nil {
		panic(err)
	}

	for _, allocType := range allocTypes {
		initAllocType(root, allocType)
	}

	// Setup global logger
	lvl := log.FromLegacyLevel(EthNodeVerbosity)
	var handler slog.Handler
	if lvl > log.LevelCrit {
		handler = log.DiscardHandler()
	} else {
		if lvl < log.LevelTrace { // clip to trace level
			lvl = log.LevelTrace
		}
		// We cannot attach a testlog logger,
		// because the global logger is shared between different independent parallel tests.
		// Tests that write to a testlogger of another finished test fail.
		handler = oplog.NewLogHandler(os.Stdout, oplog.CLIConfig{
			Level:  lvl,
			Color:  false, // some CI logs do not handle colors well
			Format: oplog.FormatTerminal,
		})
	}
	oplog.SetGlobalLogHandler(handler)
}

func initAllocType(root string, allocType AllocType) {
	devnetDir := filepath.Join(root, fmt.Sprintf(".devnet-%s", allocType))
	l1AllocsPath := filepath.Join(devnetDir, "allocs-l1.json")
	l2AllocsDir := devnetDir
	l1DeploymentsPath := filepath.Join(devnetDir, "addresses.json")
	deployConfigPath := filepath.Join(root, "packages", "contracts-bedrock", "deploy-config", "devnetL1.json")

	var missing bool
	for _, fp := range []string{devnetDir, l1AllocsPath, l1DeploymentsPath} {
		_, err := os.Stat(fp)
		if os.IsNotExist(err) {
			missing = true
			break
		}
		if err != nil {
			panic(err)
		}
	}
	if missing {
		log.Warn("allocs file not found, skipping", "allocType", allocType)
		return
	}

	l1Allocs, err := foundry.LoadForgeAllocs(l1AllocsPath)
	if err != nil {
		panic(err)
	}
	l1AllocsByType[allocType] = l1Allocs
	l2Alloc := make(map[genesis.L2AllocsMode]*foundry.ForgeAllocs)
	mustL2Allocs := func(mode genesis.L2AllocsMode) {
		name := "allocs-l2-" + string(mode)
		allocs, err := foundry.LoadForgeAllocs(filepath.Join(l2AllocsDir, name+".json"))
		if err != nil {
			panic(err)
		}
		l2Alloc[mode] = allocs
	}
	mustL2Allocs(genesis.L2AllocsGranite)
	mustL2Allocs(genesis.L2AllocsFjord)
	mustL2Allocs(genesis.L2AllocsEcotone)
	mustL2Allocs(genesis.L2AllocsDelta)
	l2AllocsByType[allocType] = l2Alloc
	l1Deployments, err := genesis.NewL1Deployments(l1DeploymentsPath)
	if err != nil {
		panic(err)
	}
	l1DeploymentsByType[allocType] = l1Deployments
	dc, err := genesis.NewDeployConfig(deployConfigPath)
	if err != nil {
		panic(err)
	}

	// Do not use clique in the in memory tests. Otherwise block building
	// would be much more complex.
	dc.L1UseClique = false
	// Set the L1 genesis block timestamp to now
	dc.L1GenesisBlockTimestamp = hexutil.Uint64(time.Now().Unix())
	dc.FundDevAccounts = true
	// Speed up the in memory tests
	dc.L1BlockTime = 2
	dc.L2BlockTime = 1
	dc.SetDeployments(l1Deployments)
	deployConfigsByType[allocType] = dc
}

func AllocTypeFromEnv() AllocType {
	allocType := os.Getenv("OP_E2E_ALLOC_TYPE")
	if allocType == "" {
		return DefaultAllocType
	}
	out := AllocType(allocType)
	if err := out.Check(); err != nil {
		panic(err)
	}
	return out
}
