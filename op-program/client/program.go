package client

import (
	"errors"
	"fmt"
	"io"
	"os"

	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum-optimism/optimism/op-program/client/claim"
	cldr "github.com/ethereum-optimism/optimism/op-program/client/driver"
	"github.com/ethereum-optimism/optimism/op-program/client/l1"
	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/log"
)

type RunProgramFlags bool

const (
	RunProgramFlagsSkipValidation RunProgramFlags = false
	RunProgramFlagsValidate       RunProgramFlags = true
)

// Main executes the client program in a detached context and exits the current process.
// The client runtime environment must be preset before calling this function.
func Main(logger log.Logger) {
	log.Info("Starting fault proof program client")
	preimageOracle := preimage.ClientPreimageChannel()
	preimageHinter := preimage.ClientHinterChannel()
	if err := RunProgram(logger, preimageOracle, preimageHinter, false); errors.Is(err, claim.ErrClaimNotValid) {
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
func RunProgram(logger log.Logger, preimageOracle io.ReadWriter, preimageHinter io.ReadWriter, flags RunProgramFlags) error {
	pClient := preimage.NewOracleClient(preimageOracle)
	hClient := preimage.NewHintWriter(preimageHinter)
	l1PreimageOracle := l1.NewCachingOracle(l1.NewPreimageOracle(pClient, hClient))
	l2PreimageOracle := l2.NewCachingOracle(l2.NewPreimageOracle(pClient, hClient))

	bootInfo := NewBootstrapClient(pClient).BootInfo()
	logger.Info("Program Bootstrapped", "bootInfo", bootInfo)

	l1Source := l1.NewOracleL1Client(logger, l1PreimageOracle, bootInfo.L1Head)
	l1BlobsSource := l1.NewBlobFetcher(logger, l1PreimageOracle)
	engineBackend, err := l2.NewOracleBackedL2Chain(
		logger, l2PreimageOracle, l1PreimageOracle /* kzg oracle */, bootInfo.L2ChainConfig, bootInfo.L2OutputRoot)
	if err != nil {
		return fmt.Errorf("failed to create oracle-backed L2 chain: %w", err)
	}
	l2Source := l2.NewOracleEngine(bootInfo.RollupConfig, logger, engineBackend)

	logger.Info("Starting derivation")
	d := cldr.NewDriver(logger, bootInfo.RollupConfig, l1Source, l1BlobsSource, l2Source, bootInfo.L2ClaimBlockNumber)
	if err := d.RunComplete(); err != nil {
		return fmt.Errorf("failed to run program to completion: %w", err)
	}

	if flags == RunProgramFlagsValidate {
		return claim.ValidateClaim(logger, bootInfo.L2ClaimBlockNumber, eth.Bytes32(bootInfo.L2Claim), l2Source)
	}
	return nil
}
