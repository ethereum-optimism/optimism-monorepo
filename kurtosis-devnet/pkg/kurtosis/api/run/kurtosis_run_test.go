package run

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/api/fake"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/api/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunKurtosis(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testErr := fmt.Errorf("test error")
	tests := []struct {
		name        string
		responses   []fake.StarlarkResponse
		kurtosisErr error
		getErr      error
		wantErr     bool
	}{
		{
			name: "successful run with all message types",
			responses: []fake.StarlarkResponse{
				{ProgressMsg: []string{"Starting deployment..."}},
				{Info: "Preparing environment"},
				{Instruction: "Executing package"},
				{Warning: "Using default config"},
				{Result: "Service started", HasResult: true},
				{ProgressMsg: []string{"Deployment complete"}},
				{IsSuccessful: true},
			},
			wantErr: false,
		},
		{
			name: "run with error",
			responses: []fake.StarlarkResponse{
				{ProgressMsg: []string{"Starting deployment..."}},
				{Err: &fake.StarlarkError{ExecutionErr: testErr}},
			},
			wantErr: true,
		},
		{
			name: "run with unsuccessful completion",
			responses: []fake.StarlarkResponse{
				{ProgressMsg: []string{"Starting deployment..."}},
				{IsSuccessful: false},
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
			responses: []fake.StarlarkResponse{
				{ProgressMsg: []string{"Using existing enclave"}},
				{IsSuccessful: true},
			},
			getErr:  nil,
			wantErr: false,
		},
		{
			name: "creates new enclave when get fails",
			responses: []fake.StarlarkResponse{
				{ProgressMsg: []string{"Creating new enclave"}},
				{IsSuccessful: true},
			},
			getErr:  fmt.Errorf("enclave not found"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert test responses to interface slice
			interfaceResponses := make([]interfaces.StarlarkResponse, len(tt.responses))
			for i := range tt.responses {
				interfaceResponses[i] = &tt.responses[i]
			}

			// Create a fake enclave context that will return our test responses
			fakeCtx := &fake.KurtosisContext{
				EnclaveCtx: &fake.EnclaveContext{
					RunErr:    tt.kurtosisErr,
					Responses: interfaceResponses,
				},
				GetErr: tt.getErr,
			}

			kurtosisRunner, err := NewKurtosisRunner(
				WithKurtosisRunnerDryRun(false),
				WithKurtosisRunnerEnclave("test-enclave"),
				WithKurtosisRunnerKurtosisContext(fakeCtx),
			)
			require.NoError(t, err)

			err = kurtosisRunner.Run(ctx, "test-package", nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
