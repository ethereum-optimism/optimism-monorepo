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
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

var (
	ErrIncorrectOutputRootType = errors.New("incorrect output root type")
	ErrL1HeadReached           = errors.New("l1 head reached")

	InvalidTransition     = []byte("invalid")
	InvalidTransitionHash = crypto.Keccak256Hash(InvalidTransition)
)

const (
	ConsolidateStep = 1023
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
		l2Oracle l2.Oracle,
	) (tasks.DerivationResult, error)

	BuildDepositOnlyBlock(
		logger log.Logger,
		rollupCfg *rollup.Config,
		l2ChainConfig *params.ChainConfig,
		l1Head common.Hash,
		agreedL2OutputRoot eth.Bytes32,
		l1Oracle l1.Oracle,
		l2Oracle l2.Oracle,
		optimisticBlock *ethtypes.Block,
	) (blockHash common.Hash, outputRoot eth.Bytes32, err error)
}

func RunInteropProgram(logger log.Logger, bootInfo *boot.BootInfoInterop, l1PreimageOracle l1.Oracle, l2PreimageOracle l2.Oracle, validateClaim bool) error {
	return runInteropProgram(logger, bootInfo, l1PreimageOracle, l2PreimageOracle, validateClaim, &interopTaskExecutor{})
}

func runInteropProgram(logger log.Logger, bootInfo *boot.BootInfoInterop, l1PreimageOracle l1.Oracle, l2PreimageOracle l2.Oracle, validateClaim bool, tasks taskExecutor) error {
	logger.Info("Interop Program Bootstrapped", "bootInfo", bootInfo)

	expected, err := stateTransition(logger, bootInfo, l1PreimageOracle, l2PreimageOracle, tasks)
	if err != nil {
		return err
	}
	if !validateClaim {
		return nil
	}
	return claim.ValidateClaim(logger, eth.Bytes32(bootInfo.Claim), eth.Bytes32(expected))
}

func stateTransition(logger log.Logger, bootInfo *boot.BootInfoInterop, l1PreimageOracle l1.Oracle, l2PreimageOracle l2.Oracle, tasks taskExecutor) (common.Hash, error) {
	if bootInfo.AgreedPrestate == InvalidTransitionHash {
		return InvalidTransitionHash, nil
	}
	transitionState, superRoot, err := parseAgreedState(bootInfo, l2PreimageOracle)
	if err != nil {
		return common.Hash{}, err
	}
	// Strictly, the state transition ends when superRoot.Timestamp == bootInfo.GameTimestamp.
	// Since the valid state transition ends at the game timestamp, there isn't any valid hash resulting from
	// an agreed prestate and so the program panics to make it clear that the setup is invalid.
	// The honest actor will never agree to a prestate where superRoot.Timestamp > bootInfo.GameTimestamp and so will
	// be unaffected by this
	if superRoot.Timestamp == bootInfo.GameTimestamp {
		return bootInfo.AgreedPrestate, nil
	} else if superRoot.Timestamp > bootInfo.GameTimestamp {
		panic(fmt.Errorf("agreed prestate timestamp %v is after the game timestamp %v", superRoot.Timestamp, bootInfo.GameTimestamp))
	}
	expectedPendingProgress := transitionState.PendingProgress
	if transitionState.Step < uint64(len(superRoot.Chains)) {
		block, err := deriveOptimisticBlock(logger, bootInfo, l1PreimageOracle, l2PreimageOracle, superRoot, transitionState, tasks)
		if errors.Is(err, ErrL1HeadReached) {
			return InvalidTransitionHash, nil
		} else if err != nil {
			return common.Hash{}, err
		}
		expectedPendingProgress = append(expectedPendingProgress, block)
	} else if transitionState.Step == ConsolidateStep {
		// sanity check
		if len(transitionState.PendingProgress) >= ConsolidateStep {
			return common.Hash{}, fmt.Errorf("pending progress length does not match the expected step")
		}
		expectedSuperRoot, err := RunConsolidation(
			logger, bootInfo, l1PreimageOracle, l2PreimageOracle, transitionState, superRoot, tasks)
		if err != nil {
			return common.Hash{}, err
		}
		return common.Hash(expectedSuperRoot), nil
	}

	finalState := &types.TransitionState{
		SuperRoot:       transitionState.SuperRoot,
		PendingProgress: expectedPendingProgress,
		Step:            transitionState.Step + 1,
	}
	return finalState.Hash(), nil
}

func parseAgreedState(bootInfo *boot.BootInfoInterop, l2PreimageOracle l2.Oracle) (*types.TransitionState, *eth.SuperV1, error) {
	// For the first step in a timestamp, we would get a SuperRoot as the agreed claim - TransitionStateByRoot will
	// automatically convert it to a TransitionState with Step: 0.
	transitionState := l2PreimageOracle.TransitionStateByRoot(bootInfo.AgreedPrestate)
	if transitionState.Version() != types.IntermediateTransitionVersion {
		return nil, nil, fmt.Errorf("%w: %v", ErrIncorrectOutputRootType, transitionState.Version())
	}

	super, err := eth.UnmarshalSuperRoot(transitionState.SuperRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid super root: %w", err)
	}
	if super.Version() != eth.SuperRootVersionV1 {
		return nil, nil, fmt.Errorf("%w: %v", ErrIncorrectOutputRootType, super.Version())
	}
	superRoot := super.(*eth.SuperV1)
	return transitionState, superRoot, nil
}

func deriveOptimisticBlock(logger log.Logger, bootInfo *boot.BootInfoInterop, l1PreimageOracle l1.Oracle, l2PreimageOracle l2.Oracle, superRoot *eth.SuperV1, transitionState *types.TransitionState, tasks taskExecutor) (types.OptimisticBlock, error) {
	chainAgreedPrestate := superRoot.Chains[transitionState.Step]
	rollupCfg, err := bootInfo.Configs.RollupConfig(chainAgreedPrestate.ChainID)
	if err != nil {
		return types.OptimisticBlock{}, fmt.Errorf("no rollup config available for chain ID %v: %w", chainAgreedPrestate.ChainID, err)
	}
	l2ChainConfig, err := bootInfo.Configs.ChainConfig(chainAgreedPrestate.ChainID)
	if err != nil {
		return types.OptimisticBlock{}, fmt.Errorf("no chain config available for chain ID %v: %w", chainAgreedPrestate.ChainID, err)
	}
	claimedBlockNumber, err := rollupCfg.TargetBlockNumber(superRoot.Timestamp + 1)
	if err != nil {
		return types.OptimisticBlock{}, err
	}
	derivationResult, err := tasks.RunDerivation(
		logger,
		rollupCfg,
		l2ChainConfig,
		bootInfo.L1Head,
		chainAgreedPrestate.Output,
		claimedBlockNumber,
		l1PreimageOracle,
		l2PreimageOracle,
	)
	if err != nil {
		return types.OptimisticBlock{}, err
	}
	if derivationResult.Head.Number < claimedBlockNumber {
		return types.OptimisticBlock{}, ErrL1HeadReached
	}

	block := types.OptimisticBlock{
		BlockHash:  derivationResult.BlockHash,
		OutputRoot: derivationResult.OutputRoot,
	}
	return block, nil
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
	l2Oracle l2.Oracle,
) (tasks.DerivationResult, error) {
	return tasks.RunDerivation(
		logger,
		rollupCfg,
		l2ChainConfig,
		l1Head,
		common.Hash(agreedOutputRoot),
		claimedBlockNumber,
		l1Oracle,
		l2Oracle,
		memorydb.New(),
		tasks.DerivationOptions{StoreBlockData: true},
	)
}

func (t *interopTaskExecutor) BuildDepositOnlyBlock(
	logger log.Logger,
	rollupCfg *rollup.Config,
	l2ChainConfig *params.ChainConfig,
	l1Head common.Hash,
	agreedL2OutputRoot eth.Bytes32,
	l1Oracle l1.Oracle,
	l2Oracle l2.Oracle,
	optimisticBlock *ethtypes.Block,
) (common.Hash, eth.Bytes32, error) {
	return tasks.BuildDepositOnlyBlock(
		logger,
		rollupCfg,
		l2ChainConfig,
		optimisticBlock,
		l1Head,
		agreedL2OutputRoot,
		l1Oracle,
		l2Oracle,
	)
}
