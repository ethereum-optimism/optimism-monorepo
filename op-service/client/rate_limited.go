package client

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/time/rate"
)

// RateLimitingClient is a wrapper around a pure RPC that implements a global rate-limit on requests.
type RateLimitingClient struct {
	c  RPC
	rl *rate.Limiter
}

// NewRateLimitingClient implements a global rate-limit for all RPC requests.
// A limit of N will ensure that over a long enough time-frame the given number of tokens per second is targeted.
// Burst limits how far off we can be from the target, by specifying how many requests are allowed at once.
func NewRateLimitingClient(c RPC, limit rate.Limit, burst int) *RateLimitingClient {
	return &RateLimitingClient{c: c, rl: rate.NewLimiter(limit, burst)}
}

func (b *RateLimitingClient) Close() {
	b.c.Close()
}

func (b *RateLimitingClient) CallContext(ctx context.Context, result any, method string, args ...any) error {
	if err := b.rl.Wait(ctx); err != nil {
		return err
	}
	cCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return b.c.CallContext(cCtx, result, method, args...)
}

func (b *RateLimitingClient) BatchCallContext(ctx context.Context, batch []rpc.BatchElem) error {
	if err := b.rl.WaitN(ctx, len(batch)); err != nil {
		return err
	}
	cCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return b.c.BatchCallContext(cCtx, batch)
}

func (b *RateLimitingClient) Subscribe(ctx context.Context, namespace string, channel any, args ...any) (ethereum.Subscription, error) {
	if err := b.rl.Wait(ctx); err != nil {
		return nil, err
	}
	return b.c.Subscribe(ctx, namespace, channel, args...)
}
