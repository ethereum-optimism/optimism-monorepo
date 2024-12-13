package kurtosisutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/enclaves"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/starlark_run_config"
	"github.com/kurtosis-tech/kurtosis/api/golang/engine/lib/kurtosis_context"
	"github.com/stretchr/testify/require"
)

func StartEnclave(t *testing.T, ctx context.Context, lgr log.Logger, pkg string, params string) *enclaves.EnclaveContext {
	kurtosisCtx, err := kurtosis_context.NewKurtosisContextFromLocalEngine()
	require.NoError(t, err)

	enclaveID := fmt.Sprintf("kurtosis-%s-%d", t.Name(), time.Now().UnixNano())
	enclaveCtx, err := kurtosisCtx.CreateEnclave(ctx, enclaveID)
	require.NoError(t, err)

	stream, _, err := enclaveCtx.RunStarlarkRemotePackage(
		ctx,
		pkg,
		&starlark_run_config.StarlarkRunConfig{
			SerializedParams: params,
		},
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		cancelCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err = kurtosisCtx.DestroyEnclave(cancelCtx, enclaveID)
		if err != nil {
			lgr.Error("Error destroying enclave", "err", err, "id", enclaveID)
			return
		}
		lgr.Info("Enclave destroyed", "enclave", enclaveID)
	})

	logKurtosisOutput := func(msg string) {
		lgr.Info(fmt.Sprintf("[KURTOSIS] %s", msg))
	}

	for responseLine := range stream {
		if responseLine.GetProgressInfo() != nil {
			stepInfo := responseLine.GetProgressInfo().CurrentStepInfo
			logKurtosisOutput(stepInfo[len(stepInfo)-1])
		} else if responseLine.GetInstruction() != nil {
			logKurtosisOutput(responseLine.GetInstruction().Description)
		} else if responseLine.GetError() != nil {
			if responseLine.GetError().GetInterpretationError() != nil {
				t.Fatalf("interpretation error: %s", responseLine.GetError().GetInterpretationError().String())
			} else if responseLine.GetError().GetValidationError() != nil {
				t.Fatalf("validation error: %s", responseLine.GetError().GetValidationError().String())
			} else if responseLine.GetError().GetExecutionError() != nil {
				t.Fatalf("execution error: %s", responseLine.GetError().GetExecutionError().String())
			}
		}
	}

	return enclaveCtx
}
