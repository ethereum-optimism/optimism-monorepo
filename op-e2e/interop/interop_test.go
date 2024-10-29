package interop

import (
	"context"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-chain-ops/interopgen"
	"github.com/ethereum-optimism/optimism/op-e2e/system/helpers"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
)

// setupAndRun is a helper function that sets up a SuperSystem
// which contains two L2 Chains, and two users on each chain.
func setupAndRun(t *testing.T, fn func(*testing.T, SuperSystem)) {
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
	s2 := NewSuperSystem(t, &recipe, worldResources)

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
	setupAndRun(t, test)
}

// TestInteropTrivial_EmitLogs tests a simple interop scenario
// Chains A and B exist, but no messages are sent between them.
// A contract is deployed on each chain, and logs are emitted repeatedly.
func TestInteropTrivial_EmitLogs(t *testing.T) {
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
			// convert payload hash to log hash
			logHash := types.PayloadHashToLogHash(expectedHash, log.Address)

			// get block for the log (for timestamp)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			block, err := client.BlockByHash(ctx, log.BlockHash)
			require.NoError(t, err)

			// make an identifier out of the sample log
			identifier := types.Identifier{
				Origin:      log.Address,
				BlockNumber: log.BlockNumber,
				LogIndex:    uint64(log.Index),
				Timestamp:   block.Time(),
				ChainID:     types.ChainIDFromBig(s2.ChainID(chainID)),
			}

			safety, error := supervisor.CheckMessage(context.Background(),
				identifier,
				logHash,
			)
			require.ErrorIs(t, error, expectedError)
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
	setupAndRun(t, test)
}
