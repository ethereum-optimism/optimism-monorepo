package kvstore

import (
	"encoding/binary"
	"encoding/json"

	"github.com/ethereum-optimism/optimism/op-program/client"
	"github.com/ethereum-optimism/optimism/op-program/host/config"
	"github.com/ethereum/go-ethereum/common"
)

type LocalPreimageSource struct {
	config *config.Config
}

func NewLocalPreimageSource(config *config.Config) *LocalPreimageSource {
	return &LocalPreimageSource{config}
}

var (
	l1HeadKey             = client.L1HeadLocalIndex.PreimageKey()
	l2OutputRootKey       = client.L2OutputRootLocalIndex.PreimageKey()
	l2ClaimKey            = client.L2ClaimLocalIndex.PreimageKey()
	l2ClaimBlockNumberKey = client.L2ClaimBlockNumberLocalIndex.PreimageKey()
	l2ChainIDKey          = client.L2ChainIDLocalIndex.PreimageKey()
	l2ChainConfigKey      = client.L2ChainConfigLocalIndex.PreimageKey()
	rollupKey             = client.RollupConfigLocalIndex.PreimageKey()
)

func (s *LocalPreimageSource) Get(key common.Hash) ([]byte, error) {
	switch [32]byte(key) {
	case l1HeadKey:
		return s.config.L1Head.Bytes(), nil
	case l2OutputRootKey:
		return s.config.L2OutputRoot.Bytes(), nil
	case l2ClaimKey:
		return s.config.L2Claim.Bytes(), nil
	case l2ClaimBlockNumberKey:
		return binary.BigEndian.AppendUint64(nil, s.config.L2ClaimBlockNumber), nil
	case l2ChainIDKey:
		return binary.BigEndian.AppendUint64(nil, s.config.L2ChainID), nil
	case l2ChainConfigKey:
		if s.config.L2ChainID != client.CustomChainIDIndicator {
			return nil, ErrNotFound
		}
		return json.Marshal(s.config.L2ChainConfig)
	case rollupKey:
		if s.config.L2ChainID != client.CustomChainIDIndicator {
			return nil, ErrNotFound
		}
		return json.Marshal(s.config.Rollup)
	default:
		return nil, ErrNotFound
	}
}
