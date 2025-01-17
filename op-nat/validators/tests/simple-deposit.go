package tests

import (
	"context"
	"fmt"
	"math/big"
	"time"

	nat "github.com/ethereum-optimism/optimism/op-nat"
	"github.com/ethereum-optimism/optimism/op-nat/network"
	"github.com/ethereum-optimism/optimism/op-nat/wallet"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/pkg/errors"
)

type SimpleDepositParams struct {
	// MaxBalanceChecks is the amount of time to poll for the balance to be updated
	MaxBalanceChecks int
	// DepositAmount is the amount of eth deposited
	DepositAmount big.Int
	// MinL1Balance is how much eth is required to run the test
	MinL1Balance big.Int
}

// SimpleTransfer is a test that runs a transfer on a network
var SimpleDeposit = nat.Test{
	ID: "simple-deposit",
	DefaultParams: SimpleDepositParams{
		MaxBalanceChecks: 12,
		DepositAmount:    *big.NewInt(1000),
		MinL1Balance:     *big.NewInt(10000),
	},

	Fn: func(ctx context.Context, log log.Logger, cfg nat.Config, params interface{}) (bool, error) {
		p := params.(SimpleDepositParams)
		wallet, err := SetupSimpleDepositTest(ctx, log, cfg, p)

		l2ProxyPortal := common.HexToAddress(cfg.SC.L2[0].Addresses.OptimismPortalProxy)
		if err != nil {
			return false, err
		}
		return SimpleDepositTest(ctx, log, cfg.L1, cfg.L2A, wallet, l2ProxyPortal, p)
	},
}

func SetupSimpleDepositTest(ctx context.Context, log log.Logger, config nat.Config, p SimpleDepositParams) (*wallet.Wallet, error) {

	wallet, err := config.GetWalletWithBalance(ctx, config.L1, &p.MinL1Balance)
	if err != nil {
		return nil, errors.Wrap(err, "SetupSimpleDepositTest failed")
	}
	log.Debug("deposit wallet",
		"public", wallet.Address(),
	)
	return wallet, nil
}

func SimpleDepositTest(ctx context.Context, log log.Logger, l1, l2 *network.Network, wallet *wallet.Wallet, portal common.Address, p SimpleDepositParams) (bool, error) {
	if l1 == nil || wallet == nil || l2 == nil || len(portal) == 0 {
		return false, errors.New("error empty arguments provided for SimpleDepositTest")
	}

	l1Pre, err := wallet.GetBalance(ctx, l1)
	if err != nil {
		return false, errors.Wrap(err, "error getting l1 balance")
	}
	l2Pre, err := wallet.GetBalance(ctx, l2)
	if err != nil {
		return false, errors.Wrap(err, "error getting l2 balance")
	}

	log.Debug("user balances pre simple deposit test",
		"address", wallet.Address().String(),
		"l1_pre_deposit", l1Pre.String(),
		"l2_pre_deposit", l2Pre.String(),
	)

	log.Debug("sending deposit",
		"deposit_value", p.DepositAmount.String(),
		"portal", portal,
	)

	tx, err := wallet.Send(ctx, l1, &p.DepositAmount, portal)
	if err != nil {
		return false, errors.Wrap(err, fmt.Sprintf("error sending simple deposit"+
			"l1_network: %s"+
			"l2_network: %s"+
			"walletA: %s"+
			l1.Name,
			l2.Name,
			wallet.Address(),
		))
	}
	log.Debug("sent deposit",
		"tx_hash", tx.Hash(),
		"to", tx.To(),
		"nonce", tx.Nonce(),
	)

	if err := l1.PollForConfirmation(ctx, log, 3, tx.Hash()); err != nil {
		return false, errors.New("polling for deposit transaction confirmation timed out")
	}

	var l1Post *big.Int
	var l2Post *big.Int

	l1Diff := false
	l2Diff := false

	l1PostExpected := new(big.Int)
	l1PostExpected.Sub(l1Pre, &p.DepositAmount)

	l2PostExpected := new(big.Int)
	l2PostExpected.Add(&p.DepositAmount, l2Pre)

	for i := 0; i < p.MaxBalanceChecks; i++ {
		l1Post, err = wallet.GetBalance(ctx, l1)
		if err != nil {
			return false, errors.Wrap(err, "error getting l1 balance")
		}

		l2Post, err = wallet.GetBalance(ctx, l2)
		if err != nil {
			return false, errors.Wrap(err, "error getting l2 balance")
		}

		log.Debug("polling balance post simple deposit test",
			"l1_post", l1Post.String(),
			"l2_post", l2Post.String(),
		)

		// Expect walletA post to be less than walletAPre - transfer value due to gas as well
		if l1Post.Cmp(l1PostExpected) < 0 && !l1Diff {
			log.Debug("l1 balance has been subtracted")
			l1Diff = true
		}

		if l2PostExpected.Cmp(l2Post) == 0 && !l2Diff {
			log.Debug("l2 balance has increased")
			l2Diff = true
		}

		if l2Diff && l1Diff {
			break
		}

		time.Sleep(5 * time.Second)
	}

	if !l1Diff {
		return false, errors.New("error l1 balance was not subtracted during deposit")
	}
	if !l2Diff {
		return false, errors.New("error l2 balance was not added to during deposit")
	}

	return true, nil
}
