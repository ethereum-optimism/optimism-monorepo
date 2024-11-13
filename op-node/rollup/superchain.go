package rollup

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/params"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/superchain-registry/superchain"
)

var OPStackSupport = params.ProtocolVersionV0{Build: [8]byte{}, Major: 9, Minor: 0, Patch: 0, PreRelease: 1}.Encode()

// LoadOPStackRollupConfig loads the rollup configuration of the requested chain ID from the superchain-registry.
// Some chains may require a SystemConfigProvider to retrieve any values not part of the registry.
func LoadOPStackRollupConfig(chainID uint64) (*Config, error) {
	chConfig, ok := superchain.OPChains[chainID]
	if !ok {
		return nil, fmt.Errorf("unknown chain ID: %d", chainID)
	}

	superChain, ok := superchain.Superchains[chConfig.Superchain]
	if !ok {
		return nil, fmt.Errorf("chain %d specifies unknown superchain: %q", chainID, chConfig.Superchain)
	}

	var genesisSysConfig eth.SystemConfig
	if sysCfg, ok := superchain.GenesisSystemConfigs[chainID]; ok {
		genesisSysConfig = eth.SystemConfig{
			BatcherAddr: common.Address(sysCfg.BatcherAddr),
			Overhead:    eth.Bytes32(sysCfg.Overhead),
			Scalar:      eth.Bytes32(sysCfg.Scalar),
			GasLimit:    sysCfg.GasLimit,
		}
	} else {
		return nil, fmt.Errorf("unable to retrieve genesis SystemConfig of chain %d", chainID)
	}

	addrs, ok := superchain.Addresses[chainID]
	if !ok {
		return nil, fmt.Errorf("unable to retrieve deposit contract address")
	}

	var altDA *AltDAConfig
	if chConfig.AltDA != nil {
		altDA = &AltDAConfig{}
		if chConfig.AltDA.DAChallengeAddress != nil {
			altDA.DAChallengeAddress = common.Address(*chConfig.AltDA.DAChallengeAddress)
		}
		if chConfig.AltDA.DAChallengeWindow != nil {
			altDA.DAChallengeWindow = *chConfig.AltDA.DAChallengeWindow
		}
		if chConfig.AltDA.DAResolveWindow != nil {
			altDA.DAResolveWindow = *chConfig.AltDA.DAResolveWindow
		}
		if chConfig.AltDA.DACommitmentType != nil {
			altDA.CommitmentType = *chConfig.AltDA.DACommitmentType
		}
	}

	regolithTime := uint64(0)
	cfg := &Config{
		Genesis: Genesis{
			L1: eth.BlockID{
				Hash:   common.Hash(chConfig.Genesis.L1.Hash),
				Number: chConfig.Genesis.L1.Number,
			},
			L2: eth.BlockID{
				Hash:   common.Hash(chConfig.Genesis.L2.Hash),
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
		SeqWindowSize:          chConfig.SequencerWindowSize,
		ChannelTimeoutBedrock:  300,
		L1ChainID:              new(big.Int).SetUint64(superChain.Config.L1.ChainID),
		L2ChainID:              new(big.Int).SetUint64(chConfig.ChainID),
		RegolithTime:           &regolithTime,
		CanyonTime:             chConfig.CanyonTime,
		DeltaTime:              chConfig.DeltaTime,
		EcotoneTime:            chConfig.EcotoneTime,
		FjordTime:              chConfig.FjordTime,
		GraniteTime:            chConfig.GraniteTime,
		HoloceneTime:           chConfig.HoloceneTime,
		BatchInboxAddress:      common.Address(chConfig.BatchInboxAddr),
		DepositContractAddress: common.Address(addrs.OptimismPortalProxy),
		L1SystemConfigAddress:  common.Address(addrs.SystemConfigProxy),
		AltDAConfig:            altDA,
	}

	if superChain.Config.ProtocolVersionsAddr != nil { // Set optional protocol versions address
		cfg.ProtocolVersionsAddress = common.Address(*superChain.Config.ProtocolVersionsAddr)
	}
	return cfg, nil
}
