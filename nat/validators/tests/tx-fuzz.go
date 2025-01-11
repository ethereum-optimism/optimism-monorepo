package tests

import (
	"crypto/ecdsa"
	"math/big"
	"math/rand"
	"time"

	nat "github.com/ethereum-optimism/nat/pkg"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/scharissis/tx-fuzz/spammer"
)

var TxFuzz = &nat.Test{
	Name: "tx-fuzz",
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
	spammer.Unstuck(config)
	for {
		if err := spammer.Airdrop(config, airdropValue); err != nil {
			return err
		}
		err := spammer.SpamTransactions(config, spamFn)
		if err != nil {
			return err
		}
		time.Sleep(time.Duration(config.SlotTime) * time.Second)
	}
}

func newConfig(c nat.Config) (*spammer.Config, error) {
	txPerAccount := uint64(1)
	genAccessList := false

	// Faucet
	faucet, err := crypto.ToECDSA(common.FromHex(c.SenderSecretKey))
	if err != nil {
		return nil, err
	}

	// Private keys
	keys := c.ReceiverPublicKeys
	var privateKeys []*ecdsa.PrivateKey
	for i := 0; i < len(keys); i++ {
		privateKeys = append(privateKeys, crypto.ToECDSAUnsafe(common.FromHex(keys[i])))
	}

	cfg, err := spammer.NewDefaultConfig(c.RPCURL, txPerAccount, genAccessList, rand.New(rand.NewSource(time.Now().UnixNano())))
	if err != nil {
		return nil, err
	}
	cfg = cfg.WithFaucet(faucet).WithKeys(privateKeys)

	return cfg, nil
}
