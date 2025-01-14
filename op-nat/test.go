package nat

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/log"
)

var _ Validator = &Test{}

type Test struct {
	ID string
	Fn func(ctx context.Context, log log.Logger, cfg Config) (bool, error)
}

func (t Test) Run(ctx context.Context, log log.Logger, cfg Config) (bool, error) {
	if t.Fn == nil {
		return false, fmt.Errorf("test function is nil")
	}
	log.Info("", "type", t.Type(), "id", t.Name())
	return t.Fn(ctx, log, cfg)
}

// Name returns the id of the test.
func (t Test) Name() string {
	return t.ID
}

// Type returns the type name of the test.
func (t Test) Type() string {
	return "Test"
}
