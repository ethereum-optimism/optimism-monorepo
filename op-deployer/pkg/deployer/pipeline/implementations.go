package pipeline

import (
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-service/jsonutil"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/opcm"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
)

func DeployImplementations(env *Env, intent *state.Intent, st *state.State) error {
	lgr := env.Logger.New("stage", "deploy-implementations")

	if !shouldDeployImplementations(intent, st) {
		lgr.Info("implementations deployment not needed")
		return nil
	}

	lgr.Info("deploying implementations")

	var contractsRelease string
	var err error
	if intent.L1ContractsLocator.IsTag() {
		contractsRelease = intent.L1ContractsLocator.Tag
	} else {
		contractsRelease = "dev"
	}

	proofParams, err := jsonutil.MergeJSON(
		state.SuperchainProofParams{
			WithdrawalDelaySeconds:          standard.WithdrawalDelaySeconds,
			MinProposalSizeBytes:            standard.MinProposalSizeBytes,
			ChallengePeriodSeconds:          standard.ChallengePeriodSeconds,
			ProofMaturityDelaySeconds:       standard.ProofMaturityDelaySeconds,
			DisputeGameFinalityDelaySeconds: standard.DisputeGameFinalityDelaySeconds,
			MIPSVersion:                     standard.MIPSVersion,
		},
		intent.GlobalDeployOverrides,
	)
	if err != nil {
		return fmt.Errorf("error merging proof params from overrides: %w", err)
	}

	dio, err := opcm.DeployImplementations(
		env.L1ScriptHost,
		opcm.DeployImplementationsInput{
			WithdrawalDelaySeconds:          new(big.Int).SetUint64(proofParams.WithdrawalDelaySeconds),
			MinProposalSizeBytes:            new(big.Int).SetUint64(proofParams.MinProposalSizeBytes),
			ChallengePeriodSeconds:          new(big.Int).SetUint64(proofParams.ChallengePeriodSeconds),
			ProofMaturityDelaySeconds:       new(big.Int).SetUint64(proofParams.ProofMaturityDelaySeconds),
			DisputeGameFinalityDelaySeconds: new(big.Int).SetUint64(proofParams.DisputeGameFinalityDelaySeconds),
			MipsVersion:                     new(big.Int).SetUint64(proofParams.MIPSVersion),
			L1ContractsRelease:              contractsRelease,
			SuperchainConfigProxy:           st.SuperchainDeployment.SuperchainConfigProxyAddress,
			ProtocolVersionsProxy:           st.SuperchainDeployment.ProtocolVersionsProxyAddress,
			SuperchainProxyAdmin:            st.SuperchainDeployment.ProxyAdminAddress,
			UpgradeController:               intent.SuperchainRoles.ProxyAdminOwner,
			UseInterop:                      intent.UseInterop,
		},
	)
	if err != nil {
		return fmt.Errorf("error deploying implementations: %w", err)
	}

	st.ImplementationsDeployment = &state.ImplementationsDeployment{
		OpcmAddress:                             dio.Opcm,
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
		AnchorStateRegistryImplAddress:          dio.AnchorStateRegistryImpl,
	}

	return nil
}

func shouldDeployImplementations(intent *state.Intent, st *state.State) bool {
	return st.ImplementationsDeployment == nil
}
