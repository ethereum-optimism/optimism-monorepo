package host

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	hostcommon "github.com/ethereum-optimism/optimism/op-program/host/common"
	"github.com/ethereum-optimism/optimism/op-program/host/config"
	"github.com/ethereum-optimism/optimism/op-program/host/flags"
	"github.com/ethereum-optimism/optimism/op-program/host/kvstore"
	"github.com/ethereum-optimism/optimism/op-program/host/prefetcher"
	opservice "github.com/ethereum-optimism/optimism/op-service"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/ctxinterrupt"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

type Prefetcher interface {
	Hint(hint string) error
	GetPreimage(ctx context.Context, key common.Hash) ([]byte, error)
}
type PrefetcherCreator func(ctx context.Context, logger log.Logger, kv kvstore.KV, cfg *config.Config) (Prefetcher, error)

type creatorsCfg struct {
	prefetcher PrefetcherCreator
}

type ProgramOpt func(c *creatorsCfg)

func WithPrefetcher(creator PrefetcherCreator) ProgramOpt {
	return func(c *creatorsCfg) {
		c.prefetcher = creator
	}
}

func Main(logger log.Logger, cfg *config.Config) error {
	if err := cfg.Check(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	opservice.ValidateEnvVars(flags.EnvVarPrefix, flags.Flags, logger)
	for _, r := range cfg.Rollups {
		r.LogDescription(logger, chaincfg.L2ChainIDToNetworkDisplayName)
	}

	hostCtx, stop := ctxinterrupt.WithSignalWaiter(context.Background())
	defer stop()
	ctx := ctxinterrupt.WithCancelOnInterrupt(hostCtx)
	if cfg.ServerMode {
		preimageChan := preimage.ClientPreimageChannel()
		hinterChan := preimage.ClientHinterChannel()
		return hostcommon.PreimageServer(ctx, logger, cfg, preimageChan, hinterChan, makeDefaultPrefetcher)
	}

	if err := FaultProofProgramWithDefaultPrefecher(ctx, logger, cfg); err != nil {
		return err
	}
	log.Info("Claim successfully verified")
	return nil
}

// FaultProofProgramWithDefaultPrefecher is the programmatic entry-point for the fault proof program
func FaultProofProgramWithDefaultPrefecher(ctx context.Context, logger log.Logger, cfg *config.Config, opts ...hostcommon.ProgramOpt) error {
	var newopts []hostcommon.ProgramOpt
	newopts = append(newopts, hostcommon.WithPrefetcher(makeDefaultPrefetcher))
	newopts = append(newopts, opts...)
	return hostcommon.FaultProofProgram(ctx, logger, cfg, newopts...)
}

func makeDefaultPrefetcher(ctx context.Context, logger log.Logger, kv kvstore.KV, cfg *config.Config) (hostcommon.Prefetcher, error) {
	if !cfg.FetchingEnabled() {
		return nil, nil
	}
	logger.Info("Connecting to L1 node", "l1", cfg.L1URL)
	l1RPC, err := client.NewRPC(ctx, logger, cfg.L1URL, client.WithDialAttempts(10))
	if err != nil {
		return nil, fmt.Errorf("failed to setup L1 RPC: %w", err)
	}

	// Small cache because we store everything to the KV store, but 0 isn't allowed.
	l1ClCfg := sources.L1ClientSimpleConfig(cfg.L1TrustRPC, cfg.L1RPCKind, 100)
	l1Cl, err := sources.NewL1Client(l1RPC, logger, nil, l1ClCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create L1 client: %w", err)
	}

	logger.Info("Connecting to L1 beacon", "l1", cfg.L1BeaconURL)
	l1Beacon := sources.NewBeaconHTTPClient(client.NewBasicHTTPClient(cfg.L1BeaconURL, logger))
	l1BlobFetcher := sources.NewL1BeaconClient(l1Beacon, sources.L1BeaconClientConfig{FetchAllSidecars: false})

	logger.Info("Initializing L2 clients")
	sources, err := prefetcher.NewRetryingL2SourcesFromURLs(ctx, logger, cfg.Rollups, cfg.L2URLs, cfg.L2ExperimentalURLs)
	if err != nil {
		return nil, fmt.Errorf("failed to create L2 sources: %w", err)
	}

	executor := MakeProgramExecutor(logger, cfg)
	var l2Head common.Hash
	var agreedPrestate []byte
	if cfg.InteropEnabled() {
		agreedPrestate = cfg.InteropInputs.AgreedPrestate
	} else {
		l2Head = cfg.PreInteropInputs.L2Head
	}
	return prefetcher.NewPrefetcher(logger, l1Cl, l1BlobFetcher, eth.ChainIDFromBig(cfg.Rollups[0].L2ChainID), sources, kv, executor, l2Head, agreedPrestate), nil
}

type programExecutor struct {
	logger log.Logger
	cfg    *config.Config
}

func (p *programExecutor) RunProgram(
	ctx context.Context,
	prefetcher hostcommon.Prefetcher,
	l2Head common.Hash,
	agreedOutputRoot common.Hash,
	blockNum uint64,
	chainID eth.ChainID,
	db l2.KeyValueStore,
) error {
	// Leave the newCfg.L2ChainID as is. It may be set to the customChainID for testing.
	// newCfg.L2ChainConfigs and newCfg.Rollups will be reconfigured to the specified chainID for the program execution.

	// Since the ProgramExecutor can be used for interop with custom chain configs, we need to
	// restrict the host's chain configuration to a single chain.
	var l2ChainConfig *params.ChainConfig
	for _, c := range p.cfg.L2ChainConfigs {
		if eth.ChainIDFromBig(c.ChainID).Cmp(chainID) == 0 {
			l2ChainConfig = c
			break
		}
	}
	if l2ChainConfig == nil {
		return fmt.Errorf("could not find L2 chain config in the host for chain ID %v", chainID)
	}
	var rollupConfig *rollup.Config
	for _, c := range p.cfg.Rollups {
		if eth.ChainIDFromBig(c.L2ChainID).Cmp(chainID) == 0 {
			rollupConfig = c
			break
		}
	}
	if rollupConfig == nil {
		return fmt.Errorf("could not find rollup config in the host for chain ID %v", chainID)
	}

	cfg := config.NewSingleChainConfig(
		rollupConfig,
		l2ChainConfig,
		p.cfg.L1Head,
		l2Head,
		agreedOutputRoot,
		// the claim is irrelevant since we skip validation. But it must be non-empty for the config check
		common.Hash{0xaa},
		blockNum,
	)
	// Since we're using the "self" prefetcher, do not configure RPC URLs
	if err := cfg.CheckInputs(); err != nil {
		return fmt.Errorf("bug: invalid config: %w", err)
	}

	withPrefetcher := hostcommon.WithPrefetcher(
		func(context.Context, log.Logger, kvstore.KV, *config.Config) (hostcommon.Prefetcher, error) {
			// TODO(#13663): prevent recursive block execution
			return prefetcher, nil
		})
	return hostcommon.FaultProofProgram(
		ctx,
		p.logger,
		cfg,
		withPrefetcher,
		hostcommon.WithSkipValidation(true),
		hostcommon.WithDB(db),
		hostcommon.WithStoreBlockData(true),
	)
}

func MakeProgramExecutor(logger log.Logger, cfg *config.Config) prefetcher.ProgramExecutor {
	return &programExecutor{
		logger: logger,
		cfg:    cfg,
	}
}
