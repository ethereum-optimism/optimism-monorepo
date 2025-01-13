package host

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	hostcommon "github.com/ethereum-optimism/optimism/op-program/host/common"
	"github.com/ethereum-optimism/optimism/op-program/host/config"
	"github.com/ethereum-optimism/optimism/op-program/host/flags"
	"github.com/ethereum-optimism/optimism/op-program/host/kvstore"
	"github.com/ethereum-optimism/optimism/op-program/host/prefetcher"
	opservice "github.com/ethereum-optimism/optimism/op-service"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/ctxinterrupt"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
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
	return prefetcher.NewPrefetcher(logger, l1Cl, l1BlobFetcher, cfg.Rollups[0].L2ChainID.Uint64(), sources, kv, executor, cfg.L2Head, cfg.AgreedPrestate), nil
}

type programExecutor struct {
	logger log.Logger
	cfg    *config.Config
}

func (p *programExecutor) RunProgram(
	ctx context.Context,
	prefetcher hostcommon.Prefetcher,
	blockNum uint64,
	chainID uint64,
) error {
	newCfg := *p.cfg
	newCfg.L2ChainID = chainID
	newCfg.L2ClaimBlockNumber = blockNum

	withPrefetcher := hostcommon.WithPrefetcher(
		func(context.Context, log.Logger, kvstore.KV, *config.Config) (hostcommon.Prefetcher, error) {
			// TODO(#13663): prevent recursive block execution
			return prefetcher, nil
		})
	return hostcommon.FaultProofProgram(ctx, p.logger, &newCfg, withPrefetcher, hostcommon.WithSkipValidation(true))
}

func MakeProgramExecutor(logger log.Logger, cfg *config.Config) prefetcher.ProgramExecutor {
	return &programExecutor{
		logger: logger,
		cfg:    cfg,
	}
}
