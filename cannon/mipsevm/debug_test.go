package mipsevm

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"
)

func TestDebugInfo_Serialization(t *testing.T) {
	debugInfo := &DebugInfo{
		Pages:                        1,
		MemoryUsed:                   2,
		NumPreimageRequests:          3,
		TotalPreimageSize:            4,
		TotalSteps:                   123456,
		RmwSuccessCount:              5,
		RmwFailCount:                 6,
		MaxStepsBetweenLLAndSC:       7,
		ReservationInvalidationCount: 8,
		ForcedPreemptionCount:        9,
		IdleStepCountThread0:         math.MaxUint64,
	}

	// Serialize to file
	dir := t.TempDir()
	path := filepath.Join(dir, "debug-info-test.txt")
	err := jsonutil.WriteJSON(debugInfo, ioutil.ToAtomicFile(path, 0o644))
	require.NoError(t, err)

	// Deserialize
	fromJson, err := jsonutil.LoadJSON[DebugInfo](path)
	require.NoError(t, err)

	require.Equal(t, debugInfo, fromJson)
}
