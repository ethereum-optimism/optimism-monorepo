package kurtosis

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/kurtosis_core_rpc_api_bindings"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/enclaves"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/starlark_run_config"
	"github.com/kurtosis-tech/kurtosis/api/golang/engine/lib/kurtosis_context"
)

// Color printers
var (
	printCyan   = color.New(color.FgCyan).SprintFunc()
	printYellow = color.New(color.FgYellow).SprintFunc()
	printRed    = color.New(color.FgRed).SprintFunc()
	printBlue   = color.New(color.FgBlue).SprintFunc()
)

// MessageHandler defines the interface for handling different types of messages
type MessageHandler interface {
	// Handle processes the message if applicable and returns:
	// - bool: whether the message was handled
	// - error: any error that occurred during handling
	Handle(context.Context, StarlarkResponse) (bool, error)
}

// MessageHandlerFunc is a function type that implements MessageHandler
type MessageHandlerFunc func(context.Context, StarlarkResponse) (bool, error)

func (f MessageHandlerFunc) Handle(ctx context.Context, resp StarlarkResponse) (bool, error) {
	return f(ctx, resp)
}

// FirstMatchHandler returns a handler that applies the first matching handler from the given handlers
func FirstMatchHandler(handlers ...MessageHandler) MessageHandler {
	return MessageHandlerFunc(func(ctx context.Context, resp StarlarkResponse) (bool, error) {
		for _, h := range handlers {
			handled, err := h.Handle(ctx, resp)
			if err != nil {
				return true, err
			}
			if handled {
				return true, nil
			}
		}
		return false, nil
	})
}

// AllHandlers returns a handler that applies all the given handlers in order
func AllHandlers(handlers ...MessageHandler) MessageHandler {
	return MessageHandlerFunc(func(ctx context.Context, resp StarlarkResponse) (bool, error) {
		anyHandled := false
		for _, h := range handlers {
			handled, err := h.Handle(ctx, resp)
			if err != nil {
				return true, err
			}
			anyHandled = anyHandled || handled
		}
		return anyHandled, nil
	})
}

// defaultHandler is the default message handler that provides standard Kurtosis output
var defaultHandler = FirstMatchHandler(
	MessageHandlerFunc(handleProgress),
	MessageHandlerFunc(handleInstruction),
	MessageHandlerFunc(handleWarning),
	MessageHandlerFunc(handleInfo),
	MessageHandlerFunc(handleResult),
	MessageHandlerFunc(handleError),
)

// handleProgress handles progress info messages
func handleProgress(ctx context.Context, resp StarlarkResponse) (bool, error) {
	if progressInfo := resp.GetProgressInfo(); progressInfo != nil {
		// ignore progress messages, same as kurtosis run does
		return true, nil
	}
	return false, nil
}

// handleInstruction handles instruction messages
func handleInstruction(ctx context.Context, resp StarlarkResponse) (bool, error) {
	if instruction := resp.GetInstruction(); instruction != nil {
		desc := instruction.GetDescription()
		fmt.Println(printCyan(desc))
		return true, nil
	}
	return false, nil
}

// handleWarning handles warning messages
func handleWarning(ctx context.Context, resp StarlarkResponse) (bool, error) {
	if warning := resp.GetWarning(); warning != nil {
		fmt.Println(printYellow(warning.GetMessage()))
		return true, nil
	}
	return false, nil
}

// handleInfo handles info messages
func handleInfo(ctx context.Context, resp StarlarkResponse) (bool, error) {
	if info := resp.GetInfo(); info != nil {
		fmt.Println(printBlue(info.GetMessage()))
		return true, nil
	}
	return false, nil
}

// handleResult handles instruction result messages
func handleResult(ctx context.Context, resp StarlarkResponse) (bool, error) {
	if result := resp.GetInstructionResult(); result != nil {
		if result.GetSerializedInstructionResult() != "" {
			fmt.Printf("%s\n\n", result.GetSerializedInstructionResult())
		}
		return true, nil
	}
	return false, nil
}

// handleError handles error messages
func handleError(ctx context.Context, resp StarlarkResponse) (bool, error) {
	if err := resp.GetError(); err != nil {
		if interpretErr := err.GetInterpretationError(); interpretErr != nil {
			return true, fmt.Errorf(printRed("interpretation error: %v"), interpretErr)
		}
		if validationErr := err.GetValidationError(); validationErr != nil {
			return true, fmt.Errorf(printRed("validation error: %v"), validationErr)
		}
		if executionErr := err.GetExecutionError(); executionErr != nil {
			return true, fmt.Errorf(printRed("execution error: %v"), executionErr)
		}
		return true, nil
	}
	return false, nil
}

// makeRunFinishedHandler creates a handler for run finished events
func makeRunFinishedHandler(isSuccessful *bool) MessageHandlerFunc {
	return func(ctx context.Context, resp StarlarkResponse) (bool, error) {
		if event := resp.GetRunFinishedEvent(); event != nil {
			*isSuccessful = event.GetIsRunSuccessful()
			return true, nil
		}
		return false, nil
	}
}

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

type enclaveContext interface {
	RunStarlarkRemotePackage(context.Context, string, *starlark_run_config.StarlarkRunConfig) (<-chan StarlarkResponse, string, error)
}

type kurtosisContextInterface interface {
	CreateEnclave(context.Context, string) (enclaveContext, error)
	GetEnclave(context.Context, string) (enclaveContext, error)
}

// Wrapper types to implement our interfaces
type kurtosisContextWrapper struct {
	*kurtosis_context.KurtosisContext
}

type enclaveContextWrapper struct {
	*enclaves.EnclaveContext
}

type starlarkResponseWrapper struct {
	*kurtosis_core_rpc_api_bindings.StarlarkRunResponseLine
}

type starlarkErrorWrapper struct {
	*kurtosis_core_rpc_api_bindings.StarlarkError
}

type progressInfoWrapper struct {
	*kurtosis_core_rpc_api_bindings.StarlarkRunProgress
}

type instructionWrapper struct {
	*kurtosis_core_rpc_api_bindings.StarlarkInstruction
}

type runFinishedEventWrapper struct {
	*kurtosis_core_rpc_api_bindings.StarlarkRunFinishedEvent
}

type warningWrapper struct {
	*kurtosis_core_rpc_api_bindings.StarlarkWarning
}

type infoWrapper struct {
	*kurtosis_core_rpc_api_bindings.StarlarkInfo
}

type instructionResultWrapper struct {
	*kurtosis_core_rpc_api_bindings.StarlarkInstructionResult
}

func (w kurtosisContextWrapper) CreateEnclave(ctx context.Context, name string) (enclaveContext, error) {
	enclaveCtx, err := w.KurtosisContext.CreateEnclave(ctx, name)
	if err != nil {
		return nil, err
	}
	return &enclaveContextWrapper{enclaveCtx}, nil
}

func (w kurtosisContextWrapper) GetEnclave(ctx context.Context, name string) (enclaveContext, error) {
	enclaveCtx, err := w.KurtosisContext.GetEnclaveContext(ctx, name)
	if err != nil {
		return nil, err
	}
	return &enclaveContextWrapper{enclaveCtx}, nil
}

func (w *enclaveContextWrapper) RunStarlarkRemotePackage(ctx context.Context, packageId string, serializedParams *starlark_run_config.StarlarkRunConfig) (<-chan StarlarkResponse, string, error) {
	stream, cancel, err := w.EnclaveContext.RunStarlarkRemotePackage(ctx, packageId, serializedParams)
	if err != nil {
		return nil, "", err
	}

	// Convert the stream
	wrappedStream := make(chan StarlarkResponse)
	go func() {
		defer close(wrappedStream)
		defer cancel()
		for line := range stream {
			wrappedStream <- &starlarkResponseWrapper{line}
		}
	}()

	return wrappedStream, "", nil
}

func (w *starlarkResponseWrapper) GetError() StarlarkError {
	if err := w.StarlarkRunResponseLine.GetError(); err != nil {
		return &starlarkErrorWrapper{err}
	}
	return nil
}

func (w *starlarkResponseWrapper) GetProgressInfo() ProgressInfo {
	if progress := w.StarlarkRunResponseLine.GetProgressInfo(); progress != nil {
		return &progressInfoWrapper{progress}
	}
	return nil
}

func (w *starlarkResponseWrapper) GetInstruction() Instruction {
	if instruction := w.StarlarkRunResponseLine.GetInstruction(); instruction != nil {
		return &instructionWrapper{instruction}
	}
	return nil
}

func (w *starlarkResponseWrapper) GetRunFinishedEvent() RunFinishedEvent {
	if event := w.StarlarkRunResponseLine.GetRunFinishedEvent(); event != nil {
		return &runFinishedEventWrapper{event}
	}
	return nil
}

func (w *starlarkResponseWrapper) GetWarning() Warning {
	if warning := w.StarlarkRunResponseLine.GetWarning(); warning != nil {
		return &warningWrapper{warning}
	}
	return nil
}

func (w *starlarkResponseWrapper) GetInfo() Info {
	if info := w.StarlarkRunResponseLine.GetInfo(); info != nil {
		return &infoWrapper{info}
	}
	return nil
}

func (w *starlarkResponseWrapper) GetInstructionResult() InstructionResult {
	if result := w.StarlarkRunResponseLine.GetInstructionResult(); result != nil {
		return &instructionResultWrapper{result}
	}
	return nil
}

func (w *progressInfoWrapper) GetCurrentStepInfo() []string {
	return w.StarlarkRunProgress.CurrentStepInfo
}

func (w *instructionWrapper) GetDescription() string {
	return w.StarlarkInstruction.Description
}

func (w *runFinishedEventWrapper) GetIsRunSuccessful() bool {
	return w.StarlarkRunFinishedEvent.IsRunSuccessful
}

func (w *starlarkErrorWrapper) GetInterpretationError() error {
	if err := w.StarlarkError.GetInterpretationError(); err != nil {
		return fmt.Errorf("%v", err)
	}
	return nil
}

func (w *starlarkErrorWrapper) GetValidationError() error {
	if err := w.StarlarkError.GetValidationError(); err != nil {
		return fmt.Errorf("%v", err)
	}
	return nil
}

func (w *starlarkErrorWrapper) GetExecutionError() error {
	if err := w.StarlarkError.GetExecutionError(); err != nil {
		return fmt.Errorf("%v", err)
	}
	return nil
}

func (w *warningWrapper) GetMessage() string {
	return w.StarlarkWarning.WarningMessage
}

func (w *infoWrapper) GetMessage() string {
	return w.StarlarkInfo.InfoMessage
}

func (w *instructionResultWrapper) GetSerializedInstructionResult() string {
	return w.StarlarkInstructionResult.SerializedInstructionResult
}

// runKurtosis executes the kurtosis package using the SDK
func (d *KurtosisDeployer) runKurtosis(ctx context.Context, args io.Reader) error {
	if d.dryRun {
		fmt.Printf("Dry run mode enabled, would run kurtosis package %s in enclave %s\n",
			d.packageName, d.enclave)
		if args != nil {
			fmt.Println("\nWith arguments:")
			if _, err := io.Copy(os.Stdout, args); err != nil {
				return fmt.Errorf("failed to dump args: %w", err)
			}
			fmt.Println()
		}
		return nil
	}

	// Create Kurtosis context if not already set (for testing)
	if d.kurtosisCtx == nil {
		var err error
		kCtx, err := kurtosis_context.NewKurtosisContextFromLocalEngine()
		if err != nil {
			return fmt.Errorf("failed to create Kurtosis context: %w", err)
		}
		d.kurtosisCtx = kurtosisContextWrapper{kCtx}
	}

	// Try to get existing enclave first
	enclaveCtx, err := d.kurtosisCtx.GetEnclave(ctx, d.enclave)
	if err != nil {
		// If enclave doesn't exist, create a new one
		fmt.Printf("Creating a new enclave for Starlark to run inside...\n")
		enclaveCtx, err = d.kurtosisCtx.CreateEnclave(ctx, d.enclave)
		if err != nil {
			return fmt.Errorf("failed to create enclave: %w", err)
		}
		fmt.Printf("Enclave '%s' created successfully\n\n", d.enclave)
	} else {
		fmt.Printf("Using existing enclave '%s'\n\n", d.enclave)
	}

	// Set up run config with args if provided
	var serializedParams string
	if args != nil {
		argsBytes, err := io.ReadAll(args)
		if err != nil {
			return fmt.Errorf("failed to read args: %w", err)
		}
		serializedParams = string(argsBytes)
	}

	runConfig := &starlark_run_config.StarlarkRunConfig{
		SerializedParams: serializedParams,
	}

	stream, _, err := enclaveCtx.RunStarlarkRemotePackage(ctx, d.packageName, runConfig)
	if err != nil {
		return fmt.Errorf("failed to run Kurtosis package: %w", err)
	}

	// Set up message handlers
	var isRunSuccessful bool
	runFinishedHandler := makeRunFinishedHandler(&isRunSuccessful)

	// Combine custom handlers with default handler and run finished handler
	handler := AllHandlers(append(d.runHandlers, defaultHandler, runFinishedHandler)...)

	// Process the output stream
	for responseLine := range stream {
		if _, err := handler.Handle(ctx, responseLine); err != nil {
			return err
		}
	}

	if !isRunSuccessful {
		return errors.New(printRed("kurtosis package execution failed"))
	}

	return nil
}
