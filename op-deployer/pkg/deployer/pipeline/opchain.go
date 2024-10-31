package pipeline

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"

	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/opcm"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func DeployOPChainLiveStrategy(ctx context.Context, env *Env, bundle ArtifactsBundle, intent *state.Intent, st *state.State, chainID common.Hash) error {
	lgr := env.Logger.New("stage", "deploy-opchain", "strategy", "live")

	if !shouldDeployOPChain(st, chainID) {
		lgr.Info("opchain deployment not needed")
		return nil
	}

	thisIntent, err := intent.Chain(chainID)
	if err != nil {
		return fmt.Errorf("failed to get chain intent: %w", err)
	}

	var deployFunc func() (opcm.DeployOPChainOutput, error)
	switch intent.L1ContractsLocator.Tag {
	case standard.ContractsV160Tag, standard.ContractsV170Beta1L2Tag:
		deployFunc = func() (opcm.DeployOPChainOutput, error) {
			input, err := makeDCIV160(intent, thisIntent, chainID, st)
			if err != nil {
				return opcm.DeployOPChainOutput{}, fmt.Errorf("error making deploy OP chain input: %w", err)
			}

			return opcm.DeployOPChainRawV160(ctx,
				env.L1Client,
				env.Broadcaster,
				env.Deployer,
				bundle.L1,
				input,
			)
		}
	default:
		deployFunc = func() (opcm.DeployOPChainOutput, error) {
			input, err := makeDCIIsthmus(intent, thisIntent, chainID, st)
			if err != nil {
				return opcm.DeployOPChainOutput{}, fmt.Errorf("error making deploy OP chain input: %w", err)
			}

			return opcm.DeployOPChainRawIsthmus(ctx,
				env.L1Client,
				env.Broadcaster,
				env.Deployer,
				bundle.L1,
				input,
			)
		}
	}

	var dco opcm.DeployOPChainOutput
	lgr.Info("deploying OP chain using existing OPCM", "id", chainID.Hex(), "opcmAddress", st.ImplementationsDeployment.OpcmProxyAddress.Hex())
	dco, err = deployFunc()
	if err != nil {
		return fmt.Errorf("error deploying OP chain: %w", err)
	}

	st.Chains = append(st.Chains, makeChainState(chainID, dco))
	opcmProxyAddress := st.ImplementationsDeployment.OpcmProxyAddress
	err = conditionallySetImplementationAddresses(ctx, env.L1Client, intent, st, dco, opcmProxyAddress)
	if err != nil {
		return fmt.Errorf("failed to set implementation addresses: %w", err)
	}

	return nil
}

// Only try to set the implementation addresses if we reused existing implementations from a release tag.
// The reason why these addresses could be empty is because only DeployOPChain.s.sol is invoked as part of the pipeline.
func conditionallySetImplementationAddresses(ctx context.Context, client *ethclient.Client, intent *state.Intent, st *state.State, dco opcm.DeployOPChainOutput, opcmProxyAddress common.Address) error {
	if !intent.L1ContractsLocator.IsTag() {
		return nil
	}

	block, err := client.BlockByNumber(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get latest block by number: %w", err)
	}
	currentBlockHash := block.Hash()

	errCh := make(chan error, 8)

	setImplementationAddressTasks := []func(){
		func() {
			setEIP1967ImplementationAddress(ctx, client, errCh, dco.DelayedWETHPermissionedGameProxy, currentBlockHash, &st.ImplementationsDeployment.DelayedWETHImplAddress)
		},
		func() {
			setEIP1967ImplementationAddress(ctx, client, errCh, dco.OptimismPortalProxy, currentBlockHash, &st.ImplementationsDeployment.OptimismPortalImplAddress)
		},
		func() {
			setEIP1967ImplementationAddress(ctx, client, errCh, dco.SystemConfigProxy, currentBlockHash, &st.ImplementationsDeployment.SystemConfigImplAddress)
		},
		func() {
			setRDPImplementationAddress(ctx, client, errCh, dco.AddressManager, &st.ImplementationsDeployment.L1CrossDomainMessengerImplAddress, "OVM_L1CrossDomainMessenger")
		},
		func() {
			setEIP1967ImplementationAddress(ctx, client, errCh, dco.L1ERC721BridgeProxy, currentBlockHash, &st.ImplementationsDeployment.L1ERC721BridgeImplAddress)
		},
		func() {
			setEIP1967ImplementationAddress(ctx, client, errCh, dco.L1StandardBridgeProxy, currentBlockHash, &st.ImplementationsDeployment.L1StandardBridgeImplAddress)
		},
		func() {
			setEIP1967ImplementationAddress(ctx, client, errCh, dco.OptimismMintableERC20FactoryProxy, currentBlockHash, &st.ImplementationsDeployment.OptimismMintableERC20FactoryImplAddress)
		},
		func() {
			setEIP1967ImplementationAddress(ctx, client, errCh, dco.DisputeGameFactoryProxy, currentBlockHash, &st.ImplementationsDeployment.DisputeGameFactoryImplAddress)
		},
		func() {
			setMipsSingletonAddress(ctx, client, intent.L1ContractsLocator, errCh, opcmProxyAddress, &st.ImplementationsDeployment.MipsSingletonAddress)
			setPreimageOracleAddress(ctx, client, errCh, st.ImplementationsDeployment.MipsSingletonAddress, &st.ImplementationsDeployment.PreimageOracleSingletonAddress)
		},
	}

	for _, task := range setImplementationAddressTasks {
		go task()
	}

	var lastTaskErr error
	for i := 0; i < len(setImplementationAddressTasks); i++ {
		taskErr := <-errCh
		if taskErr != nil {
			lastTaskErr = taskErr
		}
	}
	if lastTaskErr != nil {
		return fmt.Errorf("failed to set implementation addresses: %w", lastTaskErr)
	}

	return nil
}

func setMipsSingletonAddress(ctx context.Context, client *ethclient.Client, l1ArtifactsLocator *artifacts.Locator, errCh chan error, opcmProxyAddress common.Address, singletonAddress *common.Address) {
	if !l1ArtifactsLocator.IsTag() {
		errCh <- errors.New("L1 contracts locator is not a tag, cannot set MIPS singleton address")
		return
	}
	opcmContract := opcm.NewContract(opcmProxyAddress, client)
	mipsSingletonAddress, err := opcmContract.GetOPCMImplementationAddress(ctx, l1ArtifactsLocator.Tag, "MIPS")

	if err == nil {
		*singletonAddress = mipsSingletonAddress
	}
	errCh <- err
}

func setPreimageOracleAddress(ctx context.Context, client *ethclient.Client, errCh chan error, mipsSingletonAddress common.Address, preimageOracleAddress *common.Address) {
	opcmContract := opcm.NewContract(mipsSingletonAddress, client)
	preimageOracle, err := opcmContract.GenericAddressGetter(ctx, "oracle")
	if err == nil {
		*preimageOracleAddress = preimageOracle
	}
	errCh <- err
}

func DeployOPChainGenesisStrategy(env *Env, intent *state.Intent, st *state.State, chainID common.Hash) error {
	lgr := env.Logger.New("stage", "deploy-opchain", "strategy", "genesis")

	if !shouldDeployOPChain(st, chainID) {
		lgr.Info("opchain deployment not needed")
		return nil
	}

	thisIntent, err := intent.Chain(chainID)
	if err != nil {
		return fmt.Errorf("failed to get chain intent: %w", err)
	}

	var deployFunc func() (opcm.DeployOPChainOutput, error)
	switch intent.L1ContractsLocator.Tag {
	case standard.ContractsV160Tag, standard.ContractsV170Beta1L2Tag:
		deployFunc = func() (opcm.DeployOPChainOutput, error) {
			input, err := makeDCIV160(intent, thisIntent, chainID, st)
			if err != nil {
				return opcm.DeployOPChainOutput{}, fmt.Errorf("error making deploy OP chain input: %w", err)
			}

			return opcm.DeployOPChainV160(env.L1ScriptHost, input)
		}
	default:
		deployFunc = func() (opcm.DeployOPChainOutput, error) {
			input, err := makeDCIIsthmus(intent, thisIntent, chainID, st)
			if err != nil {
				return opcm.DeployOPChainOutput{}, fmt.Errorf("error making deploy OP chain input: %w", err)
			}

			return opcm.DeployOPChainIsthmus(env.L1ScriptHost, input)
		}
	}

	env.L1ScriptHost.ImportState(st.L1StateDump.Data)

	var dco opcm.DeployOPChainOutput
	lgr.Info("deploying OP chain using local allocs", "id", chainID.Hex())
	dco, err = deployFunc()
	if err != nil {
		return fmt.Errorf("error deploying OP chain: %w", err)
	}

	st.Chains = append(st.Chains, makeChainState(chainID, dco))

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
		OpcmProxy:                    st.ImplementationsDeployment.OpcmProxyAddress,
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

func setRDPImplementationAddress(ctx context.Context, client *ethclient.Client, errCh chan error, addressManager common.Address, implAddress *common.Address, getNameArg string) {
	if *implAddress != (common.Address{}) {
		errCh <- nil
		return
	}

	addressManagerContract := opcm.NewContract(addressManager, client)
	address, err := addressManagerContract.GetAddressByNameViaAddressManager(ctx, getNameArg)
	if err == nil {
		*implAddress = address
	}
	errCh <- err
}

func setEIP1967ImplementationAddress(ctx context.Context, client *ethclient.Client, errCh chan error, proxy common.Address, currentBlockHash common.Hash, implAddress *common.Address) {
	if *implAddress != (common.Address{}) {
		errCh <- nil
		return
	}

	storageValue, err := client.StorageAtHash(ctx, proxy, genesis.ImplementationSlot, currentBlockHash)
	if err == nil {
		*implAddress = common.HexToAddress(hex.EncodeToString(storageValue))
	}
	errCh <- err
}

func shouldDeployOPChain(st *state.State, chainID common.Hash) bool {
	for _, chain := range st.Chains {
		if chain.ID == chainID {
			return false
		}
	}

	return true
}
