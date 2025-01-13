package tests

import (
	"crypto/ecdsa"
	"math/big"
	"math/rand"
	"time"

	nat "github.com/ethereum-optimism/optimism/op-nat"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/pkg/errors"
	"github.com/scharissis/tx-fuzz/spammer"
)

// TxFuzz is a test that runs tx-fuzz.
// It runs 3 slots of spam, with 1 transaction per account.
var TxFuzz = nat.Test{
	ID: "tx-fuzz",
	Fn: func(cfg nat.Config) (bool, error) {
		err := runBasicSpam(cfg)
		if err != nil {
			return false, err
		}
		return true, nil
	},
}

func runBasicSpam(config nat.Config) error {
	fuzzCfg, err := newConfig(config)
	if err != nil {
		return err
	}
	airdropValue := new(big.Int).Mul(big.NewInt(int64((1+fuzzCfg.N)*1000000)), big.NewInt(params.GWei))
	return spam(fuzzCfg, spammer.SendBasicTransactions, airdropValue)
}

func spam(config *spammer.Config, spamFn spammer.Spam, airdropValue *big.Int) error {
	// Make sure the accounts are unstuck before sending any transactions
	if err := spammer.Unstuck(config); err != nil {
		return err
	}

	for nSlots := 0; nSlots < 12; nSlots++ {
		if err := spammer.Airdrop(config, airdropValue); err != nil {
			return err
		}
		if err := spammer.SpamTransactions(config, spamFn); err != nil {
			return err
		}
		time.Sleep(time.Duration(config.SlotTime) * time.Second)
	}
	return nil
}

func newConfig(c nat.Config) (*spammer.Config, error) {
	txPerAccount := uint64(1)
	genAccessList := false
	rpcURL := c.RPCURL
	senderSecretKey := c.SenderSecretKey
	receiverPublicKeys := c.ReceiverPublicKeys

	// Faucet
	faucet, err := crypto.ToECDSA(common.FromHex(senderSecretKey))
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert sender secret key to ECDSA")
	}

	// Private keys
	keys := receiverPublicKeys
	var privateKeys []*ecdsa.PrivateKey
	for i := 0; i < len(keys); i++ {
		privateKeys = append(privateKeys, crypto.ToECDSAUnsafe(common.FromHex(keys[i])))
	}

	cfg, err := spammer.NewDefaultConfig(rpcURL, txPerAccount, genAccessList, rand.New(rand.NewSource(time.Now().UnixNano())))
	if err != nil {
		return nil, err
	}
	cfg = cfg.WithFaucet(faucet).WithKeys(privateKeys)

	return cfg, nil
}
