package opgeth

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

var (
	rip7212Precompile = common.HexToAddress("0x0000000000000000000000000000000000000100")
	invalid7212Data   = []byte{0x00}
	// This is a valid hash, r, s, x, y params for RIP-7212 taken from:
	// https://gist.github.com/ulerdogan/8f1714895e23a54147fc529ea30517eb
	valid7212Data = common.FromHex("4cee90eb86eaa050036147a12d49004b6b9c72bd725d39d4785011fe190f0b4da73bd4903f0ce3b639bbbf6e8e80d16931ff4bcf5993d58468e8fb19086e8cac36dbcd03009df8c59286b162af3bd7fcc0450c9aa81be5d10d312af6c66b1d604aebd3099c618202fcfe16ae7770b0c49ab5eadf74b754204a3bb6060e44eff37618b065f9832de4ca6ca971a7a1adc826d0f7c00181a5fb2ddf79ae00b4e10e")
)

// TestMissingGasLimit tests that op-geth cannot build a block without gas limit while optimism is active in the chain config.
func TestMissingGasLimit(t *testing.T) {
	op_e2e.InitParallel(t)
	cfg := e2esys.DefaultSystemConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	opGeth, err := NewOpGeth(t, ctx, &cfg)
	require.NoError(t, err)
	defer opGeth.Close()

	attrs, err := opGeth.CreatePayloadAttributes()
	require.NoError(t, err)
	// Remove the GasLimit from the otherwise valid attributes
	attrs.GasLimit = nil

	res, err := opGeth.StartBlockBuilding(ctx, attrs)
	require.Error(t, err)
	var rpcErr rpc.Error
	require.ErrorAs(t, err, &rpcErr)
	require.EqualValues(t, eth.InvalidPayloadAttributes, rpcErr.ErrorCode())
	require.Nil(t, res)
}

// TestTxGasSameAsBlockGasLimit tests that op-geth rejects transactions that attempt to use the full block gas limit.
// The L1 Info deposit always takes gas so the effective gas limit is lower than the full block gas limit.
func TestTxGasSameAsBlockGasLimit(t *testing.T) {
	op_e2e.InitParallel(t)
	cfg := e2esys.DefaultSystemConfig(t)
	sys, err := cfg.Start(t)
	require.Nil(t, err, "Error starting up system")

	ethPrivKey := sys.Cfg.Secrets.Alice
	tx := types.MustSignNewTx(ethPrivKey, types.LatestSignerForChainID(cfg.L2ChainIDBig()), &types.DynamicFeeTx{
		ChainID: cfg.L2ChainIDBig(),
		Gas:     29_999_999,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	l2Seq := sys.NodeClient("sequencer")
	err = l2Seq.SendTransaction(ctx, tx)
	require.ErrorContains(t, err, txpool.ErrGasLimit.Error())
}

// TestInvalidDepositInFCU runs an invalid deposit through a FCU/GetPayload/NewPayload/FCU set of calls.
// This tests that deposits must always allow the block to be built even if they are invalid.
func TestInvalidDepositInFCU(t *testing.T) {
	op_e2e.InitParallel(t)
	cfg := e2esys.DefaultSystemConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	opGeth, err := NewOpGeth(t, ctx, &cfg)
	require.NoError(t, err)
	defer opGeth.Close()

	// Create a deposit from a new account that will always fail (not enough funds)
	fromKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	fromAddr := crypto.PubkeyToAddress(fromKey.PublicKey)
	balance, err := opGeth.L2Client.BalanceAt(ctx, fromAddr, nil)
	require.Nil(t, err)
	t.Logf("alice balance: %d, %s", balance, fromAddr)
	require.Equal(t, 0, balance.Cmp(common.Big0))

	badDepositTx := types.NewTx(&types.DepositTx{
		From:                fromAddr,
		To:                  &fromAddr, // send it to ourselves
		Value:               big.NewInt(params.Ether),
		Gas:                 25000,
		IsSystemTransaction: false,
	})

	// We are inserting a block with an invalid deposit.
	// The invalid deposit should still remain in the block.
	_, err = opGeth.AddL2Block(ctx, badDepositTx)
	require.NoError(t, err)

	// Deposit tx was included, but our account still shouldn't have any ETH
	balance, err = opGeth.L2Client.BalanceAt(ctx, fromAddr, nil)
	require.Nil(t, err)
	require.Equal(t, 0, balance.Cmp(common.Big0))
}

// TestGethOnlyPendingBlockIsLatest walks through an engine-API block building job,
// and asserts that the pending block is set to match the latest block at every stage,
// for stability and tx-privacy.
func TestGethOnlyPendingBlockIsLatest(t *testing.T) {
	op_e2e.InitParallel(t)
	cfg := e2esys.DefaultSystemConfig(t)
	cfg.DeployConfig.FundDevAccounts = true
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	opGeth, err := NewOpGeth(t, ctx, &cfg)
	require.NoError(t, err)
	defer opGeth.Close()

	checkPending := func(stage string, number uint64) {
		// TODO: pending-block ID change
		pendingBlock, err := opGeth.L2Client.BlockByNumber(ctx, big.NewInt(-1))
		require.NoError(t, err, "failed to fetch pending block at stage "+stage)
		require.Equal(t, number, pendingBlock.NumberU64(), "pending block must have expected number")
		latestBlock, err := opGeth.L2Client.BlockByNumber(ctx, nil)
		require.NoError(t, err, "failed to fetch latest block at stage "+stage)
		require.Equal(t, pendingBlock.Hash(), latestBlock.Hash(), "pending and latest do not match at stage "+stage)
	}

	checkPending("genesis", 0)

	amount := big.NewInt(42) // send 42 wei

	aliceStartBalance, err := opGeth.L2Client.PendingBalanceAt(ctx, cfg.Secrets.Addresses().Alice)
	require.NoError(t, err)
	require.True(t, aliceStartBalance.Cmp(big.NewInt(0)) > 0, "alice must be funded")

	checkPendingBalance := func() {
		pendingBalance, err := opGeth.L2Client.PendingBalanceAt(ctx, cfg.Secrets.Addresses().Alice)
		require.NoError(t, err)
		require.Equal(t, pendingBalance, aliceStartBalance, "pending balance must still be the same")
	}

	startBlock, err := opGeth.L2Client.BlockByNumber(ctx, nil)
	require.NoError(t, err)

	signer := types.LatestSigner(opGeth.L2ChainConfig)
	tip := big.NewInt(7_000_000_000) // 7 gwei tip
	tx := types.MustSignNewTx(cfg.Secrets.Alice, signer, &types.DynamicFeeTx{
		ChainID:   big.NewInt(int64(cfg.DeployConfig.L2ChainID)),
		Nonce:     0,
		GasTipCap: tip,
		GasFeeCap: new(big.Int).Add(startBlock.BaseFee(), tip),
		Gas:       1_000_000,
		To:        &cfg.Secrets.Addresses().Bob,
		Value:     amount,
		Data:      nil,
	})
	require.NoError(t, opGeth.L2Client.SendTransaction(ctx, tx), "send tx to make pending work different")
	checkPending("prepared", 0)

	// Wait for tx to be in tx-pool, for it to be picked up in block building
	var txPoolStatus struct {
		Pending hexutil.Uint64 `json:"pending"`
	}
	for i := 0; i < 5; i++ {
		require.NoError(t, opGeth.L2Client.Client().Call(&txPoolStatus, "txpool_status"))
		if txPoolStatus.Pending == 0 {
			time.Sleep(time.Second)
		} else {
			break
		}
	}
	require.NotZero(t, txPoolStatus.Pending, "must have pending tx in pool")

	checkPending("in-pool", 0)
	checkPendingBalance()

	// start building a block
	attrs, err := opGeth.CreatePayloadAttributes()
	require.NoError(t, err)
	attrs.NoTxPool = false // we want to include a tx
	fc := eth.ForkchoiceState{
		HeadBlockHash: opGeth.L2Head.BlockHash,
		SafeBlockHash: opGeth.L2Head.BlockHash,
	}
	res, err := opGeth.l2Engine.ForkchoiceUpdate(ctx, &fc, attrs)
	require.NoError(t, err)

	checkPending("building", 0)
	checkPendingBalance()

	// Now we have to wait until the block-building job picks up the tx from the tx-pool.
	// See go routine that spins up in buildPayload() func in payload_building.go in miner package.
	// We can't check it, we don't want to finish block-building prematurely, and so we have to wait.
	time.Sleep(time.Second * 4) // conservatively wait 4 seconds, CI might lag during block building.

	// retrieve the block
	envelope, err := opGeth.l2Engine.GetPayload(ctx, eth.PayloadInfo{ID: *res.PayloadID, Timestamp: uint64(attrs.Timestamp)})
	require.NoError(t, err)

	payload := envelope.ExecutionPayload
	checkPending("retrieved", 0)
	require.Len(t, payload.Transactions, 2, "must include L1 info tx and tx from alice")
	checkPendingBalance()

	// process the block
	status, err := opGeth.l2Engine.NewPayload(ctx, payload, envelope.ParentBeaconBlockRoot)
	require.NoError(t, err)
	require.Equal(t, eth.ExecutionValid, status.Status)
	checkPending("processed", 0)
	checkPendingBalance()

	// make the block canonical
	fc = eth.ForkchoiceState{
		HeadBlockHash: payload.BlockHash,
		SafeBlockHash: payload.BlockHash,
	}
	res, err = opGeth.l2Engine.ForkchoiceUpdate(ctx, &fc, nil)
	require.NoError(t, err)
	require.Equal(t, eth.ExecutionValid, res.PayloadStatus.Status)
	checkPending("canonical", 1)
}

func TestPreregolith(t *testing.T) {
	futureTimestamp := hexutil.Uint64(4)
	tests := []struct {
		name         string
		regolithTime *hexutil.Uint64
	}{
		{name: "RegolithNotScheduled"},
		{name: "RegolithNotYetActive", regolithTime: &futureTimestamp},
	}
	for _, test := range tests {
		test := test
		t.Run("GasUsed_"+test.name, func(t *testing.T) {
			op_e2e.InitParallel(t)
			// Setup an L2 EE and create a client connection to the engine.
			// We also need to setup a L1 Genesis to create the rollup genesis.
			cfg := e2esys.RegolithSystemConfig(t, test.regolithTime)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			opGeth, err := NewOpGeth(t, ctx, &cfg)
			require.NoError(t, err)
			defer opGeth.Close()

			fromAddr := cfg.Secrets.Addresses().Alice

			oldBalance, err := opGeth.L2Client.BalanceAt(ctx, fromAddr, nil)
			require.NoError(t, err)

			// Simple transfer deposit tx
			depositTx := types.NewTx(&types.DepositTx{
				From:                fromAddr,
				To:                  &fromAddr, // send it to ourselves
				Value:               big.NewInt(params.Ether),
				Gas:                 25000,
				IsSystemTransaction: false,
			})

			envelope, err := opGeth.AddL2Block(ctx, depositTx)
			require.NoError(t, err)

			// L1Info tx should report 0 gas used
			infoTx, err := opGeth.L2Client.TransactionInBlock(ctx, envelope.ExecutionPayload.BlockHash, 0)
			require.NoError(t, err)
			infoRcpt, err := opGeth.L2Client.TransactionReceipt(ctx, infoTx.Hash())
			require.NoError(t, err)
			require.Zero(t, infoRcpt.GasUsed, "should use 0 gas for system tx")

			// Deposit tx should report all gas used
			receipt, err := opGeth.L2Client.TransactionReceipt(ctx, depositTx.Hash())
			require.NoError(t, err)
			require.Equal(t, depositTx.Gas(), receipt.GasUsed, "should report all gas used")

			// Should not refund ETH for unused gas
			newBalance, err := opGeth.L2Client.BalanceAt(ctx, fromAddr, nil)
			require.NoError(t, err)
			require.Equal(t, oldBalance, newBalance, "should not repay sender for unused gas")
		})

		t.Run("DepositNonce_"+test.name, func(t *testing.T) {
			op_e2e.InitParallel(t)
			// Setup an L2 EE and create a client connection to the engine.
			// We also need to setup a L1 Genesis to create the rollup genesis.
			cfg := e2esys.RegolithSystemConfig(t, test.regolithTime)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			opGeth, err := NewOpGeth(t, ctx, &cfg)
			require.NoError(t, err)
			defer opGeth.Close()

			fromAddr := cfg.Secrets.Addresses().Alice
			// Include a tx just to ensure Alice's nonce isn't 0
			incrementNonceTx := types.NewTx(&types.DepositTx{
				From:                fromAddr,
				To:                  &fromAddr,
				Value:               big.NewInt(0),
				Gas:                 21_000,
				IsSystemTransaction: false,
			})

			// Contract creation deposit tx
			contractCreateTx := types.NewTx(&types.DepositTx{
				From:                fromAddr,
				Value:               big.NewInt(params.Ether),
				Gas:                 1000001,
				Data:                []byte{},
				IsSystemTransaction: false,
			})

			_, err = opGeth.AddL2Block(ctx, incrementNonceTx, contractCreateTx)
			require.NoError(t, err)

			expectedNonce := uint64(1)
			incorrectContractAddress := crypto.CreateAddress(fromAddr, uint64(0))
			correctContractAddress := crypto.CreateAddress(fromAddr, expectedNonce)
			createRcpt, err := opGeth.L2Client.TransactionReceipt(ctx, contractCreateTx.Hash())
			require.NoError(t, err)
			require.Equal(t, types.ReceiptStatusSuccessful, createRcpt.Status, "create should succeed")
			require.Nil(t, createRcpt.DepositNonce, "should not report deposit nonce")
			require.Equal(t, incorrectContractAddress, createRcpt.ContractAddress, "should report correct contract address")

			contractBalance, err := opGeth.L2Client.BalanceAt(ctx, incorrectContractAddress, nil)
			require.NoError(t, err)
			require.Equal(t, uint64(0), contractBalance.Uint64(), "balance unchanged on incorrect contract address")

			contractBalance, err = opGeth.L2Client.BalanceAt(ctx, correctContractAddress, nil)
			require.NoError(t, err)
			require.Equal(t, uint64(params.Ether), contractBalance.Uint64(), "balance changed on correct contract address")

			// Check the actual transaction nonce is reported correctly when retrieving the tx from the API.
			tx, _, err := opGeth.L2Client.TransactionByHash(ctx, contractCreateTx.Hash())
			require.NoError(t, err)
			require.Zero(t, *tx.EffectiveNonce(), "should report 0 as tx nonce")
		})

		t.Run("UnusedGasConsumed_"+test.name, func(t *testing.T) {
			op_e2e.InitParallel(t)
			cfg := e2esys.RegolithSystemConfig(t, test.regolithTime)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			opGeth, err := NewOpGeth(t, ctx, &cfg)
			require.NoError(t, err)
			defer opGeth.Close()

			fromAddr := cfg.Secrets.Addresses().Alice

			// Deposit TX with a high gas limit but using very little actual gas
			depositTx := types.NewTx(&types.DepositTx{
				From:  fromAddr,
				To:    &fromAddr, // send it to ourselves
				Value: big.NewInt(params.Ether),
				// SystemTx is assigned 1M gas limit
				Gas:                 uint64(cfg.DeployConfig.L2GenesisBlockGasLimit) - 1_000_000,
				IsSystemTransaction: false,
			})

			signer := types.LatestSigner(opGeth.L2ChainConfig)
			// Second tx with a gas limit that will fit in regolith but not bedrock
			tx := types.MustSignNewTx(cfg.Secrets.Bob, signer, &types.DynamicFeeTx{
				ChainID:   big.NewInt(int64(cfg.DeployConfig.L2ChainID)),
				Nonce:     0,
				GasTipCap: big.NewInt(100),
				GasFeeCap: big.NewInt(100000),
				Gas:       1_000_001,
				To:        &cfg.Secrets.Addresses().Alice,
				Value:     big.NewInt(0),
				Data:      nil,
			})

			_, err = opGeth.AddL2Block(ctx, depositTx, tx)
			// Geth checks the gas limit usage of transactions as part of validating the payload attributes and refuses to even start building the block
			require.ErrorContains(t, err, "Invalid payload attributes", "block should be invalid due to using too much gas")
		})

		t.Run("AllowSystemTx_"+test.name, func(t *testing.T) {
			op_e2e.InitParallel(t)
			cfg := e2esys.RegolithSystemConfig(t, test.regolithTime)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			opGeth, err := NewOpGeth(t, ctx, &cfg)
			require.NoError(t, err)
			defer opGeth.Close()

			rollupCfg := rollup.Config{}
			systemTx, err := derive.L1InfoDeposit(&rollupCfg, opGeth.SystemConfig, 1, opGeth.L1Head, 0)
			systemTx.IsSystemTransaction = true
			require.NoError(t, err)

			_, err = opGeth.AddL2Block(ctx, types.NewTx(systemTx))
			require.NoError(t, err, "should allow blocks containing system tx")
		})
	}
}

func TestRegolith(t *testing.T) {
	tests := []struct {
		name             string
		regolithTime     hexutil.Uint64
		activateRegolith func(ctx context.Context, t *testing.T, opGeth *OpGeth)
	}{
		{name: "ActivateAtGenesis", regolithTime: 0, activateRegolith: func(ctx context.Context, t *testing.T, opGeth *OpGeth) {}},
		{name: "ActivateAfterGenesis", regolithTime: 2, activateRegolith: func(ctx context.Context, t *testing.T, opGeth *OpGeth) {
			_, err := opGeth.AddL2Block(ctx)
			require.NoError(t, err)
		}},
	}
	for _, test := range tests {
		test := test
		t.Run("GasUsedIsAccurate_"+test.name, func(t *testing.T) {
			op_e2e.InitParallel(t)
			// Setup an L2 EE and create a client connection to the engine.
			// We also need to setup a L1 Genesis to create the rollup genesis.
			cfg := e2esys.RegolithSystemConfig(t, &test.regolithTime)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			opGeth, err := NewOpGeth(t, ctx, &cfg)
			require.NoError(t, err)
			defer opGeth.Close()

			test.activateRegolith(ctx, t, opGeth)

			fromAddr := cfg.Secrets.Addresses().Alice

			oldBalance, err := opGeth.L2Client.BalanceAt(ctx, fromAddr, nil)
			require.NoError(t, err)

			// Simple transfer deposit tx
			depositTx := types.NewTx(&types.DepositTx{
				From:                fromAddr,
				To:                  &fromAddr, // send it to ourselves
				Value:               big.NewInt(params.Ether),
				Gas:                 25000,
				IsSystemTransaction: false,
			})

			envelope, err := opGeth.AddL2Block(ctx, depositTx)
			require.NoError(t, err)

			// L1Info tx should report actual gas used, not 0 or the tx gas limit
			infoTx, err := opGeth.L2Client.TransactionInBlock(ctx, envelope.ExecutionPayload.BlockHash, 0)
			require.NoError(t, err)
			infoRcpt, err := opGeth.L2Client.TransactionReceipt(ctx, infoTx.Hash())
			require.NoError(t, err)
			require.NotZero(t, infoRcpt.GasUsed)
			require.NotEqual(t, infoTx.Gas(), infoRcpt.GasUsed)

			// Deposit tx should report actual gas used (21,000 for a normal transfer)
			receipt, err := opGeth.L2Client.TransactionReceipt(ctx, depositTx.Hash())
			require.NoError(t, err)
			require.Equal(t, uint64(21_000), receipt.GasUsed, "should report actual gas used")

			// Should not refund ETH for unused gas
			newBalance, err := opGeth.L2Client.BalanceAt(ctx, fromAddr, nil)
			require.NoError(t, err)
			require.Equal(t, oldBalance, newBalance, "should not repay sender for unused gas")
		})

		t.Run("DepositNonceCorrect_"+test.name, func(t *testing.T) {
			op_e2e.InitParallel(t)
			// Setup an L2 EE and create a client connection to the engine.
			// We also need to setup a L1 Genesis to create the rollup genesis.
			cfg := e2esys.RegolithSystemConfig(t, &test.regolithTime)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			opGeth, err := NewOpGeth(t, ctx, &cfg)
			require.NoError(t, err)
			defer opGeth.Close()

			test.activateRegolith(ctx, t, opGeth)

			fromAddr := cfg.Secrets.Addresses().Alice
			// Include a tx just to ensure Alice's nonce isn't 0
			incrementNonceTx := types.NewTx(&types.DepositTx{
				From:                fromAddr,
				To:                  &fromAddr,
				Value:               big.NewInt(0),
				Gas:                 21_000,
				IsSystemTransaction: false,
			})

			// Contract creation deposit tx
			contractCreateTx := types.NewTx(&types.DepositTx{
				From:                fromAddr,
				Value:               big.NewInt(params.Ether),
				Gas:                 1000001,
				Data:                []byte{},
				IsSystemTransaction: false,
			})

			_, err = opGeth.AddL2Block(ctx, incrementNonceTx, contractCreateTx)
			require.NoError(t, err)

			expectedNonce := uint64(1)
			correctContractAddress := crypto.CreateAddress(fromAddr, expectedNonce)
			createRcpt, err := opGeth.L2Client.TransactionReceipt(ctx, contractCreateTx.Hash())
			require.NoError(t, err)
			require.Equal(t, types.ReceiptStatusSuccessful, createRcpt.Status, "create should succeed")
			require.Equal(t, &expectedNonce, createRcpt.DepositNonce, "should report correct deposit nonce")
			require.Equal(t, correctContractAddress, createRcpt.ContractAddress, "should report correct contract address")

			contractBalance, err := opGeth.L2Client.BalanceAt(ctx, createRcpt.ContractAddress, nil)
			require.NoError(t, err)
			require.Equal(t, uint64(params.Ether), contractBalance.Uint64(), "balance changed on correct contract address")

			// Check the actual transaction nonce is reported correctly when retrieving the tx from the API.
			tx, _, err := opGeth.L2Client.TransactionByHash(ctx, contractCreateTx.Hash())
			require.NoError(t, err)
			require.Equal(t, expectedNonce, *tx.EffectiveNonce(), "should report actual tx nonce")

			// Should be able to search for logs even though there are deposit transactions in blocks.
			logs, err := opGeth.L2Client.FilterLogs(ctx, ethereum.FilterQuery{})
			require.NoError(t, err)
			require.NotNil(t, logs)
			require.Empty(t, logs)
		})

		t.Run("ReturnUnusedGasToPool_"+test.name, func(t *testing.T) {
			op_e2e.InitParallel(t)
			cfg := e2esys.RegolithSystemConfig(t, &test.regolithTime)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			opGeth, err := NewOpGeth(t, ctx, &cfg)
			require.NoError(t, err)
			defer opGeth.Close()

			test.activateRegolith(ctx, t, opGeth)

			fromAddr := cfg.Secrets.Addresses().Alice

			// Deposit TX with a high gas limit but using very little actual gas
			depositTx := types.NewTx(&types.DepositTx{
				From:  fromAddr,
				To:    &fromAddr, // send it to ourselves
				Value: big.NewInt(params.Ether),
				// SystemTx is assigned 1M gas limit
				Gas:                 uint64(cfg.DeployConfig.L2GenesisBlockGasLimit) - 1_000_000,
				IsSystemTransaction: false,
			})

			signer := types.LatestSigner(opGeth.L2ChainConfig)
			// Second tx with a gas limit that will fit in regolith but not bedrock
			tx := types.MustSignNewTx(cfg.Secrets.Bob, signer, &types.DynamicFeeTx{
				ChainID:   big.NewInt(int64(cfg.DeployConfig.L2ChainID)),
				Nonce:     0,
				GasTipCap: big.NewInt(100),
				GasFeeCap: big.NewInt(100000),
				Gas:       1_000_001,
				To:        &cfg.Secrets.Addresses().Alice,
				Value:     big.NewInt(0),
				Data:      nil,
			})

			_, err = opGeth.AddL2Block(ctx, depositTx, tx)
			require.NoError(t, err, "block should be valid as cumulativeGasUsed only tracks actual usage now")
		})

		t.Run("RejectSystemTx_"+test.name, func(t *testing.T) {
			op_e2e.InitParallel(t)
			cfg := e2esys.RegolithSystemConfig(t, &test.regolithTime)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			opGeth, err := NewOpGeth(t, ctx, &cfg)
			require.NoError(t, err)
			defer opGeth.Close()

			test.activateRegolith(ctx, t, opGeth)

			rollupCfg := rollup.Config{}
			systemTx, err := derive.L1InfoDeposit(&rollupCfg, opGeth.SystemConfig, 1, opGeth.L1Head, 0)
			systemTx.IsSystemTransaction = true
			require.NoError(t, err)

			_, err = opGeth.AddL2Block(ctx, types.NewTx(systemTx))
			require.ErrorIs(t, err, ErrNewPayloadNotValid, "should reject blocks containing system tx")
		})

		t.Run("IncludeGasRefunds_"+test.name, func(t *testing.T) {
			op_e2e.InitParallel(t)
			// Simple constructor that is prefixed to the actual contract code
			// Results in the contract code being returned as the code for the new contract
			deployPrefixSize := byte(16)
			deployPrefix := []byte{
				// Copy input data after this prefix into memory starting at address 0x00
				// CODECOPY arg size
				byte(vm.PUSH1), deployPrefixSize,
				byte(vm.CODESIZE),
				byte(vm.SUB),
				// CODECOPY arg offset
				byte(vm.PUSH1), deployPrefixSize,
				// CODECOPY arg destOffset
				byte(vm.PUSH1), 0x00,
				byte(vm.CODECOPY),

				// Return code from memory
				// RETURN arg size
				byte(vm.PUSH1), deployPrefixSize,
				byte(vm.CODESIZE),
				byte(vm.SUB),
				// RETURN arg offset
				byte(vm.PUSH1), 0x00,
				byte(vm.RETURN),
			}
			// Stores the first word from call data code to storage slot 0
			sstoreContract := []byte{
				// Load first word from call data
				byte(vm.PUSH1), 0x00,
				byte(vm.CALLDATALOAD),

				// Store it to slot 0
				byte(vm.PUSH1), 0x00,
				byte(vm.SSTORE),
			}

			deployData := append(deployPrefix, sstoreContract...)

			cfg := e2esys.RegolithSystemConfig(t, &test.regolithTime)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			opGeth, err := NewOpGeth(t, ctx, &cfg)
			require.NoError(t, err)
			defer opGeth.Close()

			test.activateRegolith(ctx, t, opGeth)
			fromAddr := cfg.Secrets.Addresses().Alice
			storeContractAddr := crypto.CreateAddress(fromAddr, 0)

			// Deposit TX to deploy a contract that lets us store an arbitrary value
			deployTx := types.NewTx(&types.DepositTx{
				From:                fromAddr,
				Value:               common.Big0,
				Data:                deployData,
				Gas:                 1_000_000,
				IsSystemTransaction: false,
			})

			// Store a non-zero value
			storeTx := types.NewTx(&types.DepositTx{
				From:                fromAddr,
				To:                  &storeContractAddr,
				Value:               common.Big0,
				Data:                []byte{0x06},
				Gas:                 1_000_000,
				IsSystemTransaction: false,
			})

			// Store a non-zero value
			zeroTx := types.NewTx(&types.DepositTx{
				From:                fromAddr,
				To:                  &storeContractAddr,
				Value:               common.Big0,
				Data:                []byte{0x00},
				Gas:                 1_000_000,
				IsSystemTransaction: false,
			})

			// Store a non-zero value again
			// Has same gas cost as zeroTx, except the first tx gets a gas refund for clearing the storage slot
			rezeroTx := types.NewTx(&types.DepositTx{
				From:                fromAddr,
				To:                  &storeContractAddr,
				Value:               common.Big0,
				Data:                []byte{0x00},
				Gas:                 1_000_001,
				IsSystemTransaction: false,
			})

			_, err = opGeth.AddL2Block(ctx, deployTx, storeTx, zeroTx, rezeroTx)
			require.NoError(t, err)

			// Sanity check the contract code deployed correctly
			code, err := opGeth.L2Client.CodeAt(ctx, storeContractAddr, nil)
			require.NoError(t, err)
			require.Equal(t, sstoreContract, code, "should create contract with expected code")

			deployReceipt, err := opGeth.L2Client.TransactionReceipt(ctx, deployTx.Hash())
			require.NoError(t, err)
			require.Equal(t, types.ReceiptStatusSuccessful, deployReceipt.Status)
			require.Equal(t, storeContractAddr, deployReceipt.ContractAddress, "should create contract at expected address")

			storeReceipt, err := opGeth.L2Client.TransactionReceipt(ctx, storeTx.Hash())
			require.NoError(t, err)
			require.Equal(t, types.ReceiptStatusSuccessful, storeReceipt.Status, "setting storage value should succeed")

			zeroReceipt, err := opGeth.L2Client.TransactionReceipt(ctx, zeroTx.Hash())
			require.NoError(t, err)
			require.Equal(t, types.ReceiptStatusSuccessful, zeroReceipt.Status, "zeroing storage value should succeed")

			rezeroReceipt, err := opGeth.L2Client.TransactionReceipt(ctx, rezeroTx.Hash())
			require.NoError(t, err)
			require.Equal(t, types.ReceiptStatusSuccessful, rezeroReceipt.Status, "rezeroing storage value should succeed")

			require.Greater(t, rezeroReceipt.GasUsed, zeroReceipt.GasUsed, "rezero should use more gas due to not getting gas refund for clearing slot")
		})
	}
}

func TestPreCanyon(t *testing.T) {
	futureTimestamp := hexutil.Uint64(4)

	tests := []struct {
		name       string
		canyonTime *hexutil.Uint64
	}{
		{name: "CanyonNotScheduled"},
		{name: "CanyonNotYetActive", canyonTime: &futureTimestamp},
	}
	for _, test := range tests {
		test := test

		t.Run(fmt.Sprintf("ReturnsNilWithdrawals_%s", test.name), func(t *testing.T) {
			op_e2e.InitParallel(t)
			cfg := e2esys.CanyonSystemConfig(t, test.canyonTime)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			opGeth, err := NewOpGeth(t, ctx, &cfg)
			require.NoError(t, err)
			defer opGeth.Close()

			b, err := opGeth.AddL2Block(ctx)
			require.NoError(t, err)
			assert.Nil(t, b.ExecutionPayload.Withdrawals, "should not have withdrawals")

			l1Block, err := opGeth.L2Client.BlockByNumber(ctx, nil)
			require.Nil(t, err)
			assert.Equal(t, types.Withdrawals(nil), l1Block.Withdrawals())
		})

		t.Run(fmt.Sprintf("RejectPushZeroTx_%s", test.name), func(t *testing.T) {
			op_e2e.InitParallel(t)
			cfg := e2esys.CanyonSystemConfig(t, test.canyonTime)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			opGeth, err := NewOpGeth(t, ctx, &cfg)
			require.NoError(t, err)
			defer opGeth.Close()

			pushZeroContractCreateTxn := types.NewTx(&types.DepositTx{
				From:  cfg.Secrets.Addresses().Alice,
				Value: big.NewInt(params.Ether),
				Gas:   1000001,
				Data: []byte{
					byte(vm.PUSH0),
				},
				IsSystemTransaction: false,
			})

			_, err = opGeth.AddL2Block(ctx, pushZeroContractCreateTxn)
			require.NoError(t, err)

			receipt, err := opGeth.L2Client.TransactionReceipt(ctx, pushZeroContractCreateTxn.Hash())
			require.NoError(t, err)
			assert.Equal(t, types.ReceiptStatusFailed, receipt.Status)
		})
	}
}

func TestCanyon(t *testing.T) {
	tests := []struct {
		name           string
		canyonTime     hexutil.Uint64
		activateCanyon func(ctx context.Context, t *testing.T, opGeth *OpGeth)
	}{
		{name: "ActivateAtGenesis", canyonTime: 0, activateCanyon: func(ctx context.Context, t *testing.T, opGeth *OpGeth) {}},
		{name: "ActivateAfterGenesis", canyonTime: 2, activateCanyon: func(ctx context.Context, t *testing.T, opGeth *OpGeth) {
			// Adding this block advances us to the fork time.
			_, err := opGeth.AddL2Block(ctx)
			require.NoError(t, err)
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(fmt.Sprintf("ReturnsEmptyWithdrawals_%s", test.name), func(t *testing.T) {
			op_e2e.InitParallel(t)
			cfg := e2esys.CanyonSystemConfig(t, &test.canyonTime)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			opGeth, err := NewOpGeth(t, ctx, &cfg)
			require.NoError(t, err)
			defer opGeth.Close()

			test.activateCanyon(ctx, t, opGeth)

			b, err := opGeth.AddL2Block(ctx)
			require.NoError(t, err)
			assert.Equal(t, *b.ExecutionPayload.Withdrawals, types.Withdrawals{})

			l1Block, err := opGeth.L2Client.BlockByNumber(ctx, nil)
			require.Nil(t, err)
			assert.Equal(t, l1Block.Withdrawals(), types.Withdrawals{})
		})

		t.Run(fmt.Sprintf("AcceptsPushZeroTxn_%s", test.name), func(t *testing.T) {
			op_e2e.InitParallel(t)
			cfg := e2esys.CanyonSystemConfig(t, &test.canyonTime)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			opGeth, err := NewOpGeth(t, ctx, &cfg)
			require.NoError(t, err)
			defer opGeth.Close()

			test.activateCanyon(ctx, t, opGeth)

			pushZeroContractCreateTxn := types.NewTx(&types.DepositTx{
				From:  cfg.Secrets.Addresses().Alice,
				Value: big.NewInt(params.Ether),
				Gas:   1000001,
				Data: []byte{
					byte(vm.PUSH0),
				},
				IsSystemTransaction: false,
			})

			_, err = opGeth.AddL2Block(ctx, pushZeroContractCreateTxn)
			require.NoError(t, err)

			receipt, err := opGeth.L2Client.TransactionReceipt(ctx, pushZeroContractCreateTxn.Hash())
			require.NoError(t, err)
			assert.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)
		})
	}
}

func TestPreEcotone(t *testing.T) {
	futureTimestamp := hexutil.Uint64(4)

	tests := []struct {
		name        string
		ecotoneTime *hexutil.Uint64
	}{
		{name: "EcotoneNotScheduled"},
		{name: "EcotoneNotYetActive", ecotoneTime: &futureTimestamp},
	}
	for _, test := range tests {
		test := test

		t.Run(fmt.Sprintf("NilParentBeaconRoot_%s", test.name), func(t *testing.T) {
			op_e2e.InitParallel(t)
			cfg := e2esys.EcotoneSystemConfig(t, test.ecotoneTime)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			opGeth, err := NewOpGeth(t, ctx, &cfg)
			require.NoError(t, err)
			defer opGeth.Close()

			b, err := opGeth.AddL2Block(ctx)
			require.NoError(t, err)
			assert.Nil(t, b.ParentBeaconBlockRoot)

			l2Block, err := opGeth.L2Client.BlockByNumber(ctx, nil)
			require.NoError(t, err)
			assert.Nil(t, l2Block.Header().ParentBeaconRoot)
		})

		t.Run(fmt.Sprintf("RejectTstoreTxn_%s", test.name), func(t *testing.T) {
			op_e2e.InitParallel(t)
			cfg := e2esys.EcotoneSystemConfig(t, test.ecotoneTime)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			opGeth, err := NewOpGeth(t, ctx, &cfg)
			require.NoError(t, err)
			defer opGeth.Close()

			tstoreTxn := types.NewTx(&types.DepositTx{
				From:  cfg.Secrets.Addresses().Alice,
				Value: big.NewInt(params.Ether),
				Gas:   1000001,
				Data: []byte{
					byte(vm.PUSH1),
					byte(vm.PUSH2),
					byte(vm.TSTORE),
				},
				IsSystemTransaction: false,
			})

			_, err = opGeth.AddL2Block(ctx, tstoreTxn)
			require.NoError(t, err)

			receipt, err := opGeth.L2Client.TransactionReceipt(ctx, tstoreTxn.Hash())
			require.NoError(t, err)
			assert.Equal(t, types.ReceiptStatusFailed, receipt.Status)
		})
	}
}

func TestEcotone(t *testing.T) {
	tests := []struct {
		name            string
		ecotoneTime     hexutil.Uint64
		activateEcotone func(ctx context.Context, t *testing.T, opGeth *OpGeth)
	}{
		{name: "ActivateAtGenesis", ecotoneTime: 0, activateEcotone: func(ctx context.Context, t *testing.T, opGeth *OpGeth) {}},
		{name: "ActivateAfterGenesis", ecotoneTime: 2, activateEcotone: func(ctx context.Context, t *testing.T, opGeth *OpGeth) {
			//	Adding this block advances us to the fork time.
			_, err := opGeth.AddL2Block(ctx)
			require.NoError(t, err)
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(fmt.Sprintf("HashParentBeaconBlockRoot_%s", test.name), func(t *testing.T) {
			op_e2e.InitParallel(t)
			cfg := e2esys.EcotoneSystemConfig(t, &test.ecotoneTime)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			opGeth, err := NewOpGeth(t, ctx, &cfg)
			require.NoError(t, err)
			defer opGeth.Close()

			test.activateEcotone(ctx, t, opGeth)

			b, err := opGeth.AddL2Block(ctx)
			require.NoError(t, err)
			require.NotNil(t, b.ParentBeaconBlockRoot)
			assert.Equal(t, b.ParentBeaconBlockRoot, opGeth.L1Head.ParentBeaconRoot())

			l2Block, err := opGeth.L2Client.BlockByNumber(ctx, nil)
			require.NoError(t, err)
			assert.NotNil(t, l2Block.Header().ParentBeaconRoot)
			assert.Equal(t, l2Block.Header().ParentBeaconRoot, opGeth.L1Head.ParentBeaconRoot())
		})

		t.Run(fmt.Sprintf("TstoreTxn_%s", test.name), func(t *testing.T) {
			op_e2e.InitParallel(t)
			cfg := e2esys.EcotoneSystemConfig(t, &test.ecotoneTime)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			opGeth, err := NewOpGeth(t, ctx, &cfg)
			require.NoError(t, err)
			defer opGeth.Close()

			test.activateEcotone(ctx, t, opGeth)

			tstoreTxn := types.NewTx(&types.DepositTx{
				From:  cfg.Secrets.Addresses().Alice,
				Value: big.NewInt(params.Ether),
				Gas:   1000001,
				Data: []byte{
					byte(vm.PUSH1), 0x01,
					byte(vm.PUSH1), 0x01,
					byte(vm.TSTORE),
					byte(vm.PUSH0),
				},
				IsSystemTransaction: false,
			})

			_, err = opGeth.AddL2Block(ctx, tstoreTxn)
			require.NoError(t, err)

			_, err = opGeth.AddL2Block(ctx, tstoreTxn)
			require.NoError(t, err)

			receipt, err := opGeth.L2Client.TransactionReceipt(ctx, tstoreTxn.Hash())
			require.NoError(t, err)
			assert.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)
		})
	}
}

func TestPreFjord(t *testing.T) {
	futureTimestamp := hexutil.Uint64(4)

	tests := []struct {
		name      string
		fjordTime *hexutil.Uint64
	}{
		{name: "FjordNotScheduled"},
		{name: "FjordNotYetActive", fjordTime: &futureTimestamp},
	}
	for _, test := range tests {
		test := test

		t.Run(fmt.Sprintf("RIP7212_%s", test.name), func(t *testing.T) {
			op_e2e.InitParallel(t)
			cfg := e2esys.FjordSystemConfig(t, test.fjordTime)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			opGeth, err := NewOpGeth(t, ctx, &cfg)
			require.NoError(t, err)
			defer opGeth.Close()

			// valid request pre-fjord returns empty response
			response, err := opGeth.L2Client.CallContract(ctx, ethereum.CallMsg{
				To:   &rip7212Precompile,
				Data: valid7212Data,
			}, nil)

			require.NoError(t, err)
			require.Equal(t, []byte{}, response, "should return empty response pre-fjord for valid signature")

			// invalid request returns returns empty response
			response, err = opGeth.L2Client.CallContract(ctx, ethereum.CallMsg{
				To:   &rip7212Precompile,
				Data: invalid7212Data,
			}, nil)

			require.NoError(t, err)
			require.Equal(t, []byte{}, response, "should return empty response for invalid signature")
		})
	}
}

func TestFjord(t *testing.T) {
	tests := []struct {
		name          string
		fjordTime     hexutil.Uint64
		activateFjord func(ctx context.Context, t *testing.T, opGeth *OpGeth)
	}{
		{name: "ActivateAtGenesis", fjordTime: 0, activateFjord: func(ctx context.Context, t *testing.T, opGeth *OpGeth) {}},
		{name: "ActivateAfterGenesis", fjordTime: 2, activateFjord: func(ctx context.Context, t *testing.T, opGeth *OpGeth) {
			//	Adding this block advances us to the fork time.
			_, err := opGeth.AddL2Block(ctx)
			require.NoError(t, err)
		}},
	}

	for _, test := range tests {
		test := test
		t.Run(fmt.Sprintf("RIP7212_%s", test.name), func(t *testing.T) {
			op_e2e.InitParallel(t)
			cfg := e2esys.FjordSystemConfig(t, &test.fjordTime)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			opGeth, err := NewOpGeth(t, ctx, &cfg)
			require.NoError(t, err)
			defer opGeth.Close()

			test.activateFjord(ctx, t, opGeth)

			// valid request returns one
			response, err := opGeth.L2Client.CallContract(ctx, ethereum.CallMsg{
				To:   &rip7212Precompile,
				Data: valid7212Data,
			}, nil)

			require.NoError(t, err)
			require.Equal(t, common.LeftPadBytes([]byte{1}, 32), response, "should return 1 for valid signature")

			// invalid request returns empty response, this is how the spec denotes an error.
			response, err = opGeth.L2Client.CallContract(ctx, ethereum.CallMsg{
				To:   &rip7212Precompile,
				Data: invalid7212Data,
			}, nil)

			require.NoError(t, err)
			require.Equal(t, []byte{}, response, "should return empty response for invalid signature")
		})
	}
}

func TestIsthmus(t *testing.T) {
	tests := []struct {
		name            string
		isthmusTime     hexutil.Uint64
		activateIsthmus func(ctx context.Context, t *testing.T, opGeth *OpGeth)
		// expectEmpty is true if calling the precompiles should result in an 0x1 success (default for empty contract)
		expectEmpty bool
	}{
		{name: "BeforeActivation", isthmusTime: 2, activateIsthmus: func(ctx context.Context, t *testing.T, opGeth *OpGeth) {}, expectEmpty: true},
		{name: "ActivateAtGenesis", isthmusTime: 0, activateIsthmus: func(ctx context.Context, t *testing.T, opGeth *OpGeth) {}, expectEmpty: false},
		{name: "ActivateAfterGenesis", isthmusTime: 2, activateIsthmus: func(ctx context.Context, t *testing.T, opGeth *OpGeth) {
			//	Adding this block advances us to the fork time.
			_, err := opGeth.AddL2Block(ctx)
			require.NoError(t, err)
		}, expectEmpty: false},
	}

	// Taken from https://eips.ethereum.org/assets/eip-2537/test-vectors
	precompilesToTest := []struct {
		precompileName        string
		precompileAddr        common.Address
		failInput             []byte
		expectedErrorContains string
		successInput          []byte
		expectedResult        []byte
	}{
		{
			precompileName:        "G1Add",
			precompileAddr:        common.BytesToAddress([]byte{0x0b}),
			failInput:             common.FromHex("0x0000000000000000000000000000000017f1d3a73197d7942695638c4fa9ac0fc3688c4f9774b905a14e3a3f171bac586c55e83ff97a1aeffb3af00adb22c6bb00000000000000000000000000000000186b28d92356c4dfec4b5201ad099dbdede3781f8998ddf929b4cd7756192185ca7b8f4ef7088f813270ac3d48868a2100000000000000000000000000000000112b98340eee2777cc3c14163dea3ec97977ac3dc5c70da32e6e87578f44912e902ccef9efe28d4a78b8999dfbca942600000000000000000000000000000000186b28d92356c4dfec4b5201ad099dbdede3781f8998ddf929b4cd7756192185ca7b8f4ef7088f813270ac3d48868a21"),
			successInput:          common.FromHex("0x0000000000000000000000000000000017f1d3a73197d7942695638c4fa9ac0fc3688c4f9774b905a14e3a3f171bac586c55e83ff97a1aeffb3af00adb22c6bb0000000000000000000000000000000008b3f481e3aaa0f1a09e30ed741d8ae4fcf5e095d5d00af600db18cb2c04b3edd03cc744a2888ae40caa232946c5e7e100000000000000000000000000000000112b98340eee2777cc3c14163dea3ec97977ac3dc5c70da32e6e87578f44912e902ccef9efe28d4a78b8999dfbca942600000000000000000000000000000000186b28d92356c4dfec4b5201ad099dbdede3781f8998ddf929b4cd7756192185ca7b8f4ef7088f813270ac3d48868a21"),
			expectedResult:        common.FromHex("0x000000000000000000000000000000000a40300ce2dec9888b60690e9a41d3004fda4886854573974fab73b046d3147ba5b7a5bde85279ffede1b45b3918d82d0000000000000000000000000000000006d3d887e9f53b9ec4eb6cedf5607226754b07c01ace7834f57f3e7315faefb739e59018e22c492006190fba4a870025"),
			expectedErrorContains: "invalid point: not on curve",
		},
		{
			precompileName:        "G2Add",
			precompileAddr:        common.BytesToAddress([]byte{0x0d}),
			failInput:             common.FromHex("0x000000000000000000000000000000001c4bb49d2a0ef12b7123acdd7110bd292b5bc659edc54dc21b81de057194c79b2a5803255959bbef8e7f56c8c12168630000000000000000000000000000000013e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e000000000000000000000000000000000ce5d527727d6e118cc9cdc6da2e351aadfd9baa8cbdd3a76d429a695160d12c923ac9cc3baca289e193548608b82801000000000000000000000000000000000606c4a02ea734cc32acd2b02bc28b99cb3e287e85a763af267492ab572e99ab3f370d275cec1da1aaa9075ff05f79be00000000000000000000000000000000103121a2ceaae586d240843a398967325f8eb5a93e8fea99b62b9f88d8556c80dd726a4b30e84a36eeabaf3592937f2700000000000000000000000000000000086b990f3da2aeac0a36143b7d7c824428215140db1bb859338764cb58458f081d92664f9053b50b3fbd2e4723121b68000000000000000000000000000000000f9e7ba9a86a8f7624aa2b42dcc8772e1af4ae115685e60abc2c9b90242167acef3d0be4050bf935eed7c3b6fc7ba77e000000000000000000000000000000000d22c3652d0dc6f0fc9316e14268477c2049ef772e852108d269d9c38dba1d4802e8dae479818184c08f9a569d878451"),
			successInput:          common.FromHex("0x00000000000000000000000000000000024aa2b2f08f0a91260805272dc51051c6e47ad4fa403b02b4510b647ae3d1770bac0326a805bbefd48056c8c121bdb80000000000000000000000000000000013e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e000000000000000000000000000000000ce5d527727d6e118cc9cdc6da2e351aadfd9baa8cbdd3a76d429a695160d12c923ac9cc3baca289e193548608b82801000000000000000000000000000000000606c4a02ea734cc32acd2b02bc28b99cb3e287e85a763af267492ab572e99ab3f370d275cec1da1aaa9075ff05f79be00000000000000000000000000000000103121a2ceaae586d240843a398967325f8eb5a93e8fea99b62b9f88d8556c80dd726a4b30e84a36eeabaf3592937f2700000000000000000000000000000000086b990f3da2aeac0a36143b7d7c824428215140db1bb859338764cb58458f081d92664f9053b50b3fbd2e4723121b68000000000000000000000000000000000f9e7ba9a86a8f7624aa2b42dcc8772e1af4ae115685e60abc2c9b90242167acef3d0be4050bf935eed7c3b6fc7ba77e000000000000000000000000000000000d22c3652d0dc6f0fc9316e14268477c2049ef772e852108d269d9c38dba1d4802e8dae479818184c08f9a569d878451"),
			expectedResult:        common.FromHex("0x000000000000000000000000000000000b54a8a7b08bd6827ed9a797de216b8c9057b3a9ca93e2f88e7f04f19accc42da90d883632b9ca4dc38d013f71ede4db00000000000000000000000000000000077eba4eecf0bd764dce8ed5f45040dd8f3b3427cb35230509482c14651713282946306247866dfe39a8e33016fcbe520000000000000000000000000000000014e60a76a29ef85cbd69f251b9f29147b67cfe3ed2823d3f9776b3a0efd2731941d47436dc6d2b58d9e65f8438bad073000000000000000000000000000000001586c3c910d95754fef7a732df78e279c3d37431c6a2b77e67a00c7c130a8fcd4d19f159cbeb997a178108fffffcbd20"),
			expectedErrorContains: "invalid fp.Element encoding",
		},
		{
			precompileName:        "G1MSM",
			precompileAddr:        common.BytesToAddress([]byte{0x0c}),
			failInput:             common.FromHex("0x0000000000000000000000000000000017f1d3a73197d7942695638c4fa9ac0fc3688c4f9774b905a14e3a3f171bac586c55e83ff97a1aeffb3af00adb22c6bb00000000000000000000000000000000186b28d92356c4dfec4b5201ad099dbdede3781f8998ddf929b4cd7756192185ca7b8f4ef7088f813270ac3d48868a21000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000112b98340eee2777cc3c14163dea3ec97977ac3dc5c70da32e6e87578f44912e902ccef9efe28d4a78b8999dfbca942600000000000000000000000000000000186b28d92356c4dfec4b5201ad099dbdede3781f8998ddf929b4cd7756192185ca7b8f4ef7088f813270ac3d48868a210000000000000000000000000000000000000000000000000000000000000002"),
			successInput:          common.FromHex("0x0000000000000000000000000000000017f1d3a73197d7942695638c4fa9ac0fc3688c4f9774b905a14e3a3f171bac586c55e83ff97a1aeffb3af00adb22c6bb0000000000000000000000000000000008b3f481e3aaa0f1a09e30ed741d8ae4fcf5e095d5d00af600db18cb2c04b3edd03cc744a2888ae40caa232946c5e7e1000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000112b98340eee2777cc3c14163dea3ec97977ac3dc5c70da32e6e87578f44912e902ccef9efe28d4a78b8999dfbca942600000000000000000000000000000000186b28d92356c4dfec4b5201ad099dbdede3781f8998ddf929b4cd7756192185ca7b8f4ef7088f813270ac3d48868a210000000000000000000000000000000000000000000000000000000000000002"),
			expectedResult:        common.FromHex("0x00000000000000000000000000000000148f92dced907361b4782ab542a75281d4b6f71f65c8abf94a5a9082388c64662d30fd6a01ced724feef3e284752038c0000000000000000000000000000000015c3634c3b67bc18e19150e12bfd8a1769306ed010f59be645a0823acb5b38f39e8e0d86e59b6353fdafc59ca971b769"),
			expectedErrorContains: "invalid point: not on curve",
		},
		{
			precompileName:        "G2MSM",
			precompileAddr:        common.BytesToAddress([]byte{0x0e}),
			failInput:             common.FromHex("0x000000000000000000000000000000001c4bb49d2a0ef12b7123acdd7110bd292b5bc659edc54dc21b81de057194c79b2a5803255959bbef8e7f56c8c12168630000000000000000000000000000000013e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e000000000000000000000000000000000ce5d527727d6e118cc9cdc6da2e351aadfd9baa8cbdd3a76d429a695160d12c923ac9cc3baca289e193548608b82801000000000000000000000000000000000606c4a02ea734cc32acd2b02bc28b99cb3e287e85a763af267492ab572e99ab3f370d275cec1da1aaa9075ff05f79be000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000103121a2ceaae586d240843a398967325f8eb5a93e8fea99b62b9f88d8556c80dd726a4b30e84a36eeabaf3592937f2700000000000000000000000000000000086b990f3da2aeac0a36143b7d7c824428215140db1bb859338764cb58458f081d92664f9053b50b3fbd2e4723121b68000000000000000000000000000000000f9e7ba9a86a8f7624aa2b42dcc8772e1af4ae115685e60abc2c9b90242167acef3d0be4050bf935eed7c3b6fc7ba77e000000000000000000000000000000000d22c3652d0dc6f0fc9316e14268477c2049ef772e852108d269d9c38dba1d4802e8dae479818184c08f9a569d8784510000000000000000000000000000000000000000000000000000000000000002"),
			successInput:          common.FromHex("0x00000000000000000000000000000000024aa2b2f08f0a91260805272dc51051c6e47ad4fa403b02b4510b647ae3d1770bac0326a805bbefd48056c8c121bdb80000000000000000000000000000000013e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e000000000000000000000000000000000ce5d527727d6e118cc9cdc6da2e351aadfd9baa8cbdd3a76d429a695160d12c923ac9cc3baca289e193548608b82801000000000000000000000000000000000606c4a02ea734cc32acd2b02bc28b99cb3e287e85a763af267492ab572e99ab3f370d275cec1da1aaa9075ff05f79be000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000103121a2ceaae586d240843a398967325f8eb5a93e8fea99b62b9f88d8556c80dd726a4b30e84a36eeabaf3592937f2700000000000000000000000000000000086b990f3da2aeac0a36143b7d7c824428215140db1bb859338764cb58458f081d92664f9053b50b3fbd2e4723121b68000000000000000000000000000000000f9e7ba9a86a8f7624aa2b42dcc8772e1af4ae115685e60abc2c9b90242167acef3d0be4050bf935eed7c3b6fc7ba77e000000000000000000000000000000000d22c3652d0dc6f0fc9316e14268477c2049ef772e852108d269d9c38dba1d4802e8dae479818184c08f9a569d8784510000000000000000000000000000000000000000000000000000000000000002"),
			expectedResult:        common.FromHex("0x00000000000000000000000000000000009cc9ed6635623ba19b340cbc1b0eb05c3a58770623986bb7e041645175b0a38d663d929afb9a949f7524656043bccc000000000000000000000000000000000c0fb19d3f083fd5641d22a861a11979da258003f888c59c33005cb4a2df4df9e5a2868832063ac289dfa3e997f21f8a00000000000000000000000000000000168bf7d87cef37cf1707849e0a6708cb856846f5392d205ae7418dd94d94ef6c8aa5b424af2e99d957567654b9dae1d90000000000000000000000000000000017e0fa3c3b2665d52c26c7d4cea9f35443f4f9007840384163d3aa3c7d4d18b21b65ff4380cf3f3b48e94b5eecb221dd"),
			expectedErrorContains: "invalid fp.Element encoding",
		},
		{
			precompileName:        "MapFpToG1",
			precompileAddr:        common.BytesToAddress([]byte{0x10}),
			failInput:             common.FromHex("0x000000000000000000000000000000002f6d9c5465982c0421b61e74579709b3b5b91e57bdd4f6015742b4ff301abb7ef895b9cce00c33c7d48f8e5fa4ac09ae"),
			successInput:          common.FromHex("0x00000000000000000000000000000000147e1ed29f06e4c5079b9d14fc89d2820d32419b990c1c7bb7dbea2a36a045124b31ffbde7c99329c05c559af1c6cc82"),
			expectedResult:        common.FromHex("0x00000000000000000000000000000000009769f3ab59bfd551d53a5f846b9984c59b97d6842b20a2c565baa167945e3d026a3755b6345df8ec7e6acb6868ae6d000000000000000000000000000000001532c00cf61aa3d0ce3e5aa20c3b531a2abd2c770a790a2613818303c6b830ffc0ecf6c357af3317b9575c567f11cd2c"),
			expectedErrorContains: "invalid fp.Element encoding",
		},
		{
			precompileName:        "MapFp2ToG2",
			precompileAddr:        common.BytesToAddress([]byte{0x11}),
			failInput:             common.FromHex("0x0000000000000000000000000000000021366f100476ce8d3be6cfc90d59fe13349e388ed12b6dd6dc31ccd267ff000e2c993a063ca66beced06f804d4b8e5af0000000000000000000000000000000002829ce3c021339ccb5caf3e187f6370e1e2a311dec9b75363117063ab2015603ff52c3d3b98f19c2f65575e99e8b78c"),
			successInput:          common.FromHex("0x0000000000000000000000000000000007355d25caf6e7f2f0cb2812ca0e513bd026ed09dda65b177500fa31714e09ea0ded3a078b526bed3307f804d4b93b040000000000000000000000000000000002829ce3c021339ccb5caf3e187f6370e1e2a311dec9b75363117063ab2015603ff52c3d3b98f19c2f65575e99e8b78c"),
			expectedResult:        common.FromHex("0x0000000000000000000000000000000000e7f4568a82b4b7dc1f14c6aaa055edf51502319c723c4dc2688c7fe5944c213f510328082396515734b6612c4e7bb700000000000000000000000000000000126b855e9e69b1f691f816e48ac6977664d24d99f8724868a184186469ddfd4617367e94527d4b74fc86413483afb35b000000000000000000000000000000000caead0fd7b6176c01436833c79d305c78be307da5f6af6c133c47311def6ff1e0babf57a0fb5539fce7ee12407b0a42000000000000000000000000000000001498aadcf7ae2b345243e281ae076df6de84455d766ab6fcdaad71fab60abb2e8b980a440043cd305db09d283c895e3d"),
			expectedErrorContains: "invalid fp.Element encoding",
		},
		{
			precompileName:        "PairingCheck",
			precompileAddr:        common.BytesToAddress([]byte{0x0f}),
			failInput:             common.FromHex("0x000000000000000000000000000000000123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef00000000000000000000000000000000193fb7cedb32b2c3adc06ec11a96bc0d661869316f5e4a577a9f7c179593987beb4fb2ee424dbb2f5dd891e228b46c4a00000000000000000000000000000000024aa2b2f08f0a91260805272dc51051c6e47ad4fa403b02b4510b647ae3d1770bac0326a805bbefd48056c8c121bdb80000000000000000000000000000000013e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e000000000000000000000000000000000ce5d527727d6e118cc9cdc6da2e351aadfd9baa8cbdd3a76d429a695160d12c923ac9cc3baca289e193548608b82801000000000000000000000000000000000606c4a02ea734cc32acd2b02bc28b99cb3e287e85a763af267492ab572e99ab3f370d275cec1da1aaa9075ff05f79be0000000000000000000000000000000017f1d3a73197d7942695638c4fa9ac0fc3688c4f9774b905a14e3a3f171bac586c55e83ff97a1aeffb3af00adb22c6bb0000000000000000000000000000000008b3f481e3aaa0f1a09e30ed741d8ae4fcf5e095d5d00af600db18cb2c04b3edd03cc744a2888ae40caa232946c5e7e100000000000000000000000000000000024aa2b2f08f0a91260805272dc51051c6e47ad4fa403b02b4510b647ae3d1770bac0326a805bbefd48056c8c121bdb80000000000000000000000000000000013e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e000000000000000000000000000000000d1b3cc2c7027888be51d9ef691d77bcb679afda66c73f17f9ee3837a55024f78c71363275a75d75d86bab79f74782aa0000000000000000000000000000000013fa4d4a0ad8b1ce186ed5061789213d993923066dddaf1040bc3ff59f825c78df74f2d75467e25e0f55f8a00fa030ed"),
			successInput:          common.FromHex("0x0000000000000000000000000000000017f1d3a73197d7942695638c4fa9ac0fc3688c4f9774b905a14e3a3f171bac586c55e83ff97a1aeffb3af00adb22c6bb0000000000000000000000000000000008b3f481e3aaa0f1a09e30ed741d8ae4fcf5e095d5d00af600db18cb2c04b3edd03cc744a2888ae40caa232946c5e7e100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000024aa2b2f08f0a91260805272dc51051c6e47ad4fa403b02b4510b647ae3d1770bac0326a805bbefd48056c8c121bdb80000000000000000000000000000000013e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e000000000000000000000000000000000ce5d527727d6e118cc9cdc6da2e351aadfd9baa8cbdd3a76d429a695160d12c923ac9cc3baca289e193548608b82801000000000000000000000000000000000606c4a02ea734cc32acd2b02bc28b99cb3e287e85a763af267492ab572e99ab3f370d275cec1da1aaa9075ff05f79be"),
			expectedResult:        common.FromHex("0x0000000000000000000000000000000000000000000000000000000000000001"),
			expectedErrorContains: "g1 point is not on correct subgroup",
		},
	}

	for _, test := range tests {
		test := test
		for _, precompileToTest := range precompilesToTest {
			precompileToTest := precompileToTest
			t.Run(fmt.Sprintf("EIP2537_%s_%s", test.name, precompileToTest.precompileName), func(t *testing.T) {
				op_e2e.InitParallel(t)
				cfg := e2esys.IsthmusSystemConfig(t, &test.isthmusTime)

				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()

				opGeth, err := NewOpGeth(t, ctx, &cfg)
				require.NoError(t, err)
				defer opGeth.Close()

				test.activateIsthmus(ctx, t, opGeth)

				// check response against test cases
				response, err := opGeth.L2Client.CallContract(ctx, ethereum.CallMsg{
					To:   &precompileToTest.precompileAddr,
					Data: precompileToTest.successInput,
				}, nil)

				if test.expectEmpty {
					require.NoError(t, err)
					require.Equal(t, []byte{}, response, "should return proper result")
					return
				}

				require.NoError(t, err)

				require.Equal(t, precompileToTest.expectedResult, response, "should return proper result")

				// invalid request reverts with an error
				_, err = opGeth.L2Client.CallContract(ctx, ethereum.CallMsg{
					To:   &precompileToTest.precompileAddr,
					Data: precompileToTest.failInput,
				}, nil)

				require.Error(t, err)
				require.ErrorContains(t, err, precompileToTest.expectedErrorContains, "should return proper error")
			})
		}
	}
}
