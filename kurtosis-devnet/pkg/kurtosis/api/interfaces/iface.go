package interfaces

import (
	"context"

	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/starlark_run_config"
)

// Interfaces for Kurtosis SDK types to make testing easier
type StarlarkError interface {
	GetInterpretationError() error
	GetValidationError() error
	GetExecutionError() error
}

type ProgressInfo interface {
	GetCurrentStepInfo() []string
}

type Instruction interface {
	GetDescription() string
}

type RunFinishedEvent interface {
	GetIsRunSuccessful() bool
}

type Warning interface {
	GetMessage() string
}

type Info interface {
	GetMessage() string
}

type InstructionResult interface {
	GetSerializedInstructionResult() string
}

type StarlarkResponse interface {
	GetError() StarlarkError
	GetProgressInfo() ProgressInfo
	GetInstruction() Instruction
	GetRunFinishedEvent() RunFinishedEvent
	GetWarning() Warning
	GetInfo() Info
	GetInstructionResult() InstructionResult
}

type EnclaveContext interface {
	RunStarlarkPackage(context.Context, string, *starlark_run_config.StarlarkRunConfig) (<-chan StarlarkResponse, string, error)
}

type KurtosisContextInterface interface {
	CreateEnclave(context.Context, string) (EnclaveContext, error)
	GetEnclave(context.Context, string) (EnclaveContext, error)
}
