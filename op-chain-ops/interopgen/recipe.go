package interopgen

import (
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/params"

	"github.com/ethereum-optimism/optimism/op-chain-ops/devkeys"
	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
)

type InteropDevRecipe struct {
	L1ChainID        uint64
	L2ChainIDs       []uint64
	GenesisTimestamp uint64
}

func (r *InteropDevRecipe) Build(addrs devkeys.Addresses) (*WorldConfig, error) {
	// L1 genesis
	l1Cfg := &L1Config{
		ChainID: new(big.Int).SetUint64(r.L1ChainID),
		DevL1DeployConfig: genesis.DevL1DeployConfig{
			L1BlockTime:             6,
			L1GenesisBlockTimestamp: hexutil.Uint64(r.GenesisTimestamp),
			L1GenesisBlockGasLimit:  30_000_000,
		},
		Prefund: make(map[common.Address]*big.Int),
	}

	// TODO(#11887): consider making the number of prefunded keys configurable.
	l1Users := devkeys.ChainUserKeys(l1Cfg.ChainID)
	for i := uint64(0); i < 20; i++ {
		userAddr, err := addrs.Address(l1Users(i))
		if err != nil {
			return nil, fmt.Errorf("failed to get L1 user addr %d: %w", i, err)
		}
		l1Cfg.Prefund[userAddr] = Ether(10_000_000)
	}

	superchainOps := devkeys.SuperchainOperatorKeys(l1Cfg.ChainID)

	superchainDeployer, err := addrs.Address(superchainOps(devkeys.SuperchainDeployerKey))
	if err != nil {
		return nil, err
	}
	superchainProxyAdmin, err := addrs.Address(superchainOps(devkeys.SuperchainProxyAdminOwner))
	if err != nil {
		return nil, err
	}
	superchainProtocolVersionsOwner, err := addrs.Address(superchainOps(devkeys.SuperchainProtocolVersionsOwner))
	if err != nil {
		return nil, err
	}
	superchainConfigGuardian, err := addrs.Address(superchainOps(devkeys.SuperchainConfigGuardianKey))
	if err != nil {
		return nil, err
	}
	l1Cfg.Prefund[superchainDeployer] = Ether(10_000_000)
	l1Cfg.Prefund[superchainProxyAdmin] = Ether(10_000_000)
	l1Cfg.Prefund[superchainConfigGuardian] = Ether(10_000_000)

	superchainCfg := &SuperchainConfig{
		ProxyAdminOwner:       superchainProxyAdmin,
		ProtocolVersionsOwner: superchainProtocolVersionsOwner,
		Deployer:              superchainDeployer,
		Implementations: OPCMImplementationsConfig{
			L1ContractsRelease: "dev",
			FaultProof: SuperFaultProofConfig{
				WithdrawalDelaySeconds:          big.NewInt(604800),
				MinProposalSizeBytes:            big.NewInt(10000),
				ChallengePeriodSeconds:          big.NewInt(120),
				ProofMaturityDelaySeconds:       big.NewInt(12),
				DisputeGameFinalityDelaySeconds: big.NewInt(6),
				MipsVersion:                     big.NewInt(1),
			},
			UseInterop:           true,
			StandardVersionsToml: standard.VersionsMainnetData,
		},
		SuperchainL1DeployConfig: genesis.SuperchainL1DeployConfig{
			RequiredProtocolVersion:    params.OPStackSupport,
			RecommendedProtocolVersion: params.OPStackSupport,
			SuperchainConfigGuardian:   superchainConfigGuardian,
		},
	}
	world := &WorldConfig{
		L1:         l1Cfg,
		Superchain: superchainCfg,
		L2s:        make(map[string]*L2Config),
	}
	for _, l2ChainID := range r.L2ChainIDs {
		l2Cfg, err := InteropL2DevConfig(r.L1ChainID, l2ChainID, addrs)
		if err != nil {
			return nil, fmt.Errorf("failed to generate L2 config for chain %d: %w", l2ChainID, err)
		}
		if err := prefundL2Accounts(l1Cfg, l2Cfg, addrs); err != nil {
			return nil, fmt.Errorf("failed to prefund addresses on L1 for L2 chain %d: %w", l2ChainID, err)
		}
		world.L2s[fmt.Sprintf("%d", l2ChainID)] = l2Cfg
	}
	return world, nil
}

func prefundL2Accounts(l1Cfg *L1Config, l2Cfg *L2Config, addrs devkeys.Addresses) error {
	l1Cfg.Prefund[l2Cfg.BatchSenderAddress] = Ether(10_000_000)
	l1Cfg.Prefund[l2Cfg.Deployer] = Ether(10_000_000)
	l1Cfg.Prefund[l2Cfg.FinalSystemOwner] = Ether(10_000_000)
	l1Cfg.Prefund[l2Cfg.SystemConfigOwner] = Ether(10_000_000)
	proposer, err := addrs.Address(devkeys.ChainOperatorKey{
		ChainID: new(big.Int).SetUint64(l2Cfg.L2ChainID),
		Role:    devkeys.ProposerRole,
	})
	if err != nil {
		return err
	}
	l1Cfg.Prefund[proposer] = Ether(10_000_000)
	challenger, err := addrs.Address(devkeys.ChainOperatorKey{
		ChainID: new(big.Int).SetUint64(l2Cfg.L2ChainID),
		Role:    devkeys.ChallengerRole,
	})
	if err != nil {
		return err
	}
	l1Cfg.Prefund[challenger] = Ether(10_000_000)
	return nil
}

func InteropL2DevConfig(l1ChainID, l2ChainID uint64, addrs devkeys.Addresses) (*L2Config, error) {
	// Padded chain ID, hex encoded, prefixed with 0xff like inboxes, then 0x02 to signify devnet.
	batchInboxAddress := common.HexToAddress(fmt.Sprintf("0xff02%016x", l2ChainID))
	chainOps := devkeys.ChainOperatorKeys(new(big.Int).SetUint64(l2ChainID))

	deployer, err := addrs.Address(chainOps(devkeys.DeployerRole))
	if err != nil {
		return nil, err
	}
	l1ProxyAdminOwner, err := addrs.Address(chainOps(devkeys.L1ProxyAdminOwnerRole))
	if err != nil {
		return nil, err
	}
	l2ProxyAdminOwner, err := addrs.Address(chainOps(devkeys.L2ProxyAdminOwnerRole))
	if err != nil {
		return nil, err
	}
	baseFeeVaultRecipient, err := addrs.Address(chainOps(devkeys.BaseFeeVaultRecipientRole))
	if err != nil {
		return nil, err
	}
	l1FeeVaultRecipient, err := addrs.Address(chainOps(devkeys.L1FeeVaultRecipientRole))
	if err != nil {
		return nil, err
	}
	sequencerFeeVaultRecipient, err := addrs.Address(chainOps(devkeys.SequencerFeeVaultRecipientRole))
	if err != nil {
		return nil, err
	}
	sequencerP2P, err := addrs.Address(chainOps(devkeys.SequencerP2PRole))
	if err != nil {
		return nil, err
	}
	batcher, err := addrs.Address(chainOps(devkeys.BatcherRole))
	if err != nil {
		return nil, err
	}
	proposer, err := addrs.Address(chainOps(devkeys.ProposerRole))
	if err != nil {
		return nil, err
	}
	challenger, err := addrs.Address(chainOps(devkeys.ChallengerRole))
	if err != nil {
		return nil, err
	}
	systemConfigOwner, err := addrs.Address(chainOps(devkeys.SystemConfigOwner))
	if err != nil {
		return nil, err
	}

	l2Cfg := &L2Config{
		Deployer:          deployer,
		Proposer:          proposer,
		Challenger:        challenger,
		SystemConfigOwner: systemConfigOwner,
		L2InitializationConfig: genesis.L2InitializationConfig{
			DevDeployConfig: genesis.DevDeployConfig{
				FundDevAccounts: true,
			},
			L2GenesisBlockDeployConfig: genesis.L2GenesisBlockDeployConfig{
				L2GenesisBlockGasLimit:      60_000_000,
				L2GenesisBlockBaseFeePerGas: (*hexutil.Big)(big.NewInt(params.InitialBaseFee)),
			},
			OwnershipDeployConfig: genesis.OwnershipDeployConfig{
				ProxyAdminOwner:  l2ProxyAdminOwner,
				FinalSystemOwner: l1ProxyAdminOwner,
			},
			L2VaultsDeployConfig: genesis.L2VaultsDeployConfig{
				BaseFeeVaultRecipient:                    baseFeeVaultRecipient,
				L1FeeVaultRecipient:                      l1FeeVaultRecipient,
				SequencerFeeVaultRecipient:               sequencerFeeVaultRecipient,
				BaseFeeVaultMinimumWithdrawalAmount:      (*hexutil.Big)(Ether(10)),
				L1FeeVaultMinimumWithdrawalAmount:        (*hexutil.Big)(Ether(10)),
				SequencerFeeVaultMinimumWithdrawalAmount: (*hexutil.Big)(Ether(10)),
				BaseFeeVaultWithdrawalNetwork:            "remote",
				L1FeeVaultWithdrawalNetwork:              "remote",
				SequencerFeeVaultWithdrawalNetwork:       "remote",
			},
			GovernanceDeployConfig: genesis.GovernanceDeployConfig{
				EnableGovernance: false,
			},
			GasPriceOracleDeployConfig: genesis.GasPriceOracleDeployConfig{
				GasPriceOracleBaseFeeScalar:     1368,
				GasPriceOracleBlobBaseFeeScalar: 810949,
			},
			GasTokenDeployConfig: genesis.GasTokenDeployConfig{
				UseCustomGasToken: false,
			},
			OperatorDeployConfig: genesis.OperatorDeployConfig{
				P2PSequencerAddress: sequencerP2P,
				BatchSenderAddress:  batcher,
			},
			EIP1559DeployConfig: genesis.EIP1559DeployConfig{
				EIP1559Elasticity:        6,
				EIP1559Denominator:       50,
				EIP1559DenominatorCanyon: 250,
			},
			UpgradeScheduleDeployConfig: genesis.UpgradeScheduleDeployConfig{
				L2GenesisRegolithTimeOffset: new(hexutil.Uint64),
				L2GenesisCanyonTimeOffset:   new(hexutil.Uint64),
				L2GenesisDeltaTimeOffset:    new(hexutil.Uint64),
				L2GenesisEcotoneTimeOffset:  new(hexutil.Uint64),
				L2GenesisFjordTimeOffset:    new(hexutil.Uint64),
				L2GenesisGraniteTimeOffset:  new(hexutil.Uint64),
				L2GenesisHoloceneTimeOffset: new(hexutil.Uint64),
				L2GenesisInteropTimeOffset:  new(hexutil.Uint64),
				L1CancunTimeOffset:          new(hexutil.Uint64),
				UseInterop:                  true,
			},
			L2CoreDeployConfig: genesis.L2CoreDeployConfig{
				L1ChainID:                 l1ChainID,
				L2ChainID:                 l2ChainID,
				L2BlockTime:               2,
				FinalizationPeriodSeconds: 2, // instant output finalization
				MaxSequencerDrift:         300,
				SequencerWindowSize:       200,
				ChannelTimeoutBedrock:     120,
				BatchInboxAddress:         batchInboxAddress,
				SystemConfigStartBlock:    0,
			},
			AltDADeployConfig: genesis.AltDADeployConfig{
				UseAltDA: false,
			},
		},
		Prefund:                 make(map[common.Address]*big.Int),
		SaltMixer:               "",
		GasLimit:                60_000_000,
		DisputeGameType:         1, // PERMISSIONED_CANNON Game Type
		DisputeAbsolutePrestate: common.HexToHash("0x038512e02c4c3f7bdaec27d00edf55b7155e0905301e1a88083e4e0a6764d54c"),
		DisputeMaxGameDepth:     73,
		DisputeSplitDepth:       30,
		DisputeClockExtension:   10800,  // 3 hours (input in seconds)
		DisputeMaxClockDuration: 302400, // 3.5 days (input in seconds)
	}

	// TODO(#11887): consider making the number of prefunded keys configurable.
	l2Users := devkeys.ChainUserKeys(new(big.Int).SetUint64(l2ChainID))
	for i := uint64(0); i < 20; i++ {
		userAddr, err := addrs.Address(l2Users(i))
		if err != nil {
			return nil, fmt.Errorf("failed to get L2 user addr %d: %w", i, err)
		}
		l2Cfg.Prefund[userAddr] = Ether(10_000_000)
	}

	l2Cfg.Prefund[l2ProxyAdminOwner] = Ether(10_000_000)

	return l2Cfg, nil
}

var etherScalar = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

// Ether converts a uint64 Ether amount into a *big.Int amount in wei units, for allocating test balances.
func Ether(v uint64) *big.Int {
	return new(big.Int).Mul(new(big.Int).SetUint64(v), etherScalar)
}
