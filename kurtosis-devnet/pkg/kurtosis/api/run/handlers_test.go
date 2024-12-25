package run

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/api/fake"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/api/interfaces"
	"github.com/stretchr/testify/assert"
)

func TestHandleProgress(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		response interfaces.StarlarkResponse
		want     bool
	}{
		{
			name: "handles progress message",
			response: &fake.StarlarkResponse{
				ProgressMsg: []string{"Step 1", "Step 2"},
			},
			want: true,
		},
		{
			name:     "ignores non-progress message",
			response: &fake.StarlarkResponse{},
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
		response interfaces.StarlarkResponse
		want     bool
	}{
		{
			name: "handles instruction message",
			response: &fake.StarlarkResponse{
				Instruction: "Execute command",
			},
			want: true,
		},
		{
			name:     "ignores non-instruction message",
			response: &fake.StarlarkResponse{},
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
		response interfaces.StarlarkResponse
		want     bool
	}{
		{
			name: "handles warning message",
			response: &fake.StarlarkResponse{
				Warning: "Warning: deprecated feature",
			},
			want: true,
		},
		{
			name:     "ignores non-warning message",
			response: &fake.StarlarkResponse{},
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
		response interfaces.StarlarkResponse
		want     bool
	}{
		{
			name: "handles info message",
			response: &fake.StarlarkResponse{
				Info: "System info",
			},
			want: true,
		},
		{
			name:     "ignores non-info message",
			response: &fake.StarlarkResponse{},
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
		response interfaces.StarlarkResponse
		want     bool
	}{
		{
			name: "handles result message",
			response: &fake.StarlarkResponse{
				Result:    "Operation completed",
				HasResult: true,
			},
			want: true,
		},
		{
			name: "handles empty result message",
			response: &fake.StarlarkResponse{
				Result:    "",
				HasResult: true,
			},
			want: true,
		},
		{
			name:     "ignores non-result message",
			response: &fake.StarlarkResponse{},
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
		response  interfaces.StarlarkResponse
		want      bool
		wantError bool
	}{
		{
			name: "handles interpretation error",
			response: &fake.StarlarkResponse{
				Err: &fake.StarlarkError{InterpretationErr: testErr},
			},
			want:      true,
			wantError: true,
		},
		{
			name: "handles validation error",
			response: &fake.StarlarkResponse{
				Err: &fake.StarlarkError{ValidationErr: testErr},
			},
			want:      true,
			wantError: true,
		},
		{
			name: "handles execution error",
			response: &fake.StarlarkResponse{
				Err: &fake.StarlarkError{ExecutionErr: testErr},
			},
			want:      true,
			wantError: true,
		},
		{
			name:     "ignores non-error message",
			response: &fake.StarlarkResponse{},
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
		response  interfaces.StarlarkResponse
		want      bool
		wantError bool
	}{
		{
			name: "first handler matches",
			handlers: []MessageHandler{
				MessageHandlerFunc(handleInfo),
				MessageHandlerFunc(handleWarning),
			},
			response: &fake.StarlarkResponse{
				Info: "test info",
			},
			want: true,
		},
		{
			name: "second handler matches",
			handlers: []MessageHandler{
				MessageHandlerFunc(handleInfo),
				MessageHandlerFunc(handleWarning),
			},
			response: &fake.StarlarkResponse{
				Warning: "test warning",
			},
			want: true,
		},
		{
			name: "no handlers match",
			handlers: []MessageHandler{
				MessageHandlerFunc(handleInfo),
				MessageHandlerFunc(handleWarning),
			},
			response: &fake.StarlarkResponse{
				Result: "test result", HasResult: true,
			},
			want: false,
		},
		{
			name: "handler returns error",
			handlers: []MessageHandler{
				MessageHandlerFunc(handleError),
			},
			response: &fake.StarlarkResponse{
				Err: &fake.StarlarkError{InterpretationErr: testErr},
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
		response  interfaces.StarlarkResponse
		want      bool
		wantError bool
	}{
		{
			name: "multiple handlers match",
			handlers: []MessageHandler{
				MessageHandlerFunc(func(ctx context.Context, resp interfaces.StarlarkResponse) (bool, error) {
					return true, nil
				}),
				MessageHandlerFunc(func(ctx context.Context, resp interfaces.StarlarkResponse) (bool, error) {
					return true, nil
				}),
			},
			response: &fake.StarlarkResponse{},
			want:     true,
		},
		{
			name: "some handlers match",
			handlers: []MessageHandler{
				MessageHandlerFunc(handleInfo),
				MessageHandlerFunc(handleWarning),
			},
			response: &fake.StarlarkResponse{
				Info: "test info",
			},
			want: true,
		},
		{
			name: "no handlers match",
			handlers: []MessageHandler{
				MessageHandlerFunc(handleInfo),
				MessageHandlerFunc(handleWarning),
			},
			response: &fake.StarlarkResponse{
				Result: "test result", HasResult: true,
			},
			want: false,
		},
		{
			name: "handler returns error",
			handlers: []MessageHandler{
				MessageHandlerFunc(handleInfo),
				MessageHandlerFunc(handleError),
			},
			response: &fake.StarlarkResponse{
				Err: &fake.StarlarkError{InterpretationErr: testErr},
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
