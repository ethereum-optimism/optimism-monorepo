package e2eutils

import (
	"encoding/hex"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-e2e/config"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
)

func TestWriteDefaultJWT(t *testing.T) {
	jwtPath := WriteDefaultJWT(t)
	data, err := os.ReadFile(jwtPath)
	require.NoError(t, err)
	require.Equal(t, "0x"+hex.EncodeToString(testingJWTSecret[:]), string(data))
}

func TestSetup(t *testing.T) {
	tp := &TestParams{
		MaxSequencerDrift:   40,
		SequencerWindowSize: 120,
		ChannelTimeout:      120,
		L1BlockTime:         15,
		AllocType:           config.AllocTypeStandard,
	}
	dp := MakeDeployParams(t, tp)
	alloc := &AllocParams{PrefundTestUsers: true}
	sd := Setup(t, dp, alloc)
	require.Contains(t, sd.L1Cfg.Alloc, dp.Addresses.Alice)
	require.Equal(t, sd.L1Cfg.Alloc[dp.Addresses.Alice].Balance, Ether(1e12))

	require.Contains(t, sd.L2Cfg.Alloc, dp.Addresses.Alice)
	require.Equal(t, sd.L2Cfg.Alloc[dp.Addresses.Alice].Balance, Ether(1e12))

	expAllocs := config.L1Deployments(config.DefaultAllocType)
	require.Contains(t, sd.L1Cfg.Alloc, expAllocs.AddressManager)
	require.Contains(t, sd.L2Cfg.Alloc, predeploys.L1BlockAddr)
}
