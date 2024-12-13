package pipeline

import (
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/opcm"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
	"github.com/ethereum/go-ethereum/common"
)

func DeployAltDA(env *Env, intent *state.Intent, st *state.State, chainID common.Hash) error {
	lgr := env.Logger.New("stage", "deploy-alt-da")

	chainIntent, err := intent.Chain(chainID)
	if err != nil {
		return fmt.Errorf("failed to get chain intent: %w", err)
	}

	chainState, err := st.Chain(chainID)
	if err != nil {
		return fmt.Errorf("failed to get chain state: %w", err)
	}

	if !shouldDeployAltDA(chainIntent, chainState) {
		lgr.Info("alt-da deployment not needed")
		return nil
	}

	var dao opcm.DeployAltDAOutput
	lgr.Info("deploying alt-da contracts")
	dao, err = opcm.DeployAltDA(env.L1ScriptHost, opcm.DeployAltDAInput{
		Salt:                     st.Create2Salt,
		ProxyAdmin:               chainState.ProxyAdminAddress,
		ChallengeContractOwner:   chainIntent.Roles.L1ProxyAdminOwner,
		ChallengeWindow:          new(big.Int).SetUint64(chainIntent.DangerousAltDAConfig.DAChallengeWindow),
		ResolveWindow:            new(big.Int).SetUint64(chainIntent.DangerousAltDAConfig.DAResolveWindow),
		BondSize:                 new(big.Int).SetUint64(chainIntent.DangerousAltDAConfig.DABondSize),
		ResolverRefundPercentage: new(big.Int).SetUint64(chainIntent.DangerousAltDAConfig.DAResolverRefundPercentage),
	})
	if err != nil {
		return fmt.Errorf("failed to deploy alt-da contracts: %w", err)
	}

	chainState.DataAvailabilityChallengeProxyAddress = dao.DataAvailabilityChallengeProxy
	chainState.DataAvailabilityChallengeImplAddress = dao.DataAvailabilityChallengeImpl
	return nil
}

func shouldDeployAltDA(chainIntent *state.ChainIntent, chainState *state.ChainState) bool {
	if !chainIntent.DangerousAltDAConfig.UseAltDA {
		return false
	}

	return chainState.DataAvailabilityChallengeImplAddress == common.Address{}
}
