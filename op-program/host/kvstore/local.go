package kvstore

import (
	"encoding/binary"
	"encoding/json"
	"errors"

	"github.com/ethereum-optimism/optimism/op-program/client/boot"
	"github.com/ethereum-optimism/optimism/op-program/host/config"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
)

type LocalPreimageSource struct {
	config *config.Config
}

func NewLocalPreimageSource(config *config.Config) *LocalPreimageSource {
	return &LocalPreimageSource{config}
}

var (
	l1HeadKey             = boot.L1HeadLocalIndex.PreimageKey()
	l2OutputRootKey       = boot.L2OutputRootLocalIndex.PreimageKey()
	l2ClaimKey            = boot.L2ClaimLocalIndex.PreimageKey()
	l2ClaimBlockNumberKey = boot.L2ClaimBlockNumberLocalIndex.PreimageKey()
	l2ChainIDKey          = boot.L2ChainIDLocalIndex.PreimageKey()
	l2ChainConfigKey      = boot.L2ChainConfigLocalIndex.PreimageKey()
	rollupKey             = boot.RollupConfigLocalIndex.PreimageKey()
)

var ErrUnexpectedLocalKey = errors.New("unexpected local key")

func (s *LocalPreimageSource) Get(key common.Hash) ([]byte, error) {
	switch [32]byte(key) {
	case l1HeadKey:
		return s.config.L1Head.Bytes(), nil
	case l2OutputRootKey:
		if s.config.InteropEnabled() {
			return s.config.InteropInputs.AgreedPrestateRoot.Bytes(), nil
		} else {
			return s.config.PreInteropInputs.L2OutputRoot.Bytes(), nil
		}
	case l2ClaimKey:
		if s.config.InteropEnabled() {
			return s.config.L2Claim.Bytes(), nil
		}
		return s.config.L2Claim.Bytes(), nil
	case l2ClaimBlockNumberKey:
		var value uint64
		if s.config.InteropEnabled() {
			value = s.config.InteropInputs.GameTimestamp
		} else {
			value = s.config.PreInteropInputs.L2ClaimBlockNumber
		}
		return binary.BigEndian.AppendUint64(nil, value), nil
	case l2ChainIDKey:
		if s.config.InteropEnabled() {
			return nil, ErrUnexpectedLocalKey
		}
		return binary.BigEndian.AppendUint64(nil, eth.EvilChainIDToUInt64(s.config.PreInteropInputs.L2ChainID)), nil
	case l2ChainConfigKey:
		if s.config.InteropEnabled() {
			return json.Marshal(s.config.L2ChainConfigs)
		} else {
			if s.config.PreInteropInputs.L2ChainID != boot.CustomChainIDIndicator {
				return nil, ErrNotFound
			}
			return json.Marshal(s.config.L2ChainConfigs[0])
		}
	case rollupKey:
		if s.config.InteropEnabled() {
			return json.Marshal(s.config.Rollups)
		} else {
			if s.config.PreInteropInputs.L2ChainID != boot.CustomChainIDIndicator {
				return nil, ErrNotFound
			}
			return json.Marshal(s.config.Rollups[0])
		}
	default:
		return nil, ErrNotFound
	}
}
