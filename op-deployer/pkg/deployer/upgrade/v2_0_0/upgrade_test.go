package v2_0_0

import (
	"context"
	"encoding/hex"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/testutils/devnet"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/broadcaster"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/testutil"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/env"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestUpgradeOPChainInput_OpChainConfigs(t *testing.T) {
	input := &UpgradeOPChainInput{
		Prank: common.Address{0xaa},
		Opcm:  common.Address{0xbb},
		EncodedChainConfigs: []OPChainConfig{
			{
				SystemConfigProxy: common.Address{0x01},
				ProxyAdmin:        common.Address{0x02},
				AbsolutePrestate:  common.Hash{0x03},
			},
			{
				SystemConfigProxy: common.Address{0x04},
				ProxyAdmin:        common.Address{0x05},
				AbsolutePrestate:  common.Hash{0x06},
			},
		},
	}
	data, err := input.OpChainConfigs()
	require.NoError(t, err)
	require.Equal(
		t,
		"0000000000000000000000000000000000000000000000000000000000000020"+
			"0000000000000000000000000000000000000000000000000000000000000002"+
			"0000000000000000000000000100000000000000000000000000000000000000"+
			"0000000000000000000000000200000000000000000000000000000000000000"+
			"0300000000000000000000000000000000000000000000000000000000000000"+
			"0000000000000000000000000400000000000000000000000000000000000000"+
			"0000000000000000000000000500000000000000000000000000000000000000"+
			"0600000000000000000000000000000000000000000000000000000000000000",
		hex.EncodeToString(data),
	)
}

func TestUpgrader_Upgrade(t *testing.T) {
	lgr := testlog.Logger(t, slog.LevelDebug)

	forkedL1, stopL1, err := devnet.NewForkedSepolia(lgr)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, stopL1())
	})
	l1RPC := forkedL1.RPCUrl()

	_, afactsFS := testutil.LocalArtifacts(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	rpcClient, err := rpc.Dial(l1RPC)
	require.NoError(t, err)

	bcast := new(broadcaster.CalldataBroadcaster)
	host, err := env.DefaultForkedScriptHost(
		ctx,
		bcast,
		lgr,
		common.Address{'D'},
		afactsFS,
		rpcClient,
	)
	require.NoError(t, err)

	configFile, err := os.ReadFile("testdata/config.json")
	require.NoError(t, err)

	upgrader := DefaultUpgrader
	require.NoError(t, upgrader.Upgrade(host, configFile))

	dump, err := bcast.Dump()
	require.NoError(t, err)

	addr := common.HexToAddress("0x1Eb2fFc903729a0F03966B917003800b145F56E2")
	require.True(t, dump[0].Value.ToInt().Cmp(common.Big0) == 0)
	// Have to do this to normalize zero values which can either set nat to nil
	// or to a zero value. They mean the same thing, but aren't equal according to
	// EqualValues.
	dump[0].Value = (*hexutil.Big)(common.Big0)

	require.EqualValues(t, []broadcaster.CalldataDump{
		{
			To: &addr,
			Data: []byte{
				0xff, 0x2d, 0xd5, 0xa1, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x03, 0x4e, 0xdd, 0x2a, 0x22, 0x5f, 0x7f, 0x42, 0x9a, 0x63, 0xe0,
				0xf1, 0xd2, 0x08, 0x4b, 0x9e, 0x0a, 0x93, 0xb5, 0x38, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x18, 0x9a, 0xba, 0xaa, 0xa8,
				0x2d, 0xfc, 0x01, 0x5a, 0x58, 0x8a, 0x7d, 0xba, 0xd6, 0xf1, 0x3b, 0x1d, 0x34,
				0x85, 0xbc, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xa, 0xbc,
			},
			Value: (*hexutil.Big)(common.Big0),
		},
	}, dump)
}
