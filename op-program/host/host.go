package host

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	cl "github.com/ethereum-optimism/optimism/op-program/client"
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
	cfg.Rollup.LogDescription(logger, chaincfg.L2ChainIDToNetworkDisplayName)

	hostCtx, stop := ctxinterrupt.WithSignalWaiter(context.Background())
	defer stop()
	ctx := ctxinterrupt.WithCancelOnInterrupt(hostCtx)
	if cfg.ServerMode {
		preimageChan := preimage.ClientPreimageChannel()
		hinterChan := preimage.ClientHinterChannel()
		return PreimageServer(ctx, logger, cfg, preimageChan, hinterChan, makeDefaultPrefetcher)
	}

	if err := FaultProofProgram(ctx, logger, cfg); err != nil {
		return err
	}
	log.Info("Claim successfully verified")
	return nil
}

// FaultProofProgram is the programmatic entry-point for the fault proof program
func FaultProofProgram(ctx context.Context, logger log.Logger, cfg *config.Config, opts ...ProgramOpt) error {
	creators := &creatorsCfg{
		prefetcher: makeDefaultPrefetcher,
	}
	for _, opt := range opts {
		opt(creators)
	}
	var (
		serverErr chan error
		pClientRW preimage.FileChannel
		hClientRW preimage.FileChannel
	)
	defer func() {
		if pClientRW != nil {
			_ = pClientRW.Close()
		}
		if hClientRW != nil {
			_ = hClientRW.Close()
		}
		if serverErr != nil {
			err := <-serverErr
			if err != nil {
				logger.Error("preimage server failed", "err", err)
			}
			logger.Debug("Preimage server stopped")
		}
	}()
	// Setup client I/O for preimage oracle interaction
	pClientRW, pHostRW, err := preimage.CreateBidirectionalChannel()
	if err != nil {
		return fmt.Errorf("failed to create preimage pipe: %w", err)
	}

	// Setup client I/O for hint comms
	hClientRW, hHostRW, err := preimage.CreateBidirectionalChannel()
	if err != nil {
		return fmt.Errorf("failed to create hints pipe: %w", err)
	}

	// Use a channel to receive the server result so we can wait for it to complete before returning
	serverErr = make(chan error)
	go func() {
		defer close(serverErr)
		serverErr <- PreimageServer(ctx, logger, cfg, pHostRW, hHostRW, creators.prefetcher)
	}()

	var cmd *exec.Cmd
	if cfg.ExecCmd != "" {
		cmd = exec.CommandContext(ctx, cfg.ExecCmd)
		cmd.ExtraFiles = make([]*os.File, cl.MaxFd-3) // not including stdin, stdout and stderr
		cmd.ExtraFiles[cl.HClientRFd-3] = hClientRW.Reader()
		cmd.ExtraFiles[cl.HClientWFd-3] = hClientRW.Writer()
		cmd.ExtraFiles[cl.PClientRFd-3] = pClientRW.Reader()
		cmd.ExtraFiles[cl.PClientWFd-3] = pClientRW.Writer()
		cmd.Stdout = os.Stdout // for debugging
		cmd.Stderr = os.Stderr // for debugging

		err := cmd.Start()
		if err != nil {
			return fmt.Errorf("program cmd failed to start: %w", err)
		}
		if err := cmd.Wait(); err != nil {
			return fmt.Errorf("failed to wait for child program: %w", err)
		}
		logger.Debug("Client program completed successfully")
		return nil
	} else {
		return cl.RunProgram(logger, pClientRW, hClientRW)
	}
}

// PreimageServer reads hints and preimage requests from the provided channels and processes those requests.
// This method will block until both the hinter and preimage handlers complete.
// If either returns an error both handlers are stopped.
// The supplied preimageChannel and hintChannel will be closed before this function returns.
func PreimageServer(ctx context.Context, logger log.Logger, cfg *config.Config, preimageChannel preimage.FileChannel, hintChannel preimage.FileChannel, prefetcherCreator PrefetcherCreator) error {
	var serverDone chan error
	var hinterDone chan error
	logger.Info("Starting preimage server")
	var kv kvstore.KV

	// Close the preimage/hint channels, and then kv store once the server and hinter have exited.
	defer func() {
		preimageChannel.Close()
		hintChannel.Close()
		if serverDone != nil {
			// Wait for pre-image server to complete
			<-serverDone
		}
		if hinterDone != nil {
			// Wait for hinter to complete
			<-hinterDone
		}

		if kv != nil {
			kv.Close()
		}
	}()

	if cfg.DataDir == "" {
		logger.Info("Using in-memory storage")
		kv = kvstore.NewMemKV()
	} else {
		if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
			return fmt.Errorf("creating datadir: %w", err)
		}
		store, err := kvstore.NewDiskKV(logger, cfg.DataDir, cfg.DataFormat)
		if err != nil {
			return fmt.Errorf("creating kvstore: %w", err)
		}
		kv = store
	}

	var (
		getPreimage kvstore.PreimageSource
		hinter      preimage.HintHandler
	)
	prefetch, err := prefetcherCreator(ctx, logger, kv, cfg)
	if err != nil {
		return fmt.Errorf("failed to create prefetcher: %w", err)
	}
	if prefetch != nil {
		getPreimage = func(key common.Hash) ([]byte, error) { return prefetch.GetPreimage(ctx, key) }
		hinter = prefetch.Hint
	} else {
		logger.Info("Using offline mode. All required pre-images must be pre-populated.")
		getPreimage = kv.Get
		hinter = func(hint string) error {
			logger.Debug("ignoring prefetch hint", "hint", hint)
			return nil
		}
	}

	localPreimageSource := kvstore.NewLocalPreimageSource(cfg)
	splitter := kvstore.NewPreimageSourceSplitter(localPreimageSource.Get, getPreimage)
	preimageGetter := preimage.WithVerification(splitter.Get)

	serverDone = launchOracleServer(logger, preimageChannel, preimageGetter)
	hinterDone = routeHints(logger, hintChannel, hinter)
	select {
	case err := <-serverDone:
		return err
	case err := <-hinterDone:
		return err
	case <-ctx.Done():
		logger.Info("Shutting down")
		if errors.Is(ctx.Err(), context.Canceled) {
			// We were asked to shutdown by the context being cancelled so don't treat it as an error condition.
			return nil
		}
		return ctx.Err()
	}
}

func makeDefaultPrefetcher(ctx context.Context, logger log.Logger, kv kvstore.KV, cfg *config.Config) (Prefetcher, error) {
	if !cfg.FetchingEnabled() {
		return nil, nil
	}
	logger.Info("Connecting to L1 node", "l1", cfg.L1URL)
	l1RPC, err := client.NewRPC(ctx, logger, cfg.L1URL, client.WithDialAttempts(10))
	if err != nil {
		return nil, fmt.Errorf("failed to setup L1 RPC: %w", err)
	}

	l1ClCfg := sources.L1ClientDefaultConfig(cfg.Rollup, cfg.L1TrustRPC, cfg.L1RPCKind)
	l1Cl, err := sources.NewL1Client(l1RPC, logger, nil, l1ClCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create L1 client: %w", err)
	}

	logger.Info("Connecting to L1 beacon", "l1", cfg.L1BeaconURL)
	l1Beacon := sources.NewBeaconHTTPClient(client.NewBasicHTTPClient(cfg.L1BeaconURL, logger))
	l1BlobFetcher := sources.NewL1BeaconClient(l1Beacon, sources.L1BeaconClientConfig{FetchAllSidecars: false})

	logger.Info("Initializing L2 clients")
	l2Client, err := NewL2Source(ctx, logger, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create L2 source: %w", err)
	}

	return prefetcher.NewPrefetcher(logger, l1Cl, l1BlobFetcher, l2Client, kv), nil
}

func routeHints(logger log.Logger, hHostRW io.ReadWriter, hinter preimage.HintHandler) chan error {
	chErr := make(chan error)
	hintReader := preimage.NewHintReader(hHostRW)
	go func() {
		defer close(chErr)
		for {
			if err := hintReader.NextHint(hinter); err != nil {
				if err == io.EOF || errors.Is(err, fs.ErrClosed) {
					logger.Debug("closing pre-image hint handler")
					return
				}
				logger.Error("pre-image hint router error", "err", err)
				chErr <- err
				return
			}
		}
	}()
	return chErr
}

func launchOracleServer(logger log.Logger, pHostRW io.ReadWriteCloser, getter preimage.PreimageGetter) chan error {
	chErr := make(chan error)
	server := preimage.NewOracleServer(pHostRW)
	go func() {
		defer close(chErr)
		for {
			if err := server.NextPreimageRequest(getter); err != nil {
				if err == io.EOF || errors.Is(err, fs.ErrClosed) {
					logger.Debug("closing pre-image server")
					return
				}
				logger.Error("pre-image server error", "error", err)
				chErr <- err
				return
			}
		}
	}()
	return chErr
}
