package boot

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-program/chainconfig"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
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
	RollupConfig(chainID eth.ChainID) (*rollup.Config, error)
	ChainConfig(chainID eth.ChainID) (*params.ChainConfig, error)
	DependencySet(chainID eth.ChainID) (depset.DependencySet, error)
}
type OracleConfigSource struct {
	oracle oracleClient

	customConfigsLoaded bool

	l2ChainConfigs map[eth.ChainID]*params.ChainConfig
	rollupConfigs  map[eth.ChainID]*rollup.Config
	depset         depset.DependencySet
}

func (c *OracleConfigSource) RollupConfig(chainID eth.ChainID) (*rollup.Config, error) {
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

func (c *OracleConfigSource) ChainConfig(chainID eth.ChainID) (*params.ChainConfig, error) {
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

func (c *OracleConfigSource) DependencySet(chainID eth.ChainID) (depset.DependencySet, error) {
	if c.depset != nil {
		return c.depset, nil
	}
	depSet, err := chainconfig.DependencySetByChainID(chainID)
	if !c.customConfigsLoaded && err != nil {
		c.loadCustomConfigs()
		if c.depset == nil {
			return nil, fmt.Errorf("%w: %v", ErrUnknownChainID, chainID)
		}
		return c.depset, nil
	} else if err != nil {
		return nil, err
	}
	c.depset = depSet
	return c.depset, nil
}

func (c *OracleConfigSource) loadCustomConfigs() {
	var rollupConfigs []*rollup.Config
	err := json.Unmarshal(c.oracle.Get(RollupConfigLocalIndex), &rollupConfigs)
	if err != nil {
		panic("failed to bootstrap rollup configs")
	}
	for _, config := range rollupConfigs {
		c.rollupConfigs[eth.ChainIDFromBig(config.L2ChainID)] = config
	}

	var chainConfigs []*params.ChainConfig
	err = json.Unmarshal(c.oracle.Get(L2ChainConfigLocalIndex), &chainConfigs)
	if err != nil {
		panic("failed to bootstrap chain configs")
	}
	for _, config := range chainConfigs {
		c.l2ChainConfigs[eth.ChainIDFromBig(config.ChainID)] = config
	}

	var depset depset.StaticConfigDependencySet
	err = json.Unmarshal(c.oracle.Get(DependencySetLocalIndex), &depset)
	if err != nil {
		panic("failed to bootstrap dependency set")
	}
	c.depset = &depset
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
			l2ChainConfigs: make(map[eth.ChainID]*params.ChainConfig),
			rollupConfigs:  make(map[eth.ChainID]*rollup.Config),
		},
		L1Head:         l1Head,
		AgreedPrestate: agreedPrestate,
		Claim:          claim,
		GameTimestamp:  claimTimestamp,
	}
}
