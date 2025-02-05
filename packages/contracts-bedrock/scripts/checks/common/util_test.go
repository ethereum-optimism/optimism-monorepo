package common

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestErrorReporter(t *testing.T) {
	os.Setenv("SUPPRESS_ERROR_REPORTER", "1")
	defer os.Unsetenv("SUPPRESS_ERROR_REPORTER")

	reporter := NewErrorReporter()

	if reporter.HasError() {
		t.Error("new reporter should not have errors")
	}

	reporter.Fail("test error")

	if !reporter.HasError() {
		t.Error("reporter should have error after Fail")
	}
}

func TestProcessFiles(t *testing.T) {
	os.Setenv("SUPPRESS_ERROR_REPORTER", "1")
	defer os.Unsetenv("SUPPRESS_ERROR_REPORTER")

	files := map[string]string{
		"file1": "path1",
		"file2": "path2",
	}

	// Test void processing (no results)
	_, err := ProcessFiles(files, func(path string) (*Void, []error) {
		return nil, nil
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Test error handling
	_, err = ProcessFiles(files, func(path string) (*Void, []error) {
		var errors []error
		errors = append(errors, os.ErrNotExist)
		return nil, errors
	})
	if err == nil {
		t.Error("expected error, got nil")
	}

	// Test successful processing with string results
	results, err := ProcessFiles(files, func(path string) (string, []error) {
		return "processed_" + path, nil
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if results["path1"] != "processed_path1" {
		t.Errorf("expected processed_path1, got %s", results["path1"])
	}

	// Test processing with struct results
	type testResult struct {
		Path    string
		Counter int
	}
	structResults, err := ProcessFiles(files, func(path string) (testResult, []error) {
		return testResult{Path: path, Counter: len(path)}, nil
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(structResults) != 2 {
		t.Errorf("expected 2 results, got %d", len(structResults))
	}
	if structResults["path1"].Counter != 5 {
		t.Errorf("expected counter 5, got %d", structResults["path1"].Counter)
	}
}

func TestProcessFilesGlob(t *testing.T) {
	os.Setenv("SUPPRESS_ERROR_REPORTER", "1")
	defer os.Unsetenv("SUPPRESS_ERROR_REPORTER")

	// Create test directory structure
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create test files
	files := map[string]string{
		"test1.txt": "content1",
		"test2.txt": "content2",
		"skip.txt":  "content3",
	}

	for name, content := range files {
		if err := os.WriteFile(name, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	includes := []string{"*.txt"}
	excludes := []string{"skip.txt"}

	// Test void processing (no results)
	processedFiles := make(map[string]bool)
	var mtx sync.Mutex
	_, err := ProcessFilesGlob(includes, excludes, func(path string) (*Void, []error) {
		mtx.Lock()
		processedFiles[filepath.Base(path)] = true
		mtx.Unlock()
		return nil, nil
	})

	if err != nil {
		t.Errorf("ProcessFiles failed: %v", err)
	}

	// Verify void processing results
	if len(processedFiles) != 2 {
		t.Errorf("expected 2 processed files, got %d", len(processedFiles))
	}
	if !processedFiles["test1.txt"] {
		t.Error("expected to process test1.txt")
	}
	if !processedFiles["test2.txt"] {
		t.Error("expected to process test2.txt")
	}
	if processedFiles["skip.txt"] {
		t.Error("skip.txt should have been excluded")
	}

	// Test processing with struct results
	type fileInfo struct {
		Size    int64
		Content string
	}

	results, err := ProcessFilesGlob(includes, excludes, func(path string) (fileInfo, []error) {
		content, err := os.ReadFile(path)
		if err != nil {
			return fileInfo{}, []error{err}
		}
		info, err := os.Stat(path)
		if err != nil {
			return fileInfo{}, []error{err}
		}
		return fileInfo{
			Size:    info.Size(),
			Content: string(content),
		}, nil
	})

	if err != nil {
		t.Errorf("ProcessFilesGlob failed: %v", err)
	}

	// Verify struct results
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if result, exists := results["test1.txt"]; !exists {
		t.Error("expected result for test1.txt")
	} else {
		if result.Content != "content1" {
			t.Errorf("expected content1, got %s", result.Content)
		}
		if result.Size != 8 {
			t.Errorf("expected size 8, got %d", result.Size)
		}
	}

	// Test error handling
	_, err = ProcessFilesGlob(includes, excludes, func(path string) (fileInfo, []error) {
		return fileInfo{}, []error{fmt.Errorf("test error")}
	})
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestFindFiles(t *testing.T) {
	// Create test directory structure
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create test files
	files := map[string]string{
		"test1.txt": "content1",
		"test2.txt": "content2",
		"skip.txt":  "content3",
	}

	for name, content := range files {
		if err := os.WriteFile(name, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Test finding files
	includes := []string{"*.txt"}
	excludes := []string{"skip.txt"}

	found, err := FindFiles(includes, excludes)
	if err != nil {
		t.Fatalf("FindFiles failed: %v", err)
	}

	// Verify results
	if len(found) != 2 {
		t.Errorf("expected 2 files, got %d", len(found))
	}
	if _, exists := found["test1.txt"]; !exists {
		t.Error("expected to find test1.txt")
	}
	if _, exists := found["test2.txt"]; !exists {
		t.Error("expected to find test2.txt")
	}
	if _, exists := found["skip.txt"]; exists {
		t.Error("skip.txt should have been excluded")
	}
}

func TestReadForgeArtifact(t *testing.T) {
	// Create a temporary test artifact
	tmpDir := t.TempDir()
	artifactContent := `{
		"abi": [],
		"bytecode": {
			"object": "0x123"
		},
		"deployedBytecode": {
			"object": "0x456"
		}
	}`
	tmpFile := filepath.Join(tmpDir, "Test.json")
	if err := os.WriteFile(tmpFile, []byte(artifactContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Test processing
	artifact, err := ReadForgeArtifact(tmpFile)
	if err != nil {
		t.Fatalf("ReadForgeArtifact failed: %v", err)
	}

	// Verify results
	if artifact.Bytecode.Object != "0x123" {
		t.Errorf("expected bytecode '0x123', got %q", artifact.Bytecode.Object)
	}
	if artifact.DeployedBytecode.Object != "0x456" {
		t.Errorf("expected deployed bytecode '0x456', got %q", artifact.DeployedBytecode.Object)
	}
}
