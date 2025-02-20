package validators

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
)

type LowLevelSystemGetter = func(context.Context) system.LowLevelSystem

type lowLevelSystemWrapper struct {
	identifier string
	l1         system.LowLevelChain
	l2         []system.LowLevelChain
}

var _ system.LowLevelSystem = (*lowLevelSystemWrapper)(nil)

func (l *lowLevelSystemWrapper) Identifier() string {
	return l.identifier
}

func (l *lowLevelSystemWrapper) L1() system.LowLevelChain {
	return l.l1
}

func (l *lowLevelSystemWrapper) L2s() []system.LowLevelChain {
	return l.l2
}

func lowLevelSystemValidator(sysMarker interface{}) systest.PreconditionValidator {
	return func(t systest.T, sys system.System) (context.Context, error) {
		lowLevelSys := &lowLevelSystemWrapper{}

		// If any chain is not a low level chain, return an error
		if l1, ok := sys.L1().(system.LowLevelChain); ok {
			lowLevelSys.l1 = l1
		} else {
			return nil, fmt.Errorf("L1 chain is not a low level chain")
		}

		for idx, l2 := range sys.L2s() {
			if l2, ok := l2.(system.LowLevelChain); ok {
				lowLevelSys.l2 = append(lowLevelSys.l2, l2)
			} else {
				return nil, fmt.Errorf("L2 chain %d is not a low level chain", idx)
			}
		}

		return context.WithValue(t.Context(), sysMarker, lowLevelSys), nil
	}
}

func AcquireLowLevelSystem() (LowLevelSystemGetter, systest.PreconditionValidator) {
	sysMarker := &struct{}{}
	validator := lowLevelSystemValidator(sysMarker)
	return func(ctx context.Context) system.LowLevelSystem {
		return ctx.Value(sysMarker).(system.LowLevelSystem)
	}, validator
}
