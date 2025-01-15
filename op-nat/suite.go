package nat

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/log"
)

var _ Validator = &Suite{}

// A Suite is a collection of tests.
type Suite struct {
	ID    string
	Tests []Test
}

// Run runs all the tests in the suite.
// Returns the overall result of the suite and an error if any of the tests failed.
func (s Suite) Run(ctx context.Context, log log.Logger, cfg Config) (ValidatorResult, error) {
	log.Info("", "type", s.Type(), "id", s.Name())
	allPassed := true
	results := []ValidatorResult{}
	var allErrors error
	for _, test := range s.Tests {
		res, err := test.Run(ctx, log, cfg)
		if err != nil || !res.Passed {
			allPassed = false
			allErrors = errors.Join(allErrors, err)
		}
		results = append(results, res)
	}
	return ValidatorResult{
		ID:         s.ID,
		Type:       s.Type(),
		Passed:     allPassed,
		Error:      allErrors,
		SubResults: results,
	}, nil
}

// Name returns the id of the suite.
func (s Suite) Name() string {
	return s.ID
}

// Type returns the type name of the suite.
func (s Suite) Type() string {
	return "Suite"
}
