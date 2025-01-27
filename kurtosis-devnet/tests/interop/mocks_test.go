package interop

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"os"
	"runtime"
	"time"

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
func (m *mockFailingChain) Wallet(ctx context.Context, constraints ...constraints.WalletConstraint) (types.Wallet, error) {
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
type RecordingT struct {
	failed  bool
	skipped bool
	logs    *bytes.Buffer
	cleanup []func()
	ctx     context.Context
}

func NewRecordingT(ctx context.Context) *RecordingT {
	return &RecordingT{
		logs: bytes.NewBuffer(nil),
		ctx:  ctx,
	}
}

var _ systest.T = (*RecordingT)(nil)

func (r *RecordingT) Context() context.Context {
	return r.ctx
}

func (r *RecordingT) WithContext(ctx context.Context) systest.T {
	return &RecordingT{
		failed:  r.failed,
		skipped: r.skipped,
		logs:    r.logs,
		cleanup: r.cleanup,
		ctx:     ctx,
	}
}

func (r *RecordingT) Deadline() (deadline time.Time, ok bool) {
	// TODO
	return time.Time{}, false
}

func (r *RecordingT) Parallel() {
	// TODO
}

func (r *RecordingT) Run(name string, f func(systest.T)) {
	// TODO
}

func (r *RecordingT) Cleanup(f func()) {
	r.cleanup = append(r.cleanup, f)
}

func (r *RecordingT) Error(args ...interface{}) {
	r.Log(args...)
	r.Fail()
}

func (r *RecordingT) Errorf(format string, args ...interface{}) {
	r.Logf(format, args...)
	r.Fail()
}

func (r *RecordingT) Fatal(args ...interface{}) {
	r.Log(args...)
	r.FailNow()
}

func (r *RecordingT) Fatalf(format string, args ...interface{}) {
	r.Logf(format, args...)
	r.FailNow()
}

func (r *RecordingT) FailNow() {
	r.Fail()
	runtime.Goexit()
}

func (r *RecordingT) Fail() {
	r.failed = true
}

func (r *RecordingT) Failed() bool {
	return r.failed
}

func (r *RecordingT) Helper() {
	// TODO
}

func (r *RecordingT) Log(args ...interface{}) {
	fmt.Fprintln(r.logs, args...)
}

func (r *RecordingT) Logf(format string, args ...interface{}) {
	fmt.Fprintf(r.logs, format, args...)
	fmt.Fprintln(r.logs)
}

func (r *RecordingT) Name() string {
	return "RecordingT" // TODO
}

func (r *RecordingT) Setenv(key, value string) {
	// Store original value
	origValue, exists := os.LookupEnv(key)

	// Set new value
	os.Setenv(key, value)

	// Register cleanup to restore original value
	r.Cleanup(func() {
		if exists {
			os.Setenv(key, origValue)
		} else {
			os.Unsetenv(key)
		}
	})

}

func (r *RecordingT) Skip(args ...interface{}) {
	r.Log(args...)
	r.SkipNow()
}

func (r *RecordingT) SkipNow() {
	r.skipped = true
}

func (r *RecordingT) Skipf(format string, args ...interface{}) {
	r.Logf(format, args...)
	r.skipped = true
}

func (r *RecordingT) Skipped() bool {
	return r.skipped
}

func (r *RecordingT) TempDir() string {
	return "" // TODO
}

func (r *RecordingT) Logs() string {
	return r.logs.String()
}

func (r *RecordingT) TestScenario(scenario systest.SystemTestFunc, sys system.System, values ...interface{}) {
	// run in a separate goroutine so we can handle runtime.Goexit()
	done := make(chan struct{})
	go func() {
		defer close(done)
		scenario(r, sys)
	}()
	<-done
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
