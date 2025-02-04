package devkeys

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// UserKey identifies an account for any user, by index, not specific to any chain.
type UserKey uint64

const (
	DefaultKey UserKey = 0
)

var _ Key = DefaultKey

func (k UserKey) HDPath() string {
	return fmt.Sprintf("m/44'/60'/0'/0/%d", uint64(k))
}

func (k UserKey) String() string {
	return fmt.Sprintf("user-key-%d", uint64(k))
}

// ChainUserKey is a user-key, but purpose-specific to a single chain.
// ChainID == 0 results in deriving the same key as a regular UserKey for any chain.
type ChainUserKey struct {
	ChainID *big.Int
	Index   uint64
}

var _ Key = ChainUserKey{}

func (k ChainUserKey) HDPath() string {
	return fmt.Sprintf("m/44'/60'/0'/%d/%d", k.ChainID, k.Index)
}

func (k ChainUserKey) String() string {
	return fmt.Sprintf("user-key-chain(%d)-%d", k.ChainID, k.Index)
}

// ChainUserKeys is a helper method to not repeat chainID for every user key
func ChainUserKeys(chainID *big.Int) func(index uint64) ChainUserKey {
	return func(index uint64) ChainUserKey {
		return ChainUserKey{ChainID: chainID, Index: index}
	}
}

type Role interface {
	Key(chainID *big.Int) Key
}

// SuperchainOperatorRole identifies an account used in the operations of superchain contracts
type SuperchainOperatorRole uint64

const (
	// SuperchainDeployerKey is the deployer of the superchain contracts.
	SuperchainDeployerKey SuperchainOperatorRole = 0
	// SuperchainProxyAdminOwner is the key that owns the superchain ProxyAdmin
	SuperchainProxyAdminOwner SuperchainOperatorRole = 1
	// SuperchainConfigGuardianKey is the Guardian of the SuperchainConfig.
	SuperchainConfigGuardianKey SuperchainOperatorRole = 2
	// SuperchainProtocolVersionsOwner is the key that can make ProtocolVersions changes.
	SuperchainProtocolVersionsOwner SuperchainOperatorRole = 3
	// DependencySetManagerKey is the key used to manage the dependency set of a superchain.
	DependencySetManagerKey SuperchainOperatorRole = 4
)

func (role SuperchainOperatorRole) String() string {
	switch role {
	case SuperchainDeployerKey:
		return "superchain-deployer"
	case SuperchainProxyAdminOwner:
		return "superchain-proxy-admin-owner"
	case SuperchainConfigGuardianKey:
		return "superchain-config-guardian"
	case SuperchainProtocolVersionsOwner:
		return "superchain-protocol-versions-owner"
	case DependencySetManagerKey:
		return "dependency-set-manager"
	default:
		return fmt.Sprintf("unknown-superchain-%d", uint64(role))
	}
}

func (role SuperchainOperatorRole) Key(chainID *big.Int) Key {
	return &SuperchainOperatorKey{
		ChainID: chainID,
		Role:    role,
	}
}

func (role *SuperchainOperatorRole) UnmarshalText(data []byte) error {
	v := string(data)
	for i := SuperchainOperatorRole(0); i < 20; i++ {
		if i.String() == v {
			*role = i
			return nil
		}
	}
	return fmt.Errorf("unknown superchain operator role %q", v)
}

func (role *SuperchainOperatorRole) MarshalText() ([]byte, error) {
	return []byte(role.String()), nil
}

// SuperchainOperatorKey is an account specific to an OperationRole of a given OP-Stack chain.
type SuperchainOperatorKey struct {
	ChainID *big.Int
	Role    SuperchainOperatorRole
}

var _ Key = SuperchainOperatorKey{}

func (k SuperchainOperatorKey) HDPath() string {
	return fmt.Sprintf("m/44'/60'/1'/%d/%d", k.ChainID, uint64(k.Role))
}

func (k SuperchainOperatorKey) String() string {
	return fmt.Sprintf("superchain(%d)-%s", k.ChainID, k.Role)
}

// SuperchainOperatorKeys is a helper method to not repeat chainID on every operator role
func SuperchainOperatorKeys(chainID *big.Int) func(role SuperchainOperatorRole) SuperchainOperatorKey {
	return func(role SuperchainOperatorRole) SuperchainOperatorKey {
		return SuperchainOperatorKey{ChainID: chainID, Role: role}
	}
}

// ChainOperatorRole identifies an account for a specific OP-Stack chain operator role.
type ChainOperatorRole uint64

const (
	// DeployerRole is the deployer of contracts for an OP-Stack chain
	DeployerRole ChainOperatorRole = 0
	// ProposerRole is the key used by op-proposer
	ProposerRole ChainOperatorRole = 1
	// BatcherRole is the key used by op-batcher
	BatcherRole ChainOperatorRole = 2
	// SequencerP2PRole is the key used to publish sequenced L2 blocks
	SequencerP2PRole ChainOperatorRole = 3
	// ChallengerRole is the key used by op-challenger
	ChallengerRole ChainOperatorRole = 4
	// L2ProxyAdminOwnerRole is the key that controls the ProxyAdmin predeploy in L2
	L2ProxyAdminOwnerRole ChainOperatorRole = 5
	// L1ProxyAdminOwnerRole is the key that owns the ProxyAdmin on the L1 side of the deployment.
	// This can be the ProxyAdmin of a L2 chain deployment, or a superchain deployment, depending on the domain.
	L1ProxyAdminOwnerRole ChainOperatorRole = 6
	// BaseFeeVaultRecipientRole is the key that receives from the BaseFeeVault predeploy
	BaseFeeVaultRecipientRole ChainOperatorRole = 7
	// L1FeeVaultRecipientRole is the key that receives from the L1FeeVault predeploy
	L1FeeVaultRecipientRole ChainOperatorRole = 8
	// SequencerFeeVaultRecipientRole is the key that receives form the SequencerFeeVault predeploy
	SequencerFeeVaultRecipientRole ChainOperatorRole = 9
	// SystemConfigOwner is the key that can make SystemConfig changes.
	SystemConfigOwner ChainOperatorRole = 10
)

func (role ChainOperatorRole) String() string {
	switch role {
	case DeployerRole:
		return "deployer"
	case ProposerRole:
		return "proposer"
	case BatcherRole:
		return "batcher"
	case SequencerP2PRole:
		return "sequencer-p2p"
	case ChallengerRole:
		return "challenger"
	case L2ProxyAdminOwnerRole:
		return "l2-proxy-admin-owner"
	case L1ProxyAdminOwnerRole:
		return "l1-proxy-admin-owner"
	case BaseFeeVaultRecipientRole:
		return "base-fee-vault-recipient"
	case L1FeeVaultRecipientRole:
		return "l1-fee-vault-recipient"
	case SequencerFeeVaultRecipientRole:
		return "sequencer-fee-vault-recipient"
	case SystemConfigOwner:
		return "system-config-owner"
	default:
		return fmt.Sprintf("unknown-operator-%d", uint64(role))
	}
}

func (role ChainOperatorRole) Key(chainID *big.Int) Key {
	return &ChainOperatorKey{
		ChainID: chainID,
		Role:    role,
	}
}

func (role *ChainOperatorRole) UnmarshalText(data []byte) error {
	v := string(data)
	for i := ChainOperatorRole(0); i < 20; i++ {
		if i.String() == v {
			*role = i
			return nil
		}
	}
	return fmt.Errorf("unknown chain operator role %q", v)
}

func (role *ChainOperatorRole) MarshalText() ([]byte, error) {
	return []byte(role.String()), nil
}

// ChainOperatorKey is an account specific to an OperationRole of a given OP-Stack chain.
type ChainOperatorKey struct {
	ChainID *big.Int
	Role    ChainOperatorRole
}

var _ Key = ChainOperatorKey{}

func (k ChainOperatorKey) HDPath() string {
	return fmt.Sprintf("m/44'/60'/2'/%d/%d", k.ChainID, uint64(k.Role))
}

func (k ChainOperatorKey) String() string {
	return fmt.Sprintf("chain(%d)-%s", k.ChainID, k.Role)
}

// ChainOperatorKeys is a helper method to not repeat chainID on every operator role
func ChainOperatorKeys(chainID *big.Int) func(ChainOperatorRole) ChainOperatorKey {
	return func(role ChainOperatorRole) ChainOperatorKey {
		return ChainOperatorKey{ChainID: chainID, Role: role}
	}
}

// Key identifies an account, and produces an HD-Path to derive the secret-key from.
//
// We organize the dev keys with a mnemonic key-path structure as following:
// BIP-44: `m / purpose' / coin_type' / account' / change / address_index`
// purpose = standard secp256k1 usage (Eth2 BLS keys use different purpose data).
// coin_type = chain type, set to 60' for ETH. See SLIP-0044.
// account = for different identities, used here to separate domains:
//
//	domain 0: users
//	domain 1: superchain operations
//	domain 2: chain operations
//
// change = to separate external and internal addresses.
//
//	Used here for chain ID, may be 0 for user accounts (any-chain addresses).
//
// address_index = used here to separate roles.
// The `'` char signifies BIP-32 hardened derivation.
//
// See:
// https://github.com/bitcoin/bips/blob/master/bip-0044.mediawiki
// https://github.com/satoshilabs/slips/blob/master/slip-0044.md
// https://github.com/bitcoin/bips/blob/master/bip-0032.mediawiki
type Key interface {
	// HDPath produces the hierarchical derivation path to (re)create this key.
	HDPath() string
	// String describes the role of the key
	String() string
}

// Secrets selects a secret-key based on a key.
// This is meant for dev-purposes only.
// Secret keys should not directly be exposed to live production services.
type Secrets interface {
	Secret(key Key) (*ecdsa.PrivateKey, error)
}

// Addresses selects an address based on a key.
// This interface is preferred in tools that do not directly rely on secret-key material.
type Addresses interface {
	// Address produces an address for the given key
	Address(key Key) (common.Address, error)
}

// Keys is a joint interface of Secrets and Addresses
type Keys interface {
	Secrets
	Addresses
}
