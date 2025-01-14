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

// SimpleTransfer is a test that runs a transfer on a network
var SimpleDeposit = nat.Test{
	ID: "simple-deposit",
	Fn: func(ctx context.Context, log log.Logger, cfg nat.Config) (bool, error) {
		l1, l2, wallet, err := SetupSimpleDepositTest(ctx, log, cfg)
		l2ProxyPortal := common.HexToAddress(cfg.SC.L2[0].Addresses.OptimismPortalProxy)
		if err != nil {
			return false, err
		}
		return SimpleDepositTest(ctx, log, l1, l2, wallet, l2ProxyPortal)
	},
}

func SetupSimpleDepositTest(ctx context.Context, log log.Logger, config nat.Config) (*network.Network, *network.Network, *wallet.Wallet, error) {
	l1, err := network.NewNetwork(ctx, log, config.L1RPCUrl, "kurtosis-1")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("SetupSimpleDeposit failed to setup network")
	}

	l2port := config.SC.L2[0].Nodes[0].Services.EL.Endpoints["rpc"].Port
	l2Addr := config.SC.L2[0].Nodes[0].Services.EL.Endpoints["rpc"].Host
	l2RPC := fmt.Sprintf("http://%s:%d", l2Addr, l2port)

	log.Info("rpc info", "l1_rpc", config.L1RPCUrl, "l2_rpc", l2RPC)

	l2, err := network.NewNetwork(ctx, log, l2RPC, "kurtosis-l2")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("SetupSimpleDepositTest failed to setup network")
	}
	wallet, err := wallet.NewWallet(config.SC.L1.Wallets["user-key-13"].PrivateKey, "user-13")
	log.Info("wallet",
		"public", wallet.Address(),
	)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "SetupSimpleDepositTest failed")
	}

	return l1, l2, wallet, nil
}

func SimpleDepositTest(ctx context.Context, log log.Logger, l1, l2 *network.Network, wallet *wallet.Wallet, portal common.Address) (bool, error) {

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

	log.Info("user balances pre simple deposit test",
		"address", wallet.Address().String(),
		"l1_pre_deposit", l1Pre.String(),
		"l2_pre_deposit", l2Pre.String(),
	)

	// Confirm wallet has more than 10m wei
	if l1Pre.Cmp(big.NewInt(10000000)) < 0 {
		return false, errors.New("error wallet A does not have enough balance to perform simple deposit")
	}

	transferValue := big.NewInt(100000)

	log.Info("sending deposit",
		"deposit_value", transferValue.String(),
		"portal", portal,
	)

	tx, err := wallet.Send(ctx, l1, transferValue, portal)
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
	log.Info("sent deposit",
		"tx_hash", tx.Hash(),
		"to", tx.To(),
		"nonce", tx.Nonce(),
	)

	var l1Post *big.Int
	var l2Post *big.Int

	l1Diff := false
	l2Diff := false

	for i := 0; i < 12; i++ {

		l1Post, err = wallet.GetBalance(ctx, l1)
		if err != nil {
			return false, errors.Wrap(err, "error getting l1 balance")
		}

		l2Post, err = wallet.GetBalance(ctx, l2)
		if err != nil {
			return false, errors.Wrap(err, "error getting l2 balance")
		}

		log.Info("polling balance post simple deposit test",
			"l1_post", l1Post.String(),
			"l2_post", l2Post.String(),
		)

		l1PostExpected := new(big.Int)
		l1PostExpected.Sub(l1Pre, transferValue)

		// Expect walletA post to be less than walletAPre - transfer value due to gas as well
		if l1Post.Cmp(l1PostExpected) < 0 && !l1Diff {
			log.Info("l1 balance has been subtracted")
			l1Diff = true
		}

		l2PostExpected := new(big.Int)
		l2PostExpected.Add(transferValue, l2Pre)

		if l2PostExpected.Cmp(l2Post) == 0 && !l2Diff {
			log.Info("l2 balance has increased")
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
