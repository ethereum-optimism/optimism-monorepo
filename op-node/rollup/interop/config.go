package interop

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-service/sources"
)

type Config struct {
	// SupervisorAddr to follow for cross-chain safety updates.
	// Non-empty if running in follow-mode.
	// Cannot be set if RPCAddr is set.
	SupervisorAddr string

	// RPCAddr address to bind RPC server to, to serve external supervisor nodes.
	// Cannot be set if SupervisorAddr is set.
	RPCAddr string
	// RPCPort port to bind RPC server to, to serve external supervisor nodes.
	// Binds to any available port if set to 0.
	// Only applicable if RPCAddr is set.
	RPCPort int
	// RPCJwtSecretPath path of JWT secret file to apply authentication to the interop server address.
	RPCJwtSecretPath string
}

func (cfg *Config) Check() error {
	// TODO(#13338): temporary workaround needs both to be configured.
	//if (cfg.SupervisorAddr == "") != (cfg.RPCAddr == "") {
	//	return errors.New("must have either a supervisor RPC endpoint to follow, or interop RPC address to serve from")
	//}
	return nil
}

func (cfg *Config) Setup(ctx context.Context, logger log.Logger) (SubSystem, error) {
	if cfg.RPCAddr != "" {
		logger.Info("Setting up Interop RPC server to serve supervisor sync work")
		// Load JWT secret, if any, generate one otherwise.
		jwtSecret, err := rpc.ObtainJWTSecret(logger, cfg.RPCJwtSecretPath, true)
		if err != nil {
			return nil, err
		}
		out := &ManagedMode{}
		out.srv = rpc.NewServer(cfg.RPCAddr, cfg.RPCPort, "v0.0.0",
			rpc.WithLogger(logger),
			rpc.WithWebsocketEnabled(), rpc.WithJWTSecret(jwtSecret[:]))
		return out, nil
	} else {
		logger.Info("Setting up Interop RPC client to sync from read-only supervisor")
		cl, err := client.NewRPC(ctx, logger, cfg.SupervisorAddr, client.WithLazyDial())
		if err != nil {
			return nil, fmt.Errorf("failed to create supervisor RPC: %w", err)
		}
		out := &StandardMode{}
		out.cl = sources.NewSupervisorClient(cl)
		return out, nil
	}
}

// TemporarySetup is a work-around until ManagedMode and StandardMode are ready for use.
func (cfg *Config) TemporarySetup(ctx context.Context, logger log.Logger, eng Engine) (
	*sources.SupervisorClient, *TemporaryInteropServer, error) {
	logger.Info("Setting up Interop RPC client run interop legacy deriver with supervisor API")
	if cfg.SupervisorAddr == "" {
		return nil, nil, errors.New("supervisor RPC is required for legacy interop deriver")
	}
	cl, err := client.NewRPC(ctx, logger, cfg.SupervisorAddr, client.WithLazyDial())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create supervisor RPC: %w", err)
	}
	scl := sources.NewSupervisorClient(cl)
	// Note: there's no JWT secret on the temp RPC server workaround
	srv := NewTemporaryInteropServer(cfg.RPCAddr, cfg.RPCPort, eng)
	if err := srv.Start(); err != nil {
		scl.Close()
		return nil, nil, fmt.Errorf("failed to start interop RPC server: %w", err)
	}
	return scl, srv, nil
}
