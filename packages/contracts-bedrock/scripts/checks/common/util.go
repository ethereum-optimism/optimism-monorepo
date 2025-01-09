package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
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

type Void struct{}

type FileProcessor[T any] func(path string) (T, []error)

func ProcessFiles[T any](files map[string]string, processor FileProcessor[T]) (map[string]T, error) {
	g := errgroup.Group{}
	g.SetLimit(runtime.NumCPU())

	reporter := NewErrorReporter()
	results := sync.Map{}

	for _, path := range files {
		path := path // Capture loop variables
		g.Go(func() error {
			result, errs := processor(path)
			if len(errs) > 0 {
				for _, err := range errs {
					reporter.Fail("%s: %v", path, err)
				}
			} else {
				results.Store(path, result)
			}
			return nil
		})
	}

	err := g.Wait()
	if err != nil {
		return nil, fmt.Errorf("processing failed: %w", err)
	}
	if reporter.HasError() {
		return nil, fmt.Errorf("processing failed")
	}

	// Convert sync.Map to regular map
	finalResults := make(map[string]T)
	results.Range(func(key, value interface{}) bool {
		finalResults[key.(string)] = value.(T)
		return true
	})

	return finalResults, nil
}

func ProcessFilesGlob[T any](includes, excludes []string, processor FileProcessor[T]) (map[string]T, error) {
	files, err := FindFiles(includes, excludes)
	if err != nil {
		return nil, err
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
			included[match] = match
		}
	}

	// Get all excluded files
	for _, pattern := range excludes {
		matches, err := doublestar.Glob(os.DirFS("."), pattern)
		if err != nil {
			return nil, fmt.Errorf("glob pattern error: %w", err)
		}
		for _, match := range matches {
			excluded[match] = struct{}{}
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

func WriteJSON(data interface{}, path string) error {
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	err := enc.Encode(data)
	if err != nil {
		return fmt.Errorf("failed to encode data: %w", err)
	}
	jsonData := out.Bytes()
	if len(jsonData) > 0 && jsonData[len(jsonData)-1] == '\n' { // strip newline
		jsonData = jsonData[:len(jsonData)-1]
	}
	if err := os.WriteFile(path, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}
