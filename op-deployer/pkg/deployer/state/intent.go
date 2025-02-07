package state

import (
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"reflect"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"

	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"
	"github.com/ethereum/go-ethereum/common"
)

type IntentConfigType string

const (
	IntentConfigTypeStandard          IntentConfigType = "standard"
	IntentConfigTypeCustom            IntentConfigType = "custom"
	IntentConfigTypeStrict            IntentConfigType = "strict"
	IntentConfigTypeStandardOverrides IntentConfigType = "standard-overrides"
	IntentConfigTypeStrictOverrides   IntentConfigType = "strict-overrides"
)

var emptyAddress common.Address
var emptyHash common.Hash

type SuperchainProofParams struct {
	WithdrawalDelaySeconds          uint64 `json:"faultGameWithdrawalDelay" toml:"faultGameWithdrawalDelay"`
	MinProposalSizeBytes            uint64 `json:"preimageOracleMinProposalSize" toml:"preimageOracleMinProposalSize"`
	ChallengePeriodSeconds          uint64 `json:"preimageOracleChallengePeriod" toml:"preimageOracleChallengePeriod"`
	ProofMaturityDelaySeconds       uint64 `json:"proofMaturityDelaySeconds" toml:"proofMaturityDelaySeconds"`
	DisputeGameFinalityDelaySeconds uint64 `json:"disputeGameFinalityDelaySeconds" toml:"disputeGameFinalityDelaySeconds"`
	MIPSVersion                     uint64 `json:"mipsVersion" toml:"mipsVersion"`
}

type Intent struct {
	ConfigType            IntentConfigType   `json:"configType" toml:"configType"`
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

var ErrSuperchainRoleZeroAddress = errors.New("SuperchainRole is set to zero address")
var ErrL1ContractsLocatorUndefined = errors.New("L1ContractsLocator undefined")
var ErrL2ContractsLocatorUndefined = errors.New("L2ContractsLocator undefined")

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

func (c *Intent) validateCustomConfig() error {
	if c.L1ContractsLocator == nil ||
		(c.L1ContractsLocator.Tag == "" && c.L1ContractsLocator.URL == &url.URL{}) {
		return ErrL1ContractsLocatorUndefined
	}
	if c.L2ContractsLocator == nil ||
		(c.L2ContractsLocator.Tag == "" && c.L2ContractsLocator.URL == &url.URL{}) {
		return ErrL2ContractsLocatorUndefined
	}

	if c.SuperchainRoles == nil {
		return errors.New("SuperchainRoles is set to nil")
	}
	if err := c.SuperchainRoles.CheckNoZeroAddresses(); err != nil {
		return err
	}

	if len(c.Chains) == 0 {
		return errors.New("must define at least one l2 chain")
	}

	for _, chain := range c.Chains {
		if err := chain.Check(); err != nil {
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
	if err := c.checkL1Prod(); err != nil {
		return err
	}
	if err := c.checkL2Prod(); err != nil {
		return err
	}

	standardSuperchainRoles, err := getStandardSuperchainRoles(c.L1ChainID)
	if err != nil {
		return fmt.Errorf("error getting standard superchain roles: %w", err)
	}
	if c.SuperchainRoles == nil || *c.SuperchainRoles != *standardSuperchainRoles {
		return fmt.Errorf("SuperchainRoles does not match standard value")
	}

	for _, chain := range c.Chains {
		if err := chain.Check(); err != nil {
			return err
		}
		if chain.Eip1559DenominatorCanyon != standard.Eip1559DenominatorCanyon ||
			chain.Eip1559Denominator != standard.Eip1559Denominator ||
			chain.Eip1559Elasticity != standard.Eip1559Elasticity {
			return fmt.Errorf("%w: chainId=%s", ErrNonStandardValue, chain.ID)
		}
		if len(chain.AdditionalDisputeGames) > 0 {
			return fmt.Errorf("%w: chainId=%s additionalDisputeGames must be nil", ErrNonStandardValue, chain.ID)
		}
	}

	return nil
}

func getStandardSuperchainRoles(l1ChainId uint64) (*SuperchainRoles, error) {
	proxyAdminOwner, err := standard.L1ProxyAdminOwner(l1ChainId)
	if err != nil {
		return nil, fmt.Errorf("error getting L1ProxyAdminOwner: %w", err)
	}
	guardian, err := standard.GuardianAddressFor(l1ChainId)
	if err != nil {
		return nil, fmt.Errorf("error getting guardian address: %w", err)
	}
	protocolVersionsOwner, err := standard.ProtocolVersionsOwner(l1ChainId)
	if err != nil {
		return nil, fmt.Errorf("error getting protocol versions owner: %w", err)
	}

	superchainRoles := &SuperchainRoles{
		ProxyAdminOwner:       proxyAdminOwner,
		ProtocolVersionsOwner: protocolVersionsOwner,
		Guardian:              guardian,
	}

	return superchainRoles, nil
}

func (c *Intent) Check() error {
	if c.L1ChainID == 0 {
		return fmt.Errorf("l1ChainID cannot be 0")
	}

	if c.L1ContractsLocator == nil {
		return ErrL1ContractsLocatorUndefined
	}

	if c.L2ContractsLocator == nil {
		return ErrL2ContractsLocatorUndefined
	}

	var err error
	switch c.ConfigType {
	case IntentConfigTypeStandard:
		err = c.validateStandardValues()
	case IntentConfigTypeCustom:
		err = c.validateCustomConfig()
	case IntentConfigTypeStrict:
		err = c.validateStrictConfig()
	case IntentConfigTypeStandardOverrides, IntentConfigTypeStrictOverrides:
		err = c.validateCustomConfig()
	default:
		return fmt.Errorf("intent-config-type unsupported: %s", c.ConfigType)
	}
	if err != nil {
		return fmt.Errorf("failed to validate intent-config-type=%s: %w", c.ConfigType, err)
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

	if _, ok := versions[c.L1ContractsLocator.Tag]; !ok {
		return fmt.Errorf("tag '%s' not found in standard versions", c.L1ContractsLocator.Tag)
	}

	return nil
}

func (c *Intent) checkL2Prod() error {
	_, err := standard.ArtifactsURLForTag(c.L2ContractsLocator.Tag)
	return err
}

func NewIntent(configType IntentConfigType, l1ChainId uint64, l2ChainIds []common.Hash) (Intent, error) {
	switch configType {
	case IntentConfigTypeCustom:
		return NewIntentCustom(l1ChainId, l2ChainIds)

	case IntentConfigTypeStandard:
		return NewIntentStandard(l1ChainId, l2ChainIds)

	case IntentConfigTypeStandardOverrides:
		return NewIntentStandardOverrides(l1ChainId, l2ChainIds)

	case IntentConfigTypeStrict:
		return NewIntentStrict(l1ChainId, l2ChainIds)

	case IntentConfigTypeStrictOverrides:
		return NewIntentStrictOverrides(l1ChainId, l2ChainIds)

	default:
		return Intent{}, fmt.Errorf("intent config type not supported")
	}
}

// Sets all Intent fields to their zero value with the expectation that the
// user will populate the values before running 'apply'
func NewIntentCustom(l1ChainId uint64, l2ChainIds []common.Hash) (Intent, error) {
	intent := Intent{
		ConfigType:         IntentConfigTypeCustom,
		L1ChainID:          l1ChainId,
		L1ContractsLocator: &artifacts.Locator{URL: &url.URL{}},
		L2ContractsLocator: &artifacts.Locator{URL: &url.URL{}},
		SuperchainRoles:    &SuperchainRoles{},
	}

	for _, l2ChainID := range l2ChainIds {
		intent.Chains = append(intent.Chains, &ChainIntent{
			ID: l2ChainID,
		})
	}
	return intent, nil
}

func NewIntentStandard(l1ChainId uint64, l2ChainIds []common.Hash) (Intent, error) {
	intent := Intent{
		ConfigType:         IntentConfigTypeStandard,
		L1ChainID:          l1ChainId,
		L1ContractsLocator: artifacts.DefaultL1ContractsLocator,
		L2ContractsLocator: artifacts.DefaultL2ContractsLocator,
	}

	superchainRoles, err := getStandardSuperchainRoles(l1ChainId)
	if err != nil {
		return Intent{}, fmt.Errorf("error getting standard superchain roles: %w", err)
	}
	intent.SuperchainRoles = superchainRoles

	for _, l2ChainID := range l2ChainIds {
		intent.Chains = append(intent.Chains, &ChainIntent{
			ID:                       l2ChainID,
			Eip1559DenominatorCanyon: standard.Eip1559DenominatorCanyon,
			Eip1559Denominator:       standard.Eip1559Denominator,
			Eip1559Elasticity:        standard.Eip1559Elasticity,
		})
	}
	return intent, nil
}

func NewIntentStandardOverrides(l1ChainId uint64, l2ChainIds []common.Hash) (Intent, error) {
	intent, err := NewIntentStandard(l1ChainId, l2ChainIds)
	if err != nil {
		return Intent{}, err
	}
	intent.ConfigType = IntentConfigTypeStandardOverrides

	return intent, nil
}

// Same as NewIntentStandard, but also sets l2 Challenger and L1ProxyAdminOwner
// addresses to standard values
func NewIntentStrict(l1ChainId uint64, l2ChainIds []common.Hash) (Intent, error) {
	intent, err := NewIntentStandard(l1ChainId, l2ChainIds)
	if err != nil {
		return Intent{}, err
	}
	intent.ConfigType = IntentConfigTypeStrict

	challenger, _ := standard.ChallengerAddressFor(l1ChainId)
	l1ProxyAdminOwner, _ := standard.L1ProxyAdminOwner(l1ChainId)
	for chainIndex := range intent.Chains {
		intent.Chains[chainIndex].Roles.Challenger = challenger
		intent.Chains[chainIndex].Roles.L1ProxyAdminOwner = l1ProxyAdminOwner
	}
	return intent, nil
}

func NewIntentStrictOverrides(l1ChainId uint64, l2ChainIds []common.Hash) (Intent, error) {
	intent, err := NewIntentStrict(l1ChainId, l2ChainIds)
	if err != nil {
		return Intent{}, err
	}
	intent.ConfigType = IntentConfigTypeStrictOverrides

	return intent, nil
}
