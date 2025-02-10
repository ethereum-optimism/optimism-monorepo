package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultCLIConfig()
	require.NoError(t, cfg.Check())
}
