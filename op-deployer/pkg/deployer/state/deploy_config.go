package state

import (
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-service/jsonutil"

	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

var (
	l2GenesisBlockBaseFeePerGas = hexutil.Big(*(big.NewInt(1000000000)))

	vaultMinWithdrawalAmount = mustHexBigFromHex("0x8ac7230489e80000")
)

func CombineDeployConfig(intent *Intent, chainIntent *ChainIntent, state *State, chainState *ChainState) (genesis.DeployConfig, error) {
	cfg := genesis.DeployConfig{
		L1DependenciesConfig: genesis.L1DependenciesConfig{
			L1StandardBridgeProxy:       chainState.L1StandardBridgeProxyAddress,
			L1CrossDomainMessengerProxy: chainState.L1CrossDomainMessengerProxyAddress,
			L1ERC721BridgeProxy:         chainState.L1ERC721BridgeProxyAddress,
			SystemConfigProxy:           chainState.SystemConfigProxyAddress,
			OptimismPortalProxy:         chainState.OptimismPortalProxyAddress,
			ProtocolVersionsProxy:       state.SuperchainDeployment.ProtocolVersionsProxyAddress,
		},
		L2InitializationConfig: genesis.L2InitializationConfig{
			L2GenesisBlockDeployConfig: genesis.L2GenesisBlockDeployConfig{
				L2GenesisBlockGasLimit:      60_000_000,
				L2GenesisBlockBaseFeePerGas: &l2GenesisBlockBaseFeePerGas,
			},
			L2VaultsDeployConfig: genesis.L2VaultsDeployConfig{
				BaseFeeVaultWithdrawalNetwork:            "local",
				L1FeeVaultWithdrawalNetwork:              "local",
				SequencerFeeVaultWithdrawalNetwork:       "local",
				SequencerFeeVaultMinimumWithdrawalAmount: vaultMinWithdrawalAmount,
				BaseFeeVaultMinimumWithdrawalAmount:      vaultMinWithdrawalAmount,
				L1FeeVaultMinimumWithdrawalAmount:        vaultMinWithdrawalAmount,
				BaseFeeVaultRecipient:                    chainIntent.BaseFeeVaultRecipient,
				L1FeeVaultRecipient:                      chainIntent.L1FeeVaultRecipient,
				SequencerFeeVaultRecipient:               chainIntent.SequencerFeeVaultRecipient,
			},
			GovernanceDeployConfig: genesis.GovernanceDeployConfig{
				EnableGovernance:      true,
				GovernanceTokenSymbol: "OP",
				GovernanceTokenName:   "Optimism",
				GovernanceTokenOwner:  common.HexToAddress("0xDeaDDEaDDeAdDeAdDEAdDEaddeAddEAdDEAdDEad"),
			},
			GasPriceOracleDeployConfig: genesis.GasPriceOracleDeployConfig{
				GasPriceOracleBaseFeeScalar:     1368,
				GasPriceOracleBlobBaseFeeScalar: 810949,
			},
			EIP1559DeployConfig: genesis.EIP1559DeployConfig{
				EIP1559Denominator:       chainIntent.Eip1559Denominator,
				EIP1559DenominatorCanyon: 250,
				EIP1559Elasticity:        chainIntent.Eip1559Elasticity,
			},
			UpgradeScheduleDeployConfig: genesis.UpgradeScheduleDeployConfig{
				L2GenesisRegolithTimeOffset: u64UtilPtr(0),
				L2GenesisCanyonTimeOffset:   u64UtilPtr(0),
				L2GenesisDeltaTimeOffset:    u64UtilPtr(0),
				L2GenesisEcotoneTimeOffset:  u64UtilPtr(0),
				L2GenesisFjordTimeOffset:    u64UtilPtr(0),
				L2GenesisGraniteTimeOffset:  u64UtilPtr(0),
				UseInterop:                  intent.UseInterop,
			},
			L2CoreDeployConfig: genesis.L2CoreDeployConfig{
				L1ChainID:                 intent.L1ChainID,
				L2ChainID:                 chainState.ID.Big().Uint64(),
				L2BlockTime:               2,
				FinalizationPeriodSeconds: 12,
				MaxSequencerDrift:         600,
				SequencerWindowSize:       3600,
				ChannelTimeoutBedrock:     300,
				SystemConfigStartBlock:    0,
				BatchInboxAddress:         calculateBatchInboxAddr(chainState.ID),
			},
			OperatorDeployConfig: genesis.OperatorDeployConfig{
				BatchSenderAddress:  chainIntent.Roles.Batcher,
				P2PSequencerAddress: chainIntent.Roles.UnsafeBlockSigner,
			},
			OwnershipDeployConfig: genesis.OwnershipDeployConfig{
				ProxyAdminOwner:  chainIntent.Roles.L2ProxyAdminOwner,
				FinalSystemOwner: chainIntent.Roles.L1ProxyAdminOwner,
			},
		},
		FaultProofDeployConfig: genesis.FaultProofDeployConfig{
			UseFaultProofs:                  true,
			FaultGameWithdrawalDelay:        604800,
			PreimageOracleMinProposalSize:   126000,
			PreimageOracleChallengePeriod:   86400,
			ProofMaturityDelaySeconds:       604800,
			DisputeGameFinalityDelaySeconds: 302400,
		},
	}

	if intent.UseInterop {
		cfg.L2InitializationConfig.UpgradeScheduleDeployConfig.L2GenesisInteropTimeOffset = u64UtilPtr(0)
	}

	if chainState.StartBlock == nil {
		// These are dummy variables - see below for rationale.
		num := rpc.LatestBlockNumber
		cfg.L1StartingBlockTag = &genesis.MarshalableRPCBlockNumberOrHash{
			BlockNumber: &num,
		}
	} else {
		startHash := chainState.StartBlock.Hash()
		cfg.L1StartingBlockTag = &genesis.MarshalableRPCBlockNumberOrHash{
			BlockHash: &startHash,
		}
	}

	if chainIntent.DangerousAltDAConfig.UseAltDA {
		cfg.AltDADeployConfig = chainIntent.DangerousAltDAConfig
		cfg.L1DependenciesConfig.DAChallengeProxy = chainState.DataAvailabilityChallengeProxyAddress
	}

	// The below dummy variables are set in order to allow the deploy
	// config to pass validation. The validation checks are useful to
	// ensure that the L2 is properly configured. They are not used by
	// the L2 genesis script itself.

	cfg.L1BlockTime = 12
	dummyAddr := common.Address{19: 0x01}
	cfg.SuperchainL1DeployConfig = genesis.SuperchainL1DeployConfig{
		SuperchainConfigGuardian: dummyAddr,
	}
	cfg.OutputOracleDeployConfig = genesis.OutputOracleDeployConfig{
		L2OutputOracleSubmissionInterval: 1,
		L2OutputOracleStartingTimestamp:  1,
		L2OutputOracleProposer:           dummyAddr,
		L2OutputOracleChallenger:         dummyAddr,
	}
	// End of dummy variables

	// Apply overrides after setting the main values.
	var err error
	if len(intent.GlobalDeployOverrides) > 0 {
		cfg, err = jsonutil.MergeJSON(cfg, intent.GlobalDeployOverrides)
		if err != nil {
			return genesis.DeployConfig{}, fmt.Errorf("error merging global L2 overrides: %w", err)

		}
	}

	if len(chainIntent.DeployOverrides) > 0 {
		cfg, err = jsonutil.MergeJSON(cfg, chainIntent.DeployOverrides)
		if err != nil {
			return genesis.DeployConfig{}, fmt.Errorf("error merging chain L2 overrides: %w", err)
		}
	}

	if err := cfg.Check(log.New(log.DiscardHandler())); err != nil {
		return cfg, fmt.Errorf("combined deploy config failed validation: %w", err)
	}

	return cfg, nil
}

func mustHexBigFromHex(hex string) *hexutil.Big {
	num := hexutil.MustDecodeBig(hex)
	hexBig := hexutil.Big(*num)
	return &hexBig
}

func u64UtilPtr(in uint64) *hexutil.Uint64 {
	util := hexutil.Uint64(in)
	return &util
}

func calculateBatchInboxAddr(chainID common.Hash) common.Address {
	var out common.Address
	copy(out[1:], crypto.Keccak256(chainID[:])[:19])
	return out
}
