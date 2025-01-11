package nat

import (
	"context"
	"time"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet-nat/pkg/network"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet-nat/pkg/wallet"
	"github.com/ethereum/go-ethereum/log"
)

type NetworkTester struct {
	ctx context.Context
	log log.Logger

	// TODO: rename l2 networks to make more sense
	L1  network.Network
	L2A network.Network
	L2B network.Network

	user0 wallet.Wallet
	user1 wallet.Wallet
}

func (n *NetworkTester) Start(ctx context.Context) error {
	n.log.Info("Starting network tester")
	n.log.Info("Available networks")
	_ = n.L1.DumpInfo(ctx)
	_ = n.L2A.DumpInfo(ctx)
	_ = n.L2B.DumpInfo(ctx)

	networks := []network.Network{n.L1, n.L2A, n.L2B}

	n.user0.Dump(ctx, n.log, networks)
	n.user1.Dump(ctx, n.log, networks)

	_, err := n.user1.Send(ctx, n.L2A, n.user0.Address())
	if err != nil {
		log.Error("error sending transcation", "err", err)
	}

	time.Sleep(5 * time.Second)
	n.user0.Dump(ctx, n.log, networks)
	n.user1.Dump(ctx, n.log, networks)

	// _ = n.L1.DumpInfo(ctx)
	// _ = n.L2A.DumpInfo(ctx)
	// _ = n.L2B.DumpInfo(ctx)

	return nil
}

func (n *NetworkTester) Stop(ctx context.Context) error {
	return nil
}

func (n *NetworkTester) Stopped() bool {
	return true
}

func New(ctx context.Context, log log.Logger) (*NetworkTester, error) {
	l1, err := network.NewNetwork(ctx, log, "http://127.0.0.1:60370", "kurtosis-l1")
	if err != nil {
		log.Error("error creating l1 network", "err", err)
		return nil, err
	}

	l2A, err := network.NewNetwork(ctx, log, "http://127.0.0.1:60403", "kurtosis-1")
	if err != nil {
		log.Error("error creating l2a network", "err", err)
		return nil, err
	}

	l2B, err := network.NewNetwork(ctx, log, "http://127.0.0.1:60418", "kurtosis-2")
	if err != nil {
		log.Error("error creating l2b network", "err", err)
		return nil, err
	}

	user0, err := wallet.NewWallet(
		"0xbcdf20249abf0ed6d944c0288fad489e33f66b3960d9e6229c1cd214ed3bbe31",
		"user0",
	)
	if err != nil {
		log.Error("error creating user0 wallet", "err", err)
		return nil, err
	}

	user1, err := wallet.NewWallet(
		"0x39725efee3fb28614de3bacaffe4cc4bd8c436257e2c8bb887c4b5c4be45e76d",
		"user1",
	)
	if err != nil {
		log.Error("error creating user1 wallet", "err", err)
		return nil, err
	}

	return &NetworkTester{
		log:   log,
		ctx:   ctx,
		L1:    *l1,
		L2A:   *l2A,
		L2B:   *l2B,
		user0: *user0,
		user1: *user1,
	}, nil
}
