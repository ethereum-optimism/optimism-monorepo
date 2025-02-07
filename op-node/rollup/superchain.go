package rollup

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/params"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/superchain"
)

var OPStackSupport = params.ProtocolVersionV0{Build: [8]byte{}, Major: 9, Minor: 0, Patch: 0, PreRelease: 0}.Encode()

// LoadOPStackRollupConfig loads the rollup configuration of the requested chain ID from the superchain-registry.
// Some chains may require a SystemConfigProvider to retrieve any values not part of the registry.
func LoadOPStackRollupConfig(chainID uint64) (*Config, error) {
	chain, err := superchain.GetChain(chainID)
	if err != nil {
		return nil, fmt.Errorf("unable to get chain %d from superchain registry: %w", chainID, err)
	}

	chConfig, err := chain.Config()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve chain %d config: %w", chainID, err)
	}
	chOpConfig := &params.OptimismConfig{
		EIP1559Elasticity:        chConfig.Optimism.EIP1559Elasticity,
		EIP1559Denominator:       chConfig.Optimism.EIP1559Denominator,
		EIP1559DenominatorCanyon: chConfig.Optimism.EIP1559DenominatorCanyon,
	}

	superConfig, err := superchain.GetSuperchain(chain.Network)
	if err != nil {
		return nil, fmt.Errorf("unable to get superchain %q from superchain registry: %w", chain.Network, err)
	}

	sysCfg := chConfig.Genesis.SystemConfig

	genesisSysConfig := eth.SystemConfig{
		BatcherAddr: sysCfg.BatcherAddr,
		Overhead:    eth.Bytes32(sysCfg.Overhead),
		Scalar:      eth.Bytes32(sysCfg.Scalar),
		GasLimit:    sysCfg.GasLimit,
	}

	addrs := chConfig.Addresses

	var altDA *AltDAConfig
	if chConfig.AltDA != nil {
		altDA = &AltDAConfig{
			DAChallengeAddress: chConfig.AltDA.DaChallengeContractAddress,
			DAChallengeWindow:  chConfig.AltDA.DaChallengeWindow,
			DAResolveWindow:    chConfig.AltDA.DaResolveWindow,
			CommitmentType:     chConfig.AltDA.DaCommitmentType,
		}
	}

	hardforks := chConfig.Hardforks
	regolithTime := uint64(0)
	cfg := &Config{
		Genesis: Genesis{
			L1: eth.BlockID{
				Hash:   chConfig.Genesis.L1.Hash,
				Number: chConfig.Genesis.L1.Number,
			},
			L2: eth.BlockID{
				Hash:   chConfig.Genesis.L2.Hash,
				Number: chConfig.Genesis.L2.Number,
			},
			L2Time:       chConfig.Genesis.L2Time,
			SystemConfig: genesisSysConfig,
		},
		// The below chain parameters can be different per OP-Stack chain,
		// therefore they are read from the superchain-registry configs.
		// Note: hardcoded values are not yet represented in the registry but should be
		// soon, then will be read and set in the same fashion.
		BlockTime:              chConfig.BlockTime,
		MaxSequencerDrift:      chConfig.MaxSequencerDrift,
		SeqWindowSize:          chConfig.SeqWindowSize,
		ChannelTimeoutBedrock:  300,
		L1ChainID:              new(big.Int).SetUint64(superConfig.L1.ChainID),
		L2ChainID:              new(big.Int).SetUint64(chConfig.ChainID),
		RegolithTime:           &regolithTime,
		CanyonTime:             hardforks.CanyonTime,
		DeltaTime:              hardforks.DeltaTime,
		EcotoneTime:            hardforks.EcotoneTime,
		FjordTime:              hardforks.FjordTime,
		GraniteTime:            hardforks.GraniteTime,
		HoloceneTime:           hardforks.HoloceneTime,
		IsthmusTime:            hardforks.IsthmusTime,
		BatchInboxAddress:      chConfig.BatchInboxAddr,
		DepositContractAddress: *addrs.OptimismPortalProxy,
		L1SystemConfigAddress:  *addrs.SystemConfigProxy,
		AltDAConfig:            altDA,
		ChainOpConfig:          chOpConfig,
	}

	cfg.ProtocolVersionsAddress = superConfig.ProtocolVersionsAddr
	return cfg, nil
}
