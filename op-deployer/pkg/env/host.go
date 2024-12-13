package env

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script/forking"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/broadcaster"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

func DefaultScriptHost(
	bcaster broadcaster.Broadcaster,
	lgr log.Logger,
	deployer common.Address,
	artifacts foundry.StatDirFs,
	additionalOpts ...script.HostOption,
) (*script.Host, error) {
	scriptCtx := script.DefaultContext
	scriptCtx.Sender = deployer
	scriptCtx.Origin = deployer
	h := script.NewHost(
		lgr,
		&foundry.ArtifactsFS{FS: artifacts},
		nil,
		scriptCtx,
		append([]script.HostOption{
			script.WithBroadcastHook(bcaster.Hook),
			script.WithIsolatedBroadcasts(),
			script.WithCreate2Deployer(),
		}, additionalOpts...)...,
	)

	if err := h.EnableCheats(); err != nil {
		return nil, fmt.Errorf("failed to enable cheats: %w", err)
	}

	return h, nil
}

func DefaultForkedScriptHost(
	ctx context.Context,
	bcaster broadcaster.Broadcaster,
	lgr log.Logger,
	deployer common.Address,
	artifacts foundry.StatDirFs,
	forkRPC *rpc.Client,
	additionalOpts ...script.HostOption,
) (*script.Host, error) {
	h, err := DefaultScriptHost(
		bcaster,
		lgr,
		deployer,
		artifacts,
		append([]script.HostOption{
			script.WithForkHook(func(cfg *script.ForkConfig) (forking.ForkSource, error) {
				src, err := forking.RPCSourceByNumber(cfg.URLOrAlias, forkRPC, *cfg.BlockNumber)
				if err != nil {
					return nil, fmt.Errorf("failed to create RPC fork source: %w", err)
				}
				return forking.Cache(src), nil
			}),
		}, additionalOpts...)...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create default script host: %w", err)
	}

	client := ethclient.NewClient(forkRPC)

	latest, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest block: %w", err)
	}

	if _, err := h.CreateSelectFork(
		script.ForkWithURLOrAlias("main"),
		script.ForkWithBlockNumberU256(latest.Number),
	); err != nil {
		return nil, fmt.Errorf("failed to select fork: %w", err)
	}

	return h, nil
}
