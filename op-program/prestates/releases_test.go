package prestates

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetReleases(t *testing.T) {
	releases, err := GetReleases()
	require.NoError(t, err, "expected no error while parsing embedded releases.json")

	foundGovernanceApproved := false
	foundCannon64Release := false
	for _, release := range releases {
		if release.GovernanceApproved {
			foundGovernanceApproved = true
			break
		}
		if release.Type == Cannon64Type {
			foundCannon64Release = true
		}
	}
	require.True(t, foundGovernanceApproved, "expected to find at least one GovernanceApproved release")
	require.True(t, foundCannon64Release, "expected to find at least one Cannon64 release")
}
