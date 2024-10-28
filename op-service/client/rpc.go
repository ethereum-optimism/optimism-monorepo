package client

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/retry"
)

var httpRegex = regexp.MustCompile("^http(s)?://")

type RPC interface {
	Close()
	CallContext(ctx context.Context, result any, method string, args ...any) error
	BatchCallContext(ctx context.Context, b []rpc.BatchElem) error
	EthSubscribe(ctx context.Context, channel any, args ...any) (ethereum.Subscription, error)
}

type rpcConfig struct {
	gethRPCOptions   []rpc.ClientOption
	httpPollInterval time.Duration
	backoffAttempts  int
	limit            float64
	burst            int
	lazy             bool
	callTimeout      time.Duration
	batchCallTimeout time.Duration
	fixedDialBackoff time.Duration
}

type RPCOption func(cfg *rpcConfig)

func WithCallTimeout(d time.Duration) RPCOption {
	return func(cfg *rpcConfig) {
		cfg.callTimeout = d
	}
}

func WithBatchCallTimeout(d time.Duration) RPCOption {
	return func(cfg *rpcConfig) {
		cfg.batchCallTimeout = d
	}
}

// WithDialAttempts configures the number of attempts for the initial dial to the RPC,
// attempts are executed with an exponential backoff strategy by default.
func WithDialAttempts(attempts int) RPCOption {
	return func(cfg *rpcConfig) {
		cfg.backoffAttempts = attempts
	}
}

// WithFixedDialBackoff makes the RPC client use a fixed delay between dial attempts of 2 seconds instead of exponential
func WithFixedDialBackoff(d time.Duration) RPCOption {
	return func(cfg *rpcConfig) {
		cfg.fixedDialBackoff = d
	}
}

// WithHttpPollInterval configures the RPC to poll at the given rate, in case RPC subscriptions are not available.
func WithHttpPollInterval(duration time.Duration) RPCOption {
	return func(cfg *rpcConfig) {
		cfg.httpPollInterval = duration
	}
}

// WithGethRPCOptions passes the list of go-ethereum RPC options to the internal RPC instance.
func WithGethRPCOptions(gethRPCOptions ...rpc.ClientOption) RPCOption {
	return func(cfg *rpcConfig) {
		cfg.gethRPCOptions = append(cfg.gethRPCOptions, gethRPCOptions...)
	}
}

// WithRateLimit configures the RPC to target the given rate limit (in requests / second).
// See NewRateLimitingClient for more details.
func WithRateLimit(rateLimit float64, burst int) RPCOption {
	return func(cfg *rpcConfig) {
		cfg.limit = rateLimit
		cfg.burst = burst
	}
}

// WithLazyDial makes the RPC client initialization defer the initial connection attempt,
// and defer to later RPC requests upon subsequent dial errors.
// Any dial-backoff option will be ignored if this option is used.
func WithLazyDial() RPCOption {
	return func(cfg *rpcConfig) {
		cfg.lazy = true
	}
}

// NewRPC returns the correct client.RPC instance for a given RPC url.
func NewRPC(ctx context.Context, lgr log.Logger, addr string, opts ...RPCOption) (RPC, error) {
	cfg := applyOptions(opts)

	var wrapped RPC
	if cfg.lazy {
		wrapped = newLazyRPC(addr, cfg)
	} else {
		underlying, err := dialRPCClientWithBackoff(ctx, lgr, addr, cfg)
		if err != nil {
			return nil, err
		}
		wrapped = wrapClient(underlying, cfg)
	}

	return NewRPCWithClient(ctx, lgr, addr, wrapped, cfg.httpPollInterval)
}

func applyOptions(opts []RPCOption) rpcConfig {
	var cfg rpcConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.backoffAttempts < 1 { // default to at least 1 attempt, or it always fails to dial.
		cfg.backoffAttempts = 1
	}
	if cfg.callTimeout == 0 {
		cfg.callTimeout = 10 * time.Second
	}
	if cfg.batchCallTimeout == 0 {
		cfg.batchCallTimeout = 20 * time.Second
	}
	return cfg
}

// NewRPCWithClient builds a new polling client with the given underlying RPC client.
func NewRPCWithClient(ctx context.Context, lgr log.Logger, addr string, underlying RPC, pollInterval time.Duration) (RPC, error) {
	if httpRegex.MatchString(addr) {
		underlying = NewPollingClient(ctx, lgr, underlying, WithPollRate(pollInterval))
	}
	return underlying, nil
}

// Dials a JSON-RPC endpoint repeatedly, with a backoff, until a client connection is established. Auth is optional.
func dialRPCClientWithBackoff(ctx context.Context, log log.Logger, addr string, cfg rpcConfig) (*rpc.Client, error) {
	bOff := retry.Exponential()
	if cfg.fixedDialBackoff != 0 {
		bOff = retry.Fixed(cfg.fixedDialBackoff)
	}
	return retry.Do(ctx, cfg.backoffAttempts, bOff, func() (*rpc.Client, error) {
		if !IsURLAvailable(ctx, addr) {
			log.Warn("failed to dial address, but may connect later", "addr", addr)
			return nil, fmt.Errorf("address unavailable (%s)", addr)
		}
		client, err := rpc.DialOptions(ctx, addr, cfg.gethRPCOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to dial address (%s): %w", addr, err)
		}
		return client, nil
	})
}

func IsURLAvailable(ctx context.Context, address string) bool {
	u, err := url.Parse(address)
	if err != nil {
		return false
	}
	addr := u.Host
	if u.Port() == "" {
		switch u.Scheme {
		case "http", "ws":
			addr += ":80"
		case "https", "wss":
			addr += ":443"
		default:
			// Fail open if we can't figure out what the port should be
			return true
		}
	}
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// BaseRPCClient is a wrapper around a concrete *rpc.Client instance to make it compliant
// with the client.RPC interface.
// It sets a default timeout of 10s on CallContext & 20s on BatchCallContext made through it.
type BaseRPCClient struct {
	c                *rpc.Client
	batchCallTimeout time.Duration
	callTimeout      time.Duration
}

func NewBaseRPCClient(c *rpc.Client, opts ...RPCOption) RPC {
	cfg := applyOptions(opts)
	return wrapClient(c, cfg)
}

func wrapClient(c *rpc.Client, cfg rpcConfig) RPC {
	var wrapped RPC
	wrapped = &BaseRPCClient{c: c, callTimeout: cfg.callTimeout, batchCallTimeout: cfg.batchCallTimeout}

	if cfg.limit != 0 {
		wrapped = NewRateLimitingClient(wrapped, rate.Limit(cfg.limit), cfg.burst)
	}
	return wrapped
}

func (b *BaseRPCClient) Close() {
	b.c.Close()
}

func (b *BaseRPCClient) CallContext(ctx context.Context, result any, method string, args ...any) error {
	cCtx, cancel := context.WithTimeout(ctx, b.callTimeout)
	defer cancel()
	return b.c.CallContext(cCtx, result, method, args...)
}

func (b *BaseRPCClient) BatchCallContext(ctx context.Context, batch []rpc.BatchElem) error {
	cCtx, cancel := context.WithTimeout(ctx, b.batchCallTimeout)
	defer cancel()
	return b.c.BatchCallContext(cCtx, batch)
}

func (b *BaseRPCClient) EthSubscribe(ctx context.Context, channel any, args ...any) (ethereum.Subscription, error) {
	return b.c.EthSubscribe(ctx, channel, args...)
}

// InstrumentedRPCClient is an RPC client that tracks
// Prometheus metrics for each call.
type InstrumentedRPCClient struct {
	c RPC
	m *metrics.RPCClientMetrics
}

// NewInstrumentedRPC creates a new instrumented RPC client.
func NewInstrumentedRPC(c RPC, m *metrics.RPCClientMetrics) *InstrumentedRPCClient {
	return &InstrumentedRPCClient{
		c: c,
		m: m,
	}
}

func (ic *InstrumentedRPCClient) Close() {
	ic.c.Close()
}

func (ic *InstrumentedRPCClient) CallContext(ctx context.Context, result any, method string, args ...any) error {
	return instrument1(ic.m, method, func() error {
		return ic.c.CallContext(ctx, result, method, args...)
	})
}

func (ic *InstrumentedRPCClient) BatchCallContext(ctx context.Context, b []rpc.BatchElem) error {
	return instrumentBatch(ic.m, func() error {
		return ic.c.BatchCallContext(ctx, b)
	}, b)
}

func (ic *InstrumentedRPCClient) EthSubscribe(ctx context.Context, channel any, args ...any) (ethereum.Subscription, error) {
	return ic.c.EthSubscribe(ctx, channel, args...)
}

// instrumentBatch handles metrics for batch calls. Request metrics are
// increased for each batch element. Request durations are tracked for
// the batch as a whole using a special <batch> method. Errors are tracked
// for each individual batch response, unless the overall request fails in
// which case the <batch> method is used.
func instrumentBatch(m *metrics.RPCClientMetrics, cb func() error, b []rpc.BatchElem) error {
	m.RPCClientRequestsTotal.WithLabelValues(metrics.BatchMethod).Inc()
	for _, elem := range b {
		m.RPCClientRequestsTotal.WithLabelValues(elem.Method).Inc()
	}
	timer := prometheus.NewTimer(m.RPCClientRequestDurationSeconds.WithLabelValues(metrics.BatchMethod))
	defer timer.ObserveDuration()

	// Track response times for batch requests separately.
	if err := cb(); err != nil {
		m.RecordRPCClientResponse(metrics.BatchMethod, err)
		return err
	}
	for _, elem := range b {
		m.RecordRPCClientResponse(elem.Method, elem.Error)
	}
	return nil
}
