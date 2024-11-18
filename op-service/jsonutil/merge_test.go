package jsonutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeJSON(t *testing.T) {
	type testStruct struct {
		A string `json:"a"`
		B int    `json:"b"`
		C bool   `json:"c"`
	}

	out, err := MergeJSON(
		testStruct{
			"hello",
			42,
			true,
		},
		map[string]any{
			"a": "world",
			"c": false,
		},
	)
	require.NoError(t, err)
	require.EqualValues(t, out, testStruct{
		"world",
		42,
		false,
	})

	_, err = MergeJSON(out, map[string]any{
		"d": "shouldn't show up",
	})
	require.Error(t, err)
}
