package nat

import (
	"context"
	"github.com/ethereum/go-ethereum/log"
)

var _ Validator = &Suite{}

// A Suite is a collection of tests.
type Suite struct {
	ID    string
	Tests []Test
}

// Run runs all the tests in the suite.
// func (s Suite) Run(cfg Config) (bool, error) {
func (s Suite) Run(ctx context.Context, log log.Logger, cfg Config) (bool, error) {
	for _, test := range s.Tests {
		ok, err := test.Run(ctx, log, cfg)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// Name returns the id of the suite.
func (s Suite) Name() string {
	return s.ID
}

// Type returns the type name of the suite.
func (s Suite) Type() string {
	return "Suite"
}
