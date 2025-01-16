package nat

import (
	"context"

	"github.com/ethereum/go-ethereum/log"
)

type Validator interface {
	Run(ctx context.Context, log log.Logger, cfg Config, params interface{}) (ValidatorResult, error)
	Name() string
	Type() string
}

type ValidatorResult struct {
	ID         string
	Type       string
	Passed     bool
	Error      error
	SubResults []ValidatorResult
}
