package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core"
	gstate "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/stateless"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	logger2 "github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethdb/remotedb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/triedb"

	op_service "github.com/ethereum-optimism/optimism/op-service"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum-optimism/optimism/op-service/ctxinterrupt"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum-optimism/optimism/op-service/sources"
)

var EnvPrefix = "OP_RUN_BLOCK"

var (
	RPCFlag = &cli.StringFlag{
		Name:     "rpc",
		Usage:    "RPC endpoint to fetch data from",
		EnvVars:  op_service.PrefixEnvVar(EnvPrefix, "RPC"),
		Required: true,
	}
	BlockPathFlag = &cli.PathFlag{
		Name:     "block",
		Usage:    "Path to local block JSON dump. This is a JSON-list of {'block': contentHere} entries, as in the response of the debug_getBadBlocks geth RPC.",
		EnvVars:  op_service.PrefixEnvVar(EnvPrefix, "BLOCK"),
		Required: true,
	}
	OutPathFlag = &cli.PathFlag{
		Name:     "out",
		Usage:    "Path to file to write trace data to. Trace data is formatted as a list of lines, 1 json entry per line, with comment lines that start with a `#`.",
		EnvVars:  op_service.PrefixEnvVar(EnvPrefix, "OUT"),
		Required: true,
	}
)

func main() {
	flags := []cli.Flag{
		RPCFlag, BlockPathFlag, OutPathFlag,
	}
	flags = append(flags, oplog.CLIFlags(EnvPrefix)...)

	app := cli.NewApp()
	app.Name = "op-run-block"
	app.Usage = "Simulate a block locally."
	app.Description = "Take a block JSON and simulate it locally."
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

	rpcEndpoint := c.String(RPCFlag.Name)
	blockPath := c.String(BlockPathFlag.Name)

	cl, err := rpc.DialContext(ctx, rpcEndpoint)
	if err != nil {
		return fmt.Errorf("failed to dial RPC: %w", err)
	}
	defer cl.Close()

	ethCl := ethclient.NewClient(cl)

	db := remotedb.New(cl)

	var config *params.ChainConfig
	if err := cl.CallContext(ctx, &config, "debug_chainConfig"); err != nil {
		return fmt.Errorf("failed to fetch chain config: %w", err)
	}

	block, err := loadBlock(blockPath)
	if err != nil {
		return fmt.Errorf("failed to load block: %w", err)
	}
	if err := block.Verify(); err != nil {
		return fmt.Errorf("block content is invalid: %w", err)
	}
	logger.Info("Loaded block",
		"hash", block.Hash, "number", uint64(block.Number), "txs", len(block.Transactions))

	parentBlock, err := ethCl.HeaderByHash(ctx, block.ParentHash)
	if err != nil {
		return fmt.Errorf("failed to fetch parent block: %w", err)
	}

	stateDB := gstate.NewDatabase(triedb.NewDatabase(db, &triedb.Config{
		Preimages: true,
	}), nil)
	state, err := gstate.New(parentBlock.Root, stateDB)
	if err != nil {
		return fmt.Errorf("failed to create in-memory state: %w", err)
	}

	header := block.RPCHeader.CreateGethHeader()

	vmCfg := vm.Config{Tracer: nil}
	consensusEng := beacon.New(&beacon.OpLegacy{})
	chCtx := &remoteChainCtx{
		consensusEng: consensusEng,
		hdr:          header,
		cfg:          config,
		cl:           ethCl,
		logger:       logger,
	}

	outPath := c.Path(OutPathFlag.Name)
	outW, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return fmt.Errorf("failed to create/open dump file: %w", err)
	}
	defer outW.Close()

	vmCfg.Tracer = logger2.NewJSONLogger(&logger2.Config{
		EnableMemory:     false,
		DisableStack:     false,
		DisableStorage:   false,
		EnableReturnData: false,
		Limit:            0,
		Overrides:        nil,
	}, outW)

	witness, err := stateless.NewWitness(header, chCtx)
	if err != nil {
		return fmt.Errorf("failed to prepare witness data collector: %w", err)
	}
	state.StartPrefetcher("debug", witness)
	defer func() { // Even if the EVM fails, try to export witness data for the state-transition up to the error.
		witnessDump := witness.ToExecutionWitness()
		out, err := json.MarshalIndent(witnessDump, "", "  ")
		if err != nil {
			logger.Error("failed to encode witness", "err", err)
			return
		}
		if err := os.WriteFile("debug_witness.json", out, 0755); err != nil {
			logger.Error("Failed to write witness", "err", err)
		}
	}()
	logger.Info("Starting block processing")
	result, err := Process(logger, config, block, state, vmCfg, chCtx, outW)
	if err != nil {
		return fmt.Errorf("failed to process: %w", err)
	}
	logger.Info("Done", "gas_used", result.GasUsed)
	return nil
}

type WrappedDump struct {
	Block *sources.RPCBlock `json:"block"`
}

func loadBlock(blockPath string) (*sources.RPCBlock, error) {
	data, err := os.ReadFile(blockPath)
	if err != nil {
		return nil, err
	}
	var out []WrappedDump
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if len(out) != 1 {
		return nil, fmt.Errorf("expected single block entry, got %d", len(out))
	}
	blockData := out[0].Block
	return blockData, nil
}

// remoteChainCtx provides access to block-headers, for usage by the state-transition,
// such as basefee computation (based on prior block) and EVM block-hash opcode.
type remoteChainCtx struct {
	consensusEng consensus.Engine
	hdr          *types.Header
	cfg          *params.ChainConfig
	cl           *ethclient.Client
	logger       log.Logger
}

var _ core.ChainContext = (*remoteChainCtx)(nil)
var _ consensus.ChainHeaderReader = (*remoteChainCtx)(nil)

// Config is part of consensus.ChainHeaderReader
func (r *remoteChainCtx) Config() *params.ChainConfig {
	return r.cfg
}

// CurrentHeader is part of consensus.ChainHeaderReader
func (r remoteChainCtx) CurrentHeader() *types.Header {
	return r.hdr
}

// GetHeaderByNumber is part of consensus.ChainHeaderReader
func (r remoteChainCtx) GetHeaderByNumber(u uint64) *types.Header {
	if r.hdr.Number.Uint64() == u {
		return r.hdr
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	hdr, err := retry.Do[*types.Header](ctx, 10, retry.Exponential(), func() (*types.Header, error) {
		r.logger.Info("fetching block header", "num", u)
		return r.cl.HeaderByNumber(ctx, new(big.Int).SetUint64(u))
	})
	if err != nil {
		r.logger.Error("failed to get block header", "err", err, "num", u)
		return nil
	}
	if hdr == nil {
		r.logger.Warn("header not found", "num", u)
	}
	return hdr
}

// GetHeaderByHash is part of consensus.ChainHeaderReader
func (r remoteChainCtx) GetHeaderByHash(hash common.Hash) *types.Header {
	if r.hdr.Hash() == hash {
		return r.hdr
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	hdr, err := retry.Do[*types.Header](ctx, 10, retry.Exponential(), func() (*types.Header, error) {
		r.logger.Info("fetching block header", "hash", hash)
		return r.cl.HeaderByHash(ctx, hash)
	})
	if err != nil {
		r.logger.Error("failed to get block header", "err", err, "hash", hash)
		return nil
	}
	if hdr == nil {
		r.logger.Warn("header not found", "hash", hash)
	}
	return hdr
}

// GetTd is part of consensus.ChainHeaderReader
func (r remoteChainCtx) GetTd(hash common.Hash, number uint64) *big.Int {
	return big.NewInt(1)
}

// Engine is part of core.ChainContext
func (r remoteChainCtx) Engine() consensus.Engine {
	return r.consensusEng
}

// GetHeader is part of both consensus.ChainHeaderReader and core.ChainContext
func (r remoteChainCtx) GetHeader(hash common.Hash, u uint64) *types.Header {
	if r.hdr.Hash() == hash {
		return r.hdr
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	hdr, err := retry.Do[*types.Header](ctx, 10, retry.Exponential(), func() (*types.Header, error) {
		r.logger.Info("fetching block header", "hash", hash, "num", u)
		return r.cl.HeaderByNumber(ctx, new(big.Int).SetUint64(u))
	})
	if err != nil {
		r.logger.Error("failed to get block header", "err", err, "hash", hash, "num", u)
		return nil
	}
	if hdr == nil {
		r.logger.Warn("header not found", "hash", hash, "num", u)
	}
	if got := hdr.Hash(); got != hash {
		r.logger.Error("fetched incompatible header", "expectedHash", hash, "fetchedHash", got, "num", u)
	}
	return hdr
}

func Process(logger log.Logger, config *params.ChainConfig,
	block *sources.RPCBlock,
	statedb *gstate.StateDB, cfg vm.Config,
	chainCtx *remoteChainCtx, outW io.Writer) (*core.ProcessResult, error) {
	var (
		receipts    types.Receipts
		usedGas     = new(uint64)
		header      = block.CreateGethHeader()
		blockHash   = block.Hash
		blockNumber = new(big.Int).SetUint64(uint64(block.Number))
		allLogs     []*types.Log
		gp          = new(core.GasPool).AddGas(uint64(block.GasLimit))
	)

	// Mutate the block and state according to any hard-fork specs
	if config.DAOForkSupport && config.DAOForkBlock != nil && config.DAOForkBlock.Cmp(blockNumber) == 0 {
		misc.ApplyDAOHardFork(statedb)
	}
	misc.EnsureCreate2Deployer(config, uint64(block.Time), statedb)
	var (
		blockContext vm.BlockContext
		signer       = types.MakeSigner(config, header.Number, header.Time)
	)
	blockContext = core.NewEVMBlockContext(header, chainCtx, nil, config, statedb)
	vmenv := vm.NewEVM(blockContext, statedb, config, cfg)
	if beaconRoot := block.ParentBeaconRoot; beaconRoot != nil {
		core.ProcessBeaconBlockRoot(*beaconRoot, vmenv)
	}
	if config.IsPrague(blockNumber, uint64(block.Time)) {
		core.ProcessParentBlockHash(block.ParentHash, vmenv)
	}
	logger.Info("Prepared EVM state")
	_, _ = fmt.Fprintf(outW, "# Prepared state\n")

	// Iterate over and process the individual transactions
	for i, tx := range block.Transactions {
		logger.Info("Processing tx", "i", i, "hash", tx.Hash())
		_, _ = fmt.Fprintf(outW, "# Processing tx %d\n", i)
		msg, err := core.TransactionToMessage(tx, signer, header.BaseFee)
		if err != nil {
			return nil, fmt.Errorf("could not apply tx %d [%v]: %w", i, tx.Hash().Hex(), err)
		}
		statedb.SetTxContext(tx.Hash(), i)

		receipt, err := core.ApplyTransactionWithEVM(msg, gp, statedb, blockNumber, blockHash, tx, usedGas, vmenv)
		if err != nil {
			return nil, fmt.Errorf("could not apply tx %d [%v]: %w", i, tx.Hash().Hex(), err)
		}
		receipts = append(receipts, receipt)
		allLogs = append(allLogs, receipt.Logs...)
	}
	logger.Info("Done with transactions")
	_, _ = fmt.Fprintf(outW, "# Done with transactions\n")

	engine := chainCtx.Engine()
	// Finalize (geth specific term, a.k.a. seal) the block,
	// applying any consensus engine specific extras (e.g. block rewards, withdrawals-root)
	engine.Finalize(chainCtx, header, statedb,
		&types.Body{Transactions: block.Transactions, Withdrawals: *block.Withdrawals})
	logger.Info("Completed block processing")
	_, _ = fmt.Fprintf(outW, "# Completed block processing\n")

	return &core.ProcessResult{
		Receipts: receipts,
		Requests: nil,
		Logs:     allLogs,
		GasUsed:  *usedGas,
	}, nil
}
