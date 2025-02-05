package deploy

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/build"
)

type PrestateInfo struct {
	URL    string            `json:"url"`
	Hashes map[string]string `json:"hashes"`
}

type localPrestateHolder struct {
	info       *PrestateInfo
	baseDir    string
	buildDir   string
	dryRun     bool
	builder    *build.PrestateBuilder
	urlBuilder func(path ...string) string
}

func (h *localPrestateHolder) GetPrestateInfo() (*PrestateInfo, error) {
	if h.info != nil {
		return h.info, nil
	}

	prestatePath := []string{"proofs", "op-program", "cannon"}
	prestateURL := h.urlBuilder(prestatePath...)

	// Create build directory with the final path structure
	buildDir := filepath.Join(append([]string{h.buildDir}, prestatePath...)...)
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create prestate build directory: %w", err)
	}

	info := &PrestateInfo{
		URL:    prestateURL,
		Hashes: make(map[string]string),
	}

	if h.dryRun {
		h.info = info
		return info, nil
	}

	// Map of known file prefixes to their keys
	fileToKey := map[string]string{
		"prestate-proof.json":         "prestate",
		"prestate-proof-mt64.json":    "prestate_mt64",
		"prestate-proof-interop.json": "prestate_interop",
	}

	// Build all prestate files directly in the target directory
	if err := h.builder.Build(buildDir); err != nil {
		return nil, fmt.Errorf("failed to build prestates: %w", err)
	}

	// Find and process all prestate files
	matches, err := filepath.Glob(filepath.Join(buildDir, "prestate-proof*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to find prestate files: %w", err)
	}

	// Process each file to rename it to its hash
	for _, filePath := range matches {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read prestate %s: %w", filepath.Base(filePath), err)
		}

		var data struct {
			Pre string `json:"pre"`
		}
		if err := json.Unmarshal(content, &data); err != nil {
			return nil, fmt.Errorf("failed to parse prestate %s: %w", filepath.Base(filePath), err)
		}

		// Store hash with its corresponding key
		if key, exists := fileToKey[filepath.Base(filePath)]; exists {
			info.Hashes[key] = data.Pre
		}

		// Rename files to hash-based names
		newFileName := data.Pre + ".json"
		hashedPath := filepath.Join(buildDir, newFileName)
		if err := os.Rename(filePath, hashedPath); err != nil {
			return nil, fmt.Errorf("failed to rename prestate %s: %w", filepath.Base(filePath), err)
		}
		log.Printf("%s available at: %s/%s\n", filepath.Base(filePath), prestateURL, newFileName)

		// Rename the corresponding binary file
		binFilePath := strings.Replace(strings.TrimSuffix(filePath, ".json"), "-proof", "", 1) + ".bin.gz"
		newBinFileName := data.Pre + ".bin.gz"
		binHashedPath := filepath.Join(buildDir, newBinFileName)
		if err := os.Rename(binFilePath, binHashedPath); err != nil {
			return nil, fmt.Errorf("failed to rename prestate %s: %w", filepath.Base(binFilePath), err)
		}
		log.Printf("%s available at: %s/%s\n", filepath.Base(binFilePath), prestateURL, newBinFileName)
	}

	h.info = info
	return info, nil
}
