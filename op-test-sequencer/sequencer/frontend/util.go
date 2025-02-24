package frontend

import (
	"errors"

	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

// toJsonError turns the error into a JSON error with error-code,
// to preserve the code when the error is wrapped
func toJsonError(err error) error {
	var x *rpc.JsonError
	if errors.As(err, &x) {
		return &rpc.JsonError{
			Code:    x.Code,
			Message: err.Error(), // Keep original message, with wrapped info
		}
	}
	return &rpc.JsonError{
		Code:    seqtypes.ErrUnknownKind.Code,
		Message: err.Error(),
	}
}
