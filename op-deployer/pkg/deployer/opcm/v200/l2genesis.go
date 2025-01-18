package opcm

import (
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum/go-ethereum/common"
)

type L2GenesisInput struct {
	L1CrossDomainMessengerProxy              common.Address
	L1StandardBridgeProxy                    common.Address
	L1ERC721BridgeProxy                      common.Address
	FundDevAccounts                          bool
	UseInterop                               bool
	Fork                                     uint8
	L2ChainId                                *big.Int
	L1ChainId                                *big.Int
	ProxyAdminOwner                          common.Address
	SequencerFeeVaultRecipient               common.Address
	SequencerFeeVaultMinimumWithdrawalAmount *big.Int
	SequencerFeeVaultWithdrawalNetwork       *big.Int
	L1FeeVaultRecipient                      common.Address
	L1FeeVaultMinimumWithdrawalAmount        *big.Int
	L1FeeVaultWithdrawalNetwork              *big.Int
	BaseFeeVaultRecipient                    common.Address
	BaseFeeVaultMinimumWithdrawalAmount      *big.Int
	BaseFeeVaultWithdrawalNetwork            *big.Int
	EnableGovernance                         bool
	GovernanceTokenOwner                     common.Address
}

type L2GenesisScript struct {
	Run func(input common.Address) error
}

func L2Genesis(l2Host *script.Host, input L2GenesisInput) error {
	l2iAddr := l2Host.NewScriptAddress()
	cleanupDGI, err := script.WithPrecompileAtAddress[*L2GenesisInput](l2Host, l2iAddr, &input)
	if err != nil {
		return fmt.Errorf("failed to insert L2GenesisInput precompile: %w", err)
	}
	defer cleanupDGI()
	l2Host.Label(l2iAddr, "L2GenesisInput")

	l2GenesisScript, cleanupL2Genesis, err := script.WithScript[L2GenesisScript](l2Host, "L2Genesis.s.sol", "L2Genesis")
	if err != nil {
		return fmt.Errorf("failed to load L2Genesis script: %w", err)
	}
	defer cleanupL2Genesis()

	if err := l2GenesisScript.Run(l2iAddr); err != nil {
		return fmt.Errorf("failed to run L2Genesis script: %w", err)
	}
	return nil
}
