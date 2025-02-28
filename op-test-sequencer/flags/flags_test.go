package flags

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	opservice "github.com/ethereum-optimism/optimism/op-service"
)

// TestOptionalFlagsDontSetRequired asserts that all flags deemed optional set
// the Required field to false.
func TestOptionalFlagsDontSetRequired(t *testing.T) {
	for _, flag := range optionalFlags {
		reqFlag, ok := flag.(cli.RequiredFlag)
		require.True(t, ok)
		require.False(t, reqFlag.IsRequired())
	}
}

// TestUniqueFlags asserts that all flag names are unique, to avoid accidental conflicts between the many flags.
func TestUniqueFlags(t *testing.T) {
	seenCLI := make(map[string]struct{})
	for _, flag := range Flags {
		for _, name := range flag.Names() {
			if _, ok := seenCLI[name]; ok {
				t.Errorf("duplicate flag %s", name)
				continue
			}
			seenCLI[name] = struct{}{}
		}
	}
}

// TestBetaFlags test that all flags starting with "beta." have "BETA_" in the env var, and vice versa.
func TestBetaFlags(t *testing.T) {
	for _, flag := range Flags {
		envFlag, ok := flag.(interface {
			GetEnvVars() []string
		})
		if !ok || len(envFlag.GetEnvVars()) == 0 { // skip flags without env-var support
			continue
		}
		name := flag.Names()[0]
		envName := envFlag.GetEnvVars()[0]
		if strings.HasPrefix(name, "beta.") {
			require.Contains(t, envName, "BETA_", "%q flag must contain BETA in env var to match \"beta.\" flag name", name)
		}
		if strings.Contains(envName, "BETA_") {
			require.True(t, strings.HasPrefix(name, "beta."), "%q flag must start with \"beta.\" in flag name to match \"BETA_\" env var", name)
		}
	}
}

func TestHasEnvVar(t *testing.T) {
	for _, flag := range Flags {
		flag := flag
		flagName := flag.Names()[0]

		t.Run(flagName, func(t *testing.T) {
			envFlagGetter, ok := flag.(interface {
				GetEnvVars() []string
			})
			envFlags := envFlagGetter.GetEnvVars()
			require.True(t, ok, "must be able to cast the flag to an EnvVar interface")
			require.Equal(t, 1, len(envFlags), "flags should have exactly one env var")
		})
	}
}

func TestEnvVarFormat(t *testing.T) {
	for _, flag := range Flags {
		flag := flag
		flagName := flag.Names()[0]

		t.Run(flagName, func(t *testing.T) {
			envFlagGetter, ok := flag.(interface {
				GetEnvVars() []string
			})
			envFlags := envFlagGetter.GetEnvVars()
			require.True(t, ok, "must be able to cast the flag to an EnvVar interface")
			require.Equal(t, 1, len(envFlags), "flags should have exactly one env var")
			expectedEnvVar := opservice.FlagNameToEnvVarName(flagName, "OP_TEST_SEQUENCER")
			require.Equal(t, expectedEnvVar, envFlags[0])
		})
	}
}
