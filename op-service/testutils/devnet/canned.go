package devnet

import (
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/log"
)

type CleanupFunc func() error

func NewForked(lgr log.Logger, rpcURL string, anvilOpts ...AnvilOption) (*Anvil, CleanupFunc, error) {
	retryProxy := NewRetryProxy(lgr, rpcURL)
	if err := retryProxy.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start retry proxy: %w", err)
	}

	anvil, err := NewAnvil(lgr, append([]AnvilOption{WithForkURL(retryProxy.Endpoint()), WithBlockTime(3)}, anvilOpts...)...)
	if err != nil {
		_ = retryProxy.Stop()
		return nil, nil, fmt.Errorf("failed to create Anvil: %w", err)
	}

	if err := anvil.Start(); err != nil {
		_ = retryProxy.Stop()
		return nil, nil, fmt.Errorf("failed to start Anvil: %w", err)
	}

	cleanup := func() error {
		if err := anvil.Stop(); err != nil {
			return fmt.Errorf("failed to stop Anvil: %w", err)
		}
		if err := retryProxy.Stop(); err != nil {
			return fmt.Errorf("failed to stop retry proxy: %w", err)
		}
		return nil
	}

	return anvil, cleanup, nil
}

func NewForkedSepolia(lgr log.Logger) (*Anvil, CleanupFunc, error) {
	url := os.Getenv("SEPOLIA_RPC_URL")
	if url == "" {
		return nil, nil, fmt.Errorf("SEPOLIA_RPC_URL not set")
	}
	return NewForked(lgr, url)
}
