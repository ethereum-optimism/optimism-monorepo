package interop

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/constraints"
	"github.com/ethereum-optimism/optimism/devnet-sdk/interfaces"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
)

// mockFailingTx implements types.WriteInvocation[any] that always fails
type mockFailingTx struct{}

func (m *mockFailingTx) Call(ctx context.Context) (any, error) {
	return nil, fmt.Errorf("simulated transaction failure")
}

func (m *mockFailingTx) Send(ctx context.Context) types.InvocationResult {
	return m
}

func (m *mockFailingTx) Error() error {
	return fmt.Errorf("transaction failure")
}

func (m *mockFailingTx) Wait() error {
	return fmt.Errorf("transaction failure")
}

// mockFailingWallet implements types.Wallet that fails on SendETH
type mockFailingWallet struct {
	addr types.Address
	key  types.Key
	bal  types.Balance
}

func (m *mockFailingWallet) Address() types.Address {
	return m.addr
}

func (m *mockFailingWallet) PrivateKey() types.Key {
	return m.key
}

func (m *mockFailingWallet) Balance() types.Balance {
	return m.bal
}

func (m *mockFailingWallet) SendETH(to types.Address, amount types.Balance) types.WriteInvocation[any] {
	return &mockFailingTx{}
}

// mockFailingChain implements system.Chain with a failing SendETH
type mockFailingChain struct {
	id     types.ChainID
	wallet types.Wallet
	reg    interfaces.ContractsRegistry
}

func (m *mockFailingChain) RPCURL() string    { return "mock://failing" }
func (m *mockFailingChain) ID() types.ChainID { return m.id }
func (m *mockFailingChain) User(ctx context.Context, constraints ...constraints.WalletConstraint) (types.Wallet, error) {
	return m.wallet, nil
}
func (m *mockFailingChain) ContractsRegistry() interfaces.ContractsRegistry { return m.reg }

// mockFailingSystem implements system.System with a failing chain
type mockFailingSystem struct {
	chain *mockFailingChain
}

func (m *mockFailingSystem) Identifier() string     { return "mock-failing" }
func (m *mockFailingSystem) L1() system.Chain       { return m.chain }
func (m *mockFailingSystem) L2(uint64) system.Chain { return m.chain }

// recordingT implements systest.T and records failures
type recordingT struct {
	testing.TB
	ctx        context.Context
	failed     bool
	failureMsg string
	name       string
}

func (r *recordingT) Context() context.Context { return r.ctx }
func (r *recordingT) WithContext(ctx context.Context) systest.T {
	r.ctx = ctx
	return r
}
func (r *recordingT) Error(args ...interface{}) {
	r.failed = true
	r.failureMsg = fmt.Sprint(args...)
}
func (r *recordingT) Errorf(format string, args ...interface{}) {
	r.failed = true
	r.failureMsg = fmt.Sprintf(format, args...)
}
func (r *recordingT) Fatal(args ...interface{}) {
	r.failed = true
	r.failureMsg = fmt.Sprint(args...)
}
func (r *recordingT) Fatalf(format string, args ...interface{}) {
	r.failed = true
	r.failureMsg = fmt.Sprintf(format, args...)
}
func (r *recordingT) FailNow() {
	r.failed = true
	// Instead of actually stopping the test, we'll use panic/recover to exit the current function
	panic("FailNow called")
}
func (r *recordingT) Helper()                                  {} // No-op implementation
func (r *recordingT) Name() string                             { return r.name }
func (r *recordingT) Cleanup(func())                           {}
func (r *recordingT) Failed() bool                             { return r.failed }
func (r *recordingT) Fail()                                    { r.failed = true }
func (r *recordingT) Skip(args ...interface{})                 {}
func (r *recordingT) SkipNow()                                 {}
func (r *recordingT) Skipf(format string, args ...interface{}) {}
func (r *recordingT) Skipped() bool                            { return false }
func (r *recordingT) TempDir() string                          { return "" }

func (r *recordingT) Run(name string, fn func(t systest.T)) {
	r.TB.(*testing.T).Run(name, func(t *testing.T) {
		fn(&recordingT{
			TB:         t,
			ctx:        r.ctx,
			failed:     r.failed,
			failureMsg: r.failureMsg,
			name:       name,
		})
	})
}

// mockBalance implements types.ReadInvocation[types.Balance]
type mockBalance struct {
	bal types.Balance
}

func (m *mockBalance) Call(ctx context.Context) (types.Balance, error) {
	return m.bal, nil
}

// mockWETH implements interfaces.SuperchainWETH
type mockWETH struct{}

func (m *mockWETH) BalanceOf(addr types.Address) types.ReadInvocation[types.Balance] {
	return &mockBalance{bal: types.NewBalance(big.NewInt(0))}
}

// mockRegistry implements interfaces.ContractsRegistry
type mockRegistry struct{}

func (m *mockRegistry) SuperchainWETH(addr types.Address) (interfaces.SuperchainWETH, error) {
	return &mockWETH{}, nil
}

// runWithRecordingT runs a test function with a recording test wrapper and returns the recorded failure info
func runWithRecordingT(testName string, ctx context.Context, testFn func(t systest.T)) (failed bool, failureMsg string) {
	rt := &recordingT{
		ctx:  ctx,
		name: testName,
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				if r != "FailNow called" {
					panic(r) // Re-panic if it's not our expected panic
				}
			}
		}()
		testFn(rt)
	}()

	return rt.failed, rt.failureMsg
}
