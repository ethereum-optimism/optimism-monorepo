package interop

import (
	"context"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/dial"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-chain-ops/interopgen"
	"github.com/ethereum-optimism/optimism/op-e2e/system/helpers"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	gethCore "github.com/ethereum/go-ethereum/core"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
)

// setupAndRun is a helper function that sets up a SuperSystem
// which contains two L2 Chains, and two users on each chain.
func setupAndRun(t *testing.T, config SuperSystemConfig, fn func(*testing.T, SuperSystem)) {
	recipe := interopgen.InteropDevRecipe{
		L1ChainID:        900100,
		L2ChainIDs:       []uint64{900200, 900201},
		GenesisTimestamp: uint64(time.Now().Unix() + 3), // start chain 3 seconds from now
	}
	worldResources := worldResourcePaths{
		foundryArtifacts: "../../packages/contracts-bedrock/forge-artifacts",
		sourceMap:        "../../packages/contracts-bedrock",
	}

	// create a super system from the recipe
	// and get the L2 IDs for use in the test
	s2 := NewSuperSystem(t, &recipe, worldResources, config)

	// create two users on all L2 chains
	s2.AddUser("Alice")
	s2.AddUser("Bob")

	// run the test
	fn(t, s2)
}

// TestInterop_IsolatedChains tests a simple interop scenario
// Chains A and B exist, but no messages are sent between them
// a transaction is sent from Alice to Bob on Chain A,
// and only Chain A is affected.
func TestInterop_IsolatedChains(t *testing.T) {
	test := func(t *testing.T, s2 SuperSystem) {
		ids := s2.L2IDs()
		chainA := ids[0]
		chainB := ids[1]

		// check the balance of Bob
		bobAddr := s2.Address(chainA, "Bob")
		clientA := s2.L2GethClient(chainA)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		bobBalance, err := clientA.BalanceAt(ctx, bobAddr, nil)
		require.NoError(t, err)
		expectedBalance, _ := big.NewInt(0).SetString("10000000000000000000000000", 10)
		require.Equal(t, expectedBalance, bobBalance)

		// send a tx from Alice to Bob
		s2.SendL2Tx(
			chainA,
			"Alice",
			func(l2Opts *helpers.TxOpts) {
				l2Opts.ToAddr = &bobAddr
				l2Opts.Value = big.NewInt(1000000)
				l2Opts.GasFeeCap = big.NewInt(1_000_000_000)
				l2Opts.GasTipCap = big.NewInt(1_000_000_000)
			},
		)

		// check the balance of Bob after the tx
		ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		bobBalance, err = clientA.BalanceAt(ctx, bobAddr, nil)
		require.NoError(t, err)
		expectedBalance, _ = big.NewInt(0).SetString("10000000000000000001000000", 10)
		require.Equal(t, expectedBalance, bobBalance)

		// check that the balance of Bob on ChainB hasn't changed
		bobAddrB := s2.Address(chainB, "Bob")
		clientB := s2.L2GethClient(chainB)
		ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		bobBalance, err = clientB.BalanceAt(ctx, bobAddrB, nil)
		require.NoError(t, err)
		expectedBalance, _ = big.NewInt(0).SetString("10000000000000000000000000", 10)
		require.Equal(t, expectedBalance, bobBalance)
	}
	config := SuperSystemConfig{
		mempoolFiltering: false,
	}
	setupAndRun(t, config, test)
}

// TestInterop_EmitLogs tests a simple interop scenario
// Chains A and B exist, but no messages are sent between them.
// A contract is deployed on each chain, and logs are emitted repeatedly.
func TestInterop_EmitLogs(t *testing.T) {
	test := func(t *testing.T, s2 SuperSystem) {
		ids := s2.L2IDs()
		chainA := ids[0]
		chainB := ids[1]
		EmitterA := s2.DeployEmitterContract(chainA, "Alice")
		EmitterB := s2.DeployEmitterContract(chainB, "Alice")
		payload1 := "SUPER JACKPOT!"
		numEmits := 10
		// emit logs on both chains in parallel
		var emitParallel sync.WaitGroup
		emitOn := func(chainID string) {
			for i := 0; i < numEmits; i++ {
				s2.EmitData(chainID, "Alice", payload1)
			}
			emitParallel.Done()
		}
		emitParallel.Add(2)
		go emitOn(chainA)
		go emitOn(chainB)
		emitParallel.Wait()

		clientA := s2.L2GethClient(chainA)
		clientB := s2.L2GethClient(chainB)
		// check that the logs are emitted on chain A
		qA := ethereum.FilterQuery{
			Addresses: []common.Address{EmitterA},
		}
		logsA, err := clientA.FilterLogs(context.Background(), qA)
		require.NoError(t, err)
		require.Len(t, logsA, numEmits)

		// check that the logs are emitted on chain B
		qB := ethereum.FilterQuery{
			Addresses: []common.Address{EmitterB},
		}
		logsB, err := clientB.FilterLogs(context.Background(), qB)
		require.NoError(t, err)
		require.Len(t, logsB, numEmits)

		// wait for cross-safety to settle
		// I've tried 30s but not all logs are cross-safe by then
		time.Sleep(60 * time.Second)

		supervisor := s2.SupervisorClient()

		// requireMessage checks the safety level of a log against the supervisor
		// it also checks that the error is as expected
		requireMessage := func(chainID string, log gethTypes.Log, expectedSafety types.SafetyLevel, expectedError error) {
			client := s2.L2GethClient(chainID)
			// construct the expected hash of the log's payload
			// (topics concatenated with data)
			msgPayload := make([]byte, 0)
			for _, topic := range log.Topics {
				msgPayload = append(msgPayload, topic.Bytes()...)
			}
			msgPayload = append(msgPayload, log.Data...)
			expectedHash := common.BytesToHash(crypto.Keccak256(msgPayload))

			// get block for the log (for timestamp)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			block, err := client.BlockByHash(ctx, log.BlockHash)
			require.NoError(t, err)

			// make an identifier out of the sample log
			identifier := types.Identifier{
				Origin:      log.Address,
				BlockNumber: log.BlockNumber,
				LogIndex:    uint32(log.Index),
				Timestamp:   block.Time(),
				ChainID:     types.ChainIDFromBig(s2.ChainID(chainID)),
			}

			safety, err := supervisor.CheckMessage(context.Background(),
				identifier,
				expectedHash,
			)
			require.ErrorIs(t, err, expectedError)
			// the supervisor could progress the safety level more quickly than we expect,
			// which is why we check for a minimum safety level
			require.True(t, safety.AtLeastAsSafe(expectedSafety), "log: %v should be at least %s, but is %s", log, expectedSafety.String(), safety.String())
		}
		// all logs should be cross-safe
		for _, log := range logsA {
			requireMessage(chainA, log, types.CrossSafe, nil)
		}
		for _, log := range logsB {
			requireMessage(chainB, log, types.CrossSafe, nil)
		}
	}
	config := SuperSystemConfig{
		mempoolFiltering: false,
	}
	setupAndRun(t, config, test)
}

func TestInteropBlockBuilding(t *testing.T) {
	logger := testlog.Logger(t, log.LevelInfo)
	oplog.SetGlobalLogHandler(logger.Handler())

	test := func(t *testing.T, s2 SuperSystem) {
		ids := s2.L2IDs()
		chainA := ids[0]
		chainB := ids[1]
		// We will initiate on chain A, and execute on chain B
		s2.DeployEmitterContract(chainA, "Alice")

		// Add chain A as dependency to chain B,
		// such that we can execute a message on B that was initiated on A.
		depRec := s2.AddDependency(chainB, s2.ChainID(chainA))
		t.Logf("Dependency set in L1 block %d", depRec.BlockNumber)

		rollupClA, err := dial.DialRollupClientWithTimeout(context.Background(), time.Second*15, logger, s2.OpNode(chainA).UserRPC().RPC())
		require.NoError(t, err)

		// Now wait for the dependency to be visible in the L2 (receipt needs to be picked up)
		require.Eventually(t, func() bool {
			status, err := rollupClA.SyncStatus(context.Background())
			require.NoError(t, err)
			return status.CrossUnsafeL2.L1Origin.Number >= depRec.BlockNumber.Uint64()
		}, time.Second*30, time.Second, "wait for L1 origin to match dependency L1 block")
		t.Log("Dependency information has been processed in L2 block")

		// emit log on chain A
		emitRec := s2.EmitData(chainA, "Alice", "hello world")
		t.Logf("Emitted a log event in block %d", emitRec.BlockNumber.Uint64())

		// Wait for initiating side to become cross-unsafe
		require.Eventually(t, func() bool {
			status, err := rollupClA.SyncStatus(context.Background())
			require.NoError(t, err)
			return status.CrossUnsafeL2.Number >= emitRec.BlockNumber.Uint64()
		}, time.Second*60, time.Second, "wait for emitted data to become cross-unsafe")
		t.Logf("Reached cross-unsafe block %d", emitRec.BlockNumber.Uint64())

		// Identify the log
		require.Len(t, emitRec.Logs, 1)
		ev := emitRec.Logs[0]
		ethCl := s2.L2GethClient(chainA)
		header, err := ethCl.HeaderByHash(context.Background(), emitRec.BlockHash)
		require.NoError(t, err)
		identifier := types.Identifier{
			Origin:      ev.Address,
			BlockNumber: ev.BlockNumber,
			LogIndex:    uint32(ev.Index),
			Timestamp:   header.Time,
			ChainID:     types.ChainIDFromBig(s2.ChainID(chainA)),
		}

		msgPayload := types.LogToMessagePayload(ev)
		payloadHash := crypto.Keccak256Hash(msgPayload)
		logHash := types.PayloadHashToLogHash(payloadHash, identifier.Origin)
		t.Logf("expected payload hash: %s", payloadHash)
		t.Logf("expected log hash: %s", logHash)

		invalidPayload := []byte("test invalid message")
		invalidPayloadHash := crypto.Keccak256Hash(invalidPayload)
		invalidLogHash := types.PayloadHashToLogHash(invalidPayloadHash, identifier.Origin)
		t.Logf("invalid payload hash: %s", invalidPayloadHash)
		t.Logf("invalid log hash: %s", invalidLogHash)

		// submit executing txs on B

		t.Log("Testing invalid message")
		{
			bobAddr := s2.Address(chainA, "Bob") // direct it to a random account without code
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
			defer cancel()
			// Send an executing message, but with different payload.
			if s2.(*interopE2ESystem).config.mempoolFiltering {
				// We expect the traqnsaction to be filtered out by the mempool if mempool filtering is enabled.
				// ExecuteMessage the ErrTxFilteredOut error is checked when sending the tx.
				_, err := s2.ExecuteMessage(ctx, chainB, "Alice", identifier, bobAddr, invalidPayload, gethCore.ErrTxFilteredOut)
				require.ErrorContains(t, err, gethCore.ErrTxFilteredOut.Error())
			} else {
				// We expect the miner to be unable to include this tx, and confirmation to thus time out, if mempool filtering is disabled.
				_, err := s2.ExecuteMessage(ctx, chainB, "Alice", identifier, bobAddr, invalidPayload, nil)
				require.ErrorIs(t, err, ctx.Err())
				require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
			}
		}

		t.Log("Testing valid message now")
		{
			bobAddr := s2.Address(chainA, "Bob") // direct it to a random account without code
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
			defer cancel()
			// Send an executing message with the correct identifier / payload
			rec, err := s2.ExecuteMessage(ctx, chainB, "Alice", identifier, bobAddr, msgPayload, nil)
			require.NoError(t, err, "expecting tx to be confirmed")
			t.Logf("confirmed executing msg in block %s", rec.BlockNumber)
		}
		t.Log("Done")
	}

	t.Run("without mempool filtering", func(t *testing.T) {
		config := SuperSystemConfig{
			mempoolFiltering: false,
		}
		setupAndRun(t, config, test)
	})

	t.Run("with mempool filtering", func(t *testing.T) {
		config := SuperSystemConfig{
			mempoolFiltering: true,
		}
		// run again with mempool filtering to observe the behavior of the mempool filter
		setupAndRun(t, config, test)
	})
}
