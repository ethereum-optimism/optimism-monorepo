package dial

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
)

const DefaultActiveSequencerFollowerCheckDuration = 2 * DefaultDialTimeout

type ethDialer func(ctx context.Context, log log.Logger, url string) (EthClientInterface, error)

// ActiveL2EndpointProvider is an interface for providing a RollupClient and l2 eth client
// It manages the lifecycle of the RollupClient and eth client for callers
// It does this by failing over down the list of rollupUrls if the current one is inactive or broken
type ActiveL2EndpointProvider struct {
	ActiveL2RollupProvider
	currentEthClient EthClientInterface
	ethClientIndex   int
	ethDialer        ethDialer
	ethUrls          []string
}

// NewActiveL2EndpointProvider creates a new ActiveL2EndpointProvider
// the checkDuration is the duration between checks to see if the current rollup client is active
// provide a checkDuration of 0 to check every time
func NewActiveL2EndpointProvider(ctx context.Context,
	ethUrls, rollupUrls []string,
	checkDuration time.Duration,
	networkTimeout time.Duration,
	logger log.Logger,
) (*ActiveL2EndpointProvider, error) {
	ethDialer := func(ctx context.Context, log log.Logger, url string) (EthClientInterface, error) {
		rpcCl, err := dialRPCClient(ctx, log, url)
		if err != nil {
			return nil, err
		}

		return ethclient.NewClient(rpcCl), nil
	}
	rollupDialer := func(ctx context.Context, log log.Logger, url string) (RollupClientInterface, error) {
		rpcCl, err := dialRPCClient(ctx, log, url)
		if err != nil {
			return nil, err
		}

		return sources.NewRollupClient(client.NewBaseRPCClient(rpcCl)), nil
	}
	return newActiveL2EndpointProvider(ctx, ethUrls, rollupUrls, checkDuration, networkTimeout, logger, ethDialer, rollupDialer)
}

func newActiveL2EndpointProvider(
	ctx context.Context,
	ethUrls, rollupUrls []string,
	checkDuration time.Duration,
	networkTimeout time.Duration,
	logger log.Logger,
	ethDialer ethDialer,
	rollupDialer rollupDialer,
) (*ActiveL2EndpointProvider, error) {
	if len(rollupUrls) == 0 {
		return nil, errors.New("empty rollup urls list, expected at least one URL")
	}
	if len(ethUrls) != len(rollupUrls) {
		return nil, fmt.Errorf("number of eth urls (%d) and rollup urls (%d) mismatch", len(ethUrls), len(rollupUrls))
	}

	rollupProvider, err := newActiveL2RollupProvider(ctx, rollupUrls, checkDuration, networkTimeout, logger, rollupDialer)
	if err != nil {
		return nil, err
	}
	p := &ActiveL2EndpointProvider{
		ActiveL2RollupProvider: *rollupProvider,
		ethDialer:              ethDialer,
		ethUrls:                ethUrls,
	}
	cctx, cancel := context.WithTimeout(ctx, networkTimeout)
	defer cancel()
	if _, err = p.EthClient(cctx); err != nil {
		return nil, fmt.Errorf("setting provider eth client: %w", err)
	}
	return p, nil
}

func (p *ActiveL2EndpointProvider) EthClient(ctx context.Context) (EthClientInterface, error) {
	p.clientLock.Lock()
	defer p.clientLock.Unlock()
	err := p.ensureActiveEndpoint(ctx)
	if err != nil {
		return nil, err
	}
	if p.ethClientIndex != p.rollupIndex || p.currentEthClient == nil {
		// we changed sequencers, dial a new EthClient
		cctx, cancel := context.WithTimeout(ctx, p.networkTimeout)
		defer cancel()
		idx := p.rollupIndex
		ep := p.ethUrls[idx]
		log.Info("sequencer changed (or ethClient was nil due to startup), dialing new eth client", "new_index", idx, "new_url", ep)
		ethClient, err := p.ethDialer(cctx, p.log, ep)
		if err != nil {
			return nil, fmt.Errorf("dialing eth client: %w", err)
		}
		if p.currentEthClient != nil {
			p.currentEthClient.Close()
		}
		p.ethClientIndex = idx
		p.currentEthClient = ethClient
	}
	return p.currentEthClient, nil
}

func (p *ActiveL2EndpointProvider) Close() {
	if p.currentEthClient != nil {
		p.currentEthClient.Close()
	}
	p.ActiveL2RollupProvider.Close()
}
