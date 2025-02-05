package faultproofs

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"math/big"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"
	"github.com/ethereum-optimism/optimism/op-e2e/config"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

func TestBenchmarkCannonFPP_Standard(t *testing.T) {
	testBenchmarkCannonFPP(t, config.AllocTypeStandard)
}

func TestBenchmarkCannonFPP_Multithreaded(t *testing.T) {
	testBenchmarkCannonFPP(t, config.AllocTypeMTCannon)
}

func testBenchmarkCannonFPP(t *testing.T, allocType config.AllocType) {
	t.Skip("TODO(client-pod#906): Compare total witness size for assertions against pages allocated by the VM")

	op_e2e.InitParallel(t, op_e2e.UsesCannon)
	ctx := context.Background()
	cfg := e2esys.DefaultSystemConfig(t, e2esys.WithAllocType(allocType))
	// We don't need a verifier - just the sequencer is enough
	delete(cfg.Nodes, "verifier")
	minTs := hexutil.Uint64(0)
	cfg.DeployConfig.L2GenesisDeltaTimeOffset = &minTs
	cfg.DeployConfig.L2GenesisEcotoneTimeOffset = &minTs

	sys, err := cfg.Start(t)
	require.Nil(t, err, "Error starting up system")

	log := testlog.Logger(t, log.LevelInfo)
	log.Info("genesis", "l2", sys.RollupConfig.Genesis.L2, "l1", sys.RollupConfig.Genesis.L1, "l2_time", sys.RollupConfig.Genesis.L2Time)

	l1Client := sys.NodeClient("l1")
	l2Seq := sys.NodeClient("sequencer")
	rollupClient := sys.RollupClient("sequencer")
	require.NoError(t, wait.ForUnsafeBlock(ctx, rollupClient, 1))

	// Agreed state: 200 Big Contracts deployed at max size - total codesize is 5.90 MB
	// In Fault Proof: Perform multicalls calling each Big Contract
	//  - induces 200 oracle.CodeByHash preimage loads
	// Assertion: Under 2000 pages requested by the program (i.e. max ~8 MB). Assumes derivation overhead; block finalization, etc, requires < 1 MB of program memory.

	const numCreates = 200
	newContracts := createBigContracts(ctx, t, cfg, l2Seq, cfg.Secrets.Alice, numCreates)
	receipt := callBigContracts(ctx, t, cfg, l2Seq, cfg.Secrets.Alice, newContracts)

	t.Log("Capture the latest L2 head that preceedes contract creations as agreed starting point")
	agreedBlock, err := l2Seq.BlockByNumber(ctx, new(big.Int).Sub(receipt.BlockNumber, big.NewInt(1)))
	require.NoError(t, err)
	agreedL2Output, err := rollupClient.OutputAtBlock(ctx, agreedBlock.NumberU64())
	require.NoError(t, err, "could not retrieve l2 agreed block")
	l2Head := agreedL2Output.BlockRef.Hash
	l2OutputRoot := agreedL2Output.OutputRoot

	t.Log("Determine L2 claim")
	l2ClaimBlockNumber := receipt.BlockNumber
	l2Output, err := rollupClient.OutputAtBlock(ctx, l2ClaimBlockNumber.Uint64())
	require.NoError(t, err, "could not get expected output")
	l2Claim := l2Output.OutputRoot

	t.Log("Determine L1 head that includes all batches required for L2 claim block")
	require.NoError(t, wait.ForSafeBlock(ctx, rollupClient, l2ClaimBlockNumber.Uint64()))
	l1HeadBlock, err := l1Client.BlockByNumber(ctx, nil)
	require.NoError(t, err, "get l1 head block")
	l1Head := l1HeadBlock.Hash()

	inputs := utils.LocalGameInputs{
		L1Head:        l1Head,
		L2Head:        l2Head,
		L2Claim:       common.Hash(l2Claim),
		L2OutputRoot:  common.Hash(l2OutputRoot),
		L2BlockNumber: l2ClaimBlockNumber,
	}
	debugfile := path.Join(t.TempDir(), "debug.json")
	runCannon(t, ctx, sys, inputs, "--debug-info", debugfile)
	data, err := os.ReadFile(debugfile)
	require.NoError(t, err)
	var debuginfo mipsevm.DebugInfo
	require.NoError(t, json.Unmarshal(data, &debuginfo))
	t.Logf("Debug info: %#v", debuginfo)
	// TODO(client-pod#906): Use maximum witness size for assertions against pages allocated by the VM
}

func createBigContracts(ctx context.Context, t *testing.T, cfg e2esys.SystemConfig, client *ethclient.Client, key *ecdsa.PrivateKey, numContracts int) []common.Address {
	/*
		contract Big {
			bytes constant foo = hex"<24.4 KB of random data>";
			function ekans() external { foo; }
		}
	*/
	createInputHex, err := os.ReadFile("bigCodeCreateInput.data")
	createInput := common.FromHex(string(createInputHex[2:]))
	require.NoError(t, err)

	nonce, err := client.NonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey), nil)
	require.NoError(t, err)

	type result struct {
		addr common.Address
		err  error
	}

	var wg sync.WaitGroup
	wg.Add(numContracts)
	results := make(chan result, numContracts)
	for i := 0; i < numContracts; i++ {
		tx := types.MustSignNewTx(key, types.LatestSignerForChainID(cfg.L2ChainIDBig()), &types.DynamicFeeTx{
			ChainID:   cfg.L2ChainIDBig(),
			Nonce:     nonce + uint64(i),
			To:        nil,
			GasTipCap: big.NewInt(10),
			GasFeeCap: big.NewInt(200),
			Gas:       10_000_000,
			Data:      createInput,
		})
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
			defer cancel()
			err := client.SendTransaction(ctx, tx)
			if err != nil {
				results <- result{err: errors.Wrap(err, "Sending L2 tx")}
				return
			}
			receipt, err := wait.ForReceiptOK(ctx, client, tx.Hash())
			if err != nil {
				results <- result{err: errors.Wrap(err, "Waiting for receipt")}
				return
			}
			results <- result{addr: receipt.ContractAddress, err: nil}
		}()
	}
	wg.Wait()
	close(results)

	var addrs []common.Address
	for r := range results {
		require.NoError(t, r.err)
		addrs = append(addrs, r.addr)
	}
	return addrs
}

func callBigContracts(ctx context.Context, t *testing.T, cfg e2esys.SystemConfig, client *ethclient.Client, key *ecdsa.PrivateKey, addrs []common.Address) *types.Receipt {
	multicall3, err := bindings.NewMultiCall3(predeploys.MultiCall3Addr, client)
	require.NoError(t, err)

	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	opts, err := bind.NewKeyedTransactorWithChainID(key, chainID)
	require.NoError(t, err)

	var calls []bindings.Multicall3Call3Value
	calldata := crypto.Keccak256([]byte("ekans()"))[:4]
	for _, addr := range addrs {
		calls = append(calls, bindings.Multicall3Call3Value{
			Target:   addr,
			CallData: calldata,
			Value:    new(big.Int),
		})
	}
	opts.GasLimit = 20_000_000
	tx, err := multicall3.Aggregate3Value(opts, calls)
	require.NoError(t, err)

	receipt, err := wait.ForReceiptOK(ctx, client, tx.Hash())
	require.NoError(t, err)
	t.Logf("Initiated %d calls to the Big Contract. gas used: %d", len(addrs), receipt.GasUsed)
	return receipt
}
