package pipeline

import (
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/opcm"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
	"github.com/ethereum/go-ethereum/common"
)

func DeployAdditionalDisputeGames(
	env *Env,
	intent *state.Intent,
	st *state.State,
	chainID common.Hash,
) error {
	lgr := env.Logger.New("stage", "deploy-additional-dispute-games")

	thisIntent, err := intent.Chain(chainID)
	if err != nil {
		return fmt.Errorf("failed to get chain intent: %w", err)
	}

	thisState, err := st.Chain(chainID)
	if err != nil {
		return fmt.Errorf("failed to get chain state: %w", err)
	}

	if !shouldDeployAdditionalDisputeGames(thisIntent, thisState) {
		lgr.Info("additional dispute games deployment not needed")
		return nil
	}

	if thisIntent.Roles.L1ProxyAdminOwner != env.Deployer {
		return fmt.Errorf("cannot deploy additional dispute games when deployer is not L1PAO")
	}

	for _, game := range thisIntent.AdditionalDisputeGames {
		if err := deployDisputeGame(env, st, thisIntent, thisState, game); err != nil {
			return fmt.Errorf("failed to deploy additional dispute game: %w", err)
		}
	}

	return nil
}

func deployDisputeGame(
	env *Env,
	st *state.State,
	thisIntent *state.ChainIntent,
	thisState *state.ChainState,
	game state.AdditionalDisputeGame,
) error {
	lgr := env.Logger.New("gameType", game.DisputeGameType)

	var oracleAddr common.Address
	if game.UseCustomOracle {
		lgr.Info("deploying custom oracle")

		out, err := opcm.DeployPreimageOracle(env.L1ScriptHost, opcm.DeployPreimageOracleInput{
			MinProposalSize: new(big.Int).SetUint64(game.OracleMinProposalSize),
			ChallengePeriod: new(big.Int).SetUint64(game.OracleChallengePeriodSeconds),
		})
		if err != nil {
			return fmt.Errorf("failed to deploy preimage oracle: %w", err)
		}
		oracleAddr = out.PreimageOracle
		lgr.Info("oracle deployed", "oracleAddr", oracleAddr)
	} else {
		lgr.Info("using existing preimage oracle")
		oracleAddr = st.ImplementationsDeployment.PreimageOracleSingletonAddress
	}

	lgr.Info("deploying VM", "vmType", game.VMType)
	var vmAddr common.Address
	switch game.VMType {
	case state.VMTypeAlphabet:
		out, err := opcm.DeployAlphabetVM(env.L1ScriptHost, opcm.DeployAlphabetVMInput{
			AbsolutePrestate: game.DisputeAbsolutePrestate,
			PreimageOracle:   oracleAddr,
		})
		if err != nil {
			return fmt.Errorf("failed to deploy Alphabet VM: %w", err)
		}
		vmAddr = out.AlphabetVM
	case state.VMTypeCannon1, state.VMTypeCannon2:
		mipsVersion := 1
		if game.VMType == state.VMTypeCannon2 {
			mipsVersion = 2
		}

		out, err := opcm.DeployMIPS(env.L1ScriptHost, opcm.DeployMIPSInput{
			MipsVersion:    uint64(mipsVersion),
			PreimageOracle: oracleAddr,
		})
		if err != nil {
			return fmt.Errorf("failed to deploy MIPS VM: %w", err)
		}
		vmAddr = out.MipsSingleton
	default:
		return fmt.Errorf("unsupported VM type: %v", game.VMType)
	}
	lgr.Info("vm deployed", "vmAddr", vmAddr)

	lgr.Info("deploying dispute game")
	out, err := opcm.DeployDisputeGame(env.L1ScriptHost, opcm.DeployDisputeGameInput{
		Release:                  "dev",
		VmAddress:                vmAddr,
		GameKind:                 "FaultDisputeGame",
		GameType:                 game.DisputeGameType,
		AbsolutePrestate:         game.DisputeAbsolutePrestate,
		MaxGameDepth:             game.DisputeMaxGameDepth,
		SplitDepth:               game.DisputeSplitDepth,
		ClockExtension:           game.DisputeClockExtension,
		MaxClockDuration:         game.DisputeMaxClockDuration,
		DelayedWethProxy:         thisState.DelayedWETHPermissionedGameProxyAddress,
		AnchorStateRegistryProxy: thisState.AnchorStateRegistryProxyAddress,
		L2ChainId:                thisIntent.ID,
		Proposer:                 thisIntent.Roles.Proposer,
		Challenger:               thisIntent.Roles.Challenger,
	})
	if err != nil {
		return fmt.Errorf("failed to deploy dispute game: %w", err)
	}
	lgr.Info("dispute game deployed", "impl", out.DisputeGameImpl)

	lgr.Info("setting dispute game impl on factory", "respected", game.MakeRespected)
	sdgiInput := opcm.SetDisputeGameImplInput{
		Factory:             thisState.DisputeGameFactoryProxyAddress,
		Impl:                out.DisputeGameImpl,
		GameType:            game.DisputeGameType,
		AnchorStateRegistry: thisState.AnchorStateRegistryProxyAddress,
	}
	if game.MakeRespected {
		sdgiInput.Portal = thisState.OptimismPortalProxyAddress
	}
	if err := opcm.SetDisputeGameImpl(
		env.L1ScriptHost,
		sdgiInput,
	); err != nil {
		return fmt.Errorf("failed to set dispute game impl: %w", err)
	}

	thisState.AdditionalDisputeGames = append(thisState.AdditionalDisputeGames, state.AdditionalDisputeGameState{
		GameType:      game.DisputeGameType,
		VMType:        game.VMType,
		GameAddress:   out.DisputeGameImpl,
		OracleAddress: oracleAddr,
		VMAddress:     vmAddr,
	})

	return nil
}

func shouldDeployAdditionalDisputeGames(thisIntent *state.ChainIntent, thisState *state.ChainState) bool {
	if len(thisIntent.AdditionalDisputeGames) == 0 {
		return false
	}

	if len(thisState.AdditionalDisputeGames) > 0 {
		return false
	}

	return true
}
