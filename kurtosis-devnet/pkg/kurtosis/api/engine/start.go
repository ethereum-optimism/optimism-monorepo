package engine

import (
	"fmt"
	"os/exec"

	"github.com/kurtosis-tech/kurtosis/api/golang/kurtosis_version"
)

// EngineManager handles running the Kurtosis engine
type EngineManager struct {
	kurtosisBinary string
	version        string
}

// Option configures an EngineManager
type Option func(*EngineManager)

// WithKurtosisBinary sets the path to the kurtosis binary
func WithKurtosisBinary(binary string) Option {
	return func(e *EngineManager) {
		e.kurtosisBinary = binary
	}
}

// WithVersion sets the engine version
func WithVersion(version string) Option {
	return func(e *EngineManager) {
		e.version = version
	}
}

// NewEngineManager creates a new EngineManager with the given options
func NewEngineManager(opts ...Option) *EngineManager {
	e := &EngineManager{
		kurtosisBinary: "kurtosis",                       // Default to expecting kurtosis in PATH
		version:        kurtosis_version.KurtosisVersion, // Default to library version
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// EnsureRunning starts the Kurtosis engine with the configured version
func (e *EngineManager) EnsureRunning() error {
	cmd := exec.Command(e.kurtosisBinary, "engine", "start", "--version", e.version)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start kurtosis engine: %w", err)
	}
	return nil
}
