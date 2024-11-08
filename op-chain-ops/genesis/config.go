package genesis

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	opparams "github.com/ethereum-optimism/optimism/op-node/params"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

var (
	ErrInvalidDeployConfig     = errors.New("invalid deploy config")
	ErrInvalidImmutablesConfig = errors.New("invalid immutables config")
	// MaximumBaseFee represents the max base fee for deposits, since
	// there is an on chain EIP-1559 curve for deposits purchasing L2 gas.
	// It is type(uint128).max in solidity.
	MaximumBaseFee, _ = new(big.Int).SetString("ffffffffffffffffffffffffffffffff", 16)
)

const (
	// MaxResourceLimit represents the maximum amount of L2 gas that a single deposit can use.
	MaxResourceLimit = 20_000_000
	// ElasticityMultiplier represents the elasticity of the deposit EIP-1559 fee market.
	ElasticityMultiplier = 10
	// BaseFeeMaxChangeDenominator represents the maximum change in base fee per block.
	BaseFeeMaxChangeDenominator = 8
	// MinimumBaseFee represents the minimum base fee for deposits.
	MinimumBaseFee = params.GWei
	// SystemTxMaxGas represents the maximum gas that a system transaction can use
	// when it is included with user deposits.
	SystemTxMaxGas = 1_000_000
)

type ConfigChecker interface {
	// Check verifies the contents of a config are correct.
	// Check may log warnings for non-critical configuration remarks.
	Check(log log.Logger) error
}

func checkConfigBundle(bundle any, log log.Logger) error {
	cfgValue := reflect.ValueOf(bundle)
	for cfgValue.Kind() == reflect.Interface || cfgValue.Kind() == reflect.Pointer {
		cfgValue = cfgValue.Elem()
	}
	if cfgValue.Kind() != reflect.Struct {
		return fmt.Errorf("bundle type %s is not a struct", cfgValue.Type().String())
	}
	for i := 0; i < cfgValue.NumField(); i++ {
		field := cfgValue.Field(i)
		if field.Kind() != reflect.Pointer { // to call pointer-receiver methods
			field = field.Addr()
		}
		name := cfgValue.Type().Field(i).Name
		if v, ok := field.Interface().(ConfigChecker); ok {
			if err := v.Check(log.New("config", name)); err != nil {
				return fmt.Errorf("config field %s failed checks: %w", name, err)
			} else {
				log.Debug("Checked config-field", "name", name)
			}
		} else {
			log.Debug("Ignoring config-field", "name", name)
		}
	}
	return nil
}

type DevDeployConfig struct {
	// FundDevAccounts configures whether to fund the dev accounts.
	// This should only be used during devnet deployments.
	FundDevAccounts bool `json:"fundDevAccounts"`
}

type L2GenesisBlockDeployConfig struct {
	L2GenesisBlockNonce         hexutil.Uint64 `json:"l2GenesisBlockNonce"`
	L2GenesisBlockGasLimit      hexutil.Uint64 `json:"l2GenesisBlockGasLimit"`
	L2GenesisBlockDifficulty    *hexutil.Big   `json:"l2GenesisBlockDifficulty"`
	L2GenesisBlockMixHash       common.Hash    `json:"l2GenesisBlockMixHash"`
	L2GenesisBlockNumber        hexutil.Uint64 `json:"l2GenesisBlockNumber"`
	L2GenesisBlockGasUsed       hexutil.Uint64 `json:"l2GenesisBlockGasUsed"`
	L2GenesisBlockParentHash    common.Hash    `json:"l2GenesisBlockParentHash"`
	L2GenesisBlockBaseFeePerGas *hexutil.Big   `json:"l2GenesisBlockBaseFeePerGas"`
	// Note that there is no L2 genesis ExtraData, as it must default to a valid Holocene eip-1559
	// configuration. See constant 'HoloceneExtraData' for the specific value used.
	// Note that there is no L2 genesis timestamp:
	// This is instead configured based on the timestamp of "l1StartingBlockTag".
}

var _ ConfigChecker = (*L2GenesisBlockDeployConfig)(nil)

func (d *L2GenesisBlockDeployConfig) Check(log log.Logger) error {
	if d.L2GenesisBlockGasLimit == 0 {
		return fmt.Errorf("%w: L2 genesis block gas limit cannot be 0", ErrInvalidDeployConfig)
	}
	// When the initial resource config is made to be configurable by the DeployConfig, ensure
	// that this check is updated to use the values from the DeployConfig instead of the defaults.
	if uint64(d.L2GenesisBlockGasLimit) < uint64(MaxResourceLimit+SystemTxMaxGas) {
		return fmt.Errorf("%w: L2 genesis block gas limit is too small", ErrInvalidDeployConfig)
	}
	if d.L2GenesisBlockBaseFeePerGas == nil {
		return fmt.Errorf("%w: L2 genesis block base fee per gas cannot be nil", ErrInvalidDeployConfig)
	}
	return nil
}

// OwnershipDeployConfig defines the ownership of an L2 chain deployment.
// This excludes superchain-wide contracts.
type OwnershipDeployConfig struct {
	// ProxyAdminOwner represents the owner of the ProxyAdmin predeploy on L2.
	ProxyAdminOwner common.Address `json:"proxyAdminOwner"`
	// FinalSystemOwner is the owner of the system on L1. Any L1 contract that is ownable has
	// this account set as its owner.
	FinalSystemOwner common.Address `json:"finalSystemOwner"`
}

var _ ConfigChecker = (*OwnershipDeployConfig)(nil)

func (d *OwnershipDeployConfig) Check(log log.Logger) error {
	if d.FinalSystemOwner == (common.Address{}) {
		return fmt.Errorf("%w: FinalSystemOwner cannot be address(0)", ErrInvalidDeployConfig)
	}
	if d.ProxyAdminOwner == (common.Address{}) {
		return fmt.Errorf("%w: ProxyAdminOwner cannot be address(0)", ErrInvalidDeployConfig)
	}
	return nil
}

type L2VaultsDeployConfig struct {
	// BaseFeeVaultRecipient represents the recipient of fees accumulated in the BaseFeeVault.
	// Can be an account on L1 or L2, depending on the BaseFeeVaultWithdrawalNetwork value.
	BaseFeeVaultRecipient common.Address `json:"baseFeeVaultRecipient"`
	// L1FeeVaultRecipient represents the recipient of fees accumulated in the L1FeeVault.
	// Can be an account on L1 or L2, depending on the L1FeeVaultWithdrawalNetwork value.
	L1FeeVaultRecipient common.Address `json:"l1FeeVaultRecipient"`
	// SequencerFeeVaultRecipient represents the recipient of fees accumulated in the SequencerFeeVault.
	// Can be an account on L1 or L2, depending on the SequencerFeeVaultWithdrawalNetwork value.
	SequencerFeeVaultRecipient common.Address `json:"sequencerFeeVaultRecipient"`
	// BaseFeeVaultMinimumWithdrawalAmount represents the minimum withdrawal amount for the BaseFeeVault.
	BaseFeeVaultMinimumWithdrawalAmount *hexutil.Big `json:"baseFeeVaultMinimumWithdrawalAmount"`
	// L1FeeVaultMinimumWithdrawalAmount represents the minimum withdrawal amount for the L1FeeVault.
	L1FeeVaultMinimumWithdrawalAmount *hexutil.Big `json:"l1FeeVaultMinimumWithdrawalAmount"`
	// SequencerFeeVaultMinimumWithdrawalAmount represents the minimum withdrawal amount for the SequencerFeeVault.
	SequencerFeeVaultMinimumWithdrawalAmount *hexutil.Big `json:"sequencerFeeVaultMinimumWithdrawalAmount"`
	// BaseFeeVaultWithdrawalNetwork represents the withdrawal network for the BaseFeeVault.
	BaseFeeVaultWithdrawalNetwork WithdrawalNetwork `json:"baseFeeVaultWithdrawalNetwork"`
	// L1FeeVaultWithdrawalNetwork represents the withdrawal network for the L1FeeVault.
	L1FeeVaultWithdrawalNetwork WithdrawalNetwork `json:"l1FeeVaultWithdrawalNetwork"`
	// SequencerFeeVaultWithdrawalNetwork represents the withdrawal network for the SequencerFeeVault.
	SequencerFeeVaultWithdrawalNetwork WithdrawalNetwork `json:"sequencerFeeVaultWithdrawalNetwork"`
}

var _ ConfigChecker = (*L2VaultsDeployConfig)(nil)

func (d *L2VaultsDeployConfig) Check(log log.Logger) error {
	if d.BaseFeeVaultRecipient == (common.Address{}) {
		return fmt.Errorf("%w: BaseFeeVaultRecipient cannot be address(0)", ErrInvalidDeployConfig)
	}
	if d.L1FeeVaultRecipient == (common.Address{}) {
		return fmt.Errorf("%w: L1FeeVaultRecipient cannot be address(0)", ErrInvalidDeployConfig)
	}
	if d.SequencerFeeVaultRecipient == (common.Address{}) {
		return fmt.Errorf("%w: SequencerFeeVaultRecipient cannot be address(0)", ErrInvalidDeployConfig)
	}
	if !d.BaseFeeVaultWithdrawalNetwork.Valid() {
		return fmt.Errorf("%w: BaseFeeVaultWithdrawalNetwork can only be 0 (L1) or 1 (L2)", ErrInvalidDeployConfig)
	}
	if !d.L1FeeVaultWithdrawalNetwork.Valid() {
		return fmt.Errorf("%w: L1FeeVaultWithdrawalNetwork can only be 0 (L1) or 1 (L2)", ErrInvalidDeployConfig)
	}
	if !d.SequencerFeeVaultWithdrawalNetwork.Valid() {
		return fmt.Errorf("%w: SequencerFeeVaultWithdrawalNetwork can only be 0 (L1) or 1 (L2)", ErrInvalidDeployConfig)
	}
	return nil
}

// GovernanceDeployConfig is exclusive to OP-Mainnet and the testing of OP-Mainnet-like chains.
type GovernanceDeployConfig struct {
	// EnableGovernance configures whether or not include governance token predeploy.
	EnableGovernance bool `json:"enableGovernance"`
	// GovernanceTokenSymbol represents the  ERC20 symbol of the GovernanceToken.
	GovernanceTokenSymbol string `json:"governanceTokenSymbol"`
	// GovernanceTokenName represents the ERC20 name of the GovernanceToken
	GovernanceTokenName string `json:"governanceTokenName"`
	// GovernanceTokenOwner represents the owner of the GovernanceToken. Has the ability
	// to mint and burn tokens.
	GovernanceTokenOwner common.Address `json:"governanceTokenOwner"`
}

var _ ConfigChecker = (*GovernanceDeployConfig)(nil)

func (d *GovernanceDeployConfig) Check(log log.Logger) error {
	if d.EnableGovernance {
		if d.GovernanceTokenName == "" {
			return fmt.Errorf("%w: GovernanceToken.name cannot be empty", ErrInvalidDeployConfig)
		}
		if d.GovernanceTokenSymbol == "" {
			return fmt.Errorf("%w: GovernanceToken.symbol cannot be empty", ErrInvalidDeployConfig)
		}
		if d.GovernanceTokenOwner == (common.Address{}) {
			return fmt.Errorf("%w: GovernanceToken owner cannot be address(0)", ErrInvalidDeployConfig)
		}
	}
	return nil
}

func (d *GovernanceDeployConfig) GovernanceEnabled() bool {
	return d.EnableGovernance
}

// GasPriceOracleDeployConfig configures the GasPriceOracle L2 predeploy.
type GasPriceOracleDeployConfig struct {
	// GasPriceOracleOverhead represents the initial value of the gas overhead in the GasPriceOracle predeploy.
	// Deprecated: Since Ecotone, this field is superseded by GasPriceOracleBaseFeeScalar and GasPriceOracleBlobBaseFeeScalar.
	GasPriceOracleOverhead uint64 `json:"gasPriceOracleOverhead"`
	// GasPriceOracleScalar represents the initial value of the gas scalar in the GasPriceOracle predeploy.
	// Deprecated: Since Ecotone, this field is superseded by GasPriceOracleBaseFeeScalar and GasPriceOracleBlobBaseFeeScalar.
	GasPriceOracleScalar uint64 `json:"gasPriceOracleScalar"`
	// GasPriceOracleBaseFeeScalar represents the value of the base fee scalar used for fee calculations.
	GasPriceOracleBaseFeeScalar uint32 `json:"gasPriceOracleBaseFeeScalar" evm:"basefeeScalar"`
	// GasPriceOracleBlobBaseFeeScalar represents the value of the blob base fee scalar used for fee calculations.
	GasPriceOracleBlobBaseFeeScalar uint32 `json:"gasPriceOracleBlobBaseFeeScalar" evm:"blobbasefeeScalar"`
}

var _ ConfigChecker = (*GasPriceOracleDeployConfig)(nil)

func (d *GasPriceOracleDeployConfig) Check(log log.Logger) error {
	if d.GasPriceOracleBaseFeeScalar == 0 {
		log.Warn("GasPriceOracleBaseFeeScalar is 0")
	}
	if d.GasPriceOracleBlobBaseFeeScalar == 0 {
		log.Warn("GasPriceOracleBlobBaseFeeScalar is 0")
	}
	return nil
}

// FeeScalar returns the raw serialized fee scalar. Uses pre-Ecotone if legacy config is present,
// otherwise uses the post-Ecotone scalar serialization.
func (d *GasPriceOracleDeployConfig) FeeScalar() [32]byte {
	if d.GasPriceOracleScalar != 0 {
		return common.BigToHash(big.NewInt(int64(d.GasPriceOracleScalar)))
	}
	return eth.EncodeScalar(eth.EcotoneScalars{
		BlobBaseFeeScalar: d.GasPriceOracleBlobBaseFeeScalar,
		BaseFeeScalar:     d.GasPriceOracleBaseFeeScalar,
	})
}

// GasTokenDeployConfig configures the optional custom gas token functionality.
type GasTokenDeployConfig struct {
	// UseCustomGasToken is a flag to indicate that a custom gas token should be used
	UseCustomGasToken bool `json:"useCustomGasToken"`
	// CustomGasTokenAddress is the address of the ERC20 token to be used to pay for gas on L2.
	CustomGasTokenAddress common.Address `json:"customGasTokenAddress"`
}

var _ ConfigChecker = (*GasTokenDeployConfig)(nil)

func (d *GasTokenDeployConfig) Check(log log.Logger) error {
	if d.UseCustomGasToken {
		if d.CustomGasTokenAddress == (common.Address{}) {
			return fmt.Errorf("%w: CustomGasTokenAddress cannot be address(0)", ErrInvalidDeployConfig)
		}
		log.Info("Using custom gas token", "address", d.CustomGasTokenAddress)
	}
	return nil
}

// OperatorDeployConfig configures the hot-key addresses for operations such as sequencing and batch-submission.
type OperatorDeployConfig struct {
	// P2PSequencerAddress is the address of the key the sequencer uses to sign blocks on the P2P layer.
	P2PSequencerAddress common.Address `json:"p2pSequencerAddress" evm:"p2pSequencerAddress"`
	// BatchSenderAddress represents the initial sequencer account that authorizes batches.
	// Transactions sent from this account to the batch inbox address are considered valid.
	BatchSenderAddress common.Address `json:"batchSenderAddress"`
}

var _ ConfigChecker = (*OperatorDeployConfig)(nil)

func (d *OperatorDeployConfig) Check(log log.Logger) error {
	if d.P2PSequencerAddress == (common.Address{}) {
		return fmt.Errorf("%w: P2PSequencerAddress cannot be address(0)", ErrInvalidDeployConfig)
	}
	if d.BatchSenderAddress == (common.Address{}) {
		return fmt.Errorf("%w: BatchSenderAddress cannot be address(0)", ErrInvalidDeployConfig)
	}
	return nil
}

// EIP1559DeployConfig configures the EIP-1559 parameters of the chain.
type EIP1559DeployConfig struct {
	// EIP1559Elasticity is the elasticity of the EIP1559 fee market.
	EIP1559Elasticity uint64 `json:"eip1559Elasticity"`
	// EIP1559Denominator is the denominator of EIP1559 base fee market.
	EIP1559Denominator uint64 `json:"eip1559Denominator"`
	// EIP1559DenominatorCanyon is the denominator of EIP1559 base fee market when Canyon is active.
	EIP1559DenominatorCanyon uint64 `json:"eip1559DenominatorCanyon"`
}

var _ ConfigChecker = (*EIP1559DeployConfig)(nil)

func (d *EIP1559DeployConfig) Check(log log.Logger) error {
	if d.EIP1559Denominator == 0 {
		return fmt.Errorf("%w: EIP1559Denominator cannot be 0", ErrInvalidDeployConfig)
	}
	if d.EIP1559Elasticity == 0 {
		return fmt.Errorf("%w: EIP1559Elasticity cannot be 0", ErrInvalidDeployConfig)
	}
	return nil
}

// UpgradeScheduleDeployConfig configures when network upgrades activate.
type UpgradeScheduleDeployConfig struct {
	// L2GenesisRegolithTimeOffset is the number of seconds after genesis block that Regolith hard fork activates.
	// Set it to 0 to activate at genesis. Nil to disable Regolith.
	L2GenesisRegolithTimeOffset *hexutil.Uint64 `json:"l2GenesisRegolithTimeOffset,omitempty"`
	// L2GenesisCanyonTimeOffset is the number of seconds after genesis block that Canyon hard fork activates.
	// Set it to 0 to activate at genesis. Nil to disable Canyon.
	L2GenesisCanyonTimeOffset *hexutil.Uint64 `json:"l2GenesisCanyonTimeOffset,omitempty"`
	// L2GenesisDeltaTimeOffset is the number of seconds after genesis block that Delta hard fork activates.
	// Set it to 0 to activate at genesis. Nil to disable Delta.
	L2GenesisDeltaTimeOffset *hexutil.Uint64 `json:"l2GenesisDeltaTimeOffset,omitempty"`
	// L2GenesisEcotoneTimeOffset is the number of seconds after genesis block that Ecotone hard fork activates.
	// Set it to 0 to activate at genesis. Nil to disable Ecotone.
	L2GenesisEcotoneTimeOffset *hexutil.Uint64 `json:"l2GenesisEcotoneTimeOffset,omitempty"`
	// L2GenesisFjordTimeOffset is the number of seconds after genesis block that Fjord hard fork activates.
	// Set it to 0 to activate at genesis. Nil to disable Fjord.
	L2GenesisFjordTimeOffset *hexutil.Uint64 `json:"l2GenesisFjordTimeOffset,omitempty"`
	// L2GenesisGraniteTimeOffset is the number of seconds after genesis block that Granite hard fork activates.
	// Set it to 0 to activate at genesis. Nil to disable Granite.
	L2GenesisGraniteTimeOffset *hexutil.Uint64 `json:"l2GenesisGraniteTimeOffset,omitempty"`
	// L2GenesisHoloceneTimeOffset is the number of seconds after genesis block that the Holocene hard fork activates.
	// Set it to 0 to activate at genesis. Nil to disable Holocene.
	L2GenesisHoloceneTimeOffset *hexutil.Uint64 `json:"l2GenesisHoloceneTimeOffset,omitempty"`
	// L2GenesisInteropTimeOffset is the number of seconds after genesis block that the Interop hard fork activates.
	// Set it to 0 to activate at genesis. Nil to disable Interop.
	L2GenesisInteropTimeOffset *hexutil.Uint64 `json:"l2GenesisInteropTimeOffset,omitempty"`

	// When Cancun activates. Relative to L1 genesis.
	L1CancunTimeOffset *hexutil.Uint64 `json:"l1CancunTimeOffset,omitempty"`

	// UseInterop is a flag that indicates if the system is using interop
	UseInterop bool `json:"useInterop,omitempty"`
}

var _ ConfigChecker = (*UpgradeScheduleDeployConfig)(nil)

func offsetToUpgradeTime(offset *hexutil.Uint64, genesisTime uint64) *uint64 {
	if offset == nil {
		return nil
	}
	v := uint64(0)
	if offset := *offset; offset > 0 {
		v = genesisTime + uint64(offset)
	}
	return &v
}

func (d *UpgradeScheduleDeployConfig) ForkTimeOffset(fork rollup.ForkName) *uint64 {
	switch fork {
	case rollup.Regolith:
		return (*uint64)(d.L2GenesisRegolithTimeOffset)
	case rollup.Canyon:
		return (*uint64)(d.L2GenesisCanyonTimeOffset)
	case rollup.Delta:
		return (*uint64)(d.L2GenesisDeltaTimeOffset)
	case rollup.Ecotone:
		return (*uint64)(d.L2GenesisEcotoneTimeOffset)
	case rollup.Fjord:
		return (*uint64)(d.L2GenesisFjordTimeOffset)
	case rollup.Granite:
		return (*uint64)(d.L2GenesisGraniteTimeOffset)
	case rollup.Holocene:
		return (*uint64)(d.L2GenesisHoloceneTimeOffset)
	case rollup.Interop:
		return (*uint64)(d.L2GenesisInteropTimeOffset)
	default:
		panic(fmt.Sprintf("unknown fork: %s", fork))
	}
}

func (d *UpgradeScheduleDeployConfig) SetForkTimeOffset(fork rollup.ForkName, offset *uint64) {
	switch fork {
	case rollup.Regolith:
		d.L2GenesisRegolithTimeOffset = (*hexutil.Uint64)(offset)
	case rollup.Canyon:
		d.L2GenesisCanyonTimeOffset = (*hexutil.Uint64)(offset)
	case rollup.Delta:
		d.L2GenesisDeltaTimeOffset = (*hexutil.Uint64)(offset)
	case rollup.Ecotone:
		d.L2GenesisEcotoneTimeOffset = (*hexutil.Uint64)(offset)
	case rollup.Fjord:
		d.L2GenesisFjordTimeOffset = (*hexutil.Uint64)(offset)
	case rollup.Granite:
		d.L2GenesisGraniteTimeOffset = (*hexutil.Uint64)(offset)
	case rollup.Holocene:
		d.L2GenesisHoloceneTimeOffset = (*hexutil.Uint64)(offset)
	case rollup.Interop:
		d.L2GenesisInteropTimeOffset = (*hexutil.Uint64)(offset)
	default:
		panic(fmt.Sprintf("unknown fork: %s", fork))
	}
}

var scheduleableForks = rollup.ForksFrom(rollup.Regolith)

// ActivateForkAtOffset activates the given fork at the given offset. Previous forks are activated
// at genesis and later forks are deactivated.
// If multiple forks should be activated at a later time than genesis, first call
// ActivateForkAtOffset with the earliest fork and then SetForkTimeOffset to individually set later
// forks.
func (d *UpgradeScheduleDeployConfig) ActivateForkAtOffset(fork rollup.ForkName, offset uint64) {
	if !rollup.IsValidFork(fork) || fork == rollup.Bedrock {
		panic(fmt.Sprintf("invalid fork: %s", fork))
	}
	ts := new(uint64)
	for i, f := range scheduleableForks {
		if f == fork {
			d.SetForkTimeOffset(fork, &offset)
			ts = nil
		} else {
			d.SetForkTimeOffset(scheduleableForks[i], ts)
		}
	}
}

// ActivateForkAtGenesis activates the given fork, and all previous forks, at genesis.
// Later forks are deactivated.
// See also [ActivateForkAtOffset].
func (d *UpgradeScheduleDeployConfig) ActivateForkAtGenesis(fork rollup.ForkName) {
	d.ActivateForkAtOffset(fork, 0)
}

func (d *UpgradeScheduleDeployConfig) RegolithTime(genesisTime uint64) *uint64 {
	return offsetToUpgradeTime(d.L2GenesisRegolithTimeOffset, genesisTime)
}

func (d *UpgradeScheduleDeployConfig) CanyonTime(genesisTime uint64) *uint64 {
	return offsetToUpgradeTime(d.L2GenesisCanyonTimeOffset, genesisTime)
}

func (d *UpgradeScheduleDeployConfig) DeltaTime(genesisTime uint64) *uint64 {
	return offsetToUpgradeTime(d.L2GenesisDeltaTimeOffset, genesisTime)
}

func (d *UpgradeScheduleDeployConfig) EcotoneTime(genesisTime uint64) *uint64 {
	return offsetToUpgradeTime(d.L2GenesisEcotoneTimeOffset, genesisTime)
}

func (d *UpgradeScheduleDeployConfig) FjordTime(genesisTime uint64) *uint64 {
	return offsetToUpgradeTime(d.L2GenesisFjordTimeOffset, genesisTime)
}

func (d *UpgradeScheduleDeployConfig) GraniteTime(genesisTime uint64) *uint64 {
	return offsetToUpgradeTime(d.L2GenesisGraniteTimeOffset, genesisTime)
}

func (d *UpgradeScheduleDeployConfig) HoloceneTime(genesisTime uint64) *uint64 {
	return offsetToUpgradeTime(d.L2GenesisHoloceneTimeOffset, genesisTime)
}

func (d *UpgradeScheduleDeployConfig) InteropTime(genesisTime uint64) *uint64 {
	return offsetToUpgradeTime(d.L2GenesisInteropTimeOffset, genesisTime)
}

func (d *UpgradeScheduleDeployConfig) AllocMode(genesisTime uint64) L2AllocsMode {
	forks := d.forks()
	for i := len(forks) - 1; i >= 0; i-- {
		if forkTime := offsetToUpgradeTime(forks[i].L2GenesisTimeOffset, genesisTime); forkTime != nil && *forkTime == 0 {
			return L2AllocsMode(forks[i].Name)
		}
		// the oldest L2AllocsMode is delta
		if forks[i].Name == string(L2AllocsDelta) {
			return L2AllocsDelta
		}
	}
	panic("should never reach here")
}

type Fork struct {
	L2GenesisTimeOffset *hexutil.Uint64
	Name                string
}

func (d *UpgradeScheduleDeployConfig) forks() []Fork {
	return []Fork{
		{L2GenesisTimeOffset: d.L2GenesisRegolithTimeOffset, Name: "regolith"},
		{L2GenesisTimeOffset: d.L2GenesisCanyonTimeOffset, Name: "canyon"},
		{L2GenesisTimeOffset: d.L2GenesisDeltaTimeOffset, Name: string(L2AllocsDelta)},
		{L2GenesisTimeOffset: d.L2GenesisEcotoneTimeOffset, Name: string(L2AllocsEcotone)},
		{L2GenesisTimeOffset: d.L2GenesisFjordTimeOffset, Name: string(L2AllocsFjord)},
		{L2GenesisTimeOffset: d.L2GenesisGraniteTimeOffset, Name: string(L2AllocsGranite)},
		{L2GenesisTimeOffset: d.L2GenesisHoloceneTimeOffset, Name: string(L2AllocsHolocene)},
	}
}

func (d *UpgradeScheduleDeployConfig) Check(log log.Logger) error {
	// checkFork checks that fork A is before or at the same time as fork B
	checkFork := func(a, b *hexutil.Uint64, aName, bName string) error {
		if a == nil && b == nil {
			return nil
		}
		if a == nil && b != nil {
			return fmt.Errorf("fork %s set (to %d), but prior fork %s missing", bName, *b, aName)
		}
		if a != nil && b == nil {
			return nil
		}
		if *a > *b {
			return fmt.Errorf("fork %s set to %d, but prior fork %s has higher offset %d", bName, *b, aName, *a)
		}
		return nil
	}
	forks := d.forks()
	for i := 0; i < len(forks)-1; i++ {
		if err := checkFork(forks[i].L2GenesisTimeOffset, forks[i+1].L2GenesisTimeOffset, forks[i].Name, forks[i+1].Name); err != nil {
			return err
		}
	}
	return nil
}

// L2CoreDeployConfig configures the core protocol parameters of the chain.
type L2CoreDeployConfig struct {
	// L1ChainID is the chain ID of the L1 chain.
	L1ChainID uint64 `json:"l1ChainID"`

	// L2ChainID is the chain ID of the L2 chain.
	L2ChainID uint64 `json:"l2ChainID"`

	// L2BlockTime is the number of seconds between each L2 block.
	L2BlockTime uint64 `json:"l2BlockTime"`
	// FinalizationPeriodSeconds represents the number of seconds before an output is considered
	// finalized. This impacts the amount of time that withdrawals take to finalize and is
	// generally set to 1 week.
	FinalizationPeriodSeconds uint64 `json:"finalizationPeriodSeconds"`
	// MaxSequencerDrift is the number of seconds after the L1 timestamp of the end of the
	// sequencing window that batches must be included, otherwise L2 blocks including
	// deposits are force included.
	MaxSequencerDrift uint64 `json:"maxSequencerDrift"`
	// SequencerWindowSize is the number of L1 blocks per sequencing window.
	SequencerWindowSize uint64 `json:"sequencerWindowSize"`
	// ChannelTimeoutBedrock is the number of L1 blocks that a frame stays valid when included in L1.
	ChannelTimeoutBedrock uint64 `json:"channelTimeout"`
	// BatchInboxAddress is the L1 account that batches are sent to.
	BatchInboxAddress common.Address `json:"batchInboxAddress"`

	// SystemConfigStartBlock represents the block at which the op-node should start syncing
	// from. It is an override to set this value on legacy networks where it is not set by
	// default. It can be removed once all networks have this value set in their storage.
	SystemConfigStartBlock uint64 `json:"systemConfigStartBlock"`
}

var _ ConfigChecker = (*L2CoreDeployConfig)(nil)

func (d *L2CoreDeployConfig) Check(log log.Logger) error {
	if d.L1ChainID == 0 {
		return fmt.Errorf("%w: L1ChainID cannot be 0", ErrInvalidDeployConfig)
	}
	if d.L2ChainID == 0 {
		return fmt.Errorf("%w: L2ChainID cannot be 0", ErrInvalidDeployConfig)
	}
	if d.MaxSequencerDrift == 0 {
		return fmt.Errorf("%w: MaxSequencerDrift cannot be 0", ErrInvalidDeployConfig)
	}
	if d.SequencerWindowSize == 0 {
		return fmt.Errorf("%w: SequencerWindowSize cannot be 0", ErrInvalidDeployConfig)
	}
	if d.ChannelTimeoutBedrock == 0 {
		return fmt.Errorf("%w: ChannelTimeout cannot be 0", ErrInvalidDeployConfig)
	}
	if d.BatchInboxAddress == (common.Address{}) {
		return fmt.Errorf("%w: BatchInboxAddress cannot be address(0)", ErrInvalidDeployConfig)
	}
	if d.L2BlockTime == 0 {
		return fmt.Errorf("%w: L2BlockTime cannot be 0", ErrInvalidDeployConfig)
	}
	if d.FinalizationPeriodSeconds == 0 {
		return fmt.Errorf("%w: FinalizationPeriodSeconds cannot be 0", ErrInvalidDeployConfig)
	}
	return nil
}

// AltDADeployConfig configures optional AltDA functionality.
type AltDADeployConfig struct {
	// UseAltDA is a flag that indicates if the system is using op-alt-da
	UseAltDA bool `json:"useAltDA" toml:"useAltDA"`
	// DACommitmentType specifies the allowed commitment
	DACommitmentType string `json:"daCommitmentType" toml:"daCommitmentType"`
	// DAChallengeWindow represents the block interval during which the availability of a data commitment can be challenged.
	DAChallengeWindow uint64 `json:"daChallengeWindow" toml:"daChallengeWindow"`
	// DAResolveWindow represents the block interval during which a data availability challenge can be resolved.
	DAResolveWindow uint64 `json:"daResolveWindow" toml:"daResolveWindow"`
	// DABondSize represents the required bond size to initiate a data availability challenge.
	DABondSize uint64 `json:"daBondSize" toml:"daBondSize"`
	// DAResolverRefundPercentage represents the percentage of the resolving cost to be refunded to the resolver
	// such as 100 means 100% refund.
	DAResolverRefundPercentage uint64 `json:"daResolverRefundPercentage" toml:"daResolverRefundPercentage"`
}

var _ ConfigChecker = (*AltDADeployConfig)(nil)

func (d *AltDADeployConfig) Check(log log.Logger) error {
	if d.UseAltDA {
		if !(d.DACommitmentType == altda.KeccakCommitmentString || d.DACommitmentType == altda.GenericCommitmentString) {
			return fmt.Errorf("%w: DACommitmentType must be either KeccakCommitment or GenericCommitment", ErrInvalidDeployConfig)
		}
		// only enforce challenge and resolve window if using alt-da mode with Keccak Commitments
		if d.DACommitmentType != altda.GenericCommitmentString {
			if d.DAChallengeWindow == 0 {
				return fmt.Errorf("%w: DAChallengeWindow cannot be 0 when using alt-da mode with Keccak Commitments", ErrInvalidDeployConfig)
			}
			if d.DAResolveWindow == 0 {
				return fmt.Errorf("%w: DAResolveWindow cannot be 0 when using alt-da mode with Keccak Commitments", ErrInvalidDeployConfig)
			}
		}
	}
	return nil
}

// L2InitializationConfig represents all L2 configuration
// data that can be configured before the deployment of any L1 contracts.
type L2InitializationConfig struct {
	DevDeployConfig
	L2GenesisBlockDeployConfig
	OwnershipDeployConfig
	L2VaultsDeployConfig
	GovernanceDeployConfig
	GasPriceOracleDeployConfig
	GasTokenDeployConfig
	OperatorDeployConfig
	EIP1559DeployConfig
	UpgradeScheduleDeployConfig
	L2CoreDeployConfig
	AltDADeployConfig
}

func (d *L2InitializationConfig) Check(log log.Logger) error {
	if err := checkConfigBundle(d, log); err != nil {
		return err
	}
	return nil
}

// DevL1DeployConfig is used to configure a L1 chain for development/testing purposes.
// A production L2 deployment does not utilize this configuration,
// except of a L1BlockTime sanity-check (set this to 12 for L1 Ethereum).
type DevL1DeployConfig struct {
	L1BlockTime                 uint64          `json:"l1BlockTime"`
	L1GenesisBlockTimestamp     hexutil.Uint64  `json:"l1GenesisBlockTimestamp"`
	L1GenesisBlockNonce         hexutil.Uint64  `json:"l1GenesisBlockNonce"`
	L1GenesisBlockGasLimit      hexutil.Uint64  `json:"l1GenesisBlockGasLimit"`
	L1GenesisBlockDifficulty    *hexutil.Big    `json:"l1GenesisBlockDifficulty"`
	L1GenesisBlockMixHash       common.Hash     `json:"l1GenesisBlockMixHash"`
	L1GenesisBlockCoinbase      common.Address  `json:"l1GenesisBlockCoinbase"`
	L1GenesisBlockNumber        hexutil.Uint64  `json:"l1GenesisBlockNumber"`
	L1GenesisBlockGasUsed       hexutil.Uint64  `json:"l1GenesisBlockGasUsed"`
	L1GenesisBlockParentHash    common.Hash     `json:"l1GenesisBlockParentHash"`
	L1GenesisBlockBaseFeePerGas *hexutil.Big    `json:"l1GenesisBlockBaseFeePerGas"`
	L1GenesisBlockExcessBlobGas *hexutil.Uint64 `json:"l1GenesisBlockExcessBlobGas,omitempty"` // EIP-4844
	L1GenesisBlockBlobGasUsed   *hexutil.Uint64 `json:"l1GenesisBlockblobGasUsed,omitempty"`   // EIP-4844
}

// SuperchainL1DeployConfig configures parameters of the superchain-wide deployed contracts to L1.
// This deployment is global, and can be reused between L2s that target the same superchain.
type SuperchainL1DeployConfig struct {
	// RequiredProtocolVersion indicates the protocol version that
	// nodes are required to adopt, to stay in sync with the network.
	RequiredProtocolVersion params.ProtocolVersion `json:"requiredProtocolVersion"`
	// RequiredProtocolVersion indicates the protocol version that
	// nodes are recommended to adopt, to stay in sync with the network.
	RecommendedProtocolVersion params.ProtocolVersion `json:"recommendedProtocolVersion"`

	// SuperchainConfigGuardian represents the GUARDIAN account in the SuperchainConfig. Has the ability to pause withdrawals.
	SuperchainConfigGuardian common.Address `json:"superchainConfigGuardian"`
}

func (d *SuperchainL1DeployConfig) Check(log log.Logger) error {
	if d.RequiredProtocolVersion == (params.ProtocolVersion{}) {
		log.Warn("RequiredProtocolVersion is empty")
	}
	if d.RecommendedProtocolVersion == (params.ProtocolVersion{}) {
		log.Warn("RecommendedProtocolVersion is empty")
	}
	if d.SuperchainConfigGuardian == (common.Address{}) {
		return fmt.Errorf("%w: SuperchainConfigGuardian cannot be address(0)", ErrInvalidDeployConfig)
	}
	return nil
}

// OutputOracleDeployConfig configures the legacy OutputOracle deployment to L1.
// This is obsoleted with Fault Proofs. See FaultProofDeployConfig.
type OutputOracleDeployConfig struct {
	// L2OutputOracleSubmissionInterval is the number of L2 blocks between outputs that are submitted
	// to the L2OutputOracle contract located on L1.
	L2OutputOracleSubmissionInterval uint64 `json:"l2OutputOracleSubmissionInterval"`
	// L2OutputOracleStartingTimestamp is the starting timestamp for the L2OutputOracle.
	// MUST be the same as the timestamp of the L2OO start block.
	L2OutputOracleStartingTimestamp int64 `json:"l2OutputOracleStartingTimestamp"`
	// L2OutputOracleStartingBlockNumber is the starting block number for the L2OutputOracle.
	// Must be greater than or equal to the first Bedrock block. The first L2 output will correspond
	// to this value plus the submission interval.
	L2OutputOracleStartingBlockNumber uint64 `json:"l2OutputOracleStartingBlockNumber"`
	// L2OutputOracleProposer is the address of the account that proposes L2 outputs.
	L2OutputOracleProposer common.Address `json:"l2OutputOracleProposer"`
	// L2OutputOracleChallenger is the address of the account that challenges L2 outputs.
	L2OutputOracleChallenger common.Address `json:"l2OutputOracleChallenger"`
}

func (d *OutputOracleDeployConfig) Check(log log.Logger) error {
	if d.L2OutputOracleSubmissionInterval == 0 {
		return fmt.Errorf("%w: L2OutputOracleSubmissionInterval cannot be 0", ErrInvalidDeployConfig)
	}
	if d.L2OutputOracleStartingTimestamp == 0 {
		log.Warn("L2OutputOracleStartingTimestamp is 0")
	}
	if d.L2OutputOracleProposer == (common.Address{}) {
		return fmt.Errorf("%w: L2OutputOracleProposer cannot be address(0)", ErrInvalidDeployConfig)
	}
	if d.L2OutputOracleChallenger == (common.Address{}) {
		return fmt.Errorf("%w: L2OutputOracleChallenger cannot be address(0)", ErrInvalidDeployConfig)
	}
	if d.L2OutputOracleStartingBlockNumber == 0 {
		log.Warn("L2OutputOracleStartingBlockNumber is 0, should only be 0 for fresh chains")
	}
	return nil
}

// FaultProofDeployConfig configures the fault-proof deployment to L1.
type FaultProofDeployConfig struct {
	// UseFaultProofs is a flag that indicates if the system is using fault
	// proofs instead of the older output oracle mechanism.
	UseFaultProofs bool `json:"useFaultProofs"`
	// FaultGameAbsolutePrestate is the absolute prestate of Cannon. This is computed
	// by generating a proof from the 0th -> 1st instruction and grabbing the prestate from
	// the output JSON. All honest challengers should agree on the setup state of the program.
	FaultGameAbsolutePrestate common.Hash `json:"faultGameAbsolutePrestate"`
	// FaultGameMaxDepth is the maximum depth of the position tree within the fault dispute game.
	// `2^{FaultGameMaxDepth}` is how many instructions the execution trace bisection game
	// supports. Ideally, this should be conservatively set so that there is always enough
	// room for a full Cannon trace.
	FaultGameMaxDepth uint64 `json:"faultGameMaxDepth"`
	// FaultGameClockExtension is the amount of time that the dispute game will set the potential grandchild claim's,
	// clock to, if the remaining time is less than this value at the time of a claim's creation.
	FaultGameClockExtension uint64 `json:"faultGameClockExtension"`
	// FaultGameMaxClockDuration is the maximum amount of time that may accumulate on a team's chess clock before they
	// may no longer respond.
	FaultGameMaxClockDuration uint64 `json:"faultGameMaxClockDuration"`
	// FaultGameGenesisBlock is the block number for genesis.
	FaultGameGenesisBlock uint64 `json:"faultGameGenesisBlock"`
	// FaultGameGenesisOutputRoot is the output root for the genesis block.
	FaultGameGenesisOutputRoot common.Hash `json:"faultGameGenesisOutputRoot"`
	// FaultGameSplitDepth is the depth at which the fault dispute game splits from output roots to execution trace claims.
	FaultGameSplitDepth uint64 `json:"faultGameSplitDepth"`
	// FaultGameWithdrawalDelay is the number of seconds that users must wait before withdrawing ETH from a fault game.
	FaultGameWithdrawalDelay uint64 `json:"faultGameWithdrawalDelay"`
	// PreimageOracleMinProposalSize is the minimum number of bytes that a large preimage oracle proposal can be.
	PreimageOracleMinProposalSize uint64 `json:"preimageOracleMinProposalSize"`
	// PreimageOracleChallengePeriod is the number of seconds that challengers have to challenge a large preimage proposal.
	PreimageOracleChallengePeriod uint64 `json:"preimageOracleChallengePeriod"`
	// ProofMaturityDelaySeconds is the number of seconds that a proof must be
	// mature before it can be used to finalize a withdrawal.
	ProofMaturityDelaySeconds uint64 `json:"proofMaturityDelaySeconds"`
	// DisputeGameFinalityDelaySeconds is an additional number of seconds a
	// dispute game must wait before it can be used to finalize a withdrawal.
	DisputeGameFinalityDelaySeconds uint64 `json:"disputeGameFinalityDelaySeconds"`
	// RespectedGameType is the dispute game type that the OptimismPortal
	// contract will respect for finalizing withdrawals.
	RespectedGameType uint32 `json:"respectedGameType"`
}

func (d *FaultProofDeployConfig) Check(log log.Logger) error {
	if d.ProofMaturityDelaySeconds == 0 {
		log.Warn("ProofMaturityDelaySeconds is 0")
	}
	if d.DisputeGameFinalityDelaySeconds == 0 {
		log.Warn("DisputeGameFinalityDelaySeconds is 0")
	}
	return nil
}

// L1DependenciesConfig is the set of addresses that affect the L2 genesis construction,
// and is dependent on prior deployment of contracts to L1. This is generally not configured in deploy-config JSON,
// but rather merged in through a L1 deployments JSON file.
type L1DependenciesConfig struct {
	// L1StandardBridgeProxy represents the address of the L1StandardBridgeProxy on L1 and is used
	// as part of building the L2 genesis state.
	L1StandardBridgeProxy common.Address `json:"l1StandardBridgeProxy"`
	// L1CrossDomainMessengerProxy represents the address of the L1CrossDomainMessengerProxy on L1 and is used
	// as part of building the L2 genesis state.
	L1CrossDomainMessengerProxy common.Address `json:"l1CrossDomainMessengerProxy"`
	// L1ERC721BridgeProxy represents the address of the L1ERC721Bridge on L1 and is used
	// as part of building the L2 genesis state.
	L1ERC721BridgeProxy common.Address `json:"l1ERC721BridgeProxy"`
	// SystemConfigProxy represents the address of the SystemConfigProxy on L1 and is used
	// as part of the derivation pipeline.
	SystemConfigProxy common.Address `json:"systemConfigProxy"`
	// OptimismPortalProxy represents the address of the OptimismPortalProxy on L1 and is used
	// as part of the derivation pipeline.
	OptimismPortalProxy common.Address `json:"optimismPortalProxy"`

	// DAChallengeProxy represents the L1 address of the DataAvailabilityChallenge contract.
	DAChallengeProxy common.Address `json:"daChallengeProxy"`

	ProtocolVersionsProxy common.Address `json:"protocolVersionsProxy"`
}

// DependencyContext is the contextual configuration needed to verify the L1 dependencies,
// used by DeployConfig.CheckAddresses.
type DependencyContext struct {
	UseAltDA         bool
	DACommitmentType string
}

func (d *L1DependenciesConfig) CheckAddresses(dependencyContext DependencyContext) error {
	if d.L1StandardBridgeProxy == (common.Address{}) {
		return fmt.Errorf("%w: L1StandardBridgeProxy cannot be address(0)", ErrInvalidDeployConfig)
	}
	if d.L1CrossDomainMessengerProxy == (common.Address{}) {
		return fmt.Errorf("%w: L1CrossDomainMessengerProxy cannot be address(0)", ErrInvalidDeployConfig)
	}
	if d.L1ERC721BridgeProxy == (common.Address{}) {
		return fmt.Errorf("%w: L1ERC721BridgeProxy cannot be address(0)", ErrInvalidDeployConfig)
	}
	if d.SystemConfigProxy == (common.Address{}) {
		return fmt.Errorf("%w: SystemConfigProxy cannot be address(0)", ErrInvalidDeployConfig)
	}
	if d.OptimismPortalProxy == (common.Address{}) {
		return fmt.Errorf("%w: OptimismPortalProxy cannot be address(0)", ErrInvalidDeployConfig)
	}

	if dependencyContext.UseAltDA && dependencyContext.DACommitmentType == altda.KeccakCommitmentString && d.DAChallengeProxy == (common.Address{}) {
		return fmt.Errorf("%w: DAChallengeContract cannot be address(0) when using alt-da mode", ErrInvalidDeployConfig)
	} else if dependencyContext.UseAltDA && dependencyContext.DACommitmentType == altda.GenericCommitmentString && d.DAChallengeProxy != (common.Address{}) {
		return fmt.Errorf("%w: DAChallengeContract must be address(0) when using generic commitments in alt-da mode", ErrInvalidDeployConfig)
	}
	return nil
}

// LegacyDeployConfig retains legacy DeployConfig attributes.
// The genesis generation may log warnings, do a best-effort support attempt,
// or ignore these attributes completely.
type LegacyDeployConfig struct {
	// CliqueSignerAddress represents the signer address for the clique consensus engine.
	// It is used in the multi-process devnet to sign blocks.
	CliqueSignerAddress common.Address `json:"cliqueSignerAddress"`
	// L1UseClique represents whether or not to use the clique consensus engine.
	L1UseClique bool `json:"l1UseClique"`

	// DeploymentWaitConfirmations is the number of confirmations to wait during
	// deployment. This is DEPRECATED and should be removed in a future PR.
	DeploymentWaitConfirmations int `json:"deploymentWaitConfirmations"`

	UnusedChannelTimeoutGranite uint64 `json:"channelTimeoutGranite,omitempty"`
}

var _ ConfigChecker = (*LegacyDeployConfig)(nil)

func (d *LegacyDeployConfig) Check(log log.Logger) error {
	if d.UnusedChannelTimeoutGranite != 0 && d.UnusedChannelTimeoutGranite != opparams.ChannelTimeoutGranite {
		return fmt.Errorf("%w: channelTimeoutGranite is no longer used. Only valid values are 0 or the protocol constant (%d)", ErrInvalidDeployConfig, opparams.ChannelTimeoutGranite)
	}
	return nil
}

// DeployConfig represents the deployment configuration for an OP Stack chain.
// It is used to deploy the L1 contracts as well as create the L2 genesis state.
type DeployConfig struct {
	// Pre-L1-deployment L2 configs
	L2InitializationConfig

	// Development purposes only
	DevL1DeployConfig

	// L1StartingBlockTag anchors the L2 at an L1 block.
	// The timestamp of the block referenced by l1StartingBlockTag is used
	// in the L2 genesis block, rollup-config, and L1 output-oracle contract.
	// The Output oracle deploy script may use it if the L2 starting timestamp is nil, assuming the L2 genesis is set up with this.
	// The L2 genesis timestamp does not affect the initial L2 account state:
	// the storage of the L1Block contract at genesis is zeroed, since the adoption of
	// the L2-genesis allocs-generation through solidity script.
	L1StartingBlockTag *MarshalableRPCBlockNumberOrHash `json:"l1StartingBlockTag" evm:"-"`

	// L1 contracts configuration.
	// The deployer of the contracts chooses which sub-systems to deploy.
	SuperchainL1DeployConfig
	OutputOracleDeployConfig
	FaultProofDeployConfig

	// Post-L1-deployment L2 configs
	L1DependenciesConfig

	// Legacy, ignored, here for strict-JSON decoding to be accepted.
	LegacyDeployConfig `evm:"-"`
}

// Copy will deeply copy the DeployConfig. This does a JSON roundtrip to copy
// which makes it easier to maintain, we do not need efficiency in this case.
func (d *DeployConfig) Copy() *DeployConfig {
	raw, err := json.Marshal(d)
	if err != nil {
		panic(err)
	}

	cpy := DeployConfig{}
	if err = json.Unmarshal(raw, &cpy); err != nil {
		panic(err)
	}
	return &cpy
}

// Check will ensure that the config is sane and return an error when it is not
func (d *DeployConfig) Check(log log.Logger) error {
	if d.L1StartingBlockTag == nil {
		return fmt.Errorf("%w: L1StartingBlockTag cannot be nil", ErrInvalidDeployConfig)
	}

	if d.L2GenesisCanyonTimeOffset != nil && d.EIP1559DenominatorCanyon == 0 {
		return fmt.Errorf("%w: EIP1559DenominatorCanyon cannot be 0 if Canyon is activated", ErrInvalidDeployConfig)
	}
	// L2 block time must always be smaller than L1 block time
	if d.L1BlockTime < d.L2BlockTime {
		return fmt.Errorf("L2 block time (%d) is larger than L1 block time (%d)", d.L2BlockTime, d.L1BlockTime)
	}
	return checkConfigBundle(d, log)
}

// CheckAddresses will return an error if the addresses are not set.
// These values are required to create the L2 genesis state and are present in the deploy config
// even though the deploy config is required to deploy the contracts on L1. This creates a
// circular dependency that should be resolved in the future.
func (d *DeployConfig) CheckAddresses() error {
	return d.L1DependenciesConfig.CheckAddresses(DependencyContext{
		UseAltDA:         d.UseAltDA,
		DACommitmentType: d.DACommitmentType,
	})
}

// SetDeployments will merge a Deployments into a DeployConfig.
func (d *DeployConfig) SetDeployments(deployments *L1Deployments) {
	d.L1StandardBridgeProxy = deployments.L1StandardBridgeProxy
	d.L1CrossDomainMessengerProxy = deployments.L1CrossDomainMessengerProxy
	d.L1ERC721BridgeProxy = deployments.L1ERC721BridgeProxy
	d.SystemConfigProxy = deployments.SystemConfigProxy
	d.OptimismPortalProxy = deployments.OptimismPortalProxy
	d.DAChallengeProxy = deployments.DataAvailabilityChallengeProxy
}

// RollupConfig converts a DeployConfig to a rollup.Config. If Ecotone is active at genesis, the
// Overhead value is considered a noop.
func (d *DeployConfig) RollupConfig(l1StartBlock *types.Header, l2GenesisBlockHash common.Hash, l2GenesisBlockNumber uint64) (*rollup.Config, error) {
	if d.OptimismPortalProxy == (common.Address{}) {
		return nil, errors.New("OptimismPortalProxy cannot be address(0)")
	}
	if d.SystemConfigProxy == (common.Address{}) {
		return nil, errors.New("SystemConfigProxy cannot be address(0)")
	}
	var altDA *rollup.AltDAConfig
	if d.UseAltDA {
		altDA = &rollup.AltDAConfig{
			CommitmentType:     d.DACommitmentType,
			DAChallengeAddress: d.DAChallengeProxy,
			DAChallengeWindow:  d.DAChallengeWindow,
			DAResolveWindow:    d.DAResolveWindow,
		}
	}

	l1StartTime := l1StartBlock.Time

	return &rollup.Config{
		Genesis: rollup.Genesis{
			L1: eth.BlockID{
				Hash:   l1StartBlock.Hash(),
				Number: l1StartBlock.Number.Uint64(),
			},
			L2: eth.BlockID{
				Hash:   l2GenesisBlockHash,
				Number: l2GenesisBlockNumber,
			},
			L2Time: l1StartBlock.Time,
			SystemConfig: eth.SystemConfig{
				BatcherAddr: d.BatchSenderAddress,
				Overhead:    eth.Bytes32(common.BigToHash(new(big.Int).SetUint64(d.GasPriceOracleOverhead))),
				Scalar:      eth.Bytes32(d.FeeScalar()),
				GasLimit:    uint64(d.L2GenesisBlockGasLimit),
			},
		},
		BlockTime:               d.L2BlockTime,
		MaxSequencerDrift:       d.MaxSequencerDrift,
		SeqWindowSize:           d.SequencerWindowSize,
		ChannelTimeoutBedrock:   d.ChannelTimeoutBedrock,
		L1ChainID:               new(big.Int).SetUint64(d.L1ChainID),
		L2ChainID:               new(big.Int).SetUint64(d.L2ChainID),
		BatchInboxAddress:       d.BatchInboxAddress,
		DepositContractAddress:  d.OptimismPortalProxy,
		L1SystemConfigAddress:   d.SystemConfigProxy,
		RegolithTime:            d.RegolithTime(l1StartTime),
		CanyonTime:              d.CanyonTime(l1StartTime),
		DeltaTime:               d.DeltaTime(l1StartTime),
		EcotoneTime:             d.EcotoneTime(l1StartTime),
		FjordTime:               d.FjordTime(l1StartTime),
		GraniteTime:             d.GraniteTime(l1StartTime),
		HoloceneTime:            d.HoloceneTime(l1StartTime),
		InteropTime:             d.InteropTime(l1StartTime),
		ProtocolVersionsAddress: d.ProtocolVersionsProxy,
		AltDAConfig:             altDA,
	}, nil
}

// NewDeployConfig reads a config file given a path on the filesystem.
func NewDeployConfig(path string) (*DeployConfig, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("deploy config at %s not found: %w", path, err)
	}

	dec := json.NewDecoder(bytes.NewReader(file))
	dec.DisallowUnknownFields()

	var config DeployConfig
	if err := dec.Decode(&config); err != nil {
		return nil, fmt.Errorf("cannot unmarshal deploy config: %w", err)
	}

	return &config, nil
}

// NewDeployConfigWithNetwork takes a path to a deploy config directory
// and the network name. The config file in the deploy config directory
// must match the network name and be a JSON file.
func NewDeployConfigWithNetwork(network, path string) (*DeployConfig, error) {
	deployConfig := filepath.Join(path, network+".json")
	return NewDeployConfig(deployConfig)
}

// L1Deployments represents a set of L1 contracts that are deployed.
// This should be consolidated with https://github.com/ethereum-optimism/superchain-registry/blob/f9702a89214244c8dde39e45f5c2955f26d857d0/superchain/superchain.go#L227
type L1Deployments struct {
	AddressManager                    common.Address `json:"AddressManager"`
	DisputeGameFactory                common.Address `json:"DisputeGameFactory"`
	DisputeGameFactoryProxy           common.Address `json:"DisputeGameFactoryProxy"`
	L1CrossDomainMessenger            common.Address `json:"L1CrossDomainMessenger"`
	L1CrossDomainMessengerProxy       common.Address `json:"L1CrossDomainMessengerProxy"`
	L1ERC721Bridge                    common.Address `json:"L1ERC721Bridge"`
	L1ERC721BridgeProxy               common.Address `json:"L1ERC721BridgeProxy"`
	L1StandardBridge                  common.Address `json:"L1StandardBridge"`
	L1StandardBridgeProxy             common.Address `json:"L1StandardBridgeProxy"`
	L2OutputOracle                    common.Address `json:"L2OutputOracle"`
	L2OutputOracleProxy               common.Address `json:"L2OutputOracleProxy"`
	OptimismMintableERC20Factory      common.Address `json:"OptimismMintableERC20Factory"`
	OptimismMintableERC20FactoryProxy common.Address `json:"OptimismMintableERC20FactoryProxy"`
	OptimismPortal                    common.Address `json:"OptimismPortal"`
	OptimismPortalProxy               common.Address `json:"OptimismPortalProxy"`
	ProxyAdmin                        common.Address `json:"ProxyAdmin"`
	SystemConfig                      common.Address `json:"SystemConfig"`
	SystemConfigProxy                 common.Address `json:"SystemConfigProxy"`
	ProtocolVersions                  common.Address `json:"ProtocolVersions"`
	ProtocolVersionsProxy             common.Address `json:"ProtocolVersionsProxy"`
	DataAvailabilityChallenge         common.Address `json:"DataAvailabilityChallenge"`
	DataAvailabilityChallengeProxy    common.Address `json:"DataAvailabilityChallengeProxy"`
}

// GetName will return the name of the contract given an address.
func (d *L1Deployments) GetName(addr common.Address) string {
	val := reflect.ValueOf(d)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	for i := 0; i < val.NumField(); i++ {
		if addr == val.Field(i).Interface().(common.Address) {
			return val.Type().Field(i).Name
		}
	}
	return ""
}

// Check will ensure that the L1Deployments are sane
func (d *L1Deployments) Check(deployConfig *DeployConfig) error {
	val := reflect.ValueOf(d)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	for i := 0; i < val.NumField(); i++ {
		name := val.Type().Field(i).Name
		if !deployConfig.UseFaultProofs &&
			(name == "DisputeGameFactory" ||
				name == "DisputeGameFactoryProxy") {
			continue
		}
		if deployConfig.UseFaultProofs &&
			(name == "OptimismPortal" || name == "L2OutputOracle" || name == "L2OutputOracleProxy") {
			continue
		}
		if !deployConfig.UseAltDA &&
			(name == "DataAvailabilityChallenge" ||
				name == "DataAvailabilityChallengeProxy") {
			continue
		}
		if val.Field(i).Interface().(common.Address) == (common.Address{}) {
			return fmt.Errorf("%s is not set", name)
		}
	}
	return nil
}

// ForEach will iterate over each contract in the L1Deployments
func (d *L1Deployments) ForEach(cb func(name string, addr common.Address)) {
	val := reflect.ValueOf(d)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	for i := 0; i < val.NumField(); i++ {
		name := val.Type().Field(i).Name
		cb(name, val.Field(i).Interface().(common.Address))
	}
}

// Copy will copy the L1Deployments struct
func (d *L1Deployments) Copy() *L1Deployments {
	cpy := L1Deployments{}
	data, err := json.Marshal(d)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(data, &cpy); err != nil {
		panic(err)
	}
	return &cpy
}

// NewL1Deployments will create a new L1Deployments from a JSON file on disk
// at the given path.
func NewL1Deployments(path string) (*L1Deployments, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("L1 deployments at %s not found: %w", path, err)
	}

	var deployments L1Deployments
	if err := json.Unmarshal(file, &deployments); err != nil {
		return nil, fmt.Errorf("cannot unmarshal L1 deployments: %w", err)
	}

	return &deployments, nil
}

type MarshalableRPCBlockNumberOrHash rpc.BlockNumberOrHash

func (m *MarshalableRPCBlockNumberOrHash) MarshalJSON() ([]byte, error) {
	r := rpc.BlockNumberOrHash(*m)
	if hash, ok := r.Hash(); ok {
		return json.Marshal(hash)
	}
	if num, ok := r.Number(); ok {
		// never errors
		text, _ := num.MarshalText()
		return json.Marshal(string(text))
	}
	return json.Marshal(nil)
}

func (m *MarshalableRPCBlockNumberOrHash) UnmarshalJSON(b []byte) error {
	var r rpc.BlockNumberOrHash
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}

	asMarshalable := MarshalableRPCBlockNumberOrHash(r)
	*m = asMarshalable
	return nil
}

// Number wraps the rpc.BlockNumberOrHash Number method.
func (m *MarshalableRPCBlockNumberOrHash) Number() (rpc.BlockNumber, bool) {
	return (*rpc.BlockNumberOrHash)(m).Number()
}

// Hash wraps the rpc.BlockNumberOrHash Hash method.
func (m *MarshalableRPCBlockNumberOrHash) Hash() (common.Hash, bool) {
	return (*rpc.BlockNumberOrHash)(m).Hash()
}

// String wraps the rpc.BlockNumberOrHash String method.
func (m *MarshalableRPCBlockNumberOrHash) String() string {
	return (*rpc.BlockNumberOrHash)(m).String()
}
