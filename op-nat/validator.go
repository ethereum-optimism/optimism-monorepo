package nat

import (
	"context"
	"github.com/ethereum/go-ethereum/log"
)

type Validator interface {
	Run(ctx context.Context, log log.Logger, cfg Config) (bool, error)
	Name() string
	Type() string
}
