package tests

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"math/rand"
	"time"

	nat "github.com/ethereum-optimism/optimism/op-nat"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	ethparams "github.com/ethereum/go-ethereum/params"
	"github.com/pkg/errors"
	"github.com/scharissis/tx-fuzz/spammer"
)

type TxFuzzParams struct {
	NSlotsToRunFor     int
	TxPerAccount       uint64
	GenerateAccessList bool
}

// TxFuzz is a test that runs tx-fuzz.
// It runs 3 slots of spam, with 1 transaction per account.
var TxFuzz = nat.Test{
	ID: "tx-fuzz",
	DefaultParams: TxFuzzParams{
		NSlotsToRunFor:     120, // Duration of the fuzzing
		TxPerAccount:       2,
		GenerateAccessList: false,
	},
	Fn: func(ctx context.Context, log log.Logger, cfg nat.Config, params interface{}) (bool, error) {
		p := params.(TxFuzzParams)
		err := runBasicSpam(cfg, p)
		if err != nil {
			return false, err
		}
		return true, nil
	},
}

func runBasicSpam(config nat.Config, params TxFuzzParams) error {
	fuzzCfg, err := newConfig(config, params)
	if err != nil {
		return err
	}
	airdropValue := new(big.Int).Mul(big.NewInt(int64((1+fuzzCfg.N)*1000000)), big.NewInt(ethparams.Wei))
	return spam(fuzzCfg, spammer.SendBasicTransactions, airdropValue, params)
}

func spam(config *spammer.Config, spamFn spammer.Spam, airdropValue *big.Int, params TxFuzzParams) error {
	// Make sure the accounts are unstuck before sending any transactions
	log.Debug("Spam config",
		"tx_per_account", config.N,
		"slot_time", config.SlotTime,
		"params", params.NSlotsToRunFor,
		"params", params.TxPerAccount,
	)
	if err := spammer.Unstuck(config); err != nil {
		return err
	}

	for nSlots := 0; nSlots < params.NSlotsToRunFor; nSlots++ {
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

func newConfig(c nat.Config, p TxFuzzParams) (*spammer.Config, error) {
	txPerAccount := p.TxPerAccount
	genAccessList := p.GenerateAccessList
	// rpcURL := c.RPCURL
	// senderSecretKey := c.SenderSecretKey
	receiverPublicKeys := c.ReceiverPublicKeys

	// Faucet
	faucet, err := crypto.ToECDSA(common.FromHex("0xe9418c9c567a7758581a1d6037722983d251c2c235fca5b4f420095f1a9383b6"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert sender secret key to ECDSA")
	}

	// Private keys
	keys := receiverPublicKeys
	var privateKeys []*ecdsa.PrivateKey
	for i := 0; i < len(keys); i++ {
		privateKeys = append(privateKeys, crypto.ToECDSAUnsafe(common.FromHex(keys[i])))
	}

	cfg, err := spammer.NewDefaultConfig(c.L1.Addr, txPerAccount, genAccessList, rand.New(rand.NewSource(time.Now().UnixNano())))
	if err != nil {
		return nil, err
	}
	cfg = cfg.WithFaucet(faucet).WithKeys(privateKeys)

	return cfg, nil
}
