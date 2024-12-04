package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/ethereum-optimism/optimism/op-chain-ops/solc"
	"golang.org/x/sync/errgroup"
)

type ErrorReporter struct {
	hasErr atomic.Bool
	outMtx sync.Mutex
}

func NewErrorReporter() *ErrorReporter {
	return &ErrorReporter{}
}

func (e *ErrorReporter) Fail(msg string, args ...any) {
	e.outMtx.Lock()
	// Useful for suppressing error reporting in tests
	if os.Getenv("SUPPRESS_ERROR_REPORTER") == "" {
		_, _ = fmt.Fprintf(os.Stderr, "❌  "+msg+"\n", args...)
	}
	e.outMtx.Unlock()
	e.hasErr.Store(true)
}

func (e *ErrorReporter) HasError() bool {
	return e.hasErr.Load()
}

type FileProcessor func(path string) []error

func ProcessFiles(files map[string]string, processor FileProcessor) error {
	g := errgroup.Group{}
	g.SetLimit(runtime.NumCPU())

	reporter := NewErrorReporter()
	for name, path := range files {
		name, path := name, path // Capture loop variables
		g.Go(func() error {
			if errs := processor(path); len(errs) > 0 {
				for _, err := range errs {
					reporter.Fail("%s: %v", name, err)
				}
			}
			return nil
		})
	}

	err := g.Wait()
	if err != nil {
		return fmt.Errorf("processing failed: %w", err)
	}
	if reporter.HasError() {
		return fmt.Errorf("processing failed")
	}
	return nil
}

func ProcessFilesGlob(includes, excludes []string, processor FileProcessor) error {
	files, err := FindFiles(includes, excludes)
	if err != nil {
		return err
	}
	return ProcessFiles(files, processor)
}

func FindFiles(includes, excludes []string) (map[string]string, error) {
	included := make(map[string]string)
	excluded := make(map[string]struct{})

	// Get all included files
	for _, pattern := range includes {
		matches, err := doublestar.Glob(os.DirFS("."), pattern)
		if err != nil {
			return nil, fmt.Errorf("glob pattern error: %w", err)
		}
		for _, match := range matches {
			name := filepath.Base(match)
			included[name] = match
		}
	}

	// Get all excluded files
	for _, pattern := range excludes {
		matches, err := doublestar.Glob(os.DirFS("."), pattern)
		if err != nil {
			return nil, fmt.Errorf("glob pattern error: %w", err)
		}
		for _, match := range matches {
			excluded[filepath.Base(match)] = struct{}{}
		}
	}

	// Remove excluded files from result
	for name := range excluded {
		delete(included, name)
	}

	return included, nil
}

func ReadForgeArtifact(path string) (*solc.ForgeArtifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read artifact: %w", err)
	}

	var artifact solc.ForgeArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, fmt.Errorf("failed to parse artifact: %w", err)
	}

	return &artifact, nil
}
