package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/versions"
)

// use the all directive to ensure the .gitkeep file is retained and avoid compiler errors

//go:embed all:embeds
var vmFS embed.FS

const baseDir = "embeds"

func ExecuteCannon(ctx context.Context, args []string, ver versions.StateVersion) error {
	if !slices.Contains(versions.StateVersionTypes, ver) {
		return errors.New("unsupported version")
	}

	cannonProgramName := vmFilename(ver)
	cannonProgramBin, err := vmFS.ReadFile(cannonProgramName)
	if err != nil {
		return err
	}
	cannonProgramPath, err := extractTempFile(filepath.Base(cannonProgramName), cannonProgramBin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error extracting %s: %v\n", cannonProgramName, err)
		os.Exit(1)
	}
	defer os.Remove(cannonProgramPath)

	if err := os.Chmod(cannonProgramPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting execute permission for %s: %v\n", cannonProgramName, err)
		os.Exit(1)
	}

	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.CommandContext(ctx, cannonProgramPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("unable to launch cannon-impl program: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// relay exit code to the parent process
			os.Exit(exitErr.ExitCode())
		} else {
			return fmt.Errorf("failed to wait for cannon-impl program: %w", err)
		}
	}
	return nil
}

func extractTempFile(name string, data []byte) (string, error) {
	tempDir := os.TempDir()
	tempFile, err := os.CreateTemp(tempDir, name+"-*")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	if _, err := tempFile.Write(data); err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}

func vmFilename(ver versions.StateVersion) string {
	return fmt.Sprintf("%s/cannon-%d", baseDir, ver)
}
