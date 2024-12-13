package depset

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

func TestDependencySet(t *testing.T) {
	d := path.Join(t.TempDir(), "tmp_dep_set.json")

	depSet, err := NewStaticConfigDependencySet(
		map[types.ChainID]*StaticConfigDependency{
			types.ChainIDFromUInt64(900): {
				ChainIndex:     900,
				ActivationTime: 42,
				HistoryMinTime: 100,
			},
			types.ChainIDFromUInt64(901): {
				ChainIndex:     901,
				ActivationTime: 30,
				HistoryMinTime: 20,
			},
		})
	require.NoError(t, err)
	data, err := json.Marshal(depSet)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(d, data, 0644))

	loader := &JsonDependencySetLoader{Path: d}
	result, err := loader.LoadDependencySet(context.Background())
	require.NoError(t, err)

	chainIDs := result.Chains()
	require.Equal(t, []types.ChainID{
		types.ChainIDFromUInt64(900),
		types.ChainIDFromUInt64(901),
	}, chainIDs)

	v, err := result.CanExecuteAt(types.ChainIDFromUInt64(900), 42)
	require.NoError(t, err)
	require.True(t, v)
	v, err = result.CanExecuteAt(types.ChainIDFromUInt64(900), 41)
	require.NoError(t, err)
	require.False(t, v)
	v, err = result.CanInitiateAt(types.ChainIDFromUInt64(900), 100)
	require.NoError(t, err)
	require.True(t, v)
	v, err = result.CanInitiateAt(types.ChainIDFromUInt64(900), 99)
	require.NoError(t, err)
	require.False(t, v)

	v, err = result.CanExecuteAt(types.ChainIDFromUInt64(901), 30)
	require.NoError(t, err)
	require.True(t, v)
	v, err = result.CanExecuteAt(types.ChainIDFromUInt64(901), 29)
	require.NoError(t, err)
	require.False(t, v)
	v, err = result.CanInitiateAt(types.ChainIDFromUInt64(901), 20)
	require.NoError(t, err)
	require.True(t, v)
	v, err = result.CanInitiateAt(types.ChainIDFromUInt64(901), 19)
	require.NoError(t, err)
	require.False(t, v)

	v, err = result.CanExecuteAt(types.ChainIDFromUInt64(902), 100000)
	require.NoError(t, err)
	require.False(t, v, "902 not a dependency")
	v, err = result.CanInitiateAt(types.ChainIDFromUInt64(902), 100000)
	require.NoError(t, err)
	require.False(t, v, "902 not a dependency")
}
