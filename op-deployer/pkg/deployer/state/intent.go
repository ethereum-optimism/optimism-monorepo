package state

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"

	"github.com/ethereum-optimism/optimism/op-chain-ops/devkeys"

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

type IntentConfigType string

const (
	IntentConfigTypeTest              IntentConfigType = "test"
	IntentConfigTypeStandard          IntentConfigType = "standard"
	IntentConfigTypeCustom            IntentConfigType = "custom"
	IntentConfigTypeStrict            IntentConfigType = "strict"
	IntentConfigTypeStandardOverrides IntentConfigType = "standard-overrides"
	IntentConfigTypeStrictOverrides   IntentConfigType = "strict-overrides"
)

var emptyAddress common.Address
var emptyHash common.Hash

type Intent struct {
	DeploymentStrategy    DeploymentStrategy `json:"deploymentStrategy" toml:"deploymentStrategy"`
	IntentConfigType      IntentConfigType   `json:"intentConfigType" toml:"intentConfigType"`
	L1ChainID             uint64             `json:"l1ChainID" toml:"l1ChainID"`
	SuperchainRoles       *SuperchainRoles   `json:"superchainRoles" toml:"superchainRoles,omitempty"`
	FundDevAccounts       bool               `json:"fundDevAccounts" toml:"fundDevAccounts"`
	UseInterop            bool               `json:"useInterop" toml:"useInterop"`
	L1ContractsLocator    *artifacts.Locator `json:"l1ContractsLocator" toml:"l1ContractsLocator"`
	L2ContractsLocator    *artifacts.Locator `json:"l2ContractsLocator" toml:"l2ContractsLocator"`
	Chains                []*ChainIntent     `json:"chains" toml:"chains"`
	GlobalDeployOverrides map[string]any     `json:"globalDeployOverrides" toml:"globalDeployOverrides"`
}

type SuperchainRoles struct {
	ProxyAdminOwner       common.Address `json:"proxyAdminOwner" toml:"proxyAdminOwner"`
	ProtocolVersionsOwner common.Address `json:"protocolVersionsOwner" toml:"protocolVersionsOwner"`
	Guardian              common.Address `json:"guardian" toml:"guardian"`
}

var ErrSuperchainRoleZeroAddress = fmt.Errorf("SuperchainRole is set to zero address")

func (s *SuperchainRoles) CheckNoZeroAddresses() error {
	val := reflect.ValueOf(*s)
	typ := reflect.TypeOf(*s)

	// Iterate through all the fields
	for i := 0; i < val.NumField(); i++ {
		fieldValue := val.Field(i)
		fieldName := typ.Field(i).Name

		if fieldValue.Interface() == (common.Address{}) {
			return fmt.Errorf("%w: %s", ErrSuperchainRoleZeroAddress, fieldName)
		}
	}
	return nil
}

func (c *Intent) L1ChainIDBig() *big.Int {
	return big.NewInt(int64(c.L1ChainID))
}

func (c *Intent) ValidateIntentConfigType() error {
	switch c.IntentConfigType {
	case IntentConfigTypeStandard:
		if err := c.validateStandardValues(); err != nil {
			return fmt.Errorf("failed to validate intent-config-type=standard: %w", err)
		}
	case IntentConfigTypeCustom:
		if err := c.validateCustomConfig(); err != nil {
			return fmt.Errorf("failed to validate intent-config-type=custom: %w", err)
		}
	case IntentConfigTypeStrict:
		if err := c.validateStrictConfig(); err != nil {
			return fmt.Errorf("failed to validate intent-config-type=strict: %w", err)
		}
	case IntentConfigTypeTest:
		return nil
	case IntentConfigTypeStandardOverrides, IntentConfigTypeStrictOverrides:
		return nil
	default:
		return fmt.Errorf("intent-config-type unsupported: %s", c.IntentConfigType)
	}
	return nil
}

func (c *Intent) validateCustomConfig() error {
	if c.L1ContractsLocator == nil || c.L1ContractsLocator.Tag == "undefined" {
		return errors.New("L1ContractsLocator undefined")
	}
	if c.L2ContractsLocator == nil || c.L2ContractsLocator.Tag == "undefined" {
		return errors.New("L2ContractsLocator undefined")
	}

	if err := c.SuperchainRoles.CheckNoZeroAddresses(); err != nil {
		return err
	}

	if len(c.Chains) == 0 {
		return errors.New("must define at least one l2 chain")
	}

	for _, chain := range c.Chains {
		if err := chain.CheckNoZeroValues(); err != nil {
			return err
		}
	}

	return nil
}

func (c *Intent) validateStrictConfig() error {
	if err := c.validateStandardValues(); err != nil {
		return err
	}

	challenger, _ := standard.ChallengerAddressFor(c.L1ChainID)
	l1ProxyAdminOwner, _ := standard.L1ProxyAdminOwner(c.L1ChainID)
	for chainIndex := range c.Chains {
		if c.Chains[chainIndex].Roles.Challenger != challenger {
			return fmt.Errorf("invalid challenger address for chain: %s", c.Chains[chainIndex].ID)
		}
		if c.Chains[chainIndex].Roles.L1ProxyAdminOwner != l1ProxyAdminOwner {
			return fmt.Errorf("invalid l1ProxyAdminOwner address for chain: %s", c.Chains[chainIndex].ID)
		}
	}

	return nil
}

// Ensures the following:
//  1. no zero-values for non-standard fields (user should have populated these)
//  2. no non-standard values for standard fields (user should not have changed these)
func (c *Intent) validateStandardValues() error {
	standardSuperchainRoles, err := getStandardSuperchainRoles(c.L1ChainID)
	if err != nil {
		return fmt.Errorf("error getting standard superchain roles: %w", err)
	}
	if *c.SuperchainRoles != *standardSuperchainRoles {
		return fmt.Errorf("SuperchainRoles does not match standard value")
	}

	for _, chain := range c.Chains {
		if chain.ID == emptyHash {
			return fmt.Errorf("missing l2 chain ID")
		}
		if err := chain.Roles.CheckNoZeroAddresses(); err != nil {
			return err
		}
		if chain.Eip1559DenominatorCanyon != standard.Eip1559DenominatorCanyon ||
			chain.Eip1559Denominator != standard.Eip1559Denominator ||
			chain.Eip1559Elasticity != standard.Eip1559Elasticity {
			return fmt.Errorf("%w: chainId=%s", ErrNonStandardValue, chain.ID)
		}
		if chain.BaseFeeVaultRecipient == emptyAddress ||
			chain.L1FeeVaultRecipient == emptyAddress ||
			chain.SequencerFeeVaultRecipient == emptyAddress {
			return fmt.Errorf("%w: chainId=%s", ErrFeeVaultZeroAddress, chain.ID)
		}
	}

	return nil
}

func getStandardSuperchainRoles(l1ChainId uint64) (*SuperchainRoles, error) {
	superCfg, err := standard.SuperchainFor(l1ChainId)
	if err != nil {
		return nil, fmt.Errorf("error getting superchain config: %w", err)
	}

	proxyAdmin, _ := standard.ManagerOwnerAddrFor(l1ChainId)
	guardian, _ := standard.GuardianAddressFor(l1ChainId)

	superchainRoles := &SuperchainRoles{
		ProxyAdminOwner:       proxyAdmin,
		ProtocolVersionsOwner: common.Address(*superCfg.Config.ProtocolVersionsAddr),
		Guardian:              guardian,
	}

	return superchainRoles, nil
}

func (c *Intent) SetInitValues(l2ChainIds []common.Hash) error {
	switch c.IntentConfigType {
	case IntentConfigTypeCustom:
		return c.setCustomValues(l2ChainIds)

	case IntentConfigTypeStandard:
		return c.setStandardValues(l2ChainIds)

	case IntentConfigTypeStrict:
		return c.setStrictValues(l2ChainIds)

	case IntentConfigTypeTest:
		return c.setTestValues(l2ChainIds)

	default:
		return fmt.Errorf("intent config type not supported")
	}
}

// Sets all Intent fields to their zero value with the expectation that the
// user will populate the values before running 'apply'
func (c *Intent) setCustomValues(l2ChainIds []common.Hash) error {
	c.L1ContractsLocator = &artifacts.Locator{Tag: "undefined"}
	c.L2ContractsLocator = &artifacts.Locator{Tag: "undefined"}

	c.SuperchainRoles = &SuperchainRoles{}
	for _, l2ChainID := range l2ChainIds {
		c.Chains = append(c.Chains, &ChainIntent{
			ID: l2ChainID,
		})
	}
	return nil
}

func (c *Intent) setStandardValues(l2ChainIds []common.Hash) error {
	superchainRoles, err := getStandardSuperchainRoles(c.L1ChainID)
	if err != nil {
		return fmt.Errorf("error getting standard superchain roles: %w", err)
	}
	c.SuperchainRoles = superchainRoles

	c.L1ContractsLocator = artifacts.DefaultL1ContractsLocator
	c.L2ContractsLocator = artifacts.DefaultL2ContractsLocator

	for _, l2ChainID := range l2ChainIds {
		c.Chains = append(c.Chains, &ChainIntent{
			ID:                       l2ChainID,
			Eip1559DenominatorCanyon: standard.Eip1559DenominatorCanyon,
			Eip1559Denominator:       standard.Eip1559Denominator,
			Eip1559Elasticity:        standard.Eip1559Elasticity,
		})
	}
	return nil
}

// Same as setStandardValues, but also sets l2 Challenger and L1ProxyAdminOwner
// addresses to standard values
func (c *Intent) setStrictValues(l2ChainIds []common.Hash) error {
	if err := c.setStandardValues(l2ChainIds); err != nil {
		return err
	}

	challenger, _ := standard.ChallengerAddressFor(c.L1ChainID)
	l1ProxyAdminOwner, _ := standard.ManagerOwnerAddrFor(c.L1ChainID)
	for chainIndex := range c.Chains {
		c.Chains[chainIndex].Roles.Challenger = challenger
		c.Chains[chainIndex].Roles.L1ProxyAdminOwner = l1ProxyAdminOwner
	}
	return nil
}

func (c *Intent) setTestValues(l2ChainIds []common.Hash) error {
	c.FundDevAccounts = true
	c.L1ContractsLocator = artifacts.DefaultL1ContractsLocator
	c.L2ContractsLocator = artifacts.DefaultL2ContractsLocator

	l1ChainIDBig := c.L1ChainIDBig()

	dk, err := devkeys.NewMnemonicDevKeys(devkeys.TestMnemonic)
	if err != nil {
		return fmt.Errorf("failed to create dev keys: %w", err)
	}

	addrFor := func(key devkeys.Key) common.Address {
		// The error below should never happen, so panic if it does.
		addr, err := dk.Address(key)
		if err != nil {
			panic(err)
		}
		return addr
	}
	c.SuperchainRoles = &SuperchainRoles{
		ProxyAdminOwner:       addrFor(devkeys.L1ProxyAdminOwnerRole.Key(l1ChainIDBig)),
		ProtocolVersionsOwner: addrFor(devkeys.SuperchainProtocolVersionsOwner.Key(l1ChainIDBig)),
		Guardian:              addrFor(devkeys.SuperchainConfigGuardianKey.Key(l1ChainIDBig)),
	}

	for _, l2ChainID := range l2ChainIds {
		l2ChainIDBig := l2ChainID.Big()
		c.Chains = append(c.Chains, &ChainIntent{
			ID:                         l2ChainID,
			BaseFeeVaultRecipient:      addrFor(devkeys.BaseFeeVaultRecipientRole.Key(l2ChainIDBig)),
			L1FeeVaultRecipient:        addrFor(devkeys.L1FeeVaultRecipientRole.Key(l2ChainIDBig)),
			SequencerFeeVaultRecipient: addrFor(devkeys.SequencerFeeVaultRecipientRole.Key(l2ChainIDBig)),
			Eip1559DenominatorCanyon:   standard.Eip1559DenominatorCanyon,
			Eip1559Denominator:         standard.Eip1559Denominator,
			Eip1559Elasticity:          standard.Eip1559Elasticity,
			Roles: ChainRoles{
				L1ProxyAdminOwner: addrFor(devkeys.L1ProxyAdminOwnerRole.Key(l2ChainIDBig)),
				L2ProxyAdminOwner: addrFor(devkeys.L2ProxyAdminOwnerRole.Key(l2ChainIDBig)),
				SystemConfigOwner: addrFor(devkeys.SystemConfigOwner.Key(l2ChainIDBig)),
				UnsafeBlockSigner: addrFor(devkeys.SequencerP2PRole.Key(l2ChainIDBig)),
				Batcher:           addrFor(devkeys.BatcherRole.Key(l2ChainIDBig)),
				Proposer:          addrFor(devkeys.ProposerRole.Key(l2ChainIDBig)),
				Challenger:        addrFor(devkeys.ChallengerRole.Key(l2ChainIDBig)),
			},
		})
	}
	return nil
}

func (c *Intent) Check() error {
	if err := c.DeploymentStrategy.Check(); err != nil {
		return err
	}

	if c.L1ChainID == 0 {
		return fmt.Errorf("l1ChainID must be set")
	}

	if c.L1ContractsLocator == nil {
		return errors.New("l1ContractsLocator must be set")
	}

	if c.L2ContractsLocator == nil {
		return errors.New("l2ContractsLocator must be set")
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
