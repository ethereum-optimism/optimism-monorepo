package main

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

func TestProofFileSystem(t *testing.T) {
	// Create mock filesystem
	mockfs := NewMockFS()

	// Add test files
	proofData := map[string]interface{}{
		"pre": "hash123",
	}
	proofJSON, _ := json.Marshal(proofData)

	mockfs.Files["/proofs/prestate-proof-test.json"] = NewMockFile(
		"prestate-proof-test.json",
		proofJSON,
	)
	mockfs.Files["/proofs/prestate-test.bin.gz"] = NewMockFile(
		"prestate-test.bin.gz",
		[]byte("mock binary data"),
	)

	// Create proof filesystem and set mock fs
	pfs := newProofFileSystem("/proofs")
	pfs.SetFS(mockfs)

	// Test scanning proof files
	t.Run("ScanProofFiles", func(t *testing.T) {
		err := pfs.scanProofFiles()
		if err != nil {
			t.Errorf("scanProofFiles failed: %v", err)
		}

		// Verify mapping was created
		if mapping, ok := pfs.proofFiles["hash123"]; !ok || mapping != "-test" {
			t.Errorf("Expected mapping for hash123 to be -test, got %v", mapping)
		}
	})

	t.Run("OpenJSONFile", func(t *testing.T) {
		file, err := pfs.Open("/hash123.json")
		if err != nil {
			t.Errorf("Failed to open JSON file: %v", err)
		}
		defer file.Close()

		// Read contents
		contents, err := io.ReadAll(file)
		if err != nil {
			t.Errorf("Failed to read file contents: %v", err)
		}

		if !bytes.Equal(contents, proofJSON) {
			t.Errorf("File contents don't match expected")
		}
	})

	t.Run("OpenBinaryFile", func(t *testing.T) {
		file, err := pfs.Open("/hash123.bin.gz")
		if err != nil {
			t.Errorf("Failed to open binary file: %v", err)
		}
		defer file.Close()

		contents, err := io.ReadAll(file)
		if err != nil {
			t.Errorf("Failed to read file contents: %v", err)
		}

		if !bytes.Equal(contents, []byte("mock binary data")) {
			t.Errorf("File contents don't match expected")
		}
	})

	t.Run("OpenInfoJSONFile", func(t *testing.T) {
		file, err := pfs.Open("/info.json")
		if err != nil {
			t.Errorf("Failed to open info.json file: %v", err)
		}
		defer file.Close()

		// Read contents
		contents, err := io.ReadAll(file)
		if err != nil {
			t.Errorf("Failed to read file contents: %v", err)
		}

		// Verify the contents contain the inverted map
		var infoData map[string]string
		err = json.Unmarshal(contents, &infoData)
		if err != nil {
			t.Errorf("Failed to parse info.json contents: %v", err)
		}

		// Check that the key has dashes replaced with underscores
		expectedKey := "prestate_test"
		if hash, ok := infoData[expectedKey]; !ok || hash != "hash123" {
			t.Errorf("Expected info.json to contain mapping from %s to hash123, got %v", expectedKey, hash)
		}
	})

	t.Run("OpenNonExistentFile", func(t *testing.T) {
		_, err := pfs.Open("/nonexistent.json")
		if err == nil {
			t.Error("Expected error opening non-existent file")
		}
	})

	t.Run("ListDirectory", func(t *testing.T) {
		dir, err := pfs.Open("/")
		if err != nil {
			t.Errorf("Failed to open root directory: %v", err)
		}
		defer dir.Close()

		files, err := dir.Readdir(-1)
		if err != nil {
			t.Errorf("Failed to read directory: %v", err)
		}

		// We expect both .json and .bin.gz files for hash123, plus info.json
		if len(files) != 3 {
			t.Errorf("Expected 3 files, got %d", len(files))
		}

		// Verify info.json is included in the directory listing
		foundInfoJson := false
		for _, file := range files {
			if file.Name() == "info.json" {
				foundInfoJson = true
				break
			}
		}
		if !foundInfoJson {
			t.Error("info.json not found in directory listing")
		}
	})
}
