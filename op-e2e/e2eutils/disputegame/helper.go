package disputegame

import (
	"context"
	"crypto/ecdsa"
	"encoding/binary"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-e2e/config"

	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts/metrics"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/outputs"
	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/challenger"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/disputegame/preimage"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/transactions"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/endpoint"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

var (
	// TestKey is the same test key that geth uses
	TestKey, _  = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	TestAddress = crypto.PubkeyToAddress(TestKey.PublicKey)
)

const (
	cannonGameType       uint32 = 0
	permissionedGameType uint32 = 1
	alphabetGameType     uint32 = 255
)

type GameCfg struct {
	allowFuture bool
	allowUnsafe bool
}
type GameOpt interface {
	Apply(cfg *GameCfg)
}
type gameOptFn func(c *GameCfg)

func (g gameOptFn) Apply(cfg *GameCfg) {
	g(cfg)
}

func WithUnsafeProposal() GameOpt {
	return gameOptFn(func(c *GameCfg) {
		c.allowUnsafe = true
	})
}

func WithFutureProposal() GameOpt {
	return gameOptFn(func(c *GameCfg) {
		c.allowFuture = true
	})
}

type DisputeSystem interface {
	L1BeaconEndpoint() endpoint.RestHTTP
	NodeEndpoint(name string) endpoint.RPC
	NodeClient(name string) *ethclient.Client
	RollupEndpoint(name string) endpoint.RPC
	RollupClient(name string) *sources.RollupClient

	L1Deployments() *genesis.L1Deployments
	RollupCfg() *rollup.Config
	L2Genesis() *core.Genesis
	AllocType() config.AllocType

	AdvanceTime(time.Duration)
}

type FactoryHelper struct {
	T           *testing.T
	Require     *require.Assertions
	System      DisputeSystem
	Client      *ethclient.Client
	Opts        *bind.TransactOpts
	PrivKey     *ecdsa.PrivateKey
	FactoryAddr common.Address
	Factory     *bindings.DisputeGameFactory
	AllocType   config.AllocType
}

type FactoryCfg struct {
	PrivKey *ecdsa.PrivateKey
}

type FactoryOption func(c *FactoryCfg)

func WithFactoryPrivKey(privKey *ecdsa.PrivateKey) FactoryOption {
	return func(c *FactoryCfg) {
		c.PrivKey = privKey
	}
}

func NewFactoryHelper(t *testing.T, ctx context.Context, system DisputeSystem, opts ...FactoryOption) *FactoryHelper {
	require := require.New(t)
	client := system.NodeClient("l1")
	chainID, err := client.ChainID(ctx)
	require.NoError(err)

	allocType := system.AllocType()
	require.True(allocType.UsesProofs(), "AllocType %v does not support proofs", allocType)

	factoryCfg := &FactoryCfg{PrivKey: TestKey}
	for _, opt := range opts {
		opt(factoryCfg)
	}
	txOpts, err := bind.NewKeyedTransactorWithChainID(factoryCfg.PrivKey, chainID)
	require.NoError(err)

	l1Deployments := system.L1Deployments()
	factoryAddr := l1Deployments.DisputeGameFactoryProxy
	factory, err := bindings.NewDisputeGameFactory(factoryAddr, client)
	require.NoError(err)

	return &FactoryHelper{
		T:           t,
		Require:     require,
		System:      system,
		Client:      client,
		Opts:        txOpts,
		PrivKey:     factoryCfg.PrivKey,
		Factory:     factory,
		FactoryAddr: factoryAddr,
		AllocType:   allocType,
	}
}

func (h *FactoryHelper) PreimageHelper(ctx context.Context) *preimage.Helper {
	opts := &bind.CallOpts{Context: ctx}
	gameAddr, err := h.Factory.GameImpls(opts, cannonGameType)
	h.Require.NoError(err)
	caller := batching.NewMultiCaller(h.Client.Client(), batching.DefaultBatchSize)
	game, err := contracts.NewFaultDisputeGameContract(ctx, metrics.NoopContractMetrics, gameAddr, caller)
	h.Require.NoError(err)
	vm, err := game.Vm(ctx)
	h.Require.NoError(err)
	oracle, err := vm.Oracle(ctx)
	h.Require.NoError(err)
	return preimage.NewHelper(h.T, h.PrivKey, h.Client, oracle)
}

func NewGameCfg(opts ...GameOpt) *GameCfg {
	cfg := &GameCfg{}
	for _, opt := range opts {
		opt.Apply(cfg)
	}
	return cfg
}

func (h *FactoryHelper) StartOutputCannonGameWithCorrectRoot(ctx context.Context, l2Node string, l2BlockNumber uint64, opts ...GameOpt) *OutputCannonGameHelper {
	cfg := NewGameCfg(opts...)
	h.WaitForBlock(l2Node, l2BlockNumber, cfg)
	output, err := h.System.RollupClient(l2Node).OutputAtBlock(ctx, l2BlockNumber)
	h.Require.NoErrorf(err, "Failed to get output at block %v", l2BlockNumber)
	return h.StartOutputCannonGame(ctx, l2Node, l2BlockNumber, common.Hash(output.OutputRoot), opts...)
}

func (h *FactoryHelper) StartOutputCannonGame(ctx context.Context, l2Node string, l2BlockNumber uint64, rootClaim common.Hash, opts ...GameOpt) *OutputCannonGameHelper {
	return h.startOutputCannonGameOfType(ctx, l2Node, l2BlockNumber, rootClaim, cannonGameType, opts...)
}

func (h *FactoryHelper) StartPermissionedGame(ctx context.Context, l2Node string, l2BlockNumber uint64, rootClaim common.Hash, opts ...GameOpt) *OutputCannonGameHelper {
	return h.startOutputCannonGameOfType(ctx, l2Node, l2BlockNumber, rootClaim, permissionedGameType, opts...)
}

func (h *FactoryHelper) startOutputCannonGameOfType(ctx context.Context, l2Node string, l2BlockNumber uint64, rootClaim common.Hash, gameType uint32, opts ...GameOpt) *OutputCannonGameHelper {
	cfg := NewGameCfg(opts...)
	logger := testlog.Logger(h.T, log.LevelInfo).New("role", "OutputCannonGameHelper")
	rollupClient := h.System.RollupClient(l2Node)
	l2Client := h.System.NodeClient(l2Node)

	extraData := h.CreateBisectionGameExtraData(l2Node, l2BlockNumber, cfg)

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	tx, err := transactions.PadGasEstimate(h.Opts, 2, func(opts *bind.TransactOpts) (*types.Transaction, error) {
		return h.Factory.Create(opts, gameType, rootClaim, extraData)
	})
	h.Require.NoError(err, "create fault dispute game")
	rcpt, err := wait.ForReceiptOK(ctx, h.Client, tx.Hash())
	h.Require.NoError(err, "wait for create fault dispute game receipt to be OK")
	h.Require.Len(rcpt.Logs, 2, "should have emitted a single DisputeGameCreated event")
	createdEvent, err := h.Factory.ParseDisputeGameCreated(*rcpt.Logs[1])
	h.Require.NoError(err)
	game, err := contracts.NewFaultDisputeGameContract(ctx, metrics.NoopContractMetrics, createdEvent.DisputeProxy, batching.NewMultiCaller(h.Client.Client(), batching.DefaultBatchSize))
	h.Require.NoError(err)

	prestateBlock, poststateBlock, err := game.GetBlockRange(ctx)
	h.Require.NoError(err, "Failed to load starting block number")
	splitDepth, err := game.GetSplitDepth(ctx)
	h.Require.NoError(err, "Failed to load split depth")
	l1Head := h.GetL1Head(ctx, game)

	prestateProvider := outputs.NewPrestateProvider(rollupClient, prestateBlock)
	provider := outputs.NewTraceProvider(logger, prestateProvider, rollupClient, l2Client, l1Head, splitDepth, prestateBlock, poststateBlock)

	return &OutputCannonGameHelper{
		OutputGameHelper: *NewOutputGameHelper(h.T, h.Require, h.Client, h.Opts, h.PrivKey, game, h.FactoryAddr, createdEvent.DisputeProxy, provider, h.System, h.AllocType),
	}
}

func (h *FactoryHelper) GetL1Head(ctx context.Context, game contracts.FaultDisputeGameContract) eth.BlockID {
	l1HeadHash, err := game.GetL1Head(ctx)
	h.Require.NoError(err, "Failed to load L1 head")
	l1Header, err := h.Client.HeaderByHash(ctx, l1HeadHash)
	h.Require.NoError(err, "Failed to load L1 header")
	l1Head := eth.HeaderBlockID(l1Header)
	return l1Head
}

func (h *FactoryHelper) StartOutputAlphabetGameWithCorrectRoot(ctx context.Context, l2Node string, l2BlockNumber uint64, opts ...GameOpt) *OutputAlphabetGameHelper {
	cfg := NewGameCfg(opts...)
	h.WaitForBlock(l2Node, l2BlockNumber, cfg)
	output, err := h.System.RollupClient(l2Node).OutputAtBlock(ctx, l2BlockNumber)
	h.Require.NoErrorf(err, "Failed to get output at block %v", l2BlockNumber)
	return h.StartOutputAlphabetGame(ctx, l2Node, l2BlockNumber, common.Hash(output.OutputRoot))
}

func (h *FactoryHelper) StartOutputAlphabetGame(ctx context.Context, l2Node string, l2BlockNumber uint64, rootClaim common.Hash, opts ...GameOpt) *OutputAlphabetGameHelper {
	cfg := NewGameCfg(opts...)
	logger := testlog.Logger(h.T, log.LevelInfo).New("role", "OutputAlphabetGameHelper")
	rollupClient := h.System.RollupClient(l2Node)
	l2Client := h.System.NodeClient(l2Node)

	extraData := h.CreateBisectionGameExtraData(l2Node, l2BlockNumber, cfg)

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	tx, err := transactions.PadGasEstimate(h.Opts, 2, func(opts *bind.TransactOpts) (*types.Transaction, error) {
		return h.Factory.Create(opts, alphabetGameType, rootClaim, extraData)
	})
	h.Require.NoError(err, "create output bisection game")
	rcpt, err := wait.ForReceiptOK(ctx, h.Client, tx.Hash())
	h.Require.NoError(err, "wait for create output bisection game receipt to be OK")
	h.Require.Len(rcpt.Logs, 2, "should have emitted a single DisputeGameCreated event")
	createdEvent, err := h.Factory.ParseDisputeGameCreated(*rcpt.Logs[1])
	h.Require.NoError(err)
	game, err := contracts.NewFaultDisputeGameContract(ctx, metrics.NoopContractMetrics, createdEvent.DisputeProxy, batching.NewMultiCaller(h.Client.Client(), batching.DefaultBatchSize))
	h.Require.NoError(err)

	prestateBlock, poststateBlock, err := game.GetBlockRange(ctx)
	h.Require.NoError(err, "Failed to load starting block number")
	splitDepth, err := game.GetSplitDepth(ctx)
	h.Require.NoError(err, "Failed to load split depth")
	l1Head := h.GetL1Head(ctx, game)
	prestateProvider := outputs.NewPrestateProvider(rollupClient, prestateBlock)

	provider := outputs.NewTraceProvider(logger, prestateProvider, rollupClient, l2Client, l1Head, splitDepth, prestateBlock, poststateBlock)

	return &OutputAlphabetGameHelper{
		OutputGameHelper: *NewOutputGameHelper(h.T, h.Require, h.Client, h.Opts, h.PrivKey, game, h.FactoryAddr, createdEvent.DisputeProxy, provider, h.System, h.AllocType),
	}
}

func (h *FactoryHelper) CreateBisectionGameExtraData(l2Node string, l2BlockNumber uint64, cfg *GameCfg) []byte {
	h.WaitForBlock(l2Node, l2BlockNumber, cfg)
	h.T.Logf("Creating game with l2 block number: %v", l2BlockNumber)
	extraData := make([]byte, 32)
	binary.BigEndian.PutUint64(extraData[24:], l2BlockNumber)
	return extraData
}

func (h *FactoryHelper) WaitForBlock(l2Node string, l2BlockNumber uint64, cfg *GameCfg) {
	if cfg.allowFuture {
		// Proposing a block that doesn't exist yet, so don't perform any checks
		return
	}

	l2Client := h.System.NodeClient(l2Node)
	if cfg.allowUnsafe {
		_, err := geth.WaitForBlock(new(big.Int).SetUint64(l2BlockNumber), l2Client, geth.WithAbsoluteTimeout(time.Minute))
		h.Require.NoErrorf(err, "Block number %v did not become unsafe", l2BlockNumber)
	} else {
		_, err := geth.WaitForBlockToBeSafe(new(big.Int).SetUint64(l2BlockNumber), l2Client, 1*time.Minute)
		h.Require.NoErrorf(err, "Block number %v did not become safe", l2BlockNumber)
	}
}

func (h *FactoryHelper) StartChallenger(ctx context.Context, name string, options ...challenger.Option) *challenger.Helper {
	opts := []challenger.Option{
		challenger.WithFactoryAddress(h.FactoryAddr),
	}
	opts = append(opts, options...)
	c := challenger.NewChallenger(h.T, ctx, h.System, name, opts...)
	h.T.Cleanup(func() {
		_ = c.Close()
	})
	return c
}
