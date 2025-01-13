package nat

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/cliapp"
)

var _ cliapp.Lifecycle = &nat{}

type nat struct {
	ctx     context.Context
	log     log.Logger
	config  *Config
	version string

	running atomic.Bool
}

func New(ctx context.Context, config *Config, log log.Logger, version string) (*nat, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}

	return &nat{
		ctx:     ctx,
		config:  config,
		log:     log,
		version: version,
	}, nil
}

// Run runs the acceptance tests and returns true if the tests pass
func (n *nat) Start(ctx context.Context) error {
	n.log.Info("Starting OpNAT")
	n.ctx = ctx
	n.running.Store(true)
	for _, validator := range n.config.Validators {
		n.log.Info("Running validator", "validator", validator.Name(), "type", reflect.TypeOf(validator))
		_, err := validator.Run(*n.config)
		if err != nil {
			n.log.Error("Error running validator", "validator", validator.Name(), "error", err)
		}
	}
	n.log.Info("OpNAT finished")
	return nil
}

func (n *nat) Stop(ctx context.Context) error {
	n.running.Store(false)
	n.log.Info("OpNAT stopped")
	return nil
}

func (n *nat) Stopped() bool {
	return n.running.Load()
}
