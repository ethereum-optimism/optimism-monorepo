package fake

import (
	"context"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/api/interfaces"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/starlark_run_config"
)

// KurtosisContext implements interfaces.KurtosisContextInterface for testing
type KurtosisContext struct {
	EnclaveCtx *EnclaveContext
	GetErr     error
	CreateErr  error
}

func (f *KurtosisContext) CreateEnclave(ctx context.Context, name string) (interfaces.EnclaveContext, error) {
	if f.CreateErr != nil {
		return nil, f.CreateErr
	}
	return f.EnclaveCtx, nil
}

func (f *KurtosisContext) GetEnclave(ctx context.Context, name string) (interfaces.EnclaveContext, error) {
	if f.GetErr != nil {
		return nil, f.GetErr
	}
	return f.EnclaveCtx, nil
}

// EnclaveContext implements interfaces.EnclaveContext for testing
type EnclaveContext struct {
	RunErr    error
	Responses []interfaces.StarlarkResponse
}

func (f *EnclaveContext) RunStarlarkPackage(ctx context.Context, pkg string, params *starlark_run_config.StarlarkRunConfig) (<-chan interfaces.StarlarkResponse, string, error) {
	if f.RunErr != nil {
		return nil, "", f.RunErr
	}

	// Create a channel and send all responses
	ch := make(chan interfaces.StarlarkResponse)
	go func() {
		defer close(ch)
		for _, resp := range f.Responses {
			ch <- resp
		}
	}()
	return ch, "", nil
}

// StarlarkResponse implements interfaces.StarlarkResponse for testing
type StarlarkResponse struct {
	Err          interfaces.StarlarkError
	ProgressMsg  []string
	Instruction  string
	IsSuccessful bool
	Warning      string
	Info         string
	Result       string
	HasResult    bool // tracks whether result was explicitly set
}

func (f *StarlarkResponse) GetError() interfaces.StarlarkError {
	return f.Err
}

func (f *StarlarkResponse) GetProgressInfo() interfaces.ProgressInfo {
	if f.ProgressMsg != nil {
		return &ProgressInfo{Info: f.ProgressMsg}
	}
	return nil
}

func (f *StarlarkResponse) GetInstruction() interfaces.Instruction {
	if f.Instruction != "" {
		return &Instruction{Desc: f.Instruction}
	}
	return nil
}

func (f *StarlarkResponse) GetRunFinishedEvent() interfaces.RunFinishedEvent {
	return &RunFinishedEvent{IsSuccessful: f.IsSuccessful}
}

func (f *StarlarkResponse) GetWarning() interfaces.Warning {
	if f.Warning != "" {
		return &Warning{Msg: f.Warning}
	}
	return nil
}

func (f *StarlarkResponse) GetInfo() interfaces.Info {
	if f.Info != "" {
		return &Info{Msg: f.Info}
	}
	return nil
}

func (f *StarlarkResponse) GetInstructionResult() interfaces.InstructionResult {
	if !f.HasResult {
		return nil
	}
	return &InstructionResult{Result: f.Result}
}

// ProgressInfo implements ProgressInfo for testing
type ProgressInfo struct {
	Info []string
}

func (f *ProgressInfo) GetCurrentStepInfo() []string {
	return f.Info
}

// Instruction implements Instruction for testing
type Instruction struct {
	Desc string
}

func (f *Instruction) GetDescription() string {
	return f.Desc
}

// StarlarkError implements StarlarkError for testing
type StarlarkError struct {
	InterpretationErr error
	ValidationErr     error
	ExecutionErr      error
}

func (f *StarlarkError) GetInterpretationError() error {
	return f.InterpretationErr
}

func (f *StarlarkError) GetValidationError() error {
	return f.ValidationErr
}

func (f *StarlarkError) GetExecutionError() error {
	return f.ExecutionErr
}

// RunFinishedEvent implements RunFinishedEvent for testing
type RunFinishedEvent struct {
	IsSuccessful bool
}

func (f *RunFinishedEvent) GetIsRunSuccessful() bool {
	return f.IsSuccessful
}

// Warning implements Warning for testing
type Warning struct {
	Msg string
}

func (f *Warning) GetMessage() string {
	return f.Msg
}

// Info implements Info for testing
type Info struct {
	Msg string
}

func (f *Info) GetMessage() string {
	return f.Msg
}

// InstructionResult implements InstructionResult for testing
type InstructionResult struct {
	Result string
}

func (f *InstructionResult) GetSerializedInstructionResult() string {
	return f.Result
}
