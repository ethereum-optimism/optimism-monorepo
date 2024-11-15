package integration_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"testing"
	"time"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/inspect"
	"github.com/ethereum-optimism/optimism/op-node/rollup"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/pipeline"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/testutil"
	"github.com/ethereum-optimism/optimism/op-service/testutils/anvil"
	"github.com/ethereum/go-ethereum/crypto"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"

	"github.com/holiman/uint256"

	"github.com/ethereum-optimism/optimism/op-chain-ops/devkeys"
	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils/kurtosisutil"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

const TestParams = `
participants:
  - el_type: geth
    el_extra_params:
      - "--gcmode=archive"
      - "--rpc.txfeecap=0"
    cl_type: lighthouse
network_params:
  prefunded_accounts: '{ "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266": { "balance": "1000000ETH" } }'
  additional_preloaded_contracts: '{
    "0x4e59b44847b379578588920cA78FbF26c0B4956C": {
      balance: "0ETH",
      code: "0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf3",
      storage: {},
      nonce: 0,
      secretKey: "0x"
    }
  }'
  network_id: "77799777"
  seconds_per_slot: 3
  genesis_delay: 0
`

type deployerKey struct{}

func (d *deployerKey) HDPath() string {
	return "m/44'/60'/0'/0/0"
}

func (d *deployerKey) String() string {
	return "deployer-key"
}

func TestEndToEndApply(t *testing.T) {
	op_e2e.InitParallel(t)
	kurtosisutil.Test(t)

	lgr := testlog.Logger(t, slog.LevelDebug)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	enclaveCtx := kurtosisutil.StartEnclave(t, ctx, lgr, "github.com/ethpandaops/ethereum-package", TestParams)

	service, err := enclaveCtx.GetServiceContext("el-1-geth-lighthouse")
	require.NoError(t, err)

	ip := service.GetMaybePublicIPAddress()
	ports := service.GetPublicPorts()
	rpcURL := fmt.Sprintf("http://%s:%d", ip, ports["rpc"].GetNumber())
	l1Client, err := ethclient.Dial(rpcURL)
	require.NoError(t, err)

	depKey := new(deployerKey)
	l1ChainID := big.NewInt(77799777)
	dk, err := devkeys.NewMnemonicDevKeys(devkeys.TestMnemonic)
	require.NoError(t, err)
	pk, err := dk.Secret(depKey)
	require.NoError(t, err)

	l2ChainID1 := uint256.NewInt(1)
	l2ChainID2 := uint256.NewInt(2)

	loc, _ := testutil.LocalArtifacts(t)
	intent, st := newIntent(t, l1ChainID, dk, l2ChainID1, loc, loc)
	cg := ethClientCodeGetter(ctx, l1Client)

	t.Run("initial chain", func(t *testing.T) {
		require.NoError(t, deployer.ApplyPipeline(
			ctx,
			deployer.ApplyPipelineOpts{
				L1RPCUrl:           rpcURL,
				DeployerPrivateKey: pk,
				Intent:             intent,
				State:              st,
				Logger:             lgr,
				StateWriter:        pipeline.NoopStateWriter(),
			},
		))

		validateSuperchainDeployment(t, st, cg)
		validateOPChainDeployment(t, cg, st, intent)
	})

	t.Run("subsequent chain", func(t *testing.T) {
		// create a new environment with wiped state to ensure we can continue using the
		// state from the previous deployment
		intent.Chains = append(intent.Chains, newChainIntent(t, dk, l1ChainID, l2ChainID2))

		require.NoError(t, deployer.ApplyPipeline(
			ctx,
			deployer.ApplyPipelineOpts{
				L1RPCUrl:           rpcURL,
				DeployerPrivateKey: pk,
				Intent:             intent,
				State:              st,
				Logger:             lgr,
				StateWriter:        pipeline.NoopStateWriter(),
			},
		))

		validateOPChainDeployment(t, cg, st, intent)
	})
}

func TestApplyExistingOPCM(t *testing.T) {
	anvil.Test(t)

	forkRPCUrl := os.Getenv("SEPOLIA_RPC_URL")
	if forkRPCUrl == "" {
		t.Skip("no fork RPC URL provided")
	}

	lgr := testlog.Logger(t, slog.LevelDebug)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	runner, err := anvil.New(
		forkRPCUrl,
		lgr,
	)
	require.NoError(t, err)

	require.NoError(t, runner.Start(ctx))
	t.Cleanup(func() {
		require.NoError(t, runner.Stop())
	})

	l1Client, err := ethclient.Dial(runner.RPCUrl())
	require.NoError(t, err)

	l1ChainID := big.NewInt(11155111)
	dk, err := devkeys.NewMnemonicDevKeys(devkeys.TestMnemonic)
	require.NoError(t, err)
	// index 0 from Anvil's test set
	pk, err := crypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	require.NoError(t, err)

	l2ChainID := uint256.NewInt(1)

	intent, st := newIntent(
		t,
		l1ChainID,
		dk,
		l2ChainID,
		artifacts.DefaultL1ContractsLocator,
		artifacts.DefaultL2ContractsLocator,
	)
	// Define a new create2 salt to avoid contract address collisions
	_, err = rand.Read(st.Create2Salt[:])
	require.NoError(t, err)

	require.NoError(t, deployer.ApplyPipeline(
		ctx,
		deployer.ApplyPipelineOpts{
			L1RPCUrl:           runner.RPCUrl(),
			DeployerPrivateKey: pk,
			Intent:             intent,
			State:              st,
			Logger:             lgr,
			StateWriter:        pipeline.NoopStateWriter(),
		},
	))

	validateOPChainDeployment(t, ethClientCodeGetter(ctx, l1Client), st, intent)

	releases := standard.L1VersionsSepolia.Releases["op-contracts/v1.6.0"]

	implTests := []struct {
		name    string
		expAddr common.Address
		actAddr common.Address
	}{
		{"OptimismPortal", releases.OptimismPortal.ImplementationAddress, st.ImplementationsDeployment.OptimismPortalImplAddress},
		{"SystemConfig,", releases.SystemConfig.ImplementationAddress, st.ImplementationsDeployment.SystemConfigImplAddress},
		{"L1CrossDomainMessenger", releases.L1CrossDomainMessenger.ImplementationAddress, st.ImplementationsDeployment.L1CrossDomainMessengerImplAddress},
		{"L1ERC721Bridge", releases.L1ERC721Bridge.ImplementationAddress, st.ImplementationsDeployment.L1ERC721BridgeImplAddress},
		{"L1StandardBridge", releases.L1StandardBridge.ImplementationAddress, st.ImplementationsDeployment.L1StandardBridgeImplAddress},
		{"OptimismMintableERC20Factory", releases.OptimismMintableERC20Factory.ImplementationAddress, st.ImplementationsDeployment.OptimismMintableERC20FactoryImplAddress},
		{"DisputeGameFactory", releases.DisputeGameFactory.ImplementationAddress, st.ImplementationsDeployment.DisputeGameFactoryImplAddress},
		{"MIPS", releases.MIPS.Address, st.ImplementationsDeployment.MipsSingletonAddress},
		{"PreimageOracle", releases.PreimageOracle.Address, st.ImplementationsDeployment.PreimageOracleSingletonAddress},
		{"DelayedWETH", releases.DelayedWETH.ImplementationAddress, st.ImplementationsDeployment.DelayedWETHImplAddress},
	}
	for _, tt := range implTests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expAddr, tt.actAddr)
		})
	}
}

func TestL2BlockTimeOverride(t *testing.T) {
	op_e2e.InitParallel(t)
	kurtosisutil.Test(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts, intent, st := setupGenesisChain(t)
	intent.GlobalDeployOverrides = map[string]interface{}{
		"l2BlockTime": float64(3),
	}

	require.NoError(t, deployer.ApplyPipeline(ctx, opts))

	cfg, err := state.CombineDeployConfig(intent, intent.Chains[0], st, st.Chains[0])
	require.NoError(t, err)
	require.Equal(t, uint64(3), cfg.L2InitializationConfig.L2CoreDeployConfig.L2BlockTime, "L2 block time should be 3 seconds")
}

func TestApplyGenesisStrategy(t *testing.T) {
	op_e2e.InitParallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts, intent, st := setupGenesisChain(t)

	require.NoError(t, deployer.ApplyPipeline(ctx, opts))

	cg := stateDumpCodeGetter(st)
	validateSuperchainDeployment(t, st, cg)

	for i := range intent.Chains {
		t.Run(fmt.Sprintf("chain-%d", i), func(t *testing.T) {
			validateOPChainDeployment(t, cg, st, intent)
		})
	}
}

func TestProofParamOverrides(t *testing.T) {
	op_e2e.InitParallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts, intent, st := setupGenesisChain(t)
	intent.GlobalDeployOverrides = map[string]any{
		"withdrawalDelaySeconds":                  standard.WithdrawalDelaySeconds + 1,
		"minProposalSizeBytes":                    standard.MinProposalSizeBytes + 1,
		"challengePeriodSeconds":                  standard.ChallengePeriodSeconds + 1,
		"proofMaturityDelaySeconds":               standard.ProofMaturityDelaySeconds + 1,
		"disputeGameFinalityDelaySeconds":         standard.DisputeGameFinalityDelaySeconds + 1,
		"mipsVersion":                             standard.MIPSVersion + 1,
		"disputeGameType":                         standard.DisputeGameType, // This must be set to the permissioned game
		"disputeAbsolutePrestate":                 common.Hash{'A', 'B', 'S', 'O', 'L', 'U', 'T', 'E'},
		"disputeMaxGameDepth":                     standard.DisputeMaxGameDepth + 1,
		"disputeSplitDepth":                       standard.DisputeSplitDepth + 1,
		"disputeClockExtension":                   standard.DisputeClockExtension + 1,
		"disputeMaxClockDuration":                 standard.DisputeMaxClockDuration + 1,
		"dangerouslyAllowCustomDisputeParameters": true,
	}

	require.NoError(t, deployer.ApplyPipeline(ctx, opts))

	allocs := st.L1StateDump.Data.Accounts
	chainState := st.Chains[0]

	uint64Caster := func(t *testing.T, val any) common.Hash {
		return common.BigToHash(new(big.Int).SetUint64(val.(uint64)))
	}

	tests := []struct {
		name    string
		caster  func(t *testing.T, val any) common.Hash
		address common.Address
	}{
		{
			"withdrawalDelaySeconds",
			uint64Caster,
			st.ImplementationsDeployment.DelayedWETHImplAddress,
		},
		{
			"minProposalSizeBytes",
			uint64Caster,
			st.ImplementationsDeployment.PreimageOracleSingletonAddress,
		},
		{
			"challengePeriodSeconds",
			uint64Caster,
			st.ImplementationsDeployment.PreimageOracleSingletonAddress,
		},
		{
			"proofMaturityDelaySeconds",
			uint64Caster,
			st.ImplementationsDeployment.OptimismPortalImplAddress,
		},
		{
			"disputeGameFinalityDelaySeconds",
			uint64Caster,
			st.ImplementationsDeployment.OptimismPortalImplAddress,
		},
		{
			"disputeAbsolutePrestate",
			func(t *testing.T, val any) common.Hash {
				return val.(common.Hash)
			},
			chainState.PermissionedDisputeGameAddress,
		},
		{
			"disputeMaxGameDepth",
			uint64Caster,
			chainState.PermissionedDisputeGameAddress,
		},
		{
			"disputeSplitDepth",
			uint64Caster,
			chainState.PermissionedDisputeGameAddress,
		},
		{
			"disputeClockExtension",
			uint64Caster,
			chainState.PermissionedDisputeGameAddress,
		},
		{
			"disputeMaxClockDuration",
			uint64Caster,
			chainState.PermissionedDisputeGameAddress,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkImmutable(t, allocs, tt.address, tt.caster(t, intent.GlobalDeployOverrides[tt.name]))
		})
	}
}

func TestInteropDeployment(t *testing.T) {
	op_e2e.InitParallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts, intent, st := setupGenesisChain(t)
	intent.UseInterop = true

	require.NoError(t, deployer.ApplyPipeline(ctx, opts))

	chainState := st.Chains[0]
	depManagerSlot := common.HexToHash("0x1708e077affb93e89be2665fb0fb72581be66f84dc00d25fed755ae911905b1c")
	checkImmutable(t, st.L1StateDump.Data.Accounts, st.ImplementationsDeployment.SystemConfigImplAddress, depManagerSlot)
	proxyAdminOwnerHash := common.BytesToHash(intent.Chains[0].Roles.SystemConfigOwner.Bytes())
	checkStorageSlot(t, st.L1StateDump.Data.Accounts, chainState.SystemConfigProxyAddress, depManagerSlot, proxyAdminOwnerHash)
}

func TestAltDADeployment(t *testing.T) {
	op_e2e.InitParallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts, intent, st := setupGenesisChain(t)
	altDACfg := genesis.AltDADeployConfig{
		UseAltDA:                   true,
		DACommitmentType:           altda.KeccakCommitmentString,
		DAChallengeWindow:          10,
		DAResolveWindow:            10,
		DABondSize:                 100,
		DAResolverRefundPercentage: 50,
	}
	intent.Chains[0].DangerousAltDAConfig = altDACfg

	require.NoError(t, deployer.ApplyPipeline(ctx, opts))

	chainState := st.Chains[0]
	require.NotEmpty(t, chainState.DataAvailabilityChallengeProxyAddress)
	require.NotEmpty(t, chainState.DataAvailabilityChallengeImplAddress)

	_, rollupCfg, err := inspect.GenesisAndRollup(st, chainState.ID)
	require.NoError(t, err)
	require.EqualValues(t, &rollup.AltDAConfig{
		CommitmentType:     altda.KeccakCommitmentString,
		DAChallengeWindow:  altDACfg.DAChallengeWindow,
		DAChallengeAddress: chainState.DataAvailabilityChallengeProxyAddress,
		DAResolveWindow:    altDACfg.DAResolveWindow,
	}, rollupCfg.AltDAConfig)
}

func TestInvalidL2Genesis(t *testing.T) {
	op_e2e.InitParallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// these tests were generated by grepping all usages of the deploy
	// config in L2Genesis.s.sol.
	tests := []struct {
		name      string
		overrides map[string]any
	}{
		{
			name: "L2 proxy admin owner not set",
			overrides: map[string]any{
				"proxyAdminOwner": nil,
			},
		},
		{
			name: "base fee vault recipient not set",
			overrides: map[string]any{
				"baseFeeVaultRecipient": nil,
			},
		},
		{
			name: "l1 fee vault recipient not set",
			overrides: map[string]any{
				"l1FeeVaultRecipient": nil,
			},
		},
		{
			name: "sequencer fee vault recipient not set",
			overrides: map[string]any{
				"sequencerFeeVaultRecipient": nil,
			},
		},
		{
			name: "l1 chain ID not set",
			overrides: map[string]any{
				"l1ChainID": nil,
			},
		},
		{
			name: "l2 chain ID not set",
			overrides: map[string]any{
				"l2ChainID": nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, intent, _ := setupGenesisChain(t)
			intent.DeploymentStrategy = state.DeploymentStrategyGenesis
			intent.GlobalDeployOverrides = tt.overrides

			err := deployer.ApplyPipeline(ctx, opts)
			require.Error(t, err)
			require.ErrorContains(t, err, "failed to combine L2 init config")
		})
	}
}

func setupGenesisChain(t *testing.T) (deployer.ApplyPipelineOpts, *state.Intent, *state.State) {
	lgr := testlog.Logger(t, slog.LevelDebug)

	depKey := new(deployerKey)
	l1ChainID := big.NewInt(77799777)
	dk, err := devkeys.NewMnemonicDevKeys(devkeys.TestMnemonic)
	require.NoError(t, err)

	l2ChainID1 := uint256.NewInt(1)

	priv, err := dk.Secret(depKey)
	require.NoError(t, err)

	loc, _ := testutil.LocalArtifacts(t)

	intent, st := newIntent(t, l1ChainID, dk, l2ChainID1, loc, loc)
	intent.Chains = append(intent.Chains, newChainIntent(t, dk, l1ChainID, l2ChainID1))
	intent.DeploymentStrategy = state.DeploymentStrategyGenesis

	opts := deployer.ApplyPipelineOpts{
		DeployerPrivateKey: priv,
		Intent:             intent,
		State:              st,
		Logger:             lgr,
		StateWriter:        pipeline.NoopStateWriter(),
	}

	return opts, intent, st
}

func addrFor(t *testing.T, dk *devkeys.MnemonicDevKeys, key devkeys.Key) common.Address {
	addr, err := dk.Address(key)
	require.NoError(t, err)
	return addr
}

func newIntent(
	t *testing.T,
	l1ChainID *big.Int,
	dk *devkeys.MnemonicDevKeys,
	l2ChainID *uint256.Int,
	l1Loc *artifacts.Locator,
	l2Loc *artifacts.Locator,
) (*state.Intent, *state.State) {
	intent := &state.Intent{
		DeploymentStrategy: state.DeploymentStrategyLive,
		L1ChainID:          l1ChainID.Uint64(),
		SuperchainRoles: &state.SuperchainRoles{
			ProxyAdminOwner:       addrFor(t, dk, devkeys.L1ProxyAdminOwnerRole.Key(l1ChainID)),
			ProtocolVersionsOwner: addrFor(t, dk, devkeys.SuperchainDeployerKey.Key(l1ChainID)),
			Guardian:              addrFor(t, dk, devkeys.SuperchainConfigGuardianKey.Key(l1ChainID)),
		},
		FundDevAccounts:    true,
		L1ContractsLocator: l1Loc,
		L2ContractsLocator: l2Loc,
		Chains: []*state.ChainIntent{
			newChainIntent(t, dk, l1ChainID, l2ChainID),
		},
	}
	st := &state.State{
		Version: 1,
	}
	return intent, st
}

func newChainIntent(t *testing.T, dk *devkeys.MnemonicDevKeys, l1ChainID *big.Int, l2ChainID *uint256.Int) *state.ChainIntent {
	return &state.ChainIntent{
		ID:                         l2ChainID.Bytes32(),
		BaseFeeVaultRecipient:      addrFor(t, dk, devkeys.BaseFeeVaultRecipientRole.Key(l1ChainID)),
		L1FeeVaultRecipient:        addrFor(t, dk, devkeys.L1FeeVaultRecipientRole.Key(l1ChainID)),
		SequencerFeeVaultRecipient: addrFor(t, dk, devkeys.SequencerFeeVaultRecipientRole.Key(l1ChainID)),
		Eip1559Denominator:         50,
		Eip1559Elasticity:          6,
		Roles: state.ChainRoles{
			L1ProxyAdminOwner: addrFor(t, dk, devkeys.L2ProxyAdminOwnerRole.Key(l1ChainID)),
			L2ProxyAdminOwner: addrFor(t, dk, devkeys.L2ProxyAdminOwnerRole.Key(l1ChainID)),
			SystemConfigOwner: addrFor(t, dk, devkeys.SystemConfigOwner.Key(l1ChainID)),
			UnsafeBlockSigner: addrFor(t, dk, devkeys.SequencerP2PRole.Key(l1ChainID)),
			Batcher:           addrFor(t, dk, devkeys.BatcherRole.Key(l1ChainID)),
			Proposer:          addrFor(t, dk, devkeys.ProposerRole.Key(l1ChainID)),
			Challenger:        addrFor(t, dk, devkeys.ChallengerRole.Key(l1ChainID)),
		},
	}
}

type codeGetter func(t *testing.T, addr common.Address) []byte

func ethClientCodeGetter(ctx context.Context, client *ethclient.Client) codeGetter {
	return func(t *testing.T, addr common.Address) []byte {
		code, err := client.CodeAt(ctx, addr, nil)
		require.NoError(t, err)
		return code
	}
}

func stateDumpCodeGetter(st *state.State) codeGetter {
	return func(t *testing.T, addr common.Address) []byte {
		acc, ok := st.L1StateDump.Data.Accounts[addr]
		require.True(t, ok, "no account found for address %s", addr)
		return acc.Code
	}
}

func validateSuperchainDeployment(t *testing.T, st *state.State, cg codeGetter) {
	addrs := []struct {
		name string
		addr common.Address
	}{
		{"SuperchainProxyAdmin", st.SuperchainDeployment.ProxyAdminAddress},
		{"SuperchainConfigProxy", st.SuperchainDeployment.SuperchainConfigProxyAddress},
		{"SuperchainConfigImpl", st.SuperchainDeployment.SuperchainConfigImplAddress},
		{"ProtocolVersionsProxy", st.SuperchainDeployment.ProtocolVersionsProxyAddress},
		{"ProtocolVersionsImpl", st.SuperchainDeployment.ProtocolVersionsImplAddress},
		{"Opcm", st.ImplementationsDeployment.OpcmAddress},
		{"PreimageOracleSingleton", st.ImplementationsDeployment.PreimageOracleSingletonAddress},
		{"MipsSingleton", st.ImplementationsDeployment.MipsSingletonAddress},
	}
	for _, addr := range addrs {
		t.Run(addr.name, func(t *testing.T) {
			code := cg(t, addr.addr)
			require.NotEmpty(t, code, "contract %s at %s has no code", addr.name, addr.addr)
		})
	}
}

func validateOPChainDeployment(t *testing.T, cg codeGetter, st *state.State, intent *state.Intent) {
	// Validate that the implementation addresses are always set, even in subsequent deployments
	// that pull from an existing OPCM deployment.
	implAddrs := []struct {
		name string
		addr common.Address
	}{
		{"DelayedWETHImplAddress", st.ImplementationsDeployment.DelayedWETHImplAddress},
		{"OptimismPortalImplAddress", st.ImplementationsDeployment.OptimismPortalImplAddress},
		{"SystemConfigImplAddress", st.ImplementationsDeployment.SystemConfigImplAddress},
		{"L1CrossDomainMessengerImplAddress", st.ImplementationsDeployment.L1CrossDomainMessengerImplAddress},
		{"L1ERC721BridgeImplAddress", st.ImplementationsDeployment.L1ERC721BridgeImplAddress},
		{"L1StandardBridgeImplAddress", st.ImplementationsDeployment.L1StandardBridgeImplAddress},
		{"OptimismMintableERC20FactoryImplAddress", st.ImplementationsDeployment.OptimismMintableERC20FactoryImplAddress},
		{"DisputeGameFactoryImplAddress", st.ImplementationsDeployment.DisputeGameFactoryImplAddress},
		{"MipsSingletonAddress", st.ImplementationsDeployment.MipsSingletonAddress},
		{"PreimageOracleSingletonAddress", st.ImplementationsDeployment.PreimageOracleSingletonAddress},
	}
	for _, addr := range implAddrs {
		require.NotEmpty(t, addr.addr, "%s should be set", addr.name)
		code := cg(t, addr.addr)
		require.NotEmpty(t, code, "contract %s at %s has no code", addr.name, addr.addr)
	}

	for i, chainState := range st.Chains {
		chainAddrs := []struct {
			name string
			addr common.Address
		}{
			{"ProxyAdminAddress", chainState.ProxyAdminAddress},
			{"AddressManagerAddress", chainState.AddressManagerAddress},
			{"L1ERC721BridgeProxyAddress", chainState.L1ERC721BridgeProxyAddress},
			{"SystemConfigProxyAddress", chainState.SystemConfigProxyAddress},
			{"OptimismMintableERC20FactoryProxyAddress", chainState.OptimismMintableERC20FactoryProxyAddress},
			{"L1StandardBridgeProxyAddress", chainState.L1StandardBridgeProxyAddress},
			{"L1CrossDomainMessengerProxyAddress", chainState.L1CrossDomainMessengerProxyAddress},
			{"OptimismPortalProxyAddress", chainState.OptimismPortalProxyAddress},
			{"DisputeGameFactoryProxyAddress", chainState.DisputeGameFactoryProxyAddress},
			{"AnchorStateRegistryProxyAddress", chainState.AnchorStateRegistryProxyAddress},
			{"FaultDisputeGameAddress", chainState.FaultDisputeGameAddress},
			{"PermissionedDisputeGameAddress", chainState.PermissionedDisputeGameAddress},
			{"DelayedWETHPermissionedGameProxyAddress", chainState.DelayedWETHPermissionedGameProxyAddress},
			// {"DelayedWETHPermissionlessGameProxyAddress", chainState.DelayedWETHPermissionlessGameProxyAddress},
		}
		for _, addr := range chainAddrs {
			// TODO Delete this `if`` block once FaultDisputeGameAddress is deployed.
			if addr.name == "FaultDisputeGameAddress" {
				continue
			}
			code := cg(t, addr.addr)
			require.NotEmpty(t, code, "contract %s at %s for chain %s has no code", addr.name, addr.addr, chainState.ID)
		}

		alloc := chainState.Allocs.Data.Accounts

		chainIntent := intent.Chains[i]
		checkImmutableBehindProxy(t, alloc, predeploys.BaseFeeVaultAddr, chainIntent.BaseFeeVaultRecipient)
		checkImmutableBehindProxy(t, alloc, predeploys.L1FeeVaultAddr, chainIntent.L1FeeVaultRecipient)
		checkImmutableBehindProxy(t, alloc, predeploys.SequencerFeeVaultAddr, chainIntent.SequencerFeeVaultRecipient)
		checkImmutableBehindProxy(t, alloc, predeploys.OptimismMintableERC721FactoryAddr, common.BigToHash(new(big.Int).SetUint64(intent.L1ChainID)))

		// ownership slots
		var addrAsSlot common.Hash
		addrAsSlot.SetBytes(chainIntent.Roles.L1ProxyAdminOwner.Bytes())
		// slot 0
		ownerSlot := common.Hash{}
		checkStorageSlot(t, alloc, predeploys.ProxyAdminAddr, ownerSlot, addrAsSlot)
		var defaultGovOwner common.Hash
		defaultGovOwner.SetBytes(common.HexToAddress("0xDeaDDEaDDeAdDeAdDEAdDEaddeAddEAdDEAdDEad").Bytes())
		checkStorageSlot(t, alloc, predeploys.GovernanceTokenAddr, common.Hash{31: 0x0a}, defaultGovOwner)

		require.Equal(t, int(chainIntent.Eip1559Denominator), 50, "EIP1559Denominator should be set")
		require.Equal(t, int(chainIntent.Eip1559Elasticity), 6, "EIP1559Elasticity should be set")
	}
}

func getEIP1967ImplementationAddress(t *testing.T, allocations types.GenesisAlloc, proxyAddress common.Address) common.Address {
	storage := allocations[proxyAddress].Storage
	storageValue := storage[genesis.ImplementationSlot]
	require.NotEmpty(t, storageValue, "Implementation address for %s should be set", proxyAddress)
	return common.HexToAddress(storageValue.Hex())
}

type bytesMarshaler interface {
	Bytes() []byte
}

func checkImmutableBehindProxy(t *testing.T, allocations types.GenesisAlloc, proxyContract common.Address, thing bytesMarshaler) {
	implementationAddress := getEIP1967ImplementationAddress(t, allocations, proxyContract)
	checkImmutable(t, allocations, implementationAddress, thing)
}

func checkImmutable(t *testing.T, allocations types.GenesisAlloc, implementationAddress common.Address, thing bytesMarshaler) {
	account, ok := allocations[implementationAddress]
	require.True(t, ok, "%s not found in allocations", implementationAddress)
	require.NotEmpty(t, account.Code, "%s should have code", implementationAddress)
	require.True(
		t,
		bytes.Contains(account.Code, thing.Bytes()),
		"%s code should contain %s immutable", implementationAddress, hex.EncodeToString(thing.Bytes()),
	)
}

func checkStorageSlot(t *testing.T, allocs types.GenesisAlloc, address common.Address, slot common.Hash, expected common.Hash) {
	account, ok := allocs[address]
	require.True(t, ok, "account not found for address %s", address)
	value, ok := account.Storage[slot]
	if expected == (common.Hash{}) {
		require.False(t, ok, "slot %s for account %s should not be set", slot, address)
		return
	}
	require.True(t, ok, "slot %s not found for account %s", slot, address)
	require.Equal(t, expected, value, "slot %s for account %s should be %s", slot, address, expected)
}
