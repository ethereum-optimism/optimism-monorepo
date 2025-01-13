package claim

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
)

var ErrClaimNotValid = errors.New("invalid claim")

func ValidateClaim(log log.Logger, l2Head eth.L2BlockRef, claimedOutputRoot eth.Bytes32, outputRoot eth.Bytes32) error {
	log.Info("Validating claim", "head", l2Head, "output", outputRoot, "claim", claimedOutputRoot)
	if claimedOutputRoot != outputRoot {
		return fmt.Errorf("%w: claim: %v actual: %v", ErrClaimNotValid, claimedOutputRoot, outputRoot)
	}
	return nil
}
