package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"path"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/superutil"

	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/holiman/uint256"
	"github.com/pkg/profile"
	"github.com/urfave/cli/v2"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	gstate "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	tracelogger "github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"

	op_service "github.com/ethereum-optimism/optimism/op-service"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum-optimism/optimism/op-service/ctxinterrupt"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
)

var EnvPrefix = "OP_SIMULATE"

var (
	RPCFlag = &cli.StringFlag{
		Name:     "rpc",
		Usage:    "RPC endpoint to fetch prestate from",
		EnvVars:  op_service.PrefixEnvVar(EnvPrefix, "RPC"),
		Required: true,
	}
	TxFlag = &cli.StringFlag{
		Name:     "tx",
		Usage:    "Transaction hash to trace and simulate",
		EnvVars:  op_service.PrefixEnvVar(EnvPrefix, "TX"),
		Required: true,
	}
	ProfFlag = &cli.BoolFlag{
		Name:     "profile",
		Usage:    "profile the tx processing",
		EnvVars:  op_service.PrefixEnvVar(EnvPrefix, "PROFILE"),
		Required: false,
	}
)

func main() {
	flags := []cli.Flag{
		RPCFlag, TxFlag, ProfFlag,
	}
	flags = append(flags, oplog.CLIFlags(EnvPrefix)...)

	app := cli.NewApp()
	app.Name = "op-simulate"
	app.Usage = "Simulate a tx locally."
	app.Description = "Fetch a tx from an RPC and simulate it locally."
	app.Flags = cliapp.ProtectFlags(flags)
	app.Action = mainAction
	app.Writer = os.Stdout
	app.ErrWriter = os.Stderr
	err := app.Run(os.Args)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Application failed: %v", err)
		os.Exit(1)
	}
}

func mainAction(c *cli.Context) error {
	ctx := ctxinterrupt.WithCancelOnInterrupt(c.Context)
	logCfg := oplog.ReadCLIConfig(c)
	logger := oplog.NewLogger(c.App.Writer, logCfg)

	endpoint := c.String(RPCFlag.Name)
	cl, err := rpc.DialContext(ctx, endpoint)
	if err != nil {
		return fmt.Errorf("failed to dial RPC %q: %w", endpoint, err)
	}
	txHashStr := c.String(TxFlag.Name)
	var txHash common.Hash
	if err := txHash.UnmarshalText([]byte(txHashStr)); err != nil {
		return fmt.Errorf("invalid tx hash: %q", txHashStr)
	}
	prestatesDir := "."
	if err := fetchPrestate(ctx, cl, prestatesDir, txHash); err != nil {
		return fmt.Errorf("failed to prepare prestate: %w", err)
	}
	chainConfig, err := fetchChainConfig(ctx, cl)
	if err != nil {
		return fmt.Errorf("failed to get chain config: %w", err)
	}
	tx, err := fetchTx(ctx, cl, txHash)
	if err != nil {
		return fmt.Errorf("failed to get TX: %w", err)
	}
	rec, err := fetchReceipt(ctx, cl, txHash)
	if err != nil {
		return fmt.Errorf("failed to get receipt: %w", err)
	}
	header, err := fetchHeader(ctx, cl, rec.BlockHash)
	if err != nil {
		return fmt.Errorf("failed to get block header: %w", err)
	}
	doProfile := c.Bool(ProfFlag.Name)
	if err := simulate(ctx, logger, chainConfig, prestateTraceFile(prestatesDir, txHash), tx, header, doProfile); err != nil {
		return fmt.Errorf("failed to simulate tx: %w", err)
	}
	return nil
}

// TraceConfig is different than Geth TraceConfig, quicknode sin't flexible
type TraceConfig struct {
	*tracelogger.Config
	Tracer  string  `json:"tracer"`
	Timeout *string `json:"timeout"`
	// Config specific to given tracer. Note struct logger
	// config are historically embedded in main object.
	TracerConfig json.RawMessage
}

func prestateTraceFile(dir string, txHash common.Hash) string {
	return path.Join(dir, "prestate_"+txHash.String()+".json")
}

func fetchPrestate(ctx context.Context, cl *rpc.Client, dir string, txHash common.Hash) error {
	dest := prestateTraceFile(dir, txHash)
	// check cache
	_, err := os.Stat(dest)
	if err == nil {
		// already known file
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to check prestate file %q: %w", dest, err)
	}
	var result json.RawMessage
	if err := cl.CallContext(ctx, &result, "debug_traceTransaction", txHash, TraceConfig{
		Config: &tracelogger.Config{
			EnableMemory:     false,
			DisableStack:     true,
			DisableStorage:   true,
			EnableReturnData: false,
			Limit:            0,
			Overrides:        nil,
		},
		Tracer:       "prestateTracer",
		Timeout:      nil,
		TracerConfig: nil,
	}); err != nil {
		return fmt.Errorf("failed to retrieve prestate trace: %w", err)
	}
	if err := os.WriteFile(dest, result, 0644); err != nil {
		return fmt.Errorf("failed to write prestate trace: %w", err)
	}
	return nil
}

func fetchChainConfig(ctx context.Context, cl *rpc.Client) (*params.ChainConfig, error) {
	// first try the chain-ID RPC, this is widely available on any RPC provider.
	var idResult hexutil.Big
	if err := cl.CallContext(ctx, &idResult, "eth_chainId"); err != nil {
		return nil, fmt.Errorf("failed to retrieve chain ID: %w", err)
	}
	// if we recognize the chain ID, we can get the chain config
	id := (*big.Int)(&idResult)
	if id.IsUint64() {
		cfg, err := superutil.LoadOPStackChainConfigFromChainID(id.Uint64())
		if err == nil {
			return cfg, nil
		}
		// ignore error, try to fetch chain config in full
	}
	// if not already recognized, then fetch the chain config manually
	var config params.ChainConfig
	if err := cl.CallContext(ctx, &config, "eth_chainConfig"); err != nil {
		return nil, fmt.Errorf("failed to retrieve chain config: %w", err)
	}
	return &config, nil
}

func fetchTx(ctx context.Context, cl *rpc.Client, txHash common.Hash) (*types.Transaction, error) {
	tx, pending, err := ethclient.NewClient(cl).TransactionByHash(ctx, txHash)
	if pending {
		return nil, fmt.Errorf("tx %s is still pending", txHash)
	}
	return tx, err
}

func fetchReceipt(ctx context.Context, cl *rpc.Client, txHash common.Hash) (*types.Receipt, error) {
	return ethclient.NewClient(cl).TransactionReceipt(ctx, txHash)
}

func fetchHeader(ctx context.Context, cl *rpc.Client, blockHash common.Hash) (*types.Header, error) {
	return ethclient.NewClient(cl).HeaderByHash(ctx, blockHash)
}

type DumpAccount struct {
	Balance hexutil.Big                 `json:"balance"`
	Nonce   uint64                      `json:"nonce"`
	Code    hexutil.Bytes               `json:"code,omitempty"`
	Storage map[common.Hash]common.Hash `json:"storage,omitempty"`
}

func readDump(prestatePath string) (map[common.Address]DumpAccount, error) {
	f, err := os.Open(prestatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load state data: %w", err)
	}
	var out map[common.Address]DumpAccount
	if err := json.NewDecoder(f).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to decode state data: %w", err)
	}
	return out, nil
}

type simChainContext struct {
	eng  consensus.Engine
	head *types.Header
}

func (d *simChainContext) Engine() consensus.Engine {
	return d.eng
}

func (d *simChainContext) GetHeader(h common.Hash, n uint64) *types.Header {
	if n == d.head.Number.Uint64() {
		return d.head
	}
	panic(fmt.Errorf("header retrieval not supported, cannot fetch %s %d", h, n))
}

func simulate(ctx context.Context, logger log.Logger, conf *params.ChainConfig,
	prestatePath string, tx *types.Transaction, header *types.Header, doProfile bool) error {
	memDB := rawdb.NewMemoryDatabase()
	stateDB := gstate.NewDatabase(triedb.NewDatabase(memDB, nil), nil)
	state, err := gstate.New(types.EmptyRootHash, stateDB)
	if err != nil {
		return fmt.Errorf("failed to create in-memory state: %w", err)
	}
	dump, err := readDump(prestatePath)
	if err != nil {
		return fmt.Errorf("failed to read prestate: %w", err)
	}
	for addr, acc := range dump {
		state.CreateAccount(addr)
		state.SetBalance(addr, uint256.MustFromBig((*big.Int)(&acc.Balance)), tracing.BalanceChangeUnspecified)
		state.SetNonce(addr, acc.Nonce)
		state.SetCode(addr, acc.Code)
		state.SetStorage(addr, acc.Storage)
	}

	// load prestate data into memory db state
	_, err = state.Commit(header.Number.Uint64()-1, true, conf.IsCancun(header.Number, header.Time))
	if err != nil {
		return fmt.Errorf("failed to write state data to underlying DB: %w", err)
	}

	rules := conf.Rules(header.Number, true, header.Time)
	signer := types.MakeSigner(conf, header.Number, header.Time)
	sender, err := signer.Sender(tx)
	if err != nil {
		return fmt.Errorf("failed to get tx sender: %w", err)
	}
	// prepare the state
	precompiles := vm.ActivePrecompiles(rules)
	state.Prepare(rules, sender, header.Coinbase, tx.To(), precompiles, tx.AccessList())
	state.SetTxContext(tx.Hash(), 0)

	cCtx := &simChainContext{eng: beacon.NewFaker(), head: header}
	gp := core.GasPool(tx.Gas())
	usedGas := uint64(0)
	vmConfig := vm.Config{}

	if doProfile {
		prof := profile.Start(profile.NoShutdownHook, profile.ProfilePath("."), profile.CPUProfile)
		defer prof.Stop()
	}

	// run the transaction
	start := time.Now()
	// nil block-author, since it defaults to header.coinbase
	blockCtx := core.NewEVMBlockContext(header, cCtx, nil, conf, state)
	evm := vm.NewEVM(blockCtx, state, conf, vmConfig)
	receipt, err := core.ApplyTransaction(evm, &gp, state, header, tx, &usedGas)
	if err != nil {
		return fmt.Errorf("failed to apply tx: %w", err)
	}
	end := time.Since(start)
	logger.Info("processed tx", "elapsed", end,
		"ok", receipt.Status == types.ReceiptStatusSuccessful, "logs", len(receipt.Logs))

	return nil
}
