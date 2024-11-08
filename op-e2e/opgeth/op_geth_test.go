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
