package dial

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum/go-ethereum/log"
)

type rollupDialer func(ctx context.Context, log log.Logger, url string) (RollupClientInterface, error)

// ActiveL2EndpointProvider is an interface for providing a RollupClient
// It manages the lifecycle of the RollupClient for callers
// It does this by failing over down the list of rollupUrls if the current one is inactive or broken
type ActiveL2RollupProvider struct {
	checkDuration  time.Duration
	networkTimeout time.Duration
	log            log.Logger

	activeTimeout time.Time

	rollupUrls          []string
	rollupDialer        rollupDialer
	currentRollupClient RollupClientInterface
	rollupIndex         int
	clientLock          *sync.Mutex

	// callback function to be called when the active provider changes
	onActiveProviderChanged func()
}

// SetOnActiveProviderChanged sets the callback function to be called when the active provider changes
func (a *ActiveL2RollupProvider) SetOnActiveProviderChanged(onActiveProviderChanged func()) {
	a.onActiveProviderChanged = onActiveProviderChanged
}

// NewActiveL2RollupProvider creates a new ActiveL2RollupProvider
// the checkDuration is the duration between checks to see if the current rollup client is active
// provide a checkDuration of 0 to check every time
func NewActiveL2RollupProvider(
	ctx context.Context,
	rollupUrls []string,
	checkDuration time.Duration,
	networkTimeout time.Duration,
	logger log.Logger,
) (*ActiveL2RollupProvider, error) {
	rollupDialer := func(ctx context.Context, log log.Logger, url string,
	) (RollupClientInterface, error) {
		rpcCl, err := dialRPCClient(ctx, log, url)
		if err != nil {
			return nil, err
		}

		return sources.NewRollupClient(client.NewBaseRPCClient(rpcCl)), nil
	}
	return newActiveL2RollupProvider(ctx, rollupUrls, checkDuration, networkTimeout, logger, rollupDialer)
}

func newActiveL2RollupProvider(
	ctx context.Context,
	rollupUrls []string,
	checkDuration time.Duration,
	networkTimeout time.Duration,
	logger log.Logger,
	dialer rollupDialer,
) (*ActiveL2RollupProvider, error) {
	if len(rollupUrls) == 0 {
		return nil, errors.New("empty rollup urls list")
	}
	p := &ActiveL2RollupProvider{
		checkDuration:  checkDuration,
		networkTimeout: networkTimeout,
		log:            logger,
		rollupUrls:     rollupUrls,
		rollupDialer:   dialer,
		clientLock:     &sync.Mutex{},
	}
	cctx, cancel := context.WithTimeout(ctx, networkTimeout)
	defer cancel()

	if _, err := p.RollupClient(cctx); err != nil {
		return nil, fmt.Errorf("setting provider rollup client: %w", err)
	}

	return p, nil
}

func (p *ActiveL2RollupProvider) RollupClient(ctx context.Context) (RollupClientInterface, error) {
	p.clientLock.Lock()
	defer p.clientLock.Unlock()
	err := p.ensureActiveEndpoint(ctx)
	if err != nil {
		return nil, err
	}
	return p.currentRollupClient, nil
}

func (p *ActiveL2RollupProvider) ensureActiveEndpoint(ctx context.Context) error {
	if !p.shouldCheck() {
		return nil
	}
	if err := p.findActiveEndpoints(ctx); err != nil {
		return err
	}
	p.activeTimeout = time.Now().Add(p.checkDuration)
	return nil
}

func (p *ActiveL2RollupProvider) shouldCheck() bool {
	return time.Now().After(p.activeTimeout)
}

func (p *ActiveL2RollupProvider) findActiveEndpoints(ctx context.Context) error {
	startIdx := p.rollupIndex
	var errs error
	for offset := range p.rollupUrls {
		idx := (startIdx + offset) % p.numEndpoints()
		if offset != 0 || p.currentRollupClient == nil {
			if err := p.dialSequencer(ctx, idx); err != nil {
				errs = errors.Join(errs, err)
				p.log.Warn("Error dialing next sequencer.", "err", err, "index", idx)
				continue
			}
		}

		ep := p.rollupUrls[idx]
		if active, err := p.checkCurrentSequencer(ctx); err != nil {
			errs = errors.Join(errs, err)
			p.log.Warn("Error querying active sequencer, trying next.", "err", err, "index", idx, "url", ep)
		} else if active {
			if offset == 0 {
				p.log.Debug("Current sequencer active.", "index", idx, "url", ep)
			} else {
				p.log.Info("Found new active sequencer.", "index", idx, "url", ep)
				if p.onActiveProviderChanged != nil {
					p.onActiveProviderChanged()
				}
			}
			return nil
		} else {
			p.log.Info("Sequencer inactive, trying next.", "index", idx, "url", ep)
		}
	}
	return fmt.Errorf("failed to find an active sequencer, tried following urls: %v; errs: %w", p.rollupUrls, errs)
}

func (p *ActiveL2RollupProvider) checkCurrentSequencer(ctx context.Context) (bool, error) {
	cctx, cancel := context.WithTimeout(ctx, p.networkTimeout)
	defer cancel()
	return p.currentRollupClient.SequencerActive(cctx)
}

func (p *ActiveL2RollupProvider) numEndpoints() int {
	return len(p.rollupUrls)
}

// dialSequencer dials the sequencer for the url at the given index.
// If successful, the currentRollupClient and rollupIndex are updated and the
// old rollup client is closed.
func (p *ActiveL2RollupProvider) dialSequencer(ctx context.Context, idx int) error {
	cctx, cancel := context.WithTimeout(ctx, p.networkTimeout)
	defer cancel()

	ep := p.rollupUrls[idx]
	p.log.Info("Dialing next sequencer.", "index", idx, "url", ep)
	rollupClient, err := p.rollupDialer(cctx, p.log, ep)
	if err != nil {
		return fmt.Errorf("dialing rollup client: %w", err)
	}
	if p.currentRollupClient != nil {
		p.currentRollupClient.Close()
	}
	p.rollupIndex = idx
	p.currentRollupClient = rollupClient
	return nil
}

func (p *ActiveL2RollupProvider) Close() {
	if p.currentRollupClient != nil {
		p.currentRollupClient.Close()
	}
}
