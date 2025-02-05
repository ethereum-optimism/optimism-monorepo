package artifacts

import (
	"crypto/sha256"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

type integrityChecker interface {
	CheckIntegrity(data []byte) error
}

type hashIntegrityChecker struct {
	hash common.Hash
}

func (h *hashIntegrityChecker) CheckIntegrity(data []byte) error {
	hash := sha256.Sum256(data)
	if hash != h.hash {
		return fmt.Errorf("integrity check failed - expected: %x, got: %x", h.hash, hash)
	}
	return nil
}

type noopIntegrityChecker struct{}

func (noopIntegrityChecker) CheckIntegrity([]byte) error {
	return nil
}
