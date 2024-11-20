package pipeline

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/opcm"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
	"github.com/ethereum/go-ethereum/common"
)

func DeployOPChain(env *Env, intent *state.Intent, st *state.State, chainID common.Hash) error {
	lgr := env.Logger.New("stage", "deploy-opchain")

	if !shouldDeployOPChain(st, chainID) {
		lgr.Info("opchain deployment not needed")
		return nil
	}

	thisIntent, err := intent.Chain(chainID)
	if err != nil {
		return fmt.Errorf("failed to get chain intent: %w", err)
	}

	var opcmAddr common.Address
	var deployFunc func() (opcm.DeployOPChainOutput, error)
	switch intent.L1ContractsLocator.Tag {
	case standard.ContractsV160Tag, standard.ContractsV170Beta1L2Tag:
		deployFunc = func() (opcm.DeployOPChainOutput, error) {
			input, err := makeDCIV160(intent, thisIntent, chainID, st)
			if err != nil {
				return opcm.DeployOPChainOutput{}, fmt.Errorf("error making deploy OP chain input: %w", err)
			}

			opcmAddr = input.Opcm
			return opcm.DeployOPChainV160(env.L1ScriptHost, input)
		}
	default:
		deployFunc = func() (opcm.DeployOPChainOutput, error) {
			input, err := makeDCIIsthmus(intent, thisIntent, chainID, st)
			if err != nil {
				return opcm.DeployOPChainOutput{}, fmt.Errorf("error making deploy OP chain input: %w", err)
			}

			opcmAddr = input.Opcm
			return opcm.DeployOPChainIsthmus(env.L1ScriptHost, input)
		}
	}

	var dco opcm.DeployOPChainOutput
	lgr.Info("deploying OP chain using local allocs", "id", chainID.Hex())
	dco, err = deployFunc()
	if err != nil {
		return fmt.Errorf("error deploying OP chain: %w", err)
	}

	st.Chains = append(st.Chains, makeChainState(chainID, dco))

	var release string
	if intent.L1ContractsLocator.IsTag() {
		release = intent.L1ContractsLocator.Tag
	} else {
		release = "dev"
	}

	readInput := opcm.ReadImplementationAddressesInput{
		DeployOPChainOutput: dco,
		Opcm:                opcmAddr,
		Release:             release,
	}
	impls, err := opcm.ReadImplementationAddresses(env.L1ScriptHost, readInput)
	if err != nil {
		return fmt.Errorf("failed to read implementation addresses: %w", err)
	}

	st.ImplementationsDeployment.DelayedWETHImplAddress = impls.DelayedWETH
	st.ImplementationsDeployment.OptimismPortalImplAddress = impls.OptimismPortal
	st.ImplementationsDeployment.SystemConfigImplAddress = impls.SystemConfig
	st.ImplementationsDeployment.L1CrossDomainMessengerImplAddress = impls.L1CrossDomainMessenger
	st.ImplementationsDeployment.L1ERC721BridgeImplAddress = impls.L1ERC721Bridge
	st.ImplementationsDeployment.L1StandardBridgeImplAddress = impls.L1StandardBridge
	st.ImplementationsDeployment.OptimismMintableERC20FactoryImplAddress = impls.OptimismMintableERC20Factory
	st.ImplementationsDeployment.DisputeGameFactoryImplAddress = impls.DisputeGameFactory
	st.ImplementationsDeployment.MipsSingletonAddress = impls.MipsSingleton
	st.ImplementationsDeployment.PreimageOracleSingletonAddress = impls.PreimageOracleSingleton

	return nil
}

type ChainProofParams struct {
	DisputeGameType                         uint32      `json:"disputeGameType" toml:"disputeGameType"`
	DisputeAbsolutePrestate                 common.Hash `json:"disputeAbsolutePrestate" toml:"disputeAbsolutePrestate"`
	DisputeMaxGameDepth                     uint64      `json:"disputeMaxGameDepth" toml:"disputeMaxGameDepth"`
	DisputeSplitDepth                       uint64      `json:"disputeSplitDepth" toml:"disputeSplitDepth"`
	DisputeClockExtension                   uint64      `json:"disputeClockExtension" toml:"disputeClockExtension"`
	DisputeMaxClockDuration                 uint64      `json:"disputeMaxClockDuration" toml:"disputeMaxClockDuration"`
	DangerouslyAllowCustomDisputeParameters bool        `json:"dangerouslyAllowCustomDisputeParameters" toml:"dangerouslyAllowCustomDisputeParameters"`
}

func makeDCIV160(intent *state.Intent, thisIntent *state.ChainIntent, chainID common.Hash, st *state.State) (opcm.DeployOPChainInputV160, error) {
	proofParams, err := jsonutil.MergeJSON(
		ChainProofParams{
			DisputeGameType:         standard.DisputeGameType,
			DisputeAbsolutePrestate: standard.DisputeAbsolutePrestate,
			DisputeMaxGameDepth:     standard.DisputeMaxGameDepth,
			DisputeSplitDepth:       standard.DisputeSplitDepth,
			DisputeClockExtension:   standard.DisputeClockExtension,
			DisputeMaxClockDuration: standard.DisputeMaxClockDuration,
		},
		intent.GlobalDeployOverrides,
		thisIntent.DeployOverrides,
	)
	if err != nil {
		return opcm.DeployOPChainInputV160{}, fmt.Errorf("error merging proof params from overrides: %w", err)
	}

	return opcm.DeployOPChainInputV160{
		OpChainProxyAdminOwner:       thisIntent.Roles.L1ProxyAdminOwner,
		SystemConfigOwner:            thisIntent.Roles.SystemConfigOwner,
		Batcher:                      thisIntent.Roles.Batcher,
		UnsafeBlockSigner:            thisIntent.Roles.UnsafeBlockSigner,
		Proposer:                     thisIntent.Roles.Proposer,
		Challenger:                   thisIntent.Roles.Challenger,
		BasefeeScalar:                standard.BasefeeScalar,
		BlobBaseFeeScalar:            standard.BlobBaseFeeScalar,
		L2ChainId:                    chainID.Big(),
		Opcm:                         st.ImplementationsDeployment.OpcmAddress,
		SaltMixer:                    st.Create2Salt.String(), // passing through salt generated at state initialization
		GasLimit:                     standard.GasLimit,
		DisputeGameType:              proofParams.DisputeGameType,
		DisputeAbsolutePrestate:      proofParams.DisputeAbsolutePrestate,
		DisputeMaxGameDepth:          proofParams.DisputeMaxGameDepth,
		DisputeSplitDepth:            proofParams.DisputeSplitDepth,
		DisputeClockExtension:        proofParams.DisputeClockExtension,   // 3 hours (input in seconds)
		DisputeMaxClockDuration:      proofParams.DisputeMaxClockDuration, // 3.5 days (input in seconds)
		AllowCustomDisputeParameters: proofParams.DangerouslyAllowCustomDisputeParameters,
	}, nil
}

func makeDCIIsthmus(intent *state.Intent, thisIntent *state.ChainIntent, chainID common.Hash, st *state.State) (opcm.DeployOPChainInputIsthmus, error) {
	dci, err := makeDCIV160(intent, thisIntent, chainID, st)
	if err != nil {
		return opcm.DeployOPChainInputIsthmus{}, fmt.Errorf("error making deploy OP chain input: %w", err)
	}

	return opcm.DeployOPChainInputIsthmus{
		DeployOPChainInputV160: dci,
		SystemConfigFeeAdmin:   common.Address{'D', 'E', 'A', 'D'},
	}, nil
}

func makeChainState(chainID common.Hash, dco opcm.DeployOPChainOutput) *state.ChainState {
	return &state.ChainState{
		ID:                                        chainID,
		ProxyAdminAddress:                         dco.OpChainProxyAdmin,
		AddressManagerAddress:                     dco.AddressManager,
		L1ERC721BridgeProxyAddress:                dco.L1ERC721BridgeProxy,
		SystemConfigProxyAddress:                  dco.SystemConfigProxy,
		OptimismMintableERC20FactoryProxyAddress:  dco.OptimismMintableERC20FactoryProxy,
		L1StandardBridgeProxyAddress:              dco.L1StandardBridgeProxy,
		L1CrossDomainMessengerProxyAddress:        dco.L1CrossDomainMessengerProxy,
		OptimismPortalProxyAddress:                dco.OptimismPortalProxy,
		DisputeGameFactoryProxyAddress:            dco.DisputeGameFactoryProxy,
		AnchorStateRegistryProxyAddress:           dco.AnchorStateRegistryProxy,
		AnchorStateRegistryImplAddress:            dco.AnchorStateRegistryImpl,
		FaultDisputeGameAddress:                   dco.FaultDisputeGame,
		PermissionedDisputeGameAddress:            dco.PermissionedDisputeGame,
		DelayedWETHPermissionedGameProxyAddress:   dco.DelayedWETHPermissionedGameProxy,
		DelayedWETHPermissionlessGameProxyAddress: dco.DelayedWETHPermissionlessGameProxy,
	}
}

func shouldDeployOPChain(st *state.State, chainID common.Hash) bool {
	for _, chain := range st.Chains {
		if chain.ID == chainID {
			return false
		}
	}

	return true
}
