package host

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum-optimism/optimism/op-program/chainconfig"
	"github.com/ethereum-optimism/optimism/op-program/client"
	"github.com/ethereum-optimism/optimism/op-program/client/l1"
	"github.com/ethereum-optimism/optimism/op-program/host/config"
	"github.com/ethereum-optimism/optimism/op-program/host/kvstore"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

func TestServerMode(t *testing.T) {
	dir := t.TempDir()

	l1Head := common.Hash{0x11}
	l2OutputRoot := common.Hash{0x33}
	cfg := config.NewConfig(chaincfg.OPSepolia(), chainconfig.OPSepoliaChainConfig(), l1Head, common.Hash{0x22}, l2OutputRoot, common.Hash{0x44}, 1000)
	cfg.DataDir = dir
	cfg.ServerMode = true

	preimageServer, preimageClient, err := preimage.CreateBidirectionalChannel()
	require.NoError(t, err)
	defer preimageClient.Close()
	hintServer, hintClient, err := preimage.CreateBidirectionalChannel()
	require.NoError(t, err)
	defer hintClient.Close()
	logger := testlog.Logger(t, log.LevelTrace)
	result := make(chan error)
	go func() {
		result <- PreimageServer(context.Background(), logger, cfg, preimageServer, hintServer, makeDefaultPrefetcher)
	}()

	pClient := preimage.NewOracleClient(preimageClient)
	hClient := preimage.NewHintWriter(hintClient)
	l1PreimageOracle := l1.NewPreimageOracle(pClient, hClient)

	require.Equal(t, l1Head.Bytes(), pClient.Get(client.L1HeadLocalIndex), "Should get l1 head preimages")
	require.Equal(t, l2OutputRoot.Bytes(), pClient.Get(client.L2OutputRootLocalIndex), "Should get l2 output root preimages")

	// Should exit when a preimage is unavailable
	require.Panics(t, func() {
		l1PreimageOracle.HeaderByBlockHash(common.HexToHash("0x1234"))
	}, "Preimage should not be available")
	require.ErrorIs(t, waitFor(result), kvstore.ErrNotFound)
}

func waitFor(ch chan error) error {
	timeout := time.After(30 * time.Second)
	select {
	case err := <-ch:
		return err
	case <-timeout:
		return errors.New("timed out")
	}
}
