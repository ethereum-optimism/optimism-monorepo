package common

import (
	"os"
	"path/filepath"
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

	// Test successful processing
	err := ProcessFiles(files, func(path string) []error {
		return nil
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Test error handling
	err = ProcessFiles(files, func(path string) []error {
		var errors []error
		errors = append(errors, os.ErrNotExist)
		return errors
	})
	if err == nil {
		t.Error("expected error, got nil")
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

	// Test processing with includes and excludes
	includes := []string{"*.txt"}
	excludes := []string{"skip.txt"}

	processedFiles := make(map[string]bool)
	err := ProcessFilesGlob(includes, excludes, func(path string) []error {
		processedFiles[filepath.Base(path)] = true
		return nil
	})

	if err != nil {
		t.Errorf("ProcessFiles failed: %v", err)
	}

	// Verify results
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
