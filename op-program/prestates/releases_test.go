package prestates

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetReleases(t *testing.T) {
	releases, err := GetReleases()
	require.NoError(t, err, "expected no error while parsing embedded releases.json")

	foundGovernanceApproved := false
	for _, release := range releases {
		if release.GovernanceApproved {
			foundGovernanceApproved = true
			break
		}
	}
	require.True(t, foundGovernanceApproved, "expected to find at least one GovernanceApproved release")
}
