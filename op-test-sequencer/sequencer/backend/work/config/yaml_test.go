package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestYamlLoader_Load(t *testing.T) {
	x := &YamlLoader{Path: filepath.Join(".", "testdata", "config.yaml")}
	result, err := x.Load(context.Background())
	require.NoError(t, err)
	static := result.(*Ensemble)
	require.NotEmpty(t, static.Builders)
	require.NotEmpty(t, static.Signers)
	require.NotEmpty(t, static.Committers)
	require.NotEmpty(t, static.Publishers)
	require.NotEmpty(t, static.Sequencers)
}

func TestYamlLoader_NotFound(t *testing.T) {
	x := &YamlLoader{Path: filepath.Join(t.TempDir(), "missing.yaml")}
	_, err := x.Load(context.Background())
	require.ErrorContains(t, err, "failed to read config")
}

func TestYamlLoader_Invalid(t *testing.T) {
	p := filepath.Join(t.TempDir(), "invalid.yaml")
	// Strictly speaking a valid yaml map, but missing all the data.
	// The config decoder is strict
	require.NoError(t, os.WriteFile(p, []byte("foobar: invalid"), 0755))

	x := &YamlLoader{Path: p}
	_, err := x.Load(context.Background())
	require.ErrorContains(t, err, "field foobar not found")
}
