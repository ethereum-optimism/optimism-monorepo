package tests

import (
	"context"
	"fmt"
	"math/big"

	nat "github.com/ethereum-optimism/optimism/op-nat"
	"github.com/ethereum-optimism/optimism/op-nat/network"
	"github.com/ethereum-optimism/optimism/op-nat/wallet"
	"github.com/ethereum/go-ethereum/log"
	"github.com/pkg/errors"
)

// SimpleTransfer is a test that runs a transfer on a network
var SimpleTransfer = nat.Test{
	ID: "simple-transfer",
	Fn: func(ctx context.Context, log log.Logger, cfg nat.Config) (bool, error) {
		network, walletA, walletB, err := SetupSimpleTransferTest(ctx, log, cfg)
		if err != nil {
			return false, err
		}
		return SimpleTransferTest(ctx, log, network, walletA, walletB)
	},
}

func SetupSimpleTransferTest(ctx context.Context, log log.Logger, config nat.Config) (*network.Network, *wallet.Wallet, *wallet.Wallet, error) {
	network, err := network.NewNetwork(ctx, log, config.L1RPCUrl, "kurtosis-l1")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("SetupSimpleTransfer failed to setup network")
	}

	walletA, err := wallet.NewWallet(config.ReceiverPrivateKeys[0], "walletA")
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "SetupSimpleTransfer failed to wallet A")
	}

	walletB, err := wallet.NewWallet(config.ReceiverPrivateKeys[1], "walletB")
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "SetupSimpleTransfer failed to wallet B")
	}

	return network, walletA, walletB, nil
}

func SimpleTransferTest(ctx context.Context, log log.Logger, network *network.Network, walletA, walletB *wallet.Wallet) (bool, error) {
	// Make sure the accounts are unstuck before sending any transactions
	if network == nil || walletA == nil || walletB == nil {
		return false, errors.New("error empty arguments provided for SimpleTransferTest")
	}

	walletABalancePre, err := walletA.GetBalance(ctx, network)
	if err != nil {
		return false, errors.Wrap(err, "error getting walletA balance")
	}

	walletBBalancePre, err := walletB.GetBalance(ctx, network)
	if err != nil {
		return false, errors.Wrap(err, "error getting walletB balance")
	}

	log.Debug("user balances pre simple transfer test",
		"wallet_a", walletABalancePre.String(),
		"wallet_a_addr", walletA.Address(),
		"wallet_b", walletBBalancePre.String(),
		"wallet_b_addr", walletB.Address(),
		"network", network.Name,
	)

	// Confirm wallet has more than 10m wei
	if walletABalancePre.Cmp(big.NewInt(10000000)) < 0 {
		return false, errors.New("error wallet A does not have enough balance to perform simple transfer")
	}

	transferValue := big.NewInt(100000)

	log.Debug("sending transfer from A to B",
		"wallet_a", walletABalancePre.String(),
		"wallet_b", walletBBalancePre.String(),
		"transfer_value", transferValue.String(),
	)

	_, err = walletA.Send(ctx, network, transferValue, walletB.Address())
	if err != nil {
		return false, errors.Wrap(err, fmt.Sprintf("error sending simple transfer"+
			"network: %s"+
			"walletA: %s"+
			"walletB: %s",
			network.Name,
			walletA.Address(),
			walletB.Address(),
		))
	}

	walletABalancePost, err := walletA.GetBalance(ctx, network)
	if err != nil {
		return false, errors.Wrap(err, "error getting walletA balance")
	}

	walletBBalancePost, err := walletA.GetBalance(ctx, network)
	if err != nil {
		return false, errors.Wrap(err, "error getting walletB balance")
	}

	log.Debug("user balances post simple transfer test",
		"wallet_a", walletABalancePost.String(),
		"wallet_b", walletBBalancePost.String(),
	)

	walletAPostExpected := new(big.Int)
	walletAPostExpected.Sub(walletABalancePre, transferValue)

	// Expect walletA post to be less than walletAPre - transfer value due to gas as well
	if walletABalancePost.Cmp(walletAPostExpected) < 0 {
		return false, errors.New("error walletA balance post transfer was incorrect")
	}

	walletBPostExpected := new(big.Int)
	walletBPostExpected.Add(transferValue, walletBBalancePre)

	if walletBBalancePost.Cmp(walletBPostExpected) == 0 {
		return false, errors.New("error walletB balance post transfer was incorrect")
	}

	return true, nil
}
