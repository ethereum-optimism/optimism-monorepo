package interop

import (
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-program/client/boot"
	"github.com/ethereum-optimism/optimism/op-program/client/claim"
	"github.com/ethereum-optimism/optimism/op-program/client/interop/types"
	"github.com/ethereum-optimism/optimism/op-program/client/l1"
	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	"github.com/ethereum-optimism/optimism/op-program/client/tasks"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

var (
	ErrIncorrectOutputRootType = errors.New("incorrect output root type")
)

type taskExecutor interface {
	RunDerivation(
		logger log.Logger,
		rollupCfg *rollup.Config,
		l2ChainConfig *params.ChainConfig,
		l1Head common.Hash,
		agreedOutputRoot eth.Bytes32,
		claimedBlockNumber uint64,
		l1Oracle l1.Oracle,
		l2Oracle l2.Oracle) (tasks.DerivationResult, error)
}

func RunInteropProgram(logger log.Logger, bootInfo *boot.BootInfo, l1PreimageOracle l1.Oracle, l2PreimageOracle l2.Oracle, validateClaim bool) error {
	return runInteropProgram(logger, bootInfo, l1PreimageOracle, l2PreimageOracle, validateClaim, &interopTaskExecutor{})
}

func runInteropProgram(logger log.Logger, bootInfo *boot.BootInfo, l1PreimageOracle l1.Oracle, l2PreimageOracle l2.Oracle, validateClaim bool, tasks taskExecutor) error {
	logger.Info("Interop Program Bootstrapped", "bootInfo", bootInfo)

	// For the first step in a timestamp, we would get a SuperRoot as the agreed claim - TransitionStateByRoot will
	// automatically convert it to a TransitionState with Step: 0.
	transitionState := l2PreimageOracle.TransitionStateByRoot(bootInfo.L2OutputRoot)
	if transitionState.Version() != types.IntermediateTransitionVersion {
		return fmt.Errorf("%w: %v", ErrIncorrectOutputRootType, transitionState.Version())
	}

	super, err := eth.UnmarshalSuperRoot(transitionState.SuperRoot)
	if err != nil {
		return fmt.Errorf("invalid super root: %w", err)
	}
	if super.Version() != eth.SuperRootVersionV1 {
		return fmt.Errorf("%w: %v", ErrIncorrectOutputRootType, super.Version())
	}
	superRoot := super.(*eth.SuperV1)
	claimedBlockNumber, err := bootInfo.RollupConfig.TargetBlockNumber(superRoot.Timestamp + 1)
	if err != nil {
		return err
	}
	derivationResult, err := tasks.RunDerivation(
		logger,
		bootInfo.RollupConfig,
		bootInfo.L2ChainConfig,
		bootInfo.L1Head,
		superRoot.Chains[transitionState.Step].Output,
		claimedBlockNumber,
		l1PreimageOracle,
		l2PreimageOracle,
	)
	if err != nil {
		return err
	}

	newPendingProgress := make([]types.OptimisticBlock, len(transitionState.PendingProgress)+1)
	copy(newPendingProgress, transitionState.PendingProgress)
	newPendingProgress[len(newPendingProgress)-1] = types.OptimisticBlock{
		BlockHash:  derivationResult.BlockHash,
		OutputRoot: derivationResult.OutputRoot,
	}
	finalState := &types.TransitionState{
		SuperRoot:       transitionState.SuperRoot,
		PendingProgress: newPendingProgress,
		Step:            transitionState.Step + 1,
	}
	expected, err := finalState.Hash()
	if err != nil {
		return err
	}
	if !validateClaim {
		return nil
	}
	return claim.ValidateClaim(logger, derivationResult.Head, eth.Bytes32(bootInfo.L2Claim), eth.Bytes32(expected))
}

type interopTaskExecutor struct {
}

func (t *interopTaskExecutor) RunDerivation(
	logger log.Logger,
	rollupCfg *rollup.Config,
	l2ChainConfig *params.ChainConfig,
	l1Head common.Hash,
	agreedOutputRoot eth.Bytes32,
	claimedBlockNumber uint64,
	l1Oracle l1.Oracle,
	l2Oracle l2.Oracle) (tasks.DerivationResult, error) {
	return tasks.RunDerivation(
		logger,
		rollupCfg,
		l2ChainConfig,
		l1Head,
		common.Hash(agreedOutputRoot),
		claimedBlockNumber,
		l1Oracle,
		l2Oracle)
}
