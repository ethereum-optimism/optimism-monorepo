package manage

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-chain-ops/devkeys"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/pipeline"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/testutil"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils/anvil"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestDependencies(t *testing.T) {
	t.Parallel()

	lgr := testlog.Logger(t, slog.LevelDebug)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	runner, err := anvil.New(
		"",
		lgr,
	)
	require.NoError(t, err)

	require.NoError(t, runner.Start(ctx))
	t.Cleanup(func() {
		require.NoError(t, runner.Stop())
	})

	// Start by deploying a chain
	l1ChainID := uint64(31337)
	l1ChainIDBig := new(big.Int).SetUint64(l1ChainID)
	l2ChainID := uint64(777)
	l2ChainIDBig := new(big.Int).SetUint64(l2ChainID)
	dk, err := devkeys.NewMnemonicDevKeys(devkeys.TestMnemonic)
	require.NoError(t, err)

	loc, _ := testutil.LocalArtifacts(t)

	addrFor := func(role devkeys.Role) common.Address {
		addr, err := dk.Address(role.Key(l1ChainIDBig))
		require.NoError(t, err)
		return addr
	}

	deployerPrivStr := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	deployerPriv, err := crypto.HexToECDSA(deployerPrivStr)
	require.NoError(t, err)
	deployerAddr := crypto.PubkeyToAddress(deployerPriv.PublicKey)

	intent := &state.Intent{
		ConfigType: state.IntentConfigTypeCustom,
		L1ChainID:  l1ChainID,
		SuperchainRoles: &state.SuperchainRoles{
			ProxyAdminOwner:       addrFor(devkeys.L1ProxyAdminOwnerRole),
			ProtocolVersionsOwner: addrFor(devkeys.SuperchainDeployerKey),
			Guardian:              addrFor(devkeys.SuperchainConfigGuardianKey),
		},
		FundDevAccounts:    true,
		UseInterop:         true,
		L1ContractsLocator: loc,
		L2ContractsLocator: loc,
		Chains: []*state.ChainIntent{
			{
				ID:                         common.BigToHash(l2ChainIDBig),
				BaseFeeVaultRecipient:      addrFor(devkeys.BaseFeeVaultRecipientRole),
				L1FeeVaultRecipient:        addrFor(devkeys.L1FeeVaultRecipientRole),
				SequencerFeeVaultRecipient: addrFor(devkeys.SequencerFeeVaultRecipientRole),
				Eip1559DenominatorCanyon:   standard.Eip1559DenominatorCanyon,
				Eip1559Denominator:         standard.Eip1559Denominator,
				Eip1559Elasticity:          standard.Eip1559Elasticity,
				Roles: state.ChainRoles{
					L1ProxyAdminOwner: addrFor(devkeys.L2ProxyAdminOwnerRole),
					L2ProxyAdminOwner: addrFor(devkeys.L2ProxyAdminOwnerRole),
					// Set to deployer addr since it's prefunded in Anvil
					SystemConfigOwner: deployerAddr,
					UnsafeBlockSigner: addrFor(devkeys.SequencerP2PRole),
					Batcher:           addrFor(devkeys.BatcherRole),
					Proposer:          addrFor(devkeys.ProposerRole),
					Challenger:        addrFor(devkeys.ChallengerRole),
				},
			},
		},
	}
	st := &state.State{
		Version: 1,
	}

	opts := deployer.ApplyPipelineOpts{
		DeploymentTarget:   deployer.DeploymentTargetLive,
		L1RPCUrl:           runner.RPCUrl(),
		DeployerPrivateKey: deployerPriv,
		Intent:             intent,
		State:              st,
		Logger:             lgr,
		StateWriter:        pipeline.NoopStateWriter(),
	}
	require.NoError(t, deployer.ApplyPipeline(ctx, opts))

	// Now we can test the Dependencies function
	for _, remove := range []bool{true, false} {
		t.Run(fmt.Sprintf("remove=%v", remove), func(t *testing.T) {
			require.NoError(t, Dependencies(ctx, DependenciesConfig{
				L1RPCUrl:         runner.RPCUrl(),
				PrivateKey:       deployerPrivStr,
				Logger:           lgr,
				ArtifactsLocator: loc,
				ChainID:          common.BigToHash(big.NewInt(1234)),
				SystemConfig:     st.Chains[0].SystemConfigProxyAddress,
				Remove:           remove,
				privateKeyECDSA:  nil,
			}))
		})
	}
}
