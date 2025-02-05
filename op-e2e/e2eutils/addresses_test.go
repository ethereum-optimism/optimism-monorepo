package e2eutils

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-e2e/config"

	"github.com/stretchr/testify/require"
)

func TestCollectAddresses(t *testing.T) {
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
	addrs := CollectAddresses(sd, dp)
	require.NotEmpty(t, addrs)
	require.Contains(t, addrs, dp.Addresses.Batcher)
}
