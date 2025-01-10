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
	"github.com/ethereum/go-ethereum/log"
)

type Config struct {
	SkipValidation bool
	InteropEnabled bool
}

// Main executes the client program in a detached context and exits the current process.
// The client runtime environment must be preset before calling this function.
func Main(logger log.Logger) {
	log.Info("Starting fault proof program client")
	preimageOracle := preimage.ClientPreimageChannel()
	preimageHinter := preimage.ClientHinterChannel()
	config := Config{
		InteropEnabled: os.Getenv("OP_PROGRAM_CLIENT_USE_INTEROP") == "true",
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
	l2PreimageOracle := l2.NewCachingOracle(l2.NewPreimageOracle(pClient, hClient))

	bootInfo := boot.NewBootstrapClient(pClient).BootInfo()
	if cfg.InteropEnabled {
		return interop.RunInteropProgram(logger, bootInfo, l1PreimageOracle, l2PreimageOracle, !cfg.SkipValidation)
	}
	return RunPreInteropProgram(logger, bootInfo, l1PreimageOracle, l2PreimageOracle)
}
