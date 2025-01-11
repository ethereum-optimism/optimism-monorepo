package nat

import (
	"context"

	wallet "github.com/ethereum-optimism/optimism/kurtosis-devnet-nat/pkg/wallet"
	"github.com/ethereum/go-ethereum/log"
)

type NetworkTester struct {
	ctx context.Context
	log log.Logger

	L1  wallet.Network
	L2A wallet.Network
	L2B wallet.Network
}

func (n *NetworkTester) Start(ctx context.Context) error {
	n.log.Info("Starting network tester")
	n.log.Info("Available networks")
	_ = n.L1.DumpInfo(ctx)
	_ = n.L2A.DumpInfo(ctx)
	_ = n.L2B.DumpInfo(ctx)
	return nil
}

func (n *NetworkTester) Stop(ctx context.Context) error {
	return nil
}

func (n *NetworkTester) Stopped() bool {
	return true
}

func New(ctx context.Context, log log.Logger) (*NetworkTester, error) {
	l1, err := wallet.NewNetwork(ctx, log, "http://127.0.0.1:60370", "kurtosis-l1")
	if err != nil {
		log.Error("error creating l1 network", "err", err)
		return nil, err
	}

	l2A, err := wallet.NewNetwork(ctx, log, "http://127.0.0.1:60403", "kurtosis-1")
	if err != nil {
		log.Error("error creating l2a network", "err", err)
		return nil, err
	}

	l2B, err := wallet.NewNetwork(ctx, log, "http://127.0.0.1:60418", "kurtosis-2")
	if err != nil {
		log.Error("error creating l2b network", "err", err)
		return nil, err
	}

	return &NetworkTester{
		log: log,
		ctx: ctx,
		L1:  *l1,
		L2A: *l2A,
		L2B: *l2B,
	}, nil
}
