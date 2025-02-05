//go:build !testonly
// +build !testonly

package wrappers

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/api/interfaces"
	"github.com/kurtosis-tech/kurtosis/api/golang/engine/lib/kurtosis_context"
)

func GetDefaultKurtosisContext() (interfaces.KurtosisContextInterface, error) {
	kCtx, err := kurtosis_context.NewKurtosisContextFromLocalEngine()
	if err != nil {
		return nil, fmt.Errorf("failed to create Kurtosis context: %w", err)
	}
	return KurtosisContextWrapper{
		KurtosisContext: kCtx,
	}, nil
}
