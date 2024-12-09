package kvstore

import (
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum-optimism/optimism/op-program/client"
	"github.com/ethereum-optimism/optimism/op-program/host/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

func TestLocalPreimageSource(t *testing.T) {
	cfg := &config.Config{
		L2ChainID:          86,
		Rollup:             chaincfg.OPSepolia(),
		L1Head:             common.HexToHash("0x1111"),
		L2OutputRoot:       common.HexToHash("0x2222"),
		L2Claim:            common.HexToHash("0x3333"),
		L2ClaimBlockNumber: 1234,
		L2ChainConfig:      params.SepoliaChainConfig,
	}
	source := NewLocalPreimageSource(cfg)
	tests := []struct {
		name     string
		key      common.Hash
		expected []byte
	}{
		{"L1Head", l1HeadKey, cfg.L1Head.Bytes()},
		{"L2OutputRoot", l2OutputRootKey, cfg.L2OutputRoot.Bytes()},
		{"L2Claim", l2ClaimKey, cfg.L2Claim.Bytes()},
		{"L2ClaimBlockNumber", l2ClaimBlockNumberKey, binary.BigEndian.AppendUint64(nil, cfg.L2ClaimBlockNumber)},
		{"L2ChainID", l2ChainIDKey, binary.BigEndian.AppendUint64(nil, 86)},
		{"Rollup", rollupKey, nil},             // Only available for custom chain configs
		{"ChainConfig", l2ChainConfigKey, nil}, // Only available for custom chain configs
		{"Unknown", preimage.LocalIndexKey(1000).PreimageKey(), nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := source.Get(test.key)
			if test.expected == nil {
				require.ErrorIs(t, err, ErrNotFound)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.expected, result)
		})
	}
}

func TestGetCustomChainConfigPreimages(t *testing.T) {
	cfg := &config.Config{
		Rollup:             chaincfg.OPSepolia(),
		L2ChainID:          client.CustomChainIDIndicator,
		L1Head:             common.HexToHash("0x1111"),
		L2OutputRoot:       common.HexToHash("0x2222"),
		L2Claim:            common.HexToHash("0x3333"),
		L2ClaimBlockNumber: 1234,
		L2ChainConfig:      params.SepoliaChainConfig,
	}
	source := NewLocalPreimageSource(cfg)
	actualRollup, err := source.Get(rollupKey)
	require.NoError(t, err)
	require.Equal(t, asJson(t, cfg.Rollup), actualRollup)
	actualChainConfig, err := source.Get(l2ChainConfigKey)
	require.NoError(t, err)
	require.Equal(t, asJson(t, cfg.L2ChainConfig), actualChainConfig)
}

func asJson(t *testing.T, v any) []byte {
	d, err := json.Marshal(v)
	require.NoError(t, err)
	return d
}
