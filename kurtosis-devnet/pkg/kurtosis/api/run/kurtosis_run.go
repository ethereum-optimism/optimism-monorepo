package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/api/interfaces"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/api/wrappers"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/starlark_run_config"
)

type KurtosisRunner struct {
	dryRun      bool
	enclave     string
	kurtosisCtx interfaces.KurtosisContextInterface
	runHandlers []MessageHandler
}

type KurtosisRunnerOptions func(*KurtosisRunner)

func WithKurtosisRunnerDryRun(dryRun bool) KurtosisRunnerOptions {
	return func(r *KurtosisRunner) {
		r.dryRun = dryRun
	}
}

func WithKurtosisRunnerEnclave(enclave string) KurtosisRunnerOptions {
	return func(r *KurtosisRunner) {
		r.enclave = enclave
	}
}

func WithKurtosisRunnerKurtosisContext(kurtosisCtx interfaces.KurtosisContextInterface) KurtosisRunnerOptions {
	return func(r *KurtosisRunner) {
		r.kurtosisCtx = kurtosisCtx
	}
}

func WithKurtosisRunnerRunHandlers(runHandlers ...MessageHandler) KurtosisRunnerOptions {
	return func(r *KurtosisRunner) {
		r.runHandlers = runHandlers
	}
}

func NewKurtosisRunner(opts ...KurtosisRunnerOptions) (*KurtosisRunner, error) {
	r := &KurtosisRunner{}
	for _, opt := range opts {
		opt(r)
	}

	if r.kurtosisCtx == nil {
		var err error
		r.kurtosisCtx, err = wrappers.GetDefaultKurtosisContext()
		if err != nil {
			return nil, fmt.Errorf("failed to create Kurtosis context: %w", err)
		}
	}
	return r, nil
}

func (r *KurtosisRunner) Run(ctx context.Context, packageName string, args io.Reader) error {
	if r.dryRun {
		fmt.Printf("Dry run mode enabled, would run kurtosis package %s in enclave %s\n",
			packageName, r.enclave)
		if args != nil {
			fmt.Println("\nWith arguments:")
			if _, err := io.Copy(os.Stdout, args); err != nil {
				return fmt.Errorf("failed to dump args: %w", err)
			}
			fmt.Println()
		}
		return nil
	}

	// Try to get existing enclave first
	enclaveCtx, err := r.kurtosisCtx.GetEnclave(ctx, r.enclave)
	if err != nil {
		// If enclave doesn't exist, create a new one
		fmt.Printf("Creating a new enclave for Starlark to run inside...\n")
		enclaveCtx, err = r.kurtosisCtx.CreateEnclave(ctx, r.enclave)
		if err != nil {
			return fmt.Errorf("failed to create enclave: %w", err)
		}
		fmt.Printf("Enclave '%s' created successfully\n\n", r.enclave)
	} else {
		fmt.Printf("Using existing enclave '%s'\n\n", r.enclave)
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

	stream, _, err := enclaveCtx.RunStarlarkPackage(ctx, packageName, runConfig)
	if err != nil {
		return fmt.Errorf("failed to run Kurtosis package: %w", err)
	}

	// Set up message handlers
	var isRunSuccessful bool
	runFinishedHandler := makeRunFinishedHandler(&isRunSuccessful)

	// Combine custom handlers with default handler and run finished handler
	handler := AllHandlers(append(r.runHandlers, defaultHandler, runFinishedHandler)...)

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
