package syncnode

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/rpc"
)

type CLISyncNodes struct {
	Endpoints      []string
	JWTSecretPaths []string
}

var _ SyncNodeCollection = (*CLISyncNodes)(nil)

func (p *CLISyncNodes) Load(ctx context.Context, logger log.Logger) ([]SyncNodeSetup, error) {
	if err := p.Check(); err != nil { // sanity-check, in case the caller did not check.
		return nil, err
	}
	if len(p.Endpoints) == 0 {
		logger.Warn("No sync sources were configured")
		return nil, nil
	}
	if len(p.JWTSecretPaths) == 0 {
		return nil, errors.New("need at least 1 JWT secret to setup sync-sources")
	}
	secrets := make([]eth.Bytes32, 0, len(p.JWTSecretPaths))
	for i, secretPath := range p.JWTSecretPaths {
		secret, err := rpc.ObtainJWTSecret(logger, secretPath, false)
		if err != nil {
			return nil, fmt.Errorf("failed to load JWT secret %d from path %q", i, secretPath)
		}
		secrets = append(secrets, secret)
	}
	setups := make([]SyncNodeSetup, 0, len(p.Endpoints))
	for i, endpoint := range p.Endpoints {
		var secret eth.Bytes32
		if i >= len(secrets) {
			secret = secrets[0] // default to the first JWT secret (there's always at least 1)
		} else {
			secret = secrets[i]
		}
		setups = append(setups, &RPCDialSetup{
			JWTSecret: secret,
			Endpoint:  endpoint,
		})
	}
	return setups, nil
}

func (p *CLISyncNodes) Check() error {
	if len(p.Endpoints) == len(p.JWTSecretPaths) {
		return nil
	}
	if len(p.JWTSecretPaths) == 1 {
		return nil // repeating JWT secret, for any number of endpoints
	}
	return fmt.Errorf("expected each sync source endpoint to come with a JWT secret, "+
		"or all share the same JWT secret, but got %d endpoints and %d secrets",
		len(p.Endpoints), len(p.JWTSecretPaths))
}
