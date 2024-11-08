package state

import (
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"

	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"
	"github.com/ethereum/go-ethereum/common"
)

type DeploymentStrategy string

const (
	DeploymentStrategyLive    DeploymentStrategy = "live"
	DeploymentStrategyGenesis DeploymentStrategy = "genesis"
)

func (d DeploymentStrategy) Check() error {
	switch d {
	case DeploymentStrategyLive, DeploymentStrategyGenesis:
		return nil
	default:
		return fmt.Errorf("deployment strategy must be 'live' or 'genesis'")
	}
}

var emptyAddress common.Address

type Intent struct {
	DeploymentStrategy DeploymentStrategy `json:"deploymentStrategy" toml:"deploymentStrategy"`

	L1ChainID uint64 `json:"l1ChainID" toml:"l1ChainID"`

	SuperchainRoles *SuperchainRoles `json:"superchainRoles" toml:"superchainRoles,omitempty"`

	FundDevAccounts bool `json:"fundDevAccounts" toml:"fundDevAccounts"`

	UseInterop bool `json:"useInterop" toml:"useInterop"`

	L1ContractsLocator *artifacts.Locator `json:"l1ContractsLocator" toml:"l1ContractsLocator"`

	L2ContractsLocator *artifacts.Locator `json:"l2ContractsLocator" toml:"l2ContractsLocator"`

	Chains []*ChainIntent `json:"chains" toml:"chains"`

	GlobalDeployOverrides map[string]any `json:"globalDeployOverrides" toml:"globalDeployOverrides"`
}

func (c *Intent) L1ChainIDBig() *big.Int {
	return big.NewInt(int64(c.L1ChainID))
}

func (c *Intent) Check() error {
	if c.DeploymentStrategy != DeploymentStrategyLive && c.DeploymentStrategy != DeploymentStrategyGenesis {
		return fmt.Errorf("deploymentStrategy must be 'live' or 'local'")
	}

	if c.L1ChainID == 0 {
		return fmt.Errorf("l1ChainID must be set")
	}

	if c.L1ContractsLocator == nil {
		c.L1ContractsLocator = artifacts.DefaultL1ContractsLocator
	}

	if c.L2ContractsLocator == nil {
		c.L2ContractsLocator = artifacts.DefaultL2ContractsLocator
	}

	var err error
	if c.L1ContractsLocator.IsTag() {
		err = c.checkL1Prod()
	} else {
		err = c.checkL1Dev()
	}
	if err != nil {
		return err
	}

	if c.L2ContractsLocator.IsTag() {
		if err := c.checkL2Prod(); err != nil {
			return err
		}
	}

	return nil
}

func (c *Intent) Chain(id common.Hash) (*ChainIntent, error) {
	for i := range c.Chains {
		if c.Chains[i].ID == id {
			return c.Chains[i], nil
		}
	}

	return nil, fmt.Errorf("chain %d not found", id)
}

func (c *Intent) WriteToFile(path string) error {
	return jsonutil.WriteTOML(c, ioutil.ToAtomicFile(path, 0o755))
}

func (c *Intent) checkL1Prod() error {
	versions, err := standard.L1VersionsFor(c.L1ChainID)
	if err != nil {
		return err
	}

	if _, ok := versions.Releases[c.L1ContractsLocator.Tag]; !ok {
		return fmt.Errorf("tag '%s' not found in standard versions", c.L1ContractsLocator.Tag)
	}

	return nil
}

func (c *Intent) checkL1Dev() error {
	if c.SuperchainRoles.ProxyAdminOwner == emptyAddress {
		return fmt.Errorf("proxyAdminOwner must be set")
	}

	if c.SuperchainRoles.ProtocolVersionsOwner == emptyAddress {
		c.SuperchainRoles.ProtocolVersionsOwner = c.SuperchainRoles.ProxyAdminOwner
	}

	if c.SuperchainRoles.Guardian == emptyAddress {
		c.SuperchainRoles.Guardian = c.SuperchainRoles.ProxyAdminOwner
	}

	return nil
}

func (c *Intent) checkL2Prod() error {
	_, err := standard.ArtifactsURLForTag(c.L2ContractsLocator.Tag)
	return err
}

type SuperchainRoles struct {
	ProxyAdminOwner common.Address `json:"proxyAdminOwner" toml:"proxyAdminOwner"`

	ProtocolVersionsOwner common.Address `json:"protocolVersionsOwner" toml:"protocolVersionsOwner"`

	Guardian common.Address `json:"guardian" toml:"guardian"`
}

type ChainIntent struct {
	ID common.Hash `json:"id" toml:"id"`

	BaseFeeVaultRecipient common.Address `json:"baseFeeVaultRecipient" toml:"baseFeeVaultRecipient"`

	L1FeeVaultRecipient common.Address `json:"l1FeeVaultRecipient" toml:"l1FeeVaultRecipient"`

	SequencerFeeVaultRecipient common.Address `json:"sequencerFeeVaultRecipient" toml:"sequencerFeeVaultRecipient"`

	Eip1559Denominator uint64 `json:"eip1559Denominator" toml:"eip1559Denominator"`

	Eip1559Elasticity uint64 `json:"eip1559Elasticity" toml:"eip1559Elasticity"`

	Roles ChainRoles `json:"roles" toml:"roles"`

	DeployOverrides map[string]any `json:"deployOverrides" toml:"deployOverrides"`

	DangerousAltDAConfig genesis.AltDADeployConfig `json:"dangerousAltDAConfig,omitempty" toml:"dangerousAltDAConfig,omitempty"`
}

type ChainRoles struct {
	L1ProxyAdminOwner common.Address `json:"l1ProxyAdminOwner" toml:"l1ProxyAdminOwner"`

	L2ProxyAdminOwner common.Address `json:"l2ProxyAdminOwner" toml:"l2ProxyAdminOwner"`

	SystemConfigOwner common.Address `json:"systemConfigOwner" toml:"systemConfigOwner"`

	UnsafeBlockSigner common.Address `json:"unsafeBlockSigner" toml:"unsafeBlockSigner"`

	Batcher common.Address `json:"batcher" toml:"batcher"`

	Proposer common.Address `json:"proposer" toml:"proposer"`

	Challenger common.Address `json:"challenger" toml:"challenger"`
}

func (c *ChainIntent) Check() error {
	var emptyHash common.Hash
	if c.ID == emptyHash {
		return fmt.Errorf("id must be set")
	}

	if c.Roles.L1ProxyAdminOwner == emptyAddress {
		return fmt.Errorf("proxyAdminOwner must be set")
	}

	if c.Roles.SystemConfigOwner == emptyAddress {
		c.Roles.SystemConfigOwner = c.Roles.L1ProxyAdminOwner
	}

	if c.Roles.L2ProxyAdminOwner == emptyAddress {
		return fmt.Errorf("l2ProxyAdminOwner must be set")
	}

	if c.Roles.UnsafeBlockSigner == emptyAddress {
		return fmt.Errorf("unsafeBlockSigner must be set")
	}

	if c.Roles.Batcher == emptyAddress {
		return fmt.Errorf("batcher must be set")
	}

	if c.DangerousAltDAConfig.UseAltDA {
		return c.DangerousAltDAConfig.Check(nil)
	}

	return nil
}
