package nat

import (
	"context"

	"github.com/ethereum/go-ethereum/log"
)

var _ Validator = &Gate{}

// A Gate is a collection of suites and/or tests.
type Gate struct {
	ID         string
	Validators []Validator
}

// Run runs all the tests in the suite.
func (g Gate) Run(ctx context.Context, log log.Logger, cfg Config) (bool, error) {
	log.Info("", "type", g.Type(), "id", g.Name())
	allPassed := true
	for _, validator := range g.Validators {
		//log.Info("", "type", validator.Type(), "validator", validator.Name())
		ok, err := validator.Run(ctx, log, cfg)
		if err != nil || !ok {
			allPassed = false
		}
	}
	return allPassed, nil
}

// Type returns the type name of the gate.
func (g Gate) Type() string {
	return "Gate"
}

// Name returns the id of the gate.
func (g Gate) Name() string {
	return g.ID
}
