package super

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-program/client/claim"
	"github.com/ethereum-optimism/optimism/op-program/host"
	"github.com/ethereum-optimism/optimism/op-program/host/config"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/require"
)

type transitionTest struct {
	name           string
	startTimestamp uint64
	agreedClaim    []byte
	disputedClaim  []byte
	expectValid    bool
}

type OptimisticBlock struct {
	BlockHash  common.Hash
	OutputRoot eth.Bytes32
}

type IntermediateRoot struct {
	SuperRoot       []byte
	PendingProgress []OptimisticBlock
	Step            uint64
}

func TestValidTransitionBetweenTimestamps(t *testing.T) {
	ctx := context.Background()
	startTimestamp := uint64(1736345621)
	endTimestamp := startTimestamp + 1
	source := createSuperRootSource(t, ctx)
	start, err := source.CreateSuperRoot(ctx, startTimestamp)
	require.NoError(t, err)
	end, err := source.CreateSuperRoot(ctx, endTimestamp)
	require.NoError(t, err)

	serializeIntermediateRoot := func(root *IntermediateRoot) []byte {
		data, err := rlp.EncodeToBytes(root)
		require.NoError(t, err)
		return data
	}

	//chain1Start, err := source.chains[0].source.OutputAtBlock(ctx, source.chains[0].blockNumberAtTime(startTimestamp))
	//require.NoError(t, err)
	chain1End, err := source.chains[0].source.OutputAtBlock(ctx, source.chains[0].blockNumberAtTime(endTimestamp))
	require.NoError(t, err)

	//chain2Start, err := source.chains[1].source.OutputAtBlock(ctx, source.chains[1].blockNumberAtTime(startTimestamp))
	//require.NoError(t, err)
	chain2End, err := source.chains[1].source.OutputAtBlock(ctx, source.chains[1].blockNumberAtTime(endTimestamp))
	require.NoError(t, err)

	step1Expected := serializeIntermediateRoot(&IntermediateRoot{
		SuperRoot: start.Marshal(),
		PendingProgress: []OptimisticBlock{
			{BlockHash: chain1End.BlockRef.Hash, OutputRoot: chain1End.OutputRoot},
		},
		Step: 1,
	})

	step2Expected := serializeIntermediateRoot(&IntermediateRoot{
		SuperRoot: start.Marshal(),
		PendingProgress: []OptimisticBlock{
			{BlockHash: chain1End.BlockRef.Hash, OutputRoot: chain1End.OutputRoot},
			{BlockHash: chain2End.BlockRef.Hash, OutputRoot: chain2End.OutputRoot},
		},
		Step: 2,
	})

	step3Expected := serializeIntermediateRoot(&IntermediateRoot{
		SuperRoot: start.Marshal(),
		PendingProgress: []OptimisticBlock{
			{BlockHash: chain1End.BlockRef.Hash, OutputRoot: chain1End.OutputRoot},
			{BlockHash: chain2End.BlockRef.Hash, OutputRoot: chain2End.OutputRoot},
		},
		Step: 3,
	})

	tests := []*transitionTest{
		{
			name:           "ClaimNoChange",
			startTimestamp: startTimestamp,
			agreedClaim:    start.Marshal(),
			disputedClaim:  start.Marshal(),
			expectValid:    false,
		},
		{
			name:           "ClaimDirectToNextTimestamp",
			startTimestamp: startTimestamp,
			agreedClaim:    start.Marshal(),
			disputedClaim:  end.Marshal(),
			expectValid:    false,
		},
		{
			name:           "FirstChainOptimisticBlock",
			startTimestamp: startTimestamp,
			agreedClaim:    start.Marshal(),
			disputedClaim:  step1Expected,
			expectValid:    true,
		},
		{
			name:           "SecondChainOptimisticBlock",
			startTimestamp: startTimestamp,
			agreedClaim:    step1Expected,
			disputedClaim:  step2Expected,
			expectValid:    true,
		},
		{
			name:           "PaddingStep",
			startTimestamp: startTimestamp,
			agreedClaim:    step2Expected,
			disputedClaim:  step3Expected,
			expectValid:    true,
		},
		{
			name:           "Consolidate",
			startTimestamp: startTimestamp,
			agreedClaim:    step3Expected,
			disputedClaim:  end.Marshal(),
			expectValid:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fmt.Printf("Timestamp: %v\n Agreed: %x\n Disputed: %x\nValid: %v\n",
				test.startTimestamp, test.agreedClaim, test.disputedClaim, test.expectValid)
			err := host.FaultProofProgram(ctx, testlog.Logger(t, slog.LevelInfo), &config.Config{
				L2ChainID:          0,
				Rollup:             nil,
				DataDir:            "",
				DataFormat:         "",
				L1Head:             common.Hash{},
				L1URL:              "",
				L1BeaconURL:        "",
				L1TrustRPC:         false,
				L1RPCKind:          "",
				L2Head:             common.Hash{},
				L2OutputRoot:       common.Hash{},
				L2URL:              "",
				L2ExperimentalURL:  "",
				L2Claim:            common.Hash{},
				L2ClaimBlockNumber: 0,
				L2ChainConfig:      nil,
				ExecCmd:            "",
				ServerMode:         false,
			})
			if test.expectValid {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, claim.ErrClaimNotValid)
			}
		})
	}
}

// Transition where only one chain has a new block

func createSuperRootSource(t *testing.T, ctx context.Context) *SuperRootSource {
	logger := testlog.Logger(t, slog.LevelInfo)
	urls := []string{
		"https://sepolia-replica-0-op-node.primary.client.dev.oplabs.cloud",
		"https://unichain-replica-0-op-node.primary.sepolia.prod.oplabs.cloud",
	}
	var sources []OutputRootSource
	for _, url := range urls {
		client, err := dial.DialRollupClientWithTimeout(ctx, time.Minute, logger, url)
		require.NoError(t, err)
		sources = append(sources, client)
	}
	source, err := NewSuperRootSource(ctx, sources...)
	require.NoError(t, err)
	return source
}
