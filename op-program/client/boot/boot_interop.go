package boot

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-program/chainconfig"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

var (
	ErrUnknownChainID = errors.New("unknown chain id")
)

type BootInfoInterop struct {
	Configs ConfigSource

	L1Head         common.Hash
	AgreedPrestate common.Hash
	Claim          common.Hash
	GameTimestamp  uint64
}

type ConfigSource interface {
	RollupConfig(chainID uint64) (*rollup.Config, error)
	ChainConfig(chainID uint64) (*params.ChainConfig, error)
}
type OracleConfigSource struct {
	oracle oracleClient

	customConfigsLoaded bool

	l2ChainConfigs map[uint64]*params.ChainConfig
	rollupConfigs  map[uint64]*rollup.Config
}

func (c *OracleConfigSource) RollupConfig(chainID uint64) (*rollup.Config, error) {
	if cfg, ok := c.rollupConfigs[chainID]; ok {
		return cfg, nil
	}
	cfg, err := chainconfig.RollupConfigByChainID(chainID)
	if !c.customConfigsLoaded && err != nil {
		c.loadCustomConfigs()
		if cfg, ok := c.rollupConfigs[chainID]; !ok {
			return nil, fmt.Errorf("%w: %v", ErrUnknownChainID, chainID)
		} else {
			return cfg, nil
		}
	} else if err != nil {
		return nil, err
	}
	c.rollupConfigs[chainID] = cfg
	return cfg, nil
}

func (c *OracleConfigSource) ChainConfig(chainID uint64) (*params.ChainConfig, error) {
	if cfg, ok := c.l2ChainConfigs[chainID]; ok {
		return cfg, nil
	}
	cfg, err := chainconfig.ChainConfigByChainID(chainID)
	if !c.customConfigsLoaded && err != nil {
		c.loadCustomConfigs()
		if cfg, ok := c.l2ChainConfigs[chainID]; !ok {
			return nil, fmt.Errorf("%w: %v", ErrUnknownChainID, chainID)
		} else {
			return cfg, nil
		}
	} else if err != nil {
		return nil, err
	}
	c.l2ChainConfigs[chainID] = cfg
	return cfg, nil
}

func (c *OracleConfigSource) loadCustomConfigs() {
	var rollupConfigs []*rollup.Config
	err := json.Unmarshal(c.oracle.Get(RollupConfigLocalIndex), &rollupConfigs)
	if err != nil {
		panic("failed to bootstrap rollup configs")
	}
	for _, config := range rollupConfigs {
		c.rollupConfigs[config.L2ChainID.Uint64()] = config
	}

	var chainConfigs []*params.ChainConfig
	err = json.Unmarshal(c.oracle.Get(L2ChainConfigLocalIndex), &chainConfigs)
	if err != nil {
		panic("failed to bootstrap chain configs")
	}
	for _, config := range chainConfigs {
		c.l2ChainConfigs[config.ChainID.Uint64()] = config
	}
	c.customConfigsLoaded = true
}

func BootstrapInterop(r oracleClient) *BootInfoInterop {
	l1Head := common.BytesToHash(r.Get(L1HeadLocalIndex))
	agreedPrestate := common.BytesToHash(r.Get(L2OutputRootLocalIndex))
	claim := common.BytesToHash(r.Get(L2ClaimLocalIndex))
	claimTimestamp := binary.BigEndian.Uint64(r.Get(L2ClaimBlockNumberLocalIndex))

	return &BootInfoInterop{
		Configs: &OracleConfigSource{
			oracle:         r,
			l2ChainConfigs: make(map[uint64]*params.ChainConfig),
			rollupConfigs:  make(map[uint64]*rollup.Config),
		},
		L1Head:         l1Head,
		AgreedPrestate: agreedPrestate,
		Claim:          claim,
		GameTimestamp:  claimTimestamp,
	}
}
