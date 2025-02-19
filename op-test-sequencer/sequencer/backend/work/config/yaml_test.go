package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestYamlLoader_Load(t *testing.T) {
	x := &YamlLoader{Path: "./testdata/config.yaml"}
	result, err := x.Load(context.Background())
	require.NoError(t, err)
	static := result.(*Ensemble)
	require.NotEmpty(t, static.Builders)
	require.NotEmpty(t, static.Signers)
	require.NotEmpty(t, static.Committers)
	require.NotEmpty(t, static.Publishers)
	require.NotEmpty(t, static.Sequencers)
}
