package nat

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/log"
)

var _ Validator = &Gate{}

// A Gate is a collection of suites and/or tests.
type Gate struct {
	ID         string
	Validators []Validator
}

// Run runs all the tests in the gate.
// Returns the overall result of the gate and an error if any of the tests failed.
func (g Gate) Run(ctx context.Context, log log.Logger, cfg Config) (ValidatorResult, error) {
	log.Info("", "type", g.Type(), "id", g.Name())
	allPassed := true
	results := []ValidatorResult{}
	var allErrors error
	for _, validator := range g.Validators {
		res, err := validator.Run(ctx, log, cfg)
		if err != nil || !res.Passed {
			allPassed = false
			allErrors = errors.Join(allErrors, err)
		}
		results = append(results, res)
	}
	return ValidatorResult{
		ID:         g.ID,
		Type:       g.Type(),
		Passed:     allPassed,
		Error:      allErrors,
		SubResults: results,
	}, nil
}

// Type returns the type name of the gate.
func (g Gate) Type() string {
	return "Gate"
}

// Name returns the id of the gate.
func (g Gate) Name() string {
	return g.ID
}
