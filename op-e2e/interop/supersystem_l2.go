package interop

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"time"

	bss "github.com/ethereum-optimism/optimism/op-batcher/batcher"
	batcherFlags "github.com/ethereum-optimism/optimism/op-batcher/flags"
	"github.com/ethereum-optimism/optimism/op-chain-ops/devkeys"
	"github.com/ethereum-optimism/optimism/op-chain-ops/interopgen"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/opnode"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/services"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/setuputils"
	"github.com/ethereum-optimism/optimism/op-node/node"
	"github.com/ethereum-optimism/optimism/op-node/p2p"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/driver"
	"github.com/ethereum-optimism/optimism/op-node/rollup/interop"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	l2os "github.com/ethereum-optimism/optimism/op-proposer/proposer"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/endpoint"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	gn "github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

type l2Node struct {
	name         string
	opNode       *opnode.Opnode
	l2Geth       *geth.GethInstance
	rollupClient *sources.RollupClient
	gethClient   *ethclient.Client
}

type l2Net struct {
	l2Out        *interopgen.L2Output
	chainID      *big.Int
	operatorKeys map[devkeys.ChainOperatorRole]ecdsa.PrivateKey
	contracts    map[string]interface{}
	userKeys     map[string]ecdsa.PrivateKey

	proposer *l2os.ProposerService
	batcher  *bss.BatcherService
	nodes    map[string]*l2Node
}

func (s *interopE2ESystem) L2GethClient(id string, name string) *ethclient.Client {
	net := s.l2s[id]
	node := net.nodes[name]
	if node.gethClient != nil {
		return node.gethClient
	}
	// create a new client for the L2 from the L2's geth instance
	var ethClient services.EthInstance = node.l2Geth
	rpcEndpoint := ethClient.UserRPC()
	rpcCl := endpoint.DialRPC(
		endpoint.PreferAnyRPC,
		rpcEndpoint,
		func(v string) *rpc.Client {
			logger := testlog.Logger(s.t, log.LevelInfo).New("node", id)
			cl, err := dial.DialRPCClientWithTimeout(context.Background(), 30*time.Second, logger, v)
			require.NoError(s.t, err, "failed to dial eth node instance %s", id)
			return cl
		})
	node.gethClient = ethclient.NewClient(rpcCl)
	return node.gethClient
}

func (s *interopE2ESystem) L2RollupClient(id string, name string) *sources.RollupClient {
	net := s.l2s[id]
	node := net.nodes[name]
	if node.rollupClient != nil {
		return node.rollupClient
	}
	rollupClA, err := dial.DialRollupClientWithTimeout(
		context.Background(),
		time.Second*15,
		s.logger,
		node.opNode.UserRPC().RPC())
	require.NoError(s.t, err, "failed to dial rollup client")
	node.rollupClient = rollupClA
	return node.rollupClient
}

// newL2 creates a new L2, starting with the L2Output from the world configuration
// and iterating through the resources needed for the L2.
// it returns a l2Set with the resources for the L2
func (s *interopE2ESystem) newL2(id string, l2Out *interopgen.L2Output) l2Net {
	operatorKeys := s.newOperatorKeysForL2(l2Out)
	l2Geth := s.newGethForL2(id, "sequencer", l2Out)
	opNode := s.newNodeForL2(id, "sequencer", l2Out, operatorKeys, l2Geth, true)
	// TODO(#11886): proposer does not work with the generated world as there is no DisputeGameFactoryProxy
	proposer := s.newProposerForL2(id, operatorKeys)
	batcher := s.newBatcherForL2(id, operatorKeys, l2Geth, opNode)

	return l2Net{
		l2Out:        l2Out,
		chainID:      l2Out.Genesis.Config.ChainID,
		nodes:        map[string]*l2Node{"sequencer": {name: "sequencer", opNode: opNode, l2Geth: l2Geth}},
		proposer:     proposer,
		batcher:      batcher,
		operatorKeys: operatorKeys,
		userKeys:     make(map[string]ecdsa.PrivateKey),
		contracts:    make(map[string]interface{}),
	}
}

func (s *interopE2ESystem) AddNode(id string, name string) {
	l2 := s.l2s[id]
	l2Geth := s.newGethForL2(id, name, l2.l2Out)
	opNode := s.newNodeForL2(id, name, l2.l2Out, l2.operatorKeys, l2Geth, false)
	l2.nodes[name] = &l2Node{name: name, opNode: opNode, l2Geth: l2Geth}

	endpoint, secret := l2.nodes[name].opNode.InteropRPC()
	err := s.SupervisorClient().AddL2RPC(context.Background(), endpoint, secret)
	require.NoError(s.t, err, "failed to add L2 RPC to supervisor")
}

// newNodeForL2 creates a new Opnode for an L2 chain
func (s *interopE2ESystem) newNodeForL2(
	id string,
	name string,
	l2Out *interopgen.L2Output,
	operatorKeys map[devkeys.ChainOperatorRole]ecdsa.PrivateKey,
	l2Geth *geth.GethInstance,
	isSequencer bool,
) *opnode.Opnode {
	logger := s.logger.New("role", "op-node-"+id+"-"+name)
	p2pKey := operatorKeys[devkeys.SequencerP2PRole]
	nodeCfg := &node.Config{
		L1: &node.PreparedL1Endpoint{
			Client: client.NewBaseRPCClient(
				endpoint.DialRPC(endpoint.PreferAnyRPC, s.l1.UserRPC(), mustDial(s.t, logger))),
			TrustRPC:        false,
			RPCProviderKind: sources.RPCKindDebugGeth,
		},
		L2: &node.L2EndpointConfig{
			L2EngineAddr:      l2Geth.AuthRPC().RPC(),
			L2EngineJWTSecret: testingJWTSecret,
		},
		Beacon: &node.L1BeaconEndpointConfig{
			BeaconAddr: s.beacon.BeaconAddr(),
		},
		Driver: driver.Config{
			SequencerEnabled: isSequencer,
		},
		Rollup: *l2Out.RollupCfg,
		P2PSigner: &p2p.PreparedSigner{
			Signer: p2p.NewLocalSigner(&p2pKey)},
		RPC: node.RPCConfig{
			ListenAddr:  "127.0.0.1",
			ListenPort:  0,
			EnableAdmin: true,
		},
		InteropConfig: &interop.Config{
			//SupervisorAddr:   s.supervisor.RPC(),
			RPCAddr:          "127.0.0.1",
			RPCPort:          0,
			RPCJwtSecretPath: "jwt.secret",
		},
		P2P:                         nil, // disabled P2P setup for now
		L1EpochPollInterval:         time.Second * 2,
		RuntimeConfigReloadInterval: 0,
		Tracer:                      nil,
		Sync: sync.Config{
			SyncMode:                       sync.CLSync,
			SkipSyncStartCheck:             false,
			SupportsPostFinalizationELSync: false,
		},
		ConfigPersistence: node.DisabledConfigPersistence{},
	}
	opNode, err := opnode.NewOpnode(logger.New("service", "op-node"),
		nodeCfg, func(err error) {
			s.t.Error(err)
		})
	require.NoError(s.t, err)
	s.t.Cleanup(func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // force-quit
		s.t.Logf("Closing op-node of chain %s", id)
		_ = opNode.Stop(ctx)
		s.t.Logf("Closed op-node of chain %s", id)
	})
	return opNode
}

// newGethForL2 creates a new Geth instance for an L2 chain
func (s *interopE2ESystem) newGethForL2(id string, node string, l2Out *interopgen.L2Output) *geth.GethInstance {
	jwtPath := writeDefaultJWT(s.t)
	name := "l2-" + id + "-" + node
	l2Geth, err := geth.InitL2(name, l2Out.Genesis, jwtPath,
		func(ethCfg *ethconfig.Config, nodeCfg *gn.Config) error {
			ethCfg.InteropMessageRPC = s.supervisor.RPC()
			ethCfg.InteropMempoolFiltering = s.config.mempoolFiltering
			return nil
		})
	require.NoError(s.t, err)
	require.NoError(s.t, l2Geth.Node.Start())
	s.t.Cleanup(func() {
		s.t.Logf("Closing L2 geth of chain %s", id)
		closeErr := l2Geth.Close()
		s.t.Logf("Closed L2 geth of chain %s: %v", id, closeErr)
	})
	return l2Geth
}

func (s *interopE2ESystem) newProposerForL2(
	id string, operatorKeys map[devkeys.ChainOperatorRole]ecdsa.PrivateKey) *l2os.ProposerService {
	key := operatorKeys[devkeys.ProposerRole]
	logger := s.logger.New("role", "proposer"+id)
	proposerCLIConfig := &l2os.CLIConfig{
		L1EthRpc:          s.l1.UserRPC().RPC(),
		SupervisorRpcs:    []string{s.Supervisor().RPC()},
		DGFAddress:        s.worldDeployment.L2s[id].DisputeGameFactoryProxy.Hex(),
		ProposalInterval:  6 * time.Second,
		DisputeGameType:   1, // Permissioned game type is the only one currently deployed
		PollInterval:      500 * time.Millisecond,
		TxMgrConfig:       setuputils.NewTxMgrConfig(s.L1().UserRPC(), &key),
		AllowNonFinalized: true,
		LogConfig: oplog.CLIConfig{
			Level:  log.LvlInfo,
			Format: oplog.FormatText,
		},
	}
	proposer, err := l2os.ProposerServiceFromCLIConfig(context.Background(), "0.0.1", proposerCLIConfig, logger)
	require.NoError(s.t, err)
	s.t.Cleanup(func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // force-quit
		s.t.Logf("Closing proposer of chain %s", id)
		require.NoError(s.t, proposer.Stop(ctx))
		s.t.Logf("Closed proposer of chain %s", id)
	})
	// Note that proposers are not started by default.
	return proposer
}

// newBatcherForL2 creates a new Batcher for an L2 chain
func (s *interopE2ESystem) newBatcherForL2(
	id string,
	operatorKeys map[devkeys.ChainOperatorRole]ecdsa.PrivateKey,
	l2Geth *geth.GethInstance,
	opNode *opnode.Opnode,
) *bss.BatcherService {
	batcherSecret := operatorKeys[devkeys.BatcherRole]
	logger := s.logger.New("role", "batcher"+id)
	batcherCLIConfig := &bss.CLIConfig{
		L1EthRpc:                 s.l1.UserRPC().RPC(),
		L2EthRpc:                 l2Geth.UserRPC().RPC(),
		RollupRpc:                opNode.UserRPC().RPC(),
		MaxPendingTransactions:   1,
		MaxChannelDuration:       1,
		MaxL1TxSize:              120_000,
		TestUseMaxTxSizeForBlobs: false,
		TargetNumFrames:          1,
		ApproxComprRatio:         0.4,
		SubSafetyMargin:          4,
		PollInterval:             50 * time.Millisecond,
		TxMgrConfig:              setuputils.NewTxMgrConfig(s.l1.UserRPC(), &batcherSecret),
		LogConfig: oplog.CLIConfig{
			Level:  log.LevelInfo,
			Format: oplog.FormatText,
		},
		Stopped:               false,
		BatchType:             derive.SpanBatchType,
		MaxBlocksPerSpanBatch: 10,
		DataAvailabilityType:  batcherFlags.CalldataType,
		CompressionAlgo:       derive.Brotli,
	}
	batcher, err := bss.BatcherServiceFromCLIConfig(
		context.Background(), "0.0.1", batcherCLIConfig,
		logger.New("service", "batcher"))
	require.NoError(s.t, err)
	require.NoError(s.t, batcher.Start(context.Background()))
	s.t.Cleanup(func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // force-quit
		s.t.Logf("Closing batcher of chain %s", id)
		_ = batcher.Stop(ctx)
		s.t.Logf("Closed batcher of chain %s", id)
	})
	return batcher
}
