package standard

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultHardforkScheduleForTag(t *testing.T) {
	sched := DefaultHardforkScheduleForTag(ContractsV160Tag)
	require.Nil(t, sched.HoloceneTime(0))

	sched = DefaultHardforkScheduleForTag(ContractsV180Tag)
	require.NotNil(t, sched.HoloceneTime(0))
}

func TestIsKnownTag(t *testing.T) {
	for tag := range knownTags {
		require.True(t, IsKnownTag(tag))
	}
	require.False(t, IsKnownTag("unknown"))
}