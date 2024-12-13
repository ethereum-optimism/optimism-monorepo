package genesis

import (
	"fmt"
	"math/big"

	hdwallet "github.com/ethereum-optimism/go-ethereum-hdwallet"
	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
)

type L2AllocsMode string

type L2AllocsModeMap map[L2AllocsMode]*foundry.ForgeAllocs

const (
	L2AllocsDelta    L2AllocsMode = "delta"
	L2AllocsEcotone  L2AllocsMode = "ecotone"
	L2AllocsFjord    L2AllocsMode = "fjord"
	L2AllocsGranite  L2AllocsMode = "granite"
	L2AllocsHolocene L2AllocsMode = "holocene"
)

var (
	// l2PredeployNamespace is the namespace for L2 predeploys
	l2PredeployNamespace = common.HexToAddress("0x4200000000000000000000000000000000000000")
	// mnemonic for the test accounts in hardhat/foundry
	testMnemonic = "test test test test test test test test test test test junk"
)

type AllocsLoader func(mode L2AllocsMode) *foundry.ForgeAllocs

// BuildL2Genesis will build the L2 genesis block.
func BuildL2Genesis(config *DeployConfig, dump *foundry.ForgeAllocs, l1StartBlock *types.Header) (*core.Genesis, error) {
	genspec, err := NewL2Genesis(config, l1StartBlock)
	if err != nil {
		return nil, err
	}
	genspec.Alloc = dump.Copy().Accounts
	// ensure the dev accounts are not funded unintentionally
	if hasDevAccounts, err := HasAnyDevAccounts(genspec.Alloc); err != nil {
		return nil, fmt.Errorf("failed to check dev accounts: %w", err)
	} else if hasDevAccounts != config.FundDevAccounts {
		return nil, fmt.Errorf("deploy config mismatch with allocs. Deploy config fundDevAccounts: %v, actual allocs: %v", config.FundDevAccounts, hasDevAccounts)
	}
	// sanity check the permit2 immutable, to verify we using the allocs for the right chain.
	if permit2 := genspec.Alloc[predeploys.Permit2Addr].Code; len(permit2) != 0 {
		if len(permit2) < 6945+32 {
			return nil, fmt.Errorf("permit2 code is too short (%d)", len(permit2))
		}
		chainID := [32]byte(permit2[6945 : 6945+32])
		expected := uint256.MustFromBig(genspec.Config.ChainID).Bytes32()
		if chainID != expected {
			return nil, fmt.Errorf("allocs were generated for chain ID %x, but expected chain %x (%d)", chainID, expected, genspec.Config.ChainID)
		}
	}
	// sanity check that all predeploys are present
	for i := 0; i < 2048; i++ {
		addr := common.BigToAddress(new(big.Int).Or(l2PredeployNamespace.Big(), big.NewInt(int64(i))))
		if !config.GovernanceEnabled() && addr == predeploys.GovernanceTokenAddr {
			continue
		}
		if len(genspec.Alloc[addr].Code) == 0 {
			return nil, fmt.Errorf("predeploy %x is missing from L2 genesis allocs", addr)
		}
	}

	return genspec, nil
}

func HasAnyDevAccounts(allocs types.GenesisAlloc) (bool, error) {
	wallet, err := hdwallet.NewFromMnemonic(testMnemonic)
	if err != nil {
		return false, fmt.Errorf("failed to create wallet: %w", err)
	}
	account := func(path string) accounts.Account {
		return accounts.Account{URL: accounts.URL{Path: path}}
	}
	for i := 0; i < 30; i++ {
		key, err := wallet.PrivateKey(account(fmt.Sprintf("m/44'/60'/0'/0/%d", i)))
		if err != nil {
			return false, err
		}
		addr := crypto.PubkeyToAddress(key.PublicKey)
		if _, ok := allocs[addr]; ok {
			return true, nil
		}
	}
	return false, nil
}
