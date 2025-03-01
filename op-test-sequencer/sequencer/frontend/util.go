package frontend

import (
	"errors"

	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

// toJsonError turns the error into a JSON error with error-code,
// to preserve the code when the error is wrapped
func toJsonError(err error) error {
	if err == nil {
		return nil
	}
	var x *rpc.JsonError
	if errors.As(err, &x) {
		return x
	}
	return &rpc.JsonError{
		Code:    seqtypes.ErrUnknownKind.Code,
		Message: err.Error(),
	}
}
