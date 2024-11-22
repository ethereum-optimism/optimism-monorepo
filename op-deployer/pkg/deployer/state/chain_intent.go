package state

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	"github.com/ethereum/go-ethereum/common"
)

type ChainIntent struct {
	ID                         common.Hash               `json:"id" toml:"id"`
	BaseFeeVaultRecipient      common.Address            `json:"baseFeeVaultRecipient" toml:"baseFeeVaultRecipient"`
	L1FeeVaultRecipient        common.Address            `json:"l1FeeVaultRecipient" toml:"l1FeeVaultRecipient"`
	SequencerFeeVaultRecipient common.Address            `json:"sequencerFeeVaultRecipient" toml:"sequencerFeeVaultRecipient"`
	Eip1559DenominatorCanyon   uint64                    `json:"eip1559DenominatorCanyon" toml:"eip1559DenominatorCanyon"`
	Eip1559Denominator         uint64                    `json:"eip1559Denominator" toml:"eip1559Denominator"`
	Eip1559Elasticity          uint64                    `json:"eip1559Elasticity" toml:"eip1559Elasticity"`
	Roles                      ChainRoles                `json:"roles" toml:"roles"`
	DeployOverrides            map[string]any            `json:"deployOverrides" toml:"deployOverrides"`
	DangerousAltDAConfig       genesis.AltDADeployConfig `json:"dangerousAltDAConfig,omitempty" toml:"dangerousAltDAConfig,omitempty"`
}

type ChainRoles struct {
	L1ProxyAdminOwner common.Address `json:"l1ProxyAdminOwner" toml:"l1ProxyAdminOwner"`
	L2ProxyAdminOwner common.Address `json:"l2ProxyAdminOwner" toml:"l2ProxyAdminOwner"`
	SystemConfigOwner common.Address `json:"systemConfigOwner" toml:"systemConfigOwner"`
	UnsafeBlockSigner common.Address `json:"unsafeBlockSigner" toml:"unsafeBlockSigner"`
	Batcher           common.Address `json:"batcher" toml:"batcher"`
	Proposer          common.Address `json:"proposer" toml:"proposer"`
	Challenger        common.Address `json:"challenger" toml:"challenger"`
}

var ErrChainRoleZeroAddress = fmt.Errorf("ChainRole is set to zero address")
var ErrFeeVaultZeroAddress = fmt.Errorf("chain has a fee vault set to zero address")
var ErrNonStandardValue = fmt.Errorf("chain contains non-standard config value")
var ErrEip1559ZeroValue = fmt.Errorf("eip1559 param is set to zero value")

func (c *ChainIntent) Check() error {
	var emptyHash common.Hash
	if c.ID == emptyHash {
		return fmt.Errorf("id must be set")
	}

	if c.Roles.L1ProxyAdminOwner == emptyAddress {
		return fmt.Errorf("proxyAdminOwner must be set")
	}

	if c.Roles.L2ProxyAdminOwner == emptyAddress {
		return fmt.Errorf("l2ProxyAdminOwner must be set")
	}

	if c.Roles.SystemConfigOwner == emptyAddress {
		c.Roles.SystemConfigOwner = c.Roles.L1ProxyAdminOwner
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

func (c *ChainIntent) CheckNoZeroValues() error {
	if c.ID == emptyHash {
		return errors.New("missing l2 chain ID")
	}
	if err := c.Roles.CheckNoZeroAddresses(); err != nil {
		return err
	}

	if c.Eip1559DenominatorCanyon == 0 ||
		c.Eip1559Denominator == 0 ||
		c.Eip1559Elasticity == 0 {
		return fmt.Errorf("%w: chainId=%s", ErrEip1559ZeroValue, c.ID)
	}
	if c.BaseFeeVaultRecipient == emptyAddress ||
		c.L1FeeVaultRecipient == emptyAddress ||
		c.SequencerFeeVaultRecipient == emptyAddress {
		return fmt.Errorf("%w: chainId=%s", ErrFeeVaultZeroAddress, c.ID)
	}

	return nil
}

// Returns an error if any fields in ChainRoles is set to common.Address{}
func (cr *ChainRoles) CheckNoZeroAddresses() error {
	val := reflect.ValueOf(*cr)
	typ := reflect.TypeOf(*cr)

	// Iterate through all the fields
	for i := 0; i < val.NumField(); i++ {
		fieldValue := val.Field(i)
		fieldName := typ.Field(i).Name

		if fieldValue.Interface() == (common.Address{}) {
			return fmt.Errorf("%w: %s", ErrChainRoleZeroAddress, fieldName)
		}
	}

	return nil
}
