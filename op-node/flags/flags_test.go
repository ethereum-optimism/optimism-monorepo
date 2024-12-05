package flags

import (
	"slices"
	"strings"
	"testing"

	opservice "github.com/ethereum-optimism/optimism/op-service"

	"github.com/stretchr/testify/require"
)

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

func TestDeprecatedFlagsAreHidden(t *testing.T) {
	for _, flag := range DeprecatedFlags {
		flag := flag
		flagName := flag.Names()[0]

		t.Run(flagName, func(t *testing.T) {
			visibleFlag, ok := flag.(interface {
				IsVisible() bool
			})
			require.True(t, ok, "Need to case the flag to the correct format")
			require.False(t, visibleFlag.IsVisible())
		})
	}
}

func TestHasEnvVar(t *testing.T) {
	// known exceptions to the number of env vars
	expEnvVars := map[string]int{
		BeaconFallbackAddrs.Name: 2,
	}

	for _, flag := range Flags {
		flag := flag
		flagName := flag.Names()[0]

		t.Run(flagName, func(t *testing.T) {
			if flagName == PeerScoringName || flagName == PeerScoreBandsName || flagName == TopicScoringName {
				t.Skipf("Skipping flag %v which is known to have no env vars", flagName)
			}
			envFlagGetter, ok := flag.(interface {
				GetEnvVars() []string
			})
			require.True(t, ok, "must be able to cast the flag to an EnvVar interface")
			envFlags := envFlagGetter.GetEnvVars()
			if numEnvVars, ok := expEnvVars[flagName]; ok {
				require.Equalf(t, numEnvVars, len(envFlags), "flags should have %d env vars", numEnvVars)
			} else {
				require.Equal(t, 1, len(envFlags), "flags should have exactly one env var")
			}
		})
	}
}

func TestEnvVarFormat(t *testing.T) {
	skippedFlags := []string{
		L1NodeAddr.Name,
		L2EngineAddr.Name,
		L2EngineJWTSecret.Name,
		L1TrustRPC.Name,
		L1RPCProviderKind.Name,
		L2EngineKind.Name,
		SnapshotLog.Name,
		BackupL2UnsafeSyncRPC.Name,
		BackupL2UnsafeSyncRPCTrustRPC.Name,
		"p2p.scoring",
		"p2p.ban.peers",
		"p2p.ban.threshold",
		"p2p.ban.duration",
		"p2p.listen.tcp",
		"p2p.listen.udp",
		"p2p.useragent",
		"p2p.gossip.mesh.lo",
		"p2p.gossip.mesh.floodpublish",
		"l2.engine-sync",
	}

	for _, flag := range Flags {
		flag := flag
		flagName := flag.Names()[0]

		t.Run(flagName, func(t *testing.T) {
			if slices.Contains(skippedFlags, flagName) {
				t.Skipf("Skipping flag %v which is known to not have a standard flag name <-> env var conversion", flagName)
			}
			if flagName == PeerScoringName || flagName == PeerScoreBandsName || flagName == TopicScoringName {
				t.Skipf("Skipping flag %v which is known to have no env vars", flagName)
			}
			envFlagGetter, ok := flag.(interface {
				GetEnvVars() []string
			})
			require.True(t, ok, "must be able to cast the flag to an EnvVar interface")
			envFlags := envFlagGetter.GetEnvVars()
			expectedEnvVar := opservice.FlagNameToEnvVarName(flagName, "OP_NODE")
			require.Equal(t, expectedEnvVar, envFlags[0])
		})
	}
}
