package p2p

import (
	"context"
	"math/big"
	"slices"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/p2p"
	"github.com/ethereum-optimism/optimism/op-node/rollup/driver"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/log"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/opnode"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/ethereum-optimism/optimism/op-e2e/system/helpers"
	rollupNode "github.com/ethereum-optimism/optimism/op-node/node"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum/go-ethereum/common"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

// TestSystemMockP2P sets up a L1 Geth node, a rollup node, and a L2 geth node and then confirms that
// the nodes can sync L2 blocks before they are confirmed on L1.
func TestSystemMockP2P(t *testing.T) {
	op_e2e.InitParallel(t)

	cfg := e2esys.DefaultSystemConfig(t)
	// Disable batcher, so we don't sync from L1 & set a large sequence window so we only have unsafe blocks
	cfg.DisableBatcher = true
	cfg.DeployConfig.SequencerWindowSize = 100_000
	cfg.DeployConfig.MaxSequencerDrift = 100_000
	// disable at the start, so we don't miss any gossiped blocks.
	cfg.Nodes["sequencer"].Driver.SequencerStopped = true

	// connect the nodes
	cfg.P2PTopology = map[string][]string{
		"verifier": {"sequencer"},
	}

	var published, received []common.Hash
	seqTracer, verifTracer := new(opnode.FnTracer), new(opnode.FnTracer)
	seqTracer.OnPublishL2PayloadFn = func(ctx context.Context, payload *eth.ExecutionPayloadEnvelope) {
		published = append(published, payload.ExecutionPayload.BlockHash)
	}
	verifTracer.OnUnsafeL2PayloadFn = func(ctx context.Context, from peer.ID, payload *eth.ExecutionPayloadEnvelope) {
		received = append(received, payload.ExecutionPayload.BlockHash)
	}
	cfg.Nodes["sequencer"].Tracer = seqTracer
	cfg.Nodes["verifier"].Tracer = verifTracer

	sys, err := cfg.Start(t)
	require.Nil(t, err, "Error starting up system")

	// Enable the sequencer now that everyone is ready to receive payloads.
	rollupClient := sys.RollupClient("sequencer")

	verifierPeerID := sys.RollupNodes["verifier"].P2P().Host().ID()
	check := func() bool {
		sequencerBlocksTopicPeers := sys.RollupNodes["sequencer"].P2P().GossipOut().AllBlockTopicsPeers()
		return slices.Contains[[]peer.ID](sequencerBlocksTopicPeers, verifierPeerID)
	}

	// poll to see if the verifier node is connected & meshed on gossip.
	// Without this verifier, we shouldn't start sending blocks around, or we'll miss them and fail the test.
	backOffStrategy := retry.Exponential()
	for i := 0; i < 10; i++ {
		if check() {
			break
		}
		time.Sleep(backOffStrategy.Duration(i))
	}
	require.True(t, check(), "verifier must be meshed with sequencer for gossip test to proceed")

	require.NoError(t, rollupClient.StartSequencer(context.Background(), sys.L2GenesisCfg.ToBlock().Hash()))

	l2Seq := sys.NodeClient("sequencer")
	l2Verif := sys.NodeClient("verifier")

	// Transactor Account
	ethPrivKey := cfg.Secrets.Alice

	// Submit TX to L2 sequencer node
	receiptSeq := helpers.SendL2Tx(t, cfg, l2Seq, ethPrivKey, func(opts *helpers.TxOpts) {
		opts.ToAddr = &common.Address{0xff, 0xff}
		opts.Value = big.NewInt(1_000_000_000)

		// Wait until the block it was first included in shows up in the safe chain on the verifier
		opts.VerifyOnClients(l2Verif)
	})

	// Verify that everything that was received was published
	require.GreaterOrEqual(t, len(published), len(received))
	require.Subset(t, published, received)

	// Verify that the tx was received via p2p
	require.Contains(t, received, receiptSeq.BlockHash)
}

// TestSystemDenseTopology sets up a dense p2p topology with 3 verifier nodes and 1 sequencer node.
func TestSystemDenseTopology(t *testing.T) {
	t.Skip("Skipping dense topology test to avoid flakiness. @refcell address in p2p scoring pr.")

	op_e2e.InitParallel(t)

	cfg := e2esys.DefaultSystemConfig(t)
	// slow down L1 blocks so we can see the L2 blocks arrive well before the L1 blocks do.
	// Keep the seq window small so the L2 chain is started quick
	cfg.DeployConfig.L1BlockTime = 10

	// Append additional nodes to the system to construct a dense p2p network
	cfg.Nodes["verifier2"] = &rollupNode.Config{
		Driver: driver.Config{
			VerifierConfDepth:  0,
			SequencerConfDepth: 0,
			SequencerEnabled:   false,
		},
		L1EpochPollInterval: time.Second * 4,
	}
	cfg.Nodes["verifier3"] = &rollupNode.Config{
		Driver: driver.Config{
			VerifierConfDepth:  0,
			SequencerConfDepth: 0,
			SequencerEnabled:   false,
		},
		L1EpochPollInterval: time.Second * 4,
	}
	cfg.Loggers["verifier2"] = testlog.Logger(t, log.LevelInfo).New("role", "verifier")
	cfg.Loggers["verifier3"] = testlog.Logger(t, log.LevelInfo).New("role", "verifier")

	// connect the nodes
	cfg.P2PTopology = map[string][]string{
		"verifier":  {"sequencer", "verifier2", "verifier3"},
		"verifier2": {"sequencer", "verifier", "verifier3"},
		"verifier3": {"sequencer", "verifier", "verifier2"},
	}

	// Set peer scoring for each node, but without banning
	for _, node := range cfg.Nodes {
		params, err := p2p.GetScoringParams("light", &node.Rollup)
		require.NoError(t, err)
		node.P2P = &p2p.Config{
			ScoringParams:  params,
			BanningEnabled: false,
		}
	}

	var published, received1, received2, received3 []common.Hash
	seqTracer, verifTracer, verifTracer2, verifTracer3 := new(opnode.FnTracer), new(opnode.FnTracer), new(opnode.FnTracer), new(opnode.FnTracer)
	seqTracer.OnPublishL2PayloadFn = func(ctx context.Context, payload *eth.ExecutionPayloadEnvelope) {
		published = append(published, payload.ExecutionPayload.BlockHash)
	}
	verifTracer.OnUnsafeL2PayloadFn = func(ctx context.Context, from peer.ID, payload *eth.ExecutionPayloadEnvelope) {
		received1 = append(received1, payload.ExecutionPayload.BlockHash)
	}
	verifTracer2.OnUnsafeL2PayloadFn = func(ctx context.Context, from peer.ID, payload *eth.ExecutionPayloadEnvelope) {
		received2 = append(received2, payload.ExecutionPayload.BlockHash)
	}
	verifTracer3.OnUnsafeL2PayloadFn = func(ctx context.Context, from peer.ID, payload *eth.ExecutionPayloadEnvelope) {
		received3 = append(received3, payload.ExecutionPayload.BlockHash)
	}
	cfg.Nodes["sequencer"].Tracer = seqTracer
	cfg.Nodes["verifier"].Tracer = verifTracer
	cfg.Nodes["verifier2"].Tracer = verifTracer2
	cfg.Nodes["verifier3"].Tracer = verifTracer3

	sys, err := cfg.Start(t)
	require.Nil(t, err, "Error starting up system")

	l2Seq := sys.NodeClient("sequencer")
	l2Verif := sys.NodeClient("verifier")
	l2Verif2 := sys.NodeClient("verifier2")
	l2Verif3 := sys.NodeClient("verifier3")

	// Transactor Account
	ethPrivKey := cfg.Secrets.Alice

	// Submit TX to L2 sequencer node
	receiptSeq := helpers.SendL2Tx(t, cfg, l2Seq, ethPrivKey, func(opts *helpers.TxOpts) {
		opts.ToAddr = &common.Address{0xff, 0xff}
		opts.Value = big.NewInt(1_000_000_000)

		// Wait until the block it was first included in shows up in the safe chain on the verifiers
		opts.VerifyOnClients(l2Verif, l2Verif2, l2Verif3)
	})

	// Verify that everything that was received was published
	require.GreaterOrEqual(t, len(published), len(received1))
	require.GreaterOrEqual(t, len(published), len(received2))
	require.GreaterOrEqual(t, len(published), len(received3))
	require.ElementsMatch(t, published, received1[:len(published)])
	require.ElementsMatch(t, published, received2[:len(published)])
	require.ElementsMatch(t, published, received3[:len(published)])

	// Verify that the tx was received via p2p
	require.Contains(t, received1, receiptSeq.BlockHash)
	require.Contains(t, received2, receiptSeq.BlockHash)
	require.Contains(t, received3, receiptSeq.BlockHash)
}
