package prestates

// This package is imported by the superchain-registry as part of chain validation
// tests. Please do not delete these files unless the downstream dependency is removed.

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed releases.json
var releasesJSON []byte

type Release struct {
	Version            string `json:"version"`
	Hash               string `json:"hash"`
	GovernanceApproved bool   `json:"governanceApproved"`
}

// GetReleases reads the contents of the releases.json file
func GetReleases() ([]Release, error) {
	var releases []Release
	err := json.Unmarshal(releasesJSON, &releases)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return releases, nil
}
