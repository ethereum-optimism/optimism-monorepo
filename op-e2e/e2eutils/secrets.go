package e2eutils

import (
	"crypto/ecdsa"
	"fmt"
	"strconv"

	hdwallet "github.com/ethereum-optimism/go-ethereum-hdwallet"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const defaultHDPathPrefix = "m/44'/60'/0'/0/"

// DefaultMnemonicConfig is the default mnemonic used in testing.
// We prefer a mnemonic rather than direct private keys to make it easier
// to export all testing keys in external tooling for use during debugging.
// If these values are changed, it is subject to breaking tests. They
// must be in sync with the values in the DeployConfig used to create the system.
var DefaultMnemonicConfig = &MnemonicConfig{
	Mnemonic: "test test test test test test test test test test test junk",
	// Note: "m/44'/60'/0'/0/0" is a legacy mnemonic path, used for the L1 clique signer.
	Proposer:     "m/44'/60'/0'/0/1",
	Batcher:      "m/44'/60'/0'/0/2",
	Deployer:     "m/44'/60'/0'/0/3",
	Alice:        "m/44'/60'/0'/0/4",
	SequencerP2P: "m/44'/60'/0'/0/5",
	Bob:          "m/44'/60'/0'/0/7",
	Mallory:      "m/44'/60'/0'/0/8",
	SysCfgOwner:  "m/44'/60'/0'/0/9",
}

// MnemonicConfig configures the private keys for the hive testnet.
// It's json-serializable, so we can ship it to e.g. the hardhat script client.
type MnemonicConfig struct {
	Mnemonic string

	Deployer    string
	SysCfgOwner string

	// rollup actors
	Proposer     string
	Batcher      string
	SequencerP2P string

	// prefunded L1/L2 accounts for testing
	Alice   string
	Bob     string
	Mallory string
}

// Secrets computes the private keys for all mnemonic paths,
// which can then be kept around for fast precomputed private key access.
func (m *MnemonicConfig) Secrets() (*Secrets, error) {
	wallet, err := hdwallet.NewFromMnemonic(m.Mnemonic)
	if err != nil {
		return nil, fmt.Errorf("failed to create wallet: %w", err)
	}
	account := func(path string) accounts.Account {
		return accounts.Account{URL: accounts.URL{Path: path}}
	}

	deployer, err := wallet.PrivateKey(account(m.Deployer))
	if err != nil {
		return nil, err
	}
	sysCfgOwner, err := wallet.PrivateKey(account(m.SysCfgOwner))
	if err != nil {
		return nil, err
	}
	proposer, err := wallet.PrivateKey(account(m.Proposer))
	if err != nil {
		return nil, err
	}
	batcher, err := wallet.PrivateKey(account(m.Batcher))
	if err != nil {
		return nil, err
	}
	sequencerP2P, err := wallet.PrivateKey(account(m.SequencerP2P))
	if err != nil {
		return nil, err
	}
	alice, err := wallet.PrivateKey(account(m.Alice))
	if err != nil {
		return nil, err
	}
	bob, err := wallet.PrivateKey(account(m.Bob))
	if err != nil {
		return nil, err
	}
	mallory, err := wallet.PrivateKey(account(m.Mallory))
	if err != nil {
		return nil, err
	}

	return &Secrets{
		Deployer:     deployer,
		SysCfgOwner:  sysCfgOwner,
		Proposer:     proposer,
		Batcher:      batcher,
		SequencerP2P: sequencerP2P,
		Alice:        alice,
		Bob:          bob,
		Mallory:      mallory,
		Wallet:       wallet,
	}, nil
}

// Secrets bundles secp256k1 private keys for all common rollup actors for testing purposes.
type Secrets struct {
	Deployer    *ecdsa.PrivateKey
	SysCfgOwner *ecdsa.PrivateKey

	// rollup actors
	Proposer     *ecdsa.PrivateKey
	Batcher      *ecdsa.PrivateKey
	SequencerP2P *ecdsa.PrivateKey

	// prefunded L1/L2 accounts for testing
	Alice   *ecdsa.PrivateKey
	Bob     *ecdsa.PrivateKey
	Mallory *ecdsa.PrivateKey

	// Share the wallet to be able to generate more accounts
	Wallet *hdwallet.Wallet
}

// Addresses computes the ethereum address of each account,
// which can then be kept around for fast precomputed address access.
func (s *Secrets) Addresses() *Addresses {
	return &Addresses{
		Deployer:     crypto.PubkeyToAddress(s.Deployer.PublicKey),
		SysCfgOwner:  crypto.PubkeyToAddress(s.SysCfgOwner.PublicKey),
		Proposer:     crypto.PubkeyToAddress(s.Proposer.PublicKey),
		Batcher:      crypto.PubkeyToAddress(s.Batcher.PublicKey),
		SequencerP2P: crypto.PubkeyToAddress(s.SequencerP2P.PublicKey),
		Alice:        crypto.PubkeyToAddress(s.Alice.PublicKey),
		Bob:          crypto.PubkeyToAddress(s.Bob.PublicKey),
		Mallory:      crypto.PubkeyToAddress(s.Mallory.PublicKey),
	}
}

// Addresses bundles the addresses for all common rollup addresses for testing purposes.
type Addresses struct {
	Deployer    common.Address
	SysCfgOwner common.Address

	// rollup actors
	Proposer     common.Address
	Batcher      common.Address
	SequencerP2P common.Address

	// prefunded L1/L2 accounts for testing
	Alice   common.Address
	Bob     common.Address
	Mallory common.Address
}

func (a *Addresses) All() []common.Address {
	return []common.Address{
		a.Deployer,
		a.SysCfgOwner,
		a.Proposer,
		a.Batcher,
		a.SequencerP2P,
		a.Alice,
		a.Bob,
		a.Mallory,
	}
}

func (s *Secrets) AccountAtIdx(idx int) *ecdsa.PrivateKey {
	account := accounts.Account{URL: accounts.URL{
		Path: defaultHDPathPrefix + strconv.Itoa(idx),
	}}
	pk, err := s.Wallet.PrivateKey(account)
	if err != nil {
		panic(err)
	}
	return pk
}
