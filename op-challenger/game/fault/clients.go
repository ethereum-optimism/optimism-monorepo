package fault

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-challenger/config"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/super"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
)

type clientProvider struct {
	ctx            context.Context
	logger         log.Logger
	cfg            *config.Config
	l2HeaderSource utils.L2HeaderSource
	rollupClient   RollupClient
	syncValidator  *syncStatusValidator
	rootProvider   super.RootProvider
	toClose        []CloseFunc
}

func (c *clientProvider) Close() {
	for _, closeFunc := range c.toClose {
		closeFunc()
	}
}

func (c *clientProvider) SingleChainClients() (utils.L2HeaderSource, RollupClient, *syncStatusValidator, error) {
	headers, err := c.L2HeaderSource()
	if err != nil {
		return nil, nil, nil, err
	}
	rollup, err := c.RollupClient()
	if err != nil {
		return nil, nil, nil, err
	}
	return headers, rollup, c.syncValidator, nil
}

func (c *clientProvider) L2HeaderSource() (utils.L2HeaderSource, error) {
	if c.l2HeaderSource != nil {
		return c.l2HeaderSource, nil
	}
	if len(c.cfg.L2Rpcs) != 1 {
		return nil, fmt.Errorf("incorrect number of L2 RPCs configured, expected 1 but got %d", len(c.cfg.L2Rpcs))
	}

	l2Client, err := ethclient.DialContext(c.ctx, c.cfg.L2Rpcs[0])
	if err != nil {
		return nil, fmt.Errorf("dial l2 client %v: %w", c.cfg.L2Rpcs[0], err)
	}
	c.l2HeaderSource = l2Client
	c.toClose = append(c.toClose, l2Client.Close)
	return l2Client, nil
}

func (c *clientProvider) RollupClient() (RollupClient, error) {
	if c.rollupClient != nil {
		return c.rollupClient, nil
	}
	rollupClient, err := dial.DialRollupClientWithTimeout(c.ctx, dial.DefaultDialTimeout, c.logger, c.cfg.RollupRpc)
	if err != nil {
		return nil, fmt.Errorf("dial rollup client %v: %w", c.cfg.RollupRpc, err)
	}
	c.rollupClient = rollupClient
	c.syncValidator = newSyncStatusValidator(rollupClient)
	c.toClose = append(c.toClose, rollupClient.Close)
	return rollupClient, nil
}

func (c *clientProvider) SuperchainClients() (super.RootProvider, *super.SyncValidator, error) {
	cl, err := client.NewRPC(context.Background(), c.logger, c.cfg.SupervisorRPC)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to dial supervisor: %w", err)
	}
	supervisorClient := sources.NewSupervisorClient(cl)
	c.rootProvider = supervisorClient
	c.toClose = append(c.toClose, supervisorClient.Close)
	return supervisorClient, super.NewSyncValidator(supervisorClient), nil
}
