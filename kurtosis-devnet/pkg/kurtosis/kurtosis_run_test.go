package kurtosis

import (
	"context"
	"fmt"
	"testing"

	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/starlark_run_config"
	"github.com/stretchr/testify/assert"
)

// fakeStarlarkResponse implements StarlarkResponse for testing
type fakeStarlarkResponse struct {
	err          StarlarkError
	progressMsg  []string
	instruction  string
	isSuccessful bool
	warning      string
	info         string
	result       string
	hasResult    bool // tracks whether result was explicitly set
}

func (f *fakeStarlarkResponse) GetError() StarlarkError {
	return f.err
}

func (f *fakeStarlarkResponse) GetProgressInfo() ProgressInfo {
	if f.progressMsg != nil {
		return &fakeProgressInfo{info: f.progressMsg}
	}
	return nil
}

func (f *fakeStarlarkResponse) GetInstruction() Instruction {
	if f.instruction != "" {
		return &fakeInstruction{desc: f.instruction}
	}
	return nil
}

func (f *fakeStarlarkResponse) GetRunFinishedEvent() RunFinishedEvent {
	return &fakeRunFinishedEvent{isSuccessful: f.isSuccessful}
}

func (f *fakeStarlarkResponse) GetWarning() Warning {
	if f.warning != "" {
		return &fakeWarning{msg: f.warning}
	}
	return nil
}

func (f *fakeStarlarkResponse) GetInfo() Info {
	if f.info != "" {
		return &fakeInfo{msg: f.info}
	}
	return nil
}

func (f *fakeStarlarkResponse) GetInstructionResult() InstructionResult {
	if !f.hasResult {
		return nil
	}
	return &fakeInstructionResult{result: f.result}
}

// fakeProgressInfo implements ProgressInfo for testing
type fakeProgressInfo struct {
	info []string
}

func (f *fakeProgressInfo) GetCurrentStepInfo() []string {
	return f.info
}

// fakeInstruction implements Instruction for testing
type fakeInstruction struct {
	desc string
}

func (f *fakeInstruction) GetDescription() string {
	return f.desc
}

// fakeStarlarkError implements StarlarkError for testing
type fakeStarlarkError struct {
	interpretationErr error
	validationErr     error
	executionErr      error
}

func (f *fakeStarlarkError) GetInterpretationError() error {
	return f.interpretationErr
}

func (f *fakeStarlarkError) GetValidationError() error {
	return f.validationErr
}

func (f *fakeStarlarkError) GetExecutionError() error {
	return f.executionErr
}

// fakeRunFinishedEvent implements RunFinishedEvent for testing
type fakeRunFinishedEvent struct {
	isSuccessful bool
}

func (f *fakeRunFinishedEvent) GetIsRunSuccessful() bool {
	return f.isSuccessful
}

// fakeWarning implements Warning for testing
type fakeWarning struct {
	msg string
}

func (f *fakeWarning) GetMessage() string {
	return f.msg
}

// fakeInfo implements Info for testing
type fakeInfo struct {
	msg string
}

func (f *fakeInfo) GetMessage() string {
	return f.msg
}

// fakeInstructionResult implements InstructionResult for testing
type fakeInstructionResult struct {
	result string
}

func (f *fakeInstructionResult) GetSerializedInstructionResult() string {
	return f.result
}

// fakeKurtosisContext implements a fake KurtosisContext for testing
type fakeKurtosisContext struct {
	enclaveCtx *fakeEnclaveContext
	createErr  error
	getErr     error
}

func (f *fakeKurtosisContext) CreateEnclave(ctx context.Context, name string) (enclaveContext, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	return f.enclaveCtx, nil
}

func (f *fakeKurtosisContext) GetEnclave(ctx context.Context, name string) (enclaveContext, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.enclaveCtx, nil
}

// fakeEnclaveContext implements a fake EnclaveContext for testing
type fakeEnclaveContext struct {
	runErr    error
	responses []fakeStarlarkResponse
}

func (f *fakeEnclaveContext) RunStarlarkRemotePackage(ctx context.Context, packageId string, serializedParams *starlark_run_config.StarlarkRunConfig) (<-chan StarlarkResponse, string, error) {
	if f.runErr != nil {
		return nil, "", f.runErr
	}

	responseChan := make(chan StarlarkResponse)
	go func() {
		defer close(responseChan)
		// Send all test responses
		for _, resp := range f.responses {
			responseChan <- &resp
		}
	}()

	return responseChan, "", nil
}

func TestRunKurtosis(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testErr := fmt.Errorf("test error")
	tests := []struct {
		name        string
		responses   []fakeStarlarkResponse
		kurtosisErr error
		getErr      error
		wantErr     bool
	}{
		{
			name: "successful run with all message types",
			responses: []fakeStarlarkResponse{
				{progressMsg: []string{"Starting deployment..."}},
				{info: "Preparing environment"},
				{instruction: "Executing package"},
				{warning: "Using default config"},
				{result: "Service started", hasResult: true},
				{progressMsg: []string{"Deployment complete"}},
				{isSuccessful: true},
			},
			wantErr: false,
		},
		{
			name: "run with error",
			responses: []fakeStarlarkResponse{
				{progressMsg: []string{"Starting deployment..."}},
				{err: &fakeStarlarkError{executionErr: testErr}},
			},
			wantErr: true,
		},
		{
			name: "run with unsuccessful completion",
			responses: []fakeStarlarkResponse{
				{progressMsg: []string{"Starting deployment..."}},
				{isSuccessful: false},
			},
			wantErr: true,
		},
		{
			name:        "kurtosis error",
			kurtosisErr: fmt.Errorf("kurtosis failed"),
			wantErr:     true,
		},
		{
			name: "uses existing enclave",
			responses: []fakeStarlarkResponse{
				{progressMsg: []string{"Using existing enclave"}},
				{isSuccessful: true},
			},
			getErr:  nil,
			wantErr: false,
		},
		{
			name: "creates new enclave when get fails",
			responses: []fakeStarlarkResponse{
				{progressMsg: []string{"Creating new enclave"}},
				{isSuccessful: true},
			},
			getErr:  fmt.Errorf("enclave not found"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake enclave context that will return our test responses
			fakeCtx := &fakeKurtosisContext{
				enclaveCtx: &fakeEnclaveContext{
					runErr:    tt.kurtosisErr,
					responses: tt.responses,
				},
				getErr: tt.getErr,
			}

			d := NewKurtosisDeployer()
			d.kurtosisCtx = fakeCtx

			err := d.runKurtosis(ctx, nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHandleProgress(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		response StarlarkResponse
		want     bool
	}{
		{
			name: "handles progress message",
			response: &fakeStarlarkResponse{
				progressMsg: []string{"Step 1", "Step 2"},
			},
			want: true,
		},
		{
			name:     "ignores non-progress message",
			response: &fakeStarlarkResponse{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handled, err := handleProgress(ctx, tt.response)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, handled)
		})
	}
}

func TestHandleInstruction(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		response StarlarkResponse
		want     bool
	}{
		{
			name: "handles instruction message",
			response: &fakeStarlarkResponse{
				instruction: "Execute command",
			},
			want: true,
		},
		{
			name:     "ignores non-instruction message",
			response: &fakeStarlarkResponse{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handled, err := handleInstruction(ctx, tt.response)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, handled)
		})
	}
}

func TestHandleWarning(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		response StarlarkResponse
		want     bool
	}{
		{
			name: "handles warning message",
			response: &fakeStarlarkResponse{
				warning: "Warning: deprecated feature",
			},
			want: true,
		},
		{
			name:     "ignores non-warning message",
			response: &fakeStarlarkResponse{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handled, err := handleWarning(ctx, tt.response)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, handled)
		})
	}
}

func TestHandleInfo(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		response StarlarkResponse
		want     bool
	}{
		{
			name: "handles info message",
			response: &fakeStarlarkResponse{
				info: "System info",
			},
			want: true,
		},
		{
			name:     "ignores non-info message",
			response: &fakeStarlarkResponse{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handled, err := handleInfo(ctx, tt.response)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, handled)
		})
	}
}

func TestHandleResult(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		response StarlarkResponse
		want     bool
	}{
		{
			name: "handles result message",
			response: &fakeStarlarkResponse{
				result:    "Operation completed",
				hasResult: true,
			},
			want: true,
		},
		{
			name: "handles empty result message",
			response: &fakeStarlarkResponse{
				result:    "",
				hasResult: true,
			},
			want: true,
		},
		{
			name:     "ignores non-result message",
			response: &fakeStarlarkResponse{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handled, err := handleResult(ctx, tt.response)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, handled)
		})
	}
}

func TestHandleError(t *testing.T) {
	ctx := context.Background()
	testErr := fmt.Errorf("test error")
	tests := []struct {
		name      string
		response  StarlarkResponse
		want      bool
		wantError bool
	}{
		{
			name: "handles interpretation error",
			response: &fakeStarlarkResponse{
				err: &fakeStarlarkError{interpretationErr: testErr},
			},
			want:      true,
			wantError: true,
		},
		{
			name: "handles validation error",
			response: &fakeStarlarkResponse{
				err: &fakeStarlarkError{validationErr: testErr},
			},
			want:      true,
			wantError: true,
		},
		{
			name: "handles execution error",
			response: &fakeStarlarkResponse{
				err: &fakeStarlarkError{executionErr: testErr},
			},
			want:      true,
			wantError: true,
		},
		{
			name:     "ignores non-error message",
			response: &fakeStarlarkResponse{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handled, err := handleError(ctx, tt.response)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, handled)
		})
	}
}

func TestFirstMatchHandler(t *testing.T) {
	ctx := context.Background()
	testErr := fmt.Errorf("test error")
	tests := []struct {
		name      string
		handlers  []MessageHandler
		response  StarlarkResponse
		want      bool
		wantError bool
	}{
		{
			name: "first handler matches",
			handlers: []MessageHandler{
				MessageHandlerFunc(handleInfo),
				MessageHandlerFunc(handleWarning),
			},
			response: &fakeStarlarkResponse{
				info: "test info",
			},
			want: true,
		},
		{
			name: "second handler matches",
			handlers: []MessageHandler{
				MessageHandlerFunc(handleInfo),
				MessageHandlerFunc(handleWarning),
			},
			response: &fakeStarlarkResponse{
				warning: "test warning",
			},
			want: true,
		},
		{
			name: "no handlers match",
			handlers: []MessageHandler{
				MessageHandlerFunc(handleInfo),
				MessageHandlerFunc(handleWarning),
			},
			response: &fakeStarlarkResponse{
				result: "test result", hasResult: true,
			},
			want: false,
		},
		{
			name: "handler returns error",
			handlers: []MessageHandler{
				MessageHandlerFunc(handleError),
			},
			response: &fakeStarlarkResponse{
				err: &fakeStarlarkError{interpretationErr: testErr},
			},
			want:      true,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := FirstMatchHandler(tt.handlers...)
			handled, err := handler.Handle(ctx, tt.response)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, handled)
		})
	}
}

func TestAllHandlers(t *testing.T) {
	ctx := context.Background()
	testErr := fmt.Errorf("test error")
	tests := []struct {
		name      string
		handlers  []MessageHandler
		response  StarlarkResponse
		want      bool
		wantError bool
	}{
		{
			name: "multiple handlers match",
			handlers: []MessageHandler{
				MessageHandlerFunc(func(ctx context.Context, resp StarlarkResponse) (bool, error) {
					return true, nil
				}),
				MessageHandlerFunc(func(ctx context.Context, resp StarlarkResponse) (bool, error) {
					return true, nil
				}),
			},
			response: &fakeStarlarkResponse{},
			want:     true,
		},
		{
			name: "some handlers match",
			handlers: []MessageHandler{
				MessageHandlerFunc(handleInfo),
				MessageHandlerFunc(handleWarning),
			},
			response: &fakeStarlarkResponse{
				info: "test info",
			},
			want: true,
		},
		{
			name: "no handlers match",
			handlers: []MessageHandler{
				MessageHandlerFunc(handleInfo),
				MessageHandlerFunc(handleWarning),
			},
			response: &fakeStarlarkResponse{
				result: "test result", hasResult: true,
			},
			want: false,
		},
		{
			name: "handler returns error",
			handlers: []MessageHandler{
				MessageHandlerFunc(handleInfo),
				MessageHandlerFunc(handleError),
			},
			response: &fakeStarlarkResponse{
				err: &fakeStarlarkError{interpretationErr: testErr},
			},
			want:      true,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := AllHandlers(tt.handlers...)
			handled, err := handler.Handle(ctx, tt.response)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, handled)
		})
	}
}
