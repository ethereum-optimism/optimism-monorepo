package pipeline

import (
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/opcm"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
)

func DeployImplementations(env *Env, intent *state.Intent, st *state.State) error {
	lgr := env.Logger.New("stage", "deploy-implementations")

	if !shouldDeployImplementations(intent, st) {
		lgr.Info("implementations deployment not needed")
		return nil
	}

	lgr.Info("deploying implementations")

	var standardVersionsTOML string
	var contractsRelease string
	var err error
	if intent.L1ContractsLocator.IsTag() && intent.DeploymentStrategy == state.DeploymentStrategyLive {
		standardVersionsTOML, err = opcm.StandardL1VersionsDataFor(intent.L1ChainID)
		if err != nil {
			return fmt.Errorf("error getting standard versions TOML: %w", err)
		}
		contractsRelease = intent.L1ContractsLocator.Tag
	} else {
		contractsRelease = "dev"
	}

	env.L1ScriptHost.ImportState(st.L1StateDump.Data)

	dio, err := opcm.DeployImplementations(
		env.L1ScriptHost,
		opcm.DeployImplementationsInput{
			Salt:                            st.Create2Salt,
			WithdrawalDelaySeconds:          big.NewInt(604800),
			MinProposalSizeBytes:            big.NewInt(126000),
			ChallengePeriodSeconds:          big.NewInt(86400),
			ProofMaturityDelaySeconds:       big.NewInt(604800),
			DisputeGameFinalityDelaySeconds: big.NewInt(302400),
			MipsVersion:                     big.NewInt(1),
			Release:                         contractsRelease,
			SuperchainConfigProxy:           st.SuperchainDeployment.SuperchainConfigProxyAddress,
			ProtocolVersionsProxy:           st.SuperchainDeployment.ProtocolVersionsProxyAddress,
			OpcmProxyOwner:                  st.SuperchainDeployment.ProxyAdminAddress,
			StandardVersionsToml:            standardVersionsTOML,
			UseInterop:                      false,
		},
	)
	if err != nil {
		return fmt.Errorf("error deploying implementations: %w", err)
	}

	st.ImplementationsDeployment = &state.ImplementationsDeployment{
		OpcmProxyAddress:                        dio.OpcmProxy,
		DelayedWETHImplAddress:                  dio.DelayedWETHImpl,
		OptimismPortalImplAddress:               dio.OptimismPortalImpl,
		PreimageOracleSingletonAddress:          dio.PreimageOracleSingleton,
		MipsSingletonAddress:                    dio.MipsSingleton,
		SystemConfigImplAddress:                 dio.SystemConfigImpl,
		L1CrossDomainMessengerImplAddress:       dio.L1CrossDomainMessengerImpl,
		L1ERC721BridgeImplAddress:               dio.L1ERC721BridgeImpl,
		L1StandardBridgeImplAddress:             dio.L1StandardBridgeImpl,
		OptimismMintableERC20FactoryImplAddress: dio.OptimismMintableERC20FactoryImpl,
		DisputeGameFactoryImplAddress:           dio.DisputeGameFactoryImpl,
	}

	return nil
}

func shouldDeployImplementations(intent *state.Intent, st *state.State) bool {
	return st.ImplementationsDeployment == nil
}
