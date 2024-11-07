package consensus

import (
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

func TestCommitAndRead(t *testing.T) {
	log := testlog.Logger(t, log.LevelInfo)
	now := uint64(time.Now().Unix())
	rollupCfg := &rollup.Config{
		CanyonTime: &now,
	}
	storageDir := "/tmp/sequencerA"
	if err := os.RemoveAll(storageDir); err != nil {
		t.Fatal(err)
	}
	raftConsensusConfig := &RaftConsensusConfig{
		ServerID:          "SequencerA",
		ListenPort:        0,
		ListenAddr:        "127.0.0.1", // local test, don't bind to external interface
		AdvertisedAddr:    "",          // use local address that the server binds to
		StorageDir:        storageDir,
		Bootstrap:         true,
		RollupCfg:         rollupCfg,
		SnapshotInterval:  120 * time.Second,
		SnapshotThreshold: 10240,
		TrailingLogs:      8192,
	}

	cons, err := NewRaftConsensus(log, raftConsensusConfig)
	require.NoError(t, err)

	// wait till it became leader
	<-cons.LeaderCh()

	// eth.BlockV1
	payload := &eth.ExecutionPayloadEnvelope{
		ExecutionPayload: &eth.ExecutionPayload{
			BlockNumber:  1,
			Timestamp:    hexutil.Uint64(now - 20),
			Transactions: []eth.Data{},
			ExtraData:    []byte{},
		},
	}

	err = cons.CommitUnsafePayload(payload)
	// ExecutionPayloadEnvelope is expected to fail when unmarshalling a blockV1
	require.Error(t, err)

	// eth.BlockV3
	one := hexutil.Uint64(1)
	hash := common.HexToHash("0x12345")
	payload = &eth.ExecutionPayloadEnvelope{
		ParentBeaconBlockRoot: &hash,
		ExecutionPayload: &eth.ExecutionPayload{
			BlockNumber:   2,
			Timestamp:     hexutil.Uint64(time.Now().Unix()),
			Transactions:  []eth.Data{},
			ExtraData:     []byte{},
			Withdrawals:   &types.Withdrawals{},
			ExcessBlobGas: &one,
			BlobGasUsed:   &one,
		},
	}

	err = cons.CommitUnsafePayload(payload)
	// ExecutionPayloadEnvelope is expected to succeed when unmarshalling a blockV3
	require.NoError(t, err)

	unsafeHead, err := cons.LatestUnsafePayload()
	require.NoError(t, err)
	require.Equal(t, payload, unsafeHead)
}
