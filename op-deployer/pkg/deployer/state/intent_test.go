package state

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestValidateStandardValues(t *testing.T) {
	intent, err := NewIntentStandard(DeploymentStrategyLive, 1, []common.Hash{common.HexToHash("0x336")})
	require.NoError(t, err)

	err = intent.Check()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrChainRoleZeroAddress)

	setChainRoles(&intent)
	err = intent.Check()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrFeeVaultZeroAddress)

	setFeeAddresses(&intent)
	err = intent.Check()
	require.NoError(t, err)

	intent.Chains[0].Eip1559Denominator = 3 // set to non-standard value
	err = intent.Check()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNonStandardValue)
}

func TestValidateCustomValues(t *testing.T) {
	intent, err := NewIntentCustom(DeploymentStrategyLive, 1, []common.Hash{common.HexToHash("0x336")})
	require.NoError(t, err)

	err = intent.Check()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSuperchainRoleZeroAddress)

	setSuperchainRoles(&intent)
	err = intent.Check()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrChainRoleZeroAddress)

	setChainRoles(&intent)
	err = intent.Check()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrEip1559ZeroValue)

	setEip1559Params(&intent)
	err = intent.Check()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrFeeVaultZeroAddress)

	setFeeAddresses(&intent)
	err = intent.Check()
	require.NoError(t, err)
}

func setSuperchainRoles(intent *Intent) {
	intent.SuperchainRoles = &SuperchainRoles{
		ProxyAdminOwner:       common.HexToAddress("0xa"),
		ProtocolVersionsOwner: common.HexToAddress("0xb"),
		Guardian:              common.HexToAddress("0xc"),
	}
}

func setEip1559Params(intent *Intent) {
	intent.Chains[0].Eip1559Denominator = 5000
	intent.Chains[0].Eip1559DenominatorCanyon = 5000
	intent.Chains[0].Eip1559Elasticity = 5000
}

func setChainRoles(intent *Intent) {
	intent.Chains[0].Roles.L1ProxyAdminOwner = common.HexToAddress("0x01")
	intent.Chains[0].Roles.L2ProxyAdminOwner = common.HexToAddress("0x02")
	intent.Chains[0].Roles.SystemConfigOwner = common.HexToAddress("0x03")
	intent.Chains[0].Roles.UnsafeBlockSigner = common.HexToAddress("0x04")
	intent.Chains[0].Roles.Batcher = common.HexToAddress("0x05")
	intent.Chains[0].Roles.Proposer = common.HexToAddress("0x06")
	intent.Chains[0].Roles.Challenger = common.HexToAddress("0x07")
}

func setFeeAddresses(intent *Intent) {
	intent.Chains[0].BaseFeeVaultRecipient = common.HexToAddress("0x08")
	intent.Chains[0].L1FeeVaultRecipient = common.HexToAddress("0x09")
	intent.Chains[0].SequencerFeeVaultRecipient = common.HexToAddress("0x0A")
}
