package main

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"time"

	clients2 "github.com/ethereum-optimism/optimism/op-chain-ops/clients"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/testutils"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/mattn/go-isatty"
	"github.com/urfave/cli/v2"
)

func main() {
	color := isatty.IsTerminal(os.Stderr.Fd())
	oplog.SetGlobalLogHandler(log.NewTerminalHandler(os.Stderr, color))

	app := cli.NewApp()
	app.Name = "check-derivation"
	app.Usage = "Optimism derivation checker"
	app.Commands = []*cli.Command{
		{
			Name:  "detect-l2-reorg",
			Usage: "Detects unsafe block reorg",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "l2-rpc-url",
					Value:   "http://127.0.0.1:9545",
					Usage:   "L2 RPC URL",
					EnvVars: []string{"L2_RPC_URL"},
				},
				&cli.DurationFlag{
					Name:  "polling-interval",
					Value: time.Millisecond * 500,
					Usage: "Polling interval",
				},
			},
			Action: detectL2Reorg,
		},
		{
			Name:  "check-consolidation",
			Usage: "Checks consolidation",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "l2-rpc-url",
					Value:   "http://127.0.0.1:9545",
					Usage:   "L2 RPC URL",
					EnvVars: []string{"L2_RPC_URL"},
				},
				&cli.DurationFlag{
					Name:  "polling-interval",
					Value: time.Millisecond * 1000,
					Usage: "Polling interval",
				},
				&cli.StringFlag{
					Name:  "private-key",
					Value: "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
					Usage: "Private key for signing L2 transactions. " +
						"Default: devnet pre-funded account",
				},
				&cli.IntFlag{
					Name:  "tx-count",
					Value: 4,
					Usage: "Number of transactions to send. Minimum value is 4 for checking every tx type.",
				},
				&cli.Uint64Flag{
					Name:  "l2-chain-id",
					Value: 901,
					Usage: "L2 chain ID",
				},
			},
			Action: checkConsolidation,
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Crit("error checking l2", "err", err)
	}
}

func newClientsFromContext(cliCtx *cli.Context) (*ethclient.Client, *sources.EthClient, error) {
	clients, err := clients2.NewClientsFromContext(cliCtx)
	if err != nil {
		return nil, nil, err
	}
	ethClCfg := sources.EthClientConfig{
		MaxRequestsPerBatch:   10,
		MaxConcurrentRequests: 10,
		ReceiptsCacheSize:     10,
		TransactionsCacheSize: 10,
		HeadersCacheSize:      10,
		PayloadsCacheSize:     10,
		TrustRPC:              false,
		MustBePostMerge:       true,
		RPCProviderKind:       sources.RPCKindStandard,
		MethodResetDuration:   time.Minute,
	}
	cl := ethclient.NewClient(clients.L2RpcClient)
	ethCl, err := sources.NewEthClient(client.NewBaseRPCClient(clients.L2RpcClient), log.Root(), nil, &ethClCfg)
	if err != nil {
		return nil, nil, err
	}
	return cl, ethCl, nil
}

func getHead(ctx context.Context, client *sources.EthClient, label eth.BlockLabel) (eth.BlockID, common.Hash, error) {
	return retry.Do2(ctx, 10, &retry.FixedStrategy{Dur: 100 * time.Millisecond}, func() (eth.BlockID, common.Hash, error) {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		blockInfo, err := client.InfoByLabel(ctx, label)
		if err != nil {
			return eth.BlockID{}, common.Hash{}, err
		}
		return eth.BlockID{Hash: blockInfo.Hash(), Number: blockInfo.NumberU64()}, blockInfo.ParentHash(), nil
	})
}

func getUnsafeHead(ctx context.Context, client *sources.EthClient) (eth.BlockID, common.Hash, error) {
	return getHead(ctx, client, eth.Unsafe)
}

func getSafeHead(ctx context.Context, client *sources.EthClient) (eth.BlockID, common.Hash, error) {
	return getHead(ctx, client, eth.Safe)
}

func checkReorg(blockMap map[uint64]common.Hash, number uint64, hash common.Hash) {
	prevHash, ok := blockMap[number]
	if ok {
		if prevHash != hash {
			log.Error("Unsafe head reorg", "blockNum:", number,
				"prevHash", prevHash.String(), "currHash", hash.String())
		}
	}
}

// detectL2Reorg polls safe heads and detects l2 unsafe block reorg.
func detectL2Reorg(cliCtx *cli.Context) error {
	ctx := context.Background()
	_, ethCl, err := newClientsFromContext(cliCtx)
	if err != nil {
		return err
	}

	pollingInterval := cliCtx.Duration("polling-interval")
	// blockMap maps blockNumber to blockHash
	blockMap := make(map[uint64]common.Hash)
	var prevUnsafeHeadNum uint64
	for {
		unsafeHeadBlockId, parentHash, err := getUnsafeHead(ctx, ethCl)
		if err != nil {
			return fmt.Errorf("failed to fetch unsafe head: %w", err)
		}
		checkReorg(blockMap, unsafeHeadBlockId.Number-1, parentHash)
		checkReorg(blockMap, unsafeHeadBlockId.Number, unsafeHeadBlockId.Hash)

		if unsafeHeadBlockId.Number > prevUnsafeHeadNum {
			log.Info("Fetched Unsafe block", "blockNum", unsafeHeadBlockId.Number, "hash", unsafeHeadBlockId.Hash.String())
		}

		blockMap[unsafeHeadBlockId.Number-1] = parentHash
		blockMap[unsafeHeadBlockId.Number] = unsafeHeadBlockId.Hash
		prevUnsafeHeadNum = unsafeHeadBlockId.Number
		time.Sleep(pollingInterval)
	}
}

// getRandomAddress returns vanity address of the form 0x000000000000000000000000[random 32 bits][prefix]
// example: 0x00000000000000000000000030bd3402deadbeef
func getRandomAddress(rng *rand.Rand, prefix uint64) common.Address {
	var vanity uint64 = prefix + (uint64(rng.Uint32()) << 32)
	return common.HexToAddress(fmt.Sprintf("0x%X", vanity))
}

func getPrivateKey(cliCtx *cli.Context) (*ecdsa.PrivateKey, error) {
	privateKey, err := crypto.HexToECDSA(cliCtx.String("private-key"))
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	return privateKey, nil
}

func getSenderAddress(privateKey *ecdsa.PrivateKey) (common.Address, error) {
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return common.Address{}, fmt.Errorf("error casting public key to ECDSA")
	}
	from := crypto.PubkeyToAddress(*publicKeyECDSA)
	return from, nil
}

// getRandomSignedTransaction returns signed tx which sends 1 wei to random address
func getRandomSignedTransaction(ctx context.Context, ethClient *ethclient.Client, rng *rand.Rand, from common.Address, privateKey *ecdsa.PrivateKey, chainId *big.Int, txType int, protected bool) (*types.Transaction, error) {
	randomAddress := getRandomAddress(rng, 0xDEADBEEF)
	amount := big.NewInt(1)
	nonce, err := ethClient.PendingNonceAt(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	gasPrice, err := ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to suggest gas price: %w", err)
	}
	data := testutils.RandomData(rng, 10)
	var txData types.TxData
	switch txType {
	case types.LegacyTxType:
		gasLimit, err := core.IntrinsicGas(data, nil, false, true, true, false)
		if err != nil {
			return nil, fmt.Errorf("failed to get intrinsicGas: %w", err)
		}
		txData = &types.LegacyTx{
			Nonce:    nonce,
			GasPrice: gasPrice,
			Gas:      gasLimit,
			To:       &randomAddress,
			Value:    amount,
			Data:     data,
		}
	case types.AccessListTxType:
		accessList := types.AccessList{types.AccessTuple{
			Address:     randomAddress,
			StorageKeys: []common.Hash{common.HexToHash("0x1234")},
		}}
		gasLimit, err := core.IntrinsicGas(data, accessList, false, true, true, false)
		if err != nil {
			return nil, fmt.Errorf("failed to get intrinsicGas: %w", err)
		}
		txData = &types.AccessListTx{
			ChainID:    chainId,
			Nonce:      nonce,
			GasPrice:   gasPrice,
			Gas:        gasLimit,
			To:         &randomAddress,
			Value:      amount,
			AccessList: accessList,
			Data:       data,
		}
	case types.DynamicFeeTxType:
		gasLimit, err := core.IntrinsicGas(data, nil, false, true, true, false)
		if err != nil {
			return nil, fmt.Errorf("failed to get intrinsicGas: %w", err)
		}
		gasTipCap, err := ethClient.SuggestGasTipCap(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get gas tip cap: %w", err)
		}
		txData = &types.DynamicFeeTx{
			ChainID:   chainId,
			Nonce:     nonce,
			GasTipCap: gasTipCap,
			GasFeeCap: gasPrice,
			Gas:       gasLimit,
			To:        &randomAddress,
			Value:     amount,
			Data:      data,
		}
	default:
		return nil, fmt.Errorf("unsupported tx type: %d", txType)
	}

	tx := types.NewTx(txData)

	signer := types.NewLondonSigner(chainId)
	if !protected {
		if txType == types.LegacyTxType {
			signer = types.HomesteadSigner{}
		} else {
			return nil, errors.New("typed tx cannot be unprotected")
		}
	}

	signedTx, err := types.SignTx(tx, signer, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}
	return signedTx, nil
}

// confirmTransaction polls receipts to confirm transaction is included in the block.
func confirmTransaction(ctx context.Context, ethClient *ethclient.Client, l2BlockTime uint64, txHash common.Hash) (eth.BlockID, error) {
	var retryCount uint64
	for {
		receipt, _ := ethClient.TransactionReceipt(ctx, txHash)
		if retryCount > 30 {
			return eth.BlockID{}, fmt.Errorf("transaction confirmation failure: txHash: %s", txHash.String())
		}
		if receipt == nil {
			log.Info("Waiting for transaction receipt", "txHash", txHash.String())
			retryCount++
			// wait at least l2 block time
			time.Sleep(time.Duration(l2BlockTime) * time.Second)
			continue
		}
		block := eth.BlockID{
			Hash:   receipt.BlockHash,
			Number: receipt.BlockNumber.Uint64(),
		}
		log.Info("Transaction receipt found", "block", block, "status", receipt.Status)
		return block, nil
	}
}

// checkConsolidation sends transactions and ensures them to be included in unsafe block.
// Then polls safe head to check unsafe blocks which includes sent tx are consolidated.
func checkConsolidation(cliCtx *cli.Context) error {
	ctx := context.Background()
	cl, ethCl, err := newClientsFromContext(cliCtx)
	if err != nil {
		return err
	}
	pollingInterval := cliCtx.Duration("polling-interval")
	privateKey, err := getPrivateKey(cliCtx)
	if err != nil {
		return err
	}
	from, err := getSenderAddress(privateKey)
	if err != nil {
		return err
	}
	txCount := cliCtx.Int("tx-count")
	if txCount < 4 {
		return fmt.Errorf("tx count %d is too low. requires minimum 4 txs to test all tx types", txCount)
	}
	l2ChainID := new(big.Int).SetUint64(cliCtx.Uint64("l2-chain-id"))
	l2BlockTime := uint64(2)
	rollupCfg, err := rollup.LoadOPStackRollupConfig(l2ChainID.Uint64())
	if err == nil {
		l2BlockTime = rollupCfg.BlockTime
	} else {
		log.Warn("Superchain config not loaded", "l2-chain-id", l2ChainID)
		log.Warn("Using default config", "l2-block-time", l2BlockTime)
	}
	rng := rand.New(rand.NewSource(1337))
	// txMap maps txHash to blockID
	txMap := make(map[common.Hash]eth.BlockID)
	// Submit random txs for each tx types
	for i := 0; i < txCount; i++ {
		txType := types.LegacyTxType
		protected := true
		// Generate all tx types alternately
		switch i % 4 {
		case 0:
			protected = false // legacy unprotected TX (Homestead)
		case 1:
			// legacy protected TX
		case 2:
			txType = types.AccessListTxType
		case 3:
			txType = types.DynamicFeeTxType
		}
		tx, err := getRandomSignedTransaction(ctx, cl, rng, from, privateKey, l2ChainID, txType, protected)
		if err != nil {
			return err
		}
		err = cl.SendTransaction(ctx, tx)
		if err != nil {
			return fmt.Errorf("failed to send transaction: %w", err)
		}
		log.Info("Sent transaction", "txType", txType, "protected", protected)
		txHash := tx.Hash()
		blockId, err := confirmTransaction(ctx, cl, l2BlockTime, txHash)
		if err != nil {
			return err
		}
		txMap[txHash] = blockId
	}
	lastSafeHeadNum := uint64(0)
	numChecked := 0
	failed := false
	for {
		safeHeadBlockId, _, err := getSafeHead(ctx, ethCl)
		if err != nil {
			return fmt.Errorf("failed to fetch safe head: %w", err)
		}
		log.Info("Fetched Safe head", "blockNum", safeHeadBlockId.Number, "hash", safeHeadBlockId.Hash.String(), "remainingTxCount", txCount-numChecked)

		for txHash, blockId := range txMap {
			if lastSafeHeadNum < blockId.Number && safeHeadBlockId.Number >= blockId.Number {
				safeBlockHash := safeHeadBlockId.Hash
				if safeHeadBlockId.Number != blockId.Number {
					safeBlock, err := retry.Do(ctx, 10, &retry.FixedStrategy{Dur: 100 * time.Millisecond}, func() (*types.Block, error) {
						return cl.BlockByNumber(ctx, new(big.Int).SetUint64(blockId.Number))
					})
					if err != nil {
						return fmt.Errorf("failed to fetch block by number: %w", err)
					}
					safeBlockHash = safeBlock.Hash()
				}
				if safeBlockHash == blockId.Hash {
					log.Info("Transaction included at safe block", "block", blockId, "txHash", txHash.String())
				} else {
					log.Error("Transaction included block is reorged", "blockNum", blockId.Number, "prevHash", blockId.Hash, "currBlock", safeBlockHash, "txHash", txHash.String())
					failed = true
				}
				numChecked++
			}
		}
		if numChecked == txCount {
			if failed {
				log.Error("Failed")
			} else {
				log.Info("Succeeded")
			}
			break
		}
		lastSafeHeadNum = safeHeadBlockId.Number
		time.Sleep(pollingInterval)
	}
	return nil
}
