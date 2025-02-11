package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/ethereum-optimism/optimism/op-service/superutil"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	"github.com/ethereum-optimism/optimism/op-program/chainconfig"
	"github.com/ethereum-optimism/optimism/op-program/client/boot"
	"github.com/ethereum-optimism/optimism/op-program/host/types"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-program/host/flags"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/urfave/cli/v2"
)

var (
	ErrNoL2Chains            = errors.New("at least one L2 chain must be specified")
	ErrMissingL2ChainID      = errors.New("missing l2 chain id")
	ErrMissingL2Genesis      = errors.New("missing l2 genesis")
	ErrNoRollupForGenesis    = errors.New("no rollup config matching l2 genesis")
	ErrNoGenesisForRollup    = errors.New("no l2 genesis for rollup")
	ErrDuplicateRollup       = errors.New("duplicate rollup")
	ErrDuplicateGenesis      = errors.New("duplicate l2 genesis")
	ErrInvalidL1Head         = errors.New("invalid l1 head")
	ErrInvalidL2Head         = errors.New("invalid l2 head")
	ErrInvalidL2OutputRoot   = errors.New("invalid l2 output root")
	ErrInvalidAgreedPrestate = errors.New("invalid l2 agreed prestate")
	ErrL1AndL2Inconsistent   = errors.New("l1 and l2 options must be specified together or both omitted")
	ErrInvalidL2Claim        = errors.New("invalid l2 claim")
	ErrInvalidL2ClaimBlock   = errors.New("invalid l2 claim block number")
	ErrDataDirRequired       = errors.New("datadir must be specified when in non-fetching mode")
	ErrNoExecInServerMode    = errors.New("exec command must not be set when in server mode")
	ErrInvalidDataFormat     = errors.New("invalid data format")
	ErrMissingAgreedPrestate = errors.New("missing agreed prestate")
)

type Config struct {
	L2ChainID eth.ChainID // TODO: Forbid for interop
	Rollups   []*rollup.Config
	// DataDir is the directory to read/write pre-image data from/to.
	// If not set, an in-memory key-value store is used and fetching data must be enabled
	DataDir string

	// DataFormat specifies the format to use for on-disk storage. Only applies when DataDir is set.
	DataFormat types.DataFormat

	// L1Head is the block hash of the L1 chain head block
	L1Head      common.Hash
	L1URL       string
	L1BeaconURL string
	L1TrustRPC  bool
	L1RPCKind   sources.RPCProviderKind

	// L2Head is the l2 block hash contained in the L2 Output referenced by the L2OutputRoot for pre-interop mode
	L2Head common.Hash // TODO: Forbid for interop
	// L2OutputRoot is the agreed L2 output root to start derivation from
	L2OutputRoot common.Hash
	// L2URLs are the URLs of the L2 nodes to fetch L2 data from, these are the canonical URL for L2 data
	// These URLs are used as a fallback for L2ExperimentalURL if the experimental URL fails or cannot retrieve the desired data
	// Must have one L2URL for each chain in Rollups
	L2URLs []string
	// L2ExperimentalURLs are the URLs of the L2 nodes (non hash db archival node, for example, reth archival node) to fetch L2 data from
	// Must have one url for each chain in Rollups
	L2ExperimentalURLs []string
	// L2Claim is the claimed L2 output root to verify
	L2Claim common.Hash
	// L2ClaimBlockNumber is the block number the claimed L2 output root is from
	// Must be above 0 and to be a valid claim needs to be above the L2Head block.
	// For interop this is the superchain root timestamp
	L2ClaimBlockNumber uint64
	// L2ChainConfigs are the op-geth chain config for the L2 execution engines
	// Must have one chain config for each rollup config
	L2ChainConfigs []*params.ChainConfig
	// ExecCmd specifies the client program to execute in a separate process.
	// If unset, the fault proof client is run in the same process.
	ExecCmd string

	// ServerMode indicates that the program should run in pre-image server mode and wait for requests.
	// No client program is run.
	ServerMode bool

	// InteropEnabled enables interop fault proof rules when running the client in-process
	InteropEnabled bool
	// AgreedPrestate is the preimage of the agreed prestate claim. Required for interop.
	AgreedPrestate []byte
}

func (c *Config) Check() error {
	if !c.InteropEnabled && c.L2ChainID == (eth.ChainID{}) {
		return ErrMissingL2ChainID
	}
	if len(c.Rollups) == 0 {
		return ErrNoL2Chains
	}
	for _, rollupCfg := range c.Rollups {
		if err := rollupCfg.Check(); err != nil {
			return fmt.Errorf("invalid rollup config for chain %v: %w", rollupCfg.L2ChainID, err)
		}
	}
	if c.L1Head == (common.Hash{}) {
		return ErrInvalidL1Head
	}
	if !c.InteropEnabled && c.L2Head == (common.Hash{}) {
		return ErrInvalidL2Head
	}
	if c.L2OutputRoot == (common.Hash{}) {
		return ErrInvalidL2OutputRoot
	}
	if c.L2ClaimBlockNumber == 0 {
		return ErrInvalidL2ClaimBlock
	}
	if len(c.L2ChainConfigs) == 0 {
		return ErrMissingL2Genesis
	}
	// Make of known rollup chain IDs to whether we have the L2 chain config for it
	chainIDToHasChainConfig := make(map[uint64]bool, len(c.Rollups))
	for _, config := range c.Rollups {
		chainID := config.L2ChainID.Uint64()
		if _, ok := chainIDToHasChainConfig[chainID]; ok {
			return fmt.Errorf("%w for chain ID %v", ErrDuplicateRollup, chainID)
		}
		chainIDToHasChainConfig[chainID] = false
	}
	for _, config := range c.L2ChainConfigs {
		chainID := config.ChainID.Uint64()
		if _, ok := chainIDToHasChainConfig[chainID]; !ok {
			return fmt.Errorf("%w for chain ID %v", ErrNoRollupForGenesis, config.ChainID)
		}
		if chainIDToHasChainConfig[chainID] {
			return fmt.Errorf("%w for chain ID %v", ErrDuplicateGenesis, config.ChainID)
		}
		chainIDToHasChainConfig[chainID] = true
	}
	for chainID, hasChainConfig := range chainIDToHasChainConfig {
		if !hasChainConfig {
			return fmt.Errorf("%w for chain ID %v", ErrNoGenesisForRollup, chainID)
		}
	}
	if (c.L1URL != "") != (len(c.L2URLs) > 0) {
		return ErrL1AndL2Inconsistent
	}
	if !c.FetchingEnabled() && c.DataDir == "" {
		return ErrDataDirRequired
	}
	if c.ServerMode && c.ExecCmd != "" {
		return ErrNoExecInServerMode
	}
	if c.DataDir != "" && !slices.Contains(types.SupportedDataFormats, c.DataFormat) {
		return ErrInvalidDataFormat
	}
	if c.InteropEnabled {
		if len(c.AgreedPrestate) == 0 {
			return ErrMissingAgreedPrestate
		}
		if crypto.Keccak256Hash(c.AgreedPrestate) != c.L2OutputRoot {
			return fmt.Errorf("%w: must be preimage of L2 output root", ErrInvalidAgreedPrestate)
		}
	}
	return nil
}

func (c *Config) FetchingEnabled() bool {
	return c.L1URL != "" && len(c.L2URLs) > 0 && c.L1BeaconURL != ""
}

func NewSingleChainConfig(
	rollupCfg *rollup.Config,
	l2ChainConfig *params.ChainConfig,
	l1Head common.Hash,
	l2Head common.Hash,
	l2OutputRoot common.Hash,
	l2Claim common.Hash,
	l2ClaimBlockNum uint64,
) *Config {
	l2ChainID := eth.ChainIDFromBig(l2ChainConfig.ChainID)
	_, err := superutil.LoadOPStackChainConfigFromChainID(eth.EvilChainIDToUInt64(l2ChainID))
	if err != nil {
		// Unknown chain ID so assume it is custom
		l2ChainID = boot.CustomChainIDIndicator
	}
	cfg := NewConfig(
		[]*rollup.Config{rollupCfg},
		[]*params.ChainConfig{l2ChainConfig},
		l1Head,
		l2Head,
		l2OutputRoot,
		l2Claim,
		l2ClaimBlockNum)
	cfg.L2ChainID = l2ChainID
	return cfg
}

// NewConfig creates a Config with all optional values set to the CLI default value
func NewConfig(
	rollupCfgs []*rollup.Config,
	l2ChainConfigs []*params.ChainConfig,
	l1Head common.Hash,
	l2Head common.Hash,
	l2OutputRoot common.Hash,
	l2Claim common.Hash,
	l2ClaimBlockNum uint64,
) *Config {
	return &Config{
		Rollups:            rollupCfgs,
		L2ChainConfigs:     l2ChainConfigs,
		L1Head:             l1Head,
		L2Head:             l2Head,
		L2OutputRoot:       l2OutputRoot,
		L2Claim:            l2Claim,
		L2ClaimBlockNumber: l2ClaimBlockNum,
		L1RPCKind:          sources.RPCKindStandard,
		DataFormat:         types.DataFormatDirectory,
	}
}

func NewConfigFromCLI(log log.Logger, ctx *cli.Context) (*Config, error) {
	if err := flags.CheckRequired(ctx); err != nil {
		return nil, err
	}

	var l2Head common.Hash
	if ctx.IsSet(flags.L2Head.Name) {
		l2Head = common.HexToHash(ctx.String(flags.L2Head.Name))
		if l2Head == (common.Hash{}) {
			return nil, ErrInvalidL2Head
		}
	}
	var l2OutputRoot common.Hash
	var agreedPrestate []byte
	var interopEnabled bool
	if ctx.IsSet(flags.L2OutputRoot.Name) {
		l2OutputRoot = common.HexToHash(ctx.String(flags.L2OutputRoot.Name))
	} else if ctx.IsSet(flags.L2AgreedPrestate.Name) {
		prestateStr := ctx.String(flags.L2AgreedPrestate.Name)
		agreedPrestate = common.FromHex(prestateStr)
		if len(agreedPrestate) == 0 {
			return nil, ErrInvalidAgreedPrestate
		}
		l2OutputRoot = crypto.Keccak256Hash(agreedPrestate)
		interopEnabled = true
	}
	if l2OutputRoot == (common.Hash{}) {
		return nil, ErrInvalidL2OutputRoot
	}
	strClaim := ctx.String(flags.L2Claim.Name)
	l2Claim := common.HexToHash(strClaim)
	// Require a valid hash, with the zero hash explicitly allowed.
	if l2Claim == (common.Hash{}) &&
		strClaim != "0x0000000000000000000000000000000000000000000000000000000000000000" &&
		strClaim != "0000000000000000000000000000000000000000000000000000000000000000" {
		return nil, fmt.Errorf("%w: %v", ErrInvalidL2Claim, strClaim)
	}
	l2ClaimBlockNum := ctx.Uint64(flags.L2BlockNumber.Name)
	l1Head := common.HexToHash(ctx.String(flags.L1Head.Name))
	if l1Head == (common.Hash{}) {
		return nil, ErrInvalidL1Head
	}

	var err error
	var rollupCfgs []*rollup.Config
	var l2ChainConfigs []*params.ChainConfig
	var l2ChainID eth.ChainID
	networkNames := ctx.StringSlice(flags.Network.Name)
	for _, networkName := range networkNames {
		var chainID eth.ChainID
		if chainID, err = eth.ParseDecimalChainID(networkName); err != nil {
			ch := chaincfg.ChainByName(networkName)
			if ch == nil {
				return nil, fmt.Errorf("invalid network: %q", networkName)
			}
			chainID = eth.ChainIDFromUInt64(ch.ChainID)
		}

		l2ChainConfig, err := chainconfig.ChainConfigByChainID(chainID)
		if err != nil {
			return nil, fmt.Errorf("failed to load chain config for chain %d: %w", chainID, err)
		}
		l2ChainConfigs = append(l2ChainConfigs, l2ChainConfig)
		rollupCfg, err := chainconfig.RollupConfigByChainID(chainID)
		if err != nil {
			return nil, fmt.Errorf("failed to load rollup config for chain %d: %w", chainID, err)
		}
		rollupCfgs = append(rollupCfgs, rollupCfg)
		l2ChainID = chainID
	}

	genesisPaths := ctx.StringSlice(flags.L2GenesisPath.Name)
	for _, l2GenesisPath := range genesisPaths {
		l2ChainConfig, err := loadChainConfigFromGenesis(l2GenesisPath)
		if err != nil {
			return nil, fmt.Errorf("invalid genesis: %w", err)
		}
		l2ChainConfigs = append(l2ChainConfigs, l2ChainConfig)
		l2ChainID = eth.ChainIDFromBig(l2ChainConfig.ChainID)
	}

	rollupPaths := ctx.StringSlice(flags.RollupConfig.Name)
	for _, rollupConfigPath := range rollupPaths {
		rollupCfg, err := loadRollupConfig(rollupConfigPath)
		if err != nil {
			return nil, fmt.Errorf("invalid rollup config: %w", err)
		}
		rollupCfgs = append(rollupCfgs, rollupCfg)

	}
	if ctx.Bool(flags.L2Custom.Name) {
		log.Warn("Using custom chain configuration via preimage oracle. This is not compatible with on-chain execution.")
		l2ChainID = boot.CustomChainIDIndicator
	} else if len(rollupCfgs) > 1 {
		// L2ChainID is not applicable when multiple L2 sources are used and not using custom configs
		l2ChainID = eth.ChainID{}
	}

	dbFormat := types.DataFormat(ctx.String(flags.DataFormat.Name))
	if !slices.Contains(types.SupportedDataFormats, dbFormat) {
		return nil, fmt.Errorf("invalid %w: %v", ErrInvalidDataFormat, dbFormat)
	}
	return &Config{
		L2ChainID:          l2ChainID,
		Rollups:            rollupCfgs,
		DataDir:            ctx.String(flags.DataDir.Name),
		DataFormat:         dbFormat,
		L2URLs:             ctx.StringSlice(flags.L2NodeAddr.Name),
		L2ExperimentalURLs: ctx.StringSlice(flags.L2NodeExperimentalAddr.Name),
		L2ChainConfigs:     l2ChainConfigs,
		L2Head:             l2Head,
		L2OutputRoot:       l2OutputRoot,
		AgreedPrestate:     agreedPrestate,
		L2Claim:            l2Claim,
		L2ClaimBlockNumber: l2ClaimBlockNum,
		L1Head:             l1Head,
		L1URL:              ctx.String(flags.L1NodeAddr.Name),
		L1BeaconURL:        ctx.String(flags.L1BeaconAddr.Name),
		L1TrustRPC:         ctx.Bool(flags.L1TrustRPC.Name),
		L1RPCKind:          sources.RPCProviderKind(ctx.String(flags.L1RPCProviderKind.Name)),
		ExecCmd:            ctx.String(flags.Exec.Name),
		ServerMode:         ctx.Bool(flags.Server.Name),
		InteropEnabled:     interopEnabled,
	}, nil
}

func loadChainConfigFromGenesis(path string) (*params.ChainConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read l2 genesis file: %w", err)
	}
	var genesis core.Genesis
	err = json.Unmarshal(data, &genesis)
	if err != nil {
		return nil, fmt.Errorf("parse l2 genesis file: %w", err)
	}
	return genesis.Config, nil
}

func loadRollupConfig(rollupConfigPath string) (*rollup.Config, error) {
	file, err := os.Open(rollupConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read rollup config: %w", err)
	}
	defer file.Close()

	var rollupConfig rollup.Config
	return &rollupConfig, rollupConfig.ParseRollupConfig(file)
}
