package pipeline

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/opcm"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
)

func DeploySuperchain(env *Env, intent *state.Intent, st *state.State) error {
	lgr := env.Logger.New("stage", "deploy-superchain")

	if !shouldDeploySuperchain(intent, st) {
		lgr.Info("superchain deployment not needed")
		return nil
	}

	lgr.Info("deploying superchain")

	dso, err := opcm.DeploySuperchain(
		env.L1ScriptHost,
		opcm.DeploySuperchainInput{
			SuperchainProxyAdminOwner:  intent.SuperchainRoles.ProxyAdminOwner,
			ProtocolVersionsOwner:      intent.SuperchainRoles.ProtocolVersionsOwner,
			Guardian:                   intent.SuperchainRoles.Guardian,
			Paused:                     false,
			RequiredProtocolVersion:    rollup.OPStackSupport,
			RecommendedProtocolVersion: rollup.OPStackSupport,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to deploy superchain: %w", err)
	}

	st.SuperchainDeployment = &state.SuperchainDeployment{
		ProxyAdminAddress:            dso.SuperchainProxyAdmin,
		SuperchainConfigProxyAddress: dso.SuperchainConfigProxy,
		SuperchainConfigImplAddress:  dso.SuperchainConfigImpl,
		ProtocolVersionsProxyAddress: dso.ProtocolVersionsProxy,
		ProtocolVersionsImplAddress:  dso.ProtocolVersionsImpl,
	}

	return nil
}

func shouldDeploySuperchain(intent *state.Intent, st *state.State) bool {
	return st.SuperchainDeployment == nil
}
