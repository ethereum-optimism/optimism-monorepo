package client

import (
	"errors"
	"io"
	"os"

	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum-optimism/optimism/op-program/client/boot"
	"github.com/ethereum-optimism/optimism/op-program/client/claim"
	"github.com/ethereum-optimism/optimism/op-program/client/interop"
	"github.com/ethereum-optimism/optimism/op-program/client/l1"
	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	"github.com/ethereum-optimism/optimism/op-program/client/tasks"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/log"
)

type Config struct {
	SkipValidation bool
	InteropEnabled bool
	DB             l2.KeyValueStore
	StoreBlockData bool
}

// Main executes the client program in a detached context and exits the current process.
// The client runtime environment must be preset before calling this function.
func Main(useInterop bool) {
	// Default to a machine parsable but relatively human friendly log format.
	// Don't do anything fancy to detect if color output is supported.
	logger := oplog.NewLogger(os.Stdout, oplog.CLIConfig{
		Level:  log.LevelInfo,
		Format: oplog.FormatLogFmt,
		Color:  false,
	})
	oplog.SetGlobalLogHandler(logger.Handler())

	logger.Info("Starting fault proof program client", "useInterop", useInterop)
	preimageOracle := preimage.ClientPreimageChannel()
	preimageHinter := preimage.ClientHinterChannel()
	config := Config{
		InteropEnabled: useInterop,
		DB:             memorydb.New(),
	}
	if err := RunProgram(logger, preimageOracle, preimageHinter, config); errors.Is(err, claim.ErrClaimNotValid) {
		log.Error("Claim is invalid", "err", err)
		os.Exit(1)
	} else if err != nil {
		log.Error("Program failed", "err", err)
		os.Exit(2)
	} else {
		log.Info("Claim successfully verified")
		os.Exit(0)
	}
}

// RunProgram executes the Program, while attached to an IO based pre-image oracle, to be served by a host.
func RunProgram(logger log.Logger, preimageOracle io.ReadWriter, preimageHinter io.ReadWriter, cfg Config) error {
	pClient := preimage.NewOracleClient(preimageOracle)
	hClient := preimage.NewHintWriter(preimageHinter)
	l1PreimageOracle := l1.NewCachingOracle(l1.NewPreimageOracle(pClient, hClient))
	l2PreimageOracle := l2.NewCachingOracle(l2.NewPreimageOracle(pClient, hClient, cfg.InteropEnabled))

	if cfg.InteropEnabled {
		bootInfo := boot.BootstrapInterop(pClient)
		return interop.RunInteropProgram(logger, bootInfo, l1PreimageOracle, l2PreimageOracle, !cfg.SkipValidation)
	}
	if cfg.DB == nil {
		return errors.New("db config is required")
	}
	bootInfo := boot.NewBootstrapClient(pClient).BootInfo()
	derivationOptions := tasks.DerivationOptions{StoreBlockData: cfg.StoreBlockData}
	return RunPreInteropProgram(logger, bootInfo, l1PreimageOracle, l2PreimageOracle, cfg.DB, derivationOptions)
}
