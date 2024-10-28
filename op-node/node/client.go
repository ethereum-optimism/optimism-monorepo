package node

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/sources"

	"github.com/ethereum/go-ethereum/log"
	gn "github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
)

type L2EndpointSetup interface {
	// Setup a RPC client to a L2 execution engine to process rollup blocks with.
	Setup(ctx context.Context, log log.Logger, rollupCfg *rollup.Config) (cl client.RPC, rpcCfg *sources.EngineClientConfig, err error)
	Check() error
}

type L1EndpointSetup interface {
	// Setup a RPC client to a L1 node to pull rollup input-data from.
	// The results of the RPC client may be trusted for faster processing, or strictly validated.
	// The kind of the RPC may be non-basic, to optimize RPC usage.
	Setup(ctx context.Context, log log.Logger, rollupCfg *rollup.Config) (cl client.RPC, rpcCfg *sources.L1ClientConfig, err error)
	Check() error
}

type L1BeaconEndpointSetup interface {
	Setup(ctx context.Context, log log.Logger) (cl sources.BeaconClient, fb []sources.BlobSideCarsFetcher, err error)
	// ShouldIgnoreBeaconCheck returns true if the Beacon-node version check should not halt startup.
	ShouldIgnoreBeaconCheck() bool
	ShouldFetchAllSidecars() bool
	Check() error
}

type L2EndpointConfig struct {
	// L2EngineAddr is the address of the L2 Engine JSON-RPC endpoint to use. The engine and eth
	// namespaces must be enabled by the endpoint.
	L2EngineAddr string

	// JWT secrets for L2 Engine API authentication during HTTP or initial Websocket communication.
	// Any value for an IPC connection.
	L2EngineJWTSecret [32]byte
}

var _ L2EndpointSetup = (*L2EndpointConfig)(nil)

func (cfg *L2EndpointConfig) Check() error {
	if cfg.L2EngineAddr == "" {
		return errors.New("empty L2 Engine Address")
	}

	return nil
}

func (cfg *L2EndpointConfig) Setup(ctx context.Context, log log.Logger, rollupCfg *rollup.Config) (client.RPC, *sources.EngineClientConfig, error) {
	if err := cfg.Check(); err != nil {
		return nil, nil, err
	}
	auth := rpc.WithHTTPAuth(gn.NewJWTAuth(cfg.L2EngineJWTSecret))
	opts := []client.RPCOption{
		client.WithGethRPCOptions(auth),
		client.WithDialAttempts(10),
	}
	l2Node, err := client.NewRPC(ctx, log, cfg.L2EngineAddr, opts...)
	if err != nil {
		return nil, nil, err
	}

	return l2Node, sources.EngineClientDefaultConfig(rollupCfg), nil
}

// PreparedL2Endpoints enables testing with in-process pre-setup RPC connections to L2 engines
type PreparedL2Endpoints struct {
	Client client.RPC
}

func (p *PreparedL2Endpoints) Check() error {
	if p.Client == nil {
		return errors.New("client cannot be nil")
	}
	return nil
}

var _ L2EndpointSetup = (*PreparedL2Endpoints)(nil)

func (p *PreparedL2Endpoints) Setup(ctx context.Context, log log.Logger, rollupCfg *rollup.Config) (client.RPC, *sources.EngineClientConfig, error) {
	return p.Client, sources.EngineClientDefaultConfig(rollupCfg), nil
}

type L1EndpointConfig struct {
	L1NodeAddr string // Address of L1 User JSON-RPC endpoint to use (eth namespace required)

	// L1TrustRPC: if we trust the L1 RPC we do not have to validate L1 response contents like headers
	// against block hashes, or cached transaction sender addresses.
	// Thus we can sync faster at the risk of the source RPC being wrong.
	L1TrustRPC bool

	// L1RPCKind identifies the RPC provider kind that serves the RPC,
	// to inform the optimal usage of the RPC for transaction receipts fetching.
	L1RPCKind sources.RPCProviderKind

	// RateLimit specifies a self-imposed rate-limit on L1 requests. 0 is no rate-limit.
	RateLimit float64

	// BatchSize specifies the maximum batch-size, which also applies as L1 rate-limit burst amount (if set).
	BatchSize int

	// MaxConcurrency specifies the maximum number of concurrent requests to the L1 RPC.
	MaxConcurrency int

	// HttpPollInterval specifies the interval between polling for the latest L1 block,
	// when the RPC is detected to be an HTTP type.
	// It is recommended to use websockets or IPC for efficient following of the changing block.
	// Setting this to 0 disables polling.
	HttpPollInterval time.Duration
}

var _ L1EndpointSetup = (*L1EndpointConfig)(nil)

func (cfg *L1EndpointConfig) Check() error {
	if cfg.BatchSize < 1 || cfg.BatchSize > 500 {
		return fmt.Errorf("batch size is invalid or unreasonable: %d", cfg.BatchSize)
	}
	if cfg.RateLimit < 0 {
		return fmt.Errorf("rate limit cannot be negative")
	}
	if cfg.MaxConcurrency < 1 {
		return fmt.Errorf("max concurrent requests cannot be less than 1, was %d", cfg.MaxConcurrency)
	}
	return nil
}

func (cfg *L1EndpointConfig) Setup(ctx context.Context, log log.Logger, rollupCfg *rollup.Config) (client.RPC, *sources.L1ClientConfig, error) {
	opts := []client.RPCOption{
		client.WithHttpPollInterval(cfg.HttpPollInterval),
		client.WithDialAttempts(10),
	}
	if cfg.RateLimit != 0 {
		opts = append(opts, client.WithRateLimit(cfg.RateLimit, cfg.BatchSize))
	}

	l1Node, err := client.NewRPC(ctx, log, cfg.L1NodeAddr, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to dial L1 address (%s): %w", cfg.L1NodeAddr, err)
	}
	rpcCfg := sources.L1ClientDefaultConfig(rollupCfg, cfg.L1TrustRPC, cfg.L1RPCKind)
	rpcCfg.MaxRequestsPerBatch = cfg.BatchSize
	rpcCfg.MaxConcurrentRequests = cfg.MaxConcurrency
	return l1Node, rpcCfg, nil
}

// PreparedL1Endpoint enables testing with an in-process pre-setup RPC connection to L1
type PreparedL1Endpoint struct {
	Client          client.RPC
	TrustRPC        bool
	RPCProviderKind sources.RPCProviderKind
}

var _ L1EndpointSetup = (*PreparedL1Endpoint)(nil)

func (p *PreparedL1Endpoint) Setup(ctx context.Context, log log.Logger, rollupCfg *rollup.Config) (client.RPC, *sources.L1ClientConfig, error) {
	return p.Client, sources.L1ClientDefaultConfig(rollupCfg, p.TrustRPC, p.RPCProviderKind), nil
}

func (cfg *PreparedL1Endpoint) Check() error {
	if cfg.Client == nil {
		return errors.New("rpc client cannot be nil")
	}

	return nil
}

type L1BeaconEndpointConfig struct {
	BeaconAddr             string   // Address of L1 User Beacon-API endpoint to use (beacon namespace required)
	BeaconHeader           string   // Optional HTTP header for all requests to L1 Beacon
	BeaconFallbackAddrs    []string // Addresses of L1 Beacon-API fallback endpoints (only for blob sidecars retrieval)
	BeaconCheckIgnore      bool     // When false, halt startup if the beacon version endpoint fails
	BeaconFetchAllSidecars bool     // Whether to fetch all blob sidecars and filter locally
}

var _ L1BeaconEndpointSetup = (*L1BeaconEndpointConfig)(nil)

func (cfg *L1BeaconEndpointConfig) Setup(ctx context.Context, log log.Logger) (cl sources.BeaconClient, fb []sources.BlobSideCarsFetcher, err error) {
	var opts []client.BasicHTTPClientOption
	if cfg.BeaconHeader != "" {
		hdr, err := parseHTTPHeader(cfg.BeaconHeader)
		if err != nil {
			return nil, nil, fmt.Errorf("parsing beacon header: %w", err)
		}
		opts = append(opts, client.WithHeader(hdr))
	}

	for _, addr := range cfg.BeaconFallbackAddrs {
		b := client.NewBasicHTTPClient(addr, log)
		fb = append(fb, sources.NewBeaconHTTPClient(b))
	}

	a := client.NewBasicHTTPClient(cfg.BeaconAddr, log, opts...)
	return sources.NewBeaconHTTPClient(a), fb, nil
}

func (cfg *L1BeaconEndpointConfig) Check() error {
	if cfg.BeaconAddr == "" && !cfg.BeaconCheckIgnore {
		return errors.New("expected L1 Beacon API endpoint, but got none")
	}
	return nil
}

func (cfg *L1BeaconEndpointConfig) ShouldIgnoreBeaconCheck() bool {
	return cfg.BeaconCheckIgnore
}

func (cfg *L1BeaconEndpointConfig) ShouldFetchAllSidecars() bool {
	return cfg.BeaconFetchAllSidecars
}

func parseHTTPHeader(headerStr string) (http.Header, error) {
	h := make(http.Header, 1)
	s := strings.SplitN(headerStr, ": ", 2)
	if len(s) != 2 {
		return nil, errors.New("invalid header format")
	}
	h.Add(s[0], s[1])
	return h, nil
}

type SupervisorEndpointSetup interface {
	SupervisorClient(ctx context.Context, log log.Logger) (*sources.SupervisorClient, error)
	Check() error
}

type SupervisorEndpointConfig struct {
	SupervisorAddr string
}

var _ SupervisorEndpointSetup = (*SupervisorEndpointConfig)(nil)

func (cfg *SupervisorEndpointConfig) Check() error {
	if cfg.SupervisorAddr == "" {
		return errors.New("supervisor RPC address is not set")
	}
	return nil
}

func (cfg *SupervisorEndpointConfig) SupervisorClient(ctx context.Context, log log.Logger) (*sources.SupervisorClient, error) {
	cl, err := client.NewRPC(ctx, log, cfg.SupervisorAddr, client.WithLazyDial())
	if err != nil {
		return nil, fmt.Errorf("failed to create supervisor RPC: %w", err)
	}
	return sources.NewSupervisorClient(cl), nil
}
