package interop

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"os"
	"path"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/eth/ethconfig"
	gn "github.com/ethereum/go-ethereum/node"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/interop/contracts/bindings/emit"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/interop/contracts/bindings/inbox"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/interop/contracts/bindings/systemconfig"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"

	bss "github.com/ethereum-optimism/optimism/op-batcher/batcher"
	batcherFlags "github.com/ethereum-optimism/optimism/op-batcher/flags"
	"github.com/ethereum-optimism/optimism/op-chain-ops/devkeys"
	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-chain-ops/interopgen"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/fakebeacon"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/opnode"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/services"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/setuputils"
	"github.com/ethereum-optimism/optimism/op-e2e/system/helpers"
	"github.com/ethereum-optimism/optimism/op-node/node"
	"github.com/ethereum-optimism/optimism/op-node/p2p"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/driver"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	l2os "github.com/ethereum-optimism/optimism/op-proposer/proposer"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/endpoint"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	supervisorConfig "github.com/ethereum-optimism/optimism/op-supervisor/config"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	supervisortypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// SuperSystem is an interface for the system (collection of connected resources)
// it provides a way to get the resources for a network by network ID
// and provides a way to get the list of network IDs
// this is useful for testing multiple network backends,
// for example, interopE2ESystem is the default implementation, but a shim to
// kurtosis or another testing framework could be implemented
type SuperSystem interface {
	// get the supervisor
	Supervisor() *supervisor.SupervisorService
	// get the supervisor client
	SupervisorClient() *sources.SupervisorClient
	// get the batcher for a network
	Batcher(network string) *bss.BatcherService
	// get the proposer for a network
	Proposer(network string) *l2os.ProposerService
	// get the opnode for a network
	OpNode(network string) *opnode.Opnode
	// get the geth instance for a network
	L2Geth(network string) *geth.GethInstance
	// get the L2 geth client for a network
	L2GethClient(network string) *ethclient.Client
	// get the secret for a network and role
	L2OperatorKey(network string, role devkeys.ChainOperatorRole) ecdsa.PrivateKey
	// get the list of network IDs as key-strings
	L2IDs() []string
	// get the chain ID for a network
	ChainID(network string) *big.Int
	// register a username to an account on all L2s
	AddUser(username string)
	// get the user key for a user on an L2
	UserKey(id, username string) ecdsa.PrivateKey
	// send a transaction on an L2 on the given network, from the given user
	SendL2Tx(network string, username string, applyTxOpts helpers.TxOptsFn) *types.Receipt
	// get the address for a user on an L2
	Address(network string, username string) common.Address
	// Deploy the Emitter Contract, which emits Event Logs
	DeployEmitterContract(network string, username string) common.Address
	// Use the Emitter Contract to emit an Event Log
	EmitData(network string, username string, data string) *types.Receipt
	// AddDependency adds a dependency (by chain ID) to the given chain
	AddDependency(network string, dep *big.Int) *types.Receipt
	// ExecuteMessage calls the CrossL2Inbox executeMessage function
	ExecuteMessage(
		ctx context.Context,
		id string,
		sender string,
		msgIdentifier supervisortypes.Identifier,
		target common.Address,
		message []byte,
		expectedError error,
	) (*types.Receipt, error)
	// Access a contract on a network by name
	Contract(network string, contractName string) interface{}
}
type SuperSystemConfig struct {
	mempoolFiltering bool
}

// NewSuperSystem creates a new SuperSystem from a recipe. It creates an interopE2ESystem.
func NewSuperSystem(t *testing.T, recipe *interopgen.InteropDevRecipe, w worldResourcePaths, config SuperSystemConfig) SuperSystem {
	s2 := &interopE2ESystem{recipe: recipe, config: &config}
	s2.prepare(t, w)
	return s2
}

// interopE2ESystem implements the SuperSystem interface
// it prepares network resources and provides access to them
// the functionality is broken down into smaller functions so that
// the system can be prepared iteratively if desired
type interopE2ESystem struct {
	t               *testing.T
	recipe          *interopgen.InteropDevRecipe
	logger          log.Logger
	hdWallet        *devkeys.MnemonicDevKeys
	worldDeployment *interopgen.WorldDeployment
	worldOutput     *interopgen.WorldOutput
	beacon          *fakebeacon.FakeBeacon
	l1              *geth.GethInstance
	l2s             map[string]l2Set
	l1GethClient    *ethclient.Client
	l2GethClients   map[string]*ethclient.Client
	supervisor      *supervisor.SupervisorService
	superClient     *sources.SupervisorClient
	config          *SuperSystemConfig
}

// l2Set is a set of resources for an L2 chain
type l2Set struct {
	chainID      *big.Int
	opNode       *opnode.Opnode
	l2Geth       *geth.GethInstance
	proposer     *l2os.ProposerService
	batcher      *bss.BatcherService
	operatorKeys map[devkeys.ChainOperatorRole]ecdsa.PrivateKey
	userKeys     map[string]ecdsa.PrivateKey
	contracts    map[string]interface{}
}

// prepareHDWallet creates a new HD wallet to derive keys from
func (s *interopE2ESystem) prepareHDWallet() *devkeys.MnemonicDevKeys {
	hdWallet, err := devkeys.NewMnemonicDevKeys(devkeys.TestMnemonic)
	require.NoError(s.t, err)
	return hdWallet
}

type worldResourcePaths struct {
	foundryArtifacts string
	sourceMap        string
}

// prepareWorld creates the world configuration from the recipe and deploys it
func (s *interopE2ESystem) prepareWorld(w worldResourcePaths) (*interopgen.WorldDeployment, *interopgen.WorldOutput) {
	// Build the world configuration from the recipe and the HD wallet
	worldCfg, err := s.recipe.Build(s.hdWallet)
	require.NoError(s.t, err)

	// create a logger for the world configuration
	logger := s.logger.New("role", "world")
	require.NoError(s.t, worldCfg.Check(logger))

	// create the foundry artifacts and source map
	foundryArtifacts := foundry.OpenArtifactsDir(w.foundryArtifacts)
	sourceMap := foundry.NewSourceMapFS(os.DirFS(w.sourceMap))

	// deploy the world, using the logger, foundry artifacts, source map, and world configuration
	worldDeployment, worldOutput, err := interopgen.Deploy(logger, foundryArtifacts, sourceMap, worldCfg)
	require.NoError(s.t, err)

	return worldDeployment, worldOutput
}

// prepareL1 creates the L1 chain resources
func (s *interopE2ESystem) prepareL1() (*fakebeacon.FakeBeacon, *geth.GethInstance) {
	// Create a fake Beacon node to hold on to blobs created by the L1 miner, and to serve them to L2
	genesisTimestampL1 := s.worldOutput.L1.Genesis.Timestamp
	blockTimeL1 := uint64(6)
	blobPath := s.t.TempDir()
	bcn := fakebeacon.NewBeacon(s.logger.New("role", "l1_cl"),
		e2eutils.NewBlobStore(), genesisTimestampL1, blockTimeL1)
	s.t.Cleanup(func() {
		_ = bcn.Close()
	})
	require.NoError(s.t, bcn.Start("127.0.0.1:0"))
	beaconApiAddr := bcn.BeaconAddr()
	require.NotEmpty(s.t, beaconApiAddr, "beacon API listener must be up")

	l1FinalizedDistance := uint64(3)
	l1Clock := clock.SystemClock
	// Start the L1 chain
	l1Geth, err := geth.InitL1(
		blockTimeL1,
		l1FinalizedDistance,
		s.worldOutput.L1.Genesis,
		l1Clock,
		filepath.Join(blobPath, "l1_el"),
		bcn)

	require.NoError(s.t, err)
	require.NoError(s.t, l1Geth.Node.Start())
	s.t.Cleanup(func() {
		s.t.Logf("Closing L1 geth")
		_ = l1Geth.Close()
	})
	return bcn, l1Geth
}

// newOperatorKeysForL2 creates the operator keys for an L2 chain
// it uses an L2Output to determine the chain ID and configuration,
// and then makes a key for each operator role [SequencerP2PRole, ProposerRole, BatcherRole]
func (s *interopE2ESystem) newOperatorKeysForL2(l2Out *interopgen.L2Output) map[devkeys.ChainOperatorRole]ecdsa.PrivateKey {
	// Create operatorKeys for the L2 chain actors
	operatorKeys := map[devkeys.ChainOperatorRole]ecdsa.PrivateKey{}
	// create the sequencer P2P secret
	seqP2PSecret, err := s.hdWallet.Secret(devkeys.ChainOperatorKey{
		ChainID: l2Out.Genesis.Config.ChainID,
		Role:    devkeys.SequencerP2PRole,
	})
	require.NoError(s.t, err)
	operatorKeys[devkeys.SequencerP2PRole] = *seqP2PSecret
	// create the proposer secret
	proposerSecret, err := s.hdWallet.Secret(devkeys.ChainOperatorKey{
		ChainID: l2Out.Genesis.Config.ChainID,
		Role:    devkeys.ProposerRole,
	})
	require.NoError(s.t, err)
	operatorKeys[devkeys.ProposerRole] = *proposerSecret
	// create the batcher secret
	batcherSecret, err := s.hdWallet.Secret(devkeys.ChainOperatorKey{
		ChainID: l2Out.Genesis.Config.ChainID,
		Role:    devkeys.BatcherRole,
	})
	require.NoError(s.t, err)
	operatorKeys[devkeys.BatcherRole] = *batcherSecret
	return operatorKeys
}

// newGethForL2 creates a new Geth instance for an L2 chain
func (s *interopE2ESystem) newGethForL2(id string, l2Out *interopgen.L2Output) *geth.GethInstance {
	jwtPath := writeDefaultJWT(s.t)
	name := "l2-" + id
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

// newNodeForL2 creates a new Opnode for an L2 chain
func (s *interopE2ESystem) newNodeForL2(
	id string,
	l2Out *interopgen.L2Output,
	operatorKeys map[devkeys.ChainOperatorRole]ecdsa.PrivateKey,
	l2Geth *geth.GethInstance,
) *opnode.Opnode {
	logger := s.logger.New("role", "op-node-"+id)
	p2pKey := operatorKeys[devkeys.SequencerP2PRole]
	nodeCfg := &node.Config{
		L1: &node.PreparedL1Endpoint{
			Client: client.NewBaseRPCClient(endpoint.DialRPC(
				endpoint.PreferAnyRPC,
				s.l1.UserRPC(),
				mustDial(s.t, logger))),
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
			SequencerEnabled: true,
		},
		Rollup: *l2Out.RollupCfg,
		P2PSigner: &p2p.PreparedSigner{
			Signer: p2p.NewLocalSigner(&p2pKey)},
		RPC: node.RPCConfig{
			ListenAddr:  "127.0.0.1",
			ListenPort:  0,
			EnableAdmin: true,
		},
		Supervisor: &node.SupervisorEndpointConfig{
			SupervisorAddr: s.supervisor.RPC(),
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

// newProposerForL2 creates a new Proposer for an L2 chain
// it is currently unused, as the generated world does not have a DisputeGameFactoryProxy
// TODO(#11888): name this function "newProposerForL2" and use it in the prepareL2s function when the DisputeGameFactoryProxy is available
func (s *interopE2ESystem) _(
	id string,
	operatorKeys map[devkeys.ChainOperatorRole]ecdsa.PrivateKey,
	opNode *opnode.Opnode,
) *l2os.ProposerService {
	proposerSecret := operatorKeys[devkeys.ProposerRole]
	logger := s.logger.New("role", "proposer"+id)
	proposerCLIConfig := &l2os.CLIConfig{
		L1EthRpc:          s.l1.UserRPC().RPC(),
		RollupRpc:         opNode.UserRPC().RPC(),
		DGFAddress:        s.worldDeployment.L2s[id].DisputeGameFactoryProxy.Hex(),
		ProposalInterval:  6 * time.Second,
		DisputeGameType:   254, // Fast game type
		PollInterval:      500 * time.Millisecond,
		TxMgrConfig:       setuputils.NewTxMgrConfig(s.l1.UserRPC(), &proposerSecret),
		AllowNonFinalized: false,
		LogConfig: oplog.CLIConfig{
			Level:  log.LvlInfo,
			Format: oplog.FormatText,
		},
	}
	proposer, err := l2os.ProposerServiceFromCLIConfig(
		context.Background(),
		"0.0.1",
		proposerCLIConfig,
		logger.New("service", "proposer"))
	require.NoError(s.t, err, "must start proposer")
	require.NoError(s.t, proposer.Start(context.Background()))
	s.t.Cleanup(func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // force-quit
		_ = proposer.Stop(ctx)
	})
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

// newL2 creates a new L2, starting with the L2Output from the world configuration
// and iterating through the resources needed for the L2.
// it returns a l2Set with the resources for the L2
func (s *interopE2ESystem) newL2(id string, l2Out *interopgen.L2Output) l2Set {
	operatorKeys := s.newOperatorKeysForL2(l2Out)
	l2Geth := s.newGethForL2(id, l2Out)
	opNode := s.newNodeForL2(id, l2Out, operatorKeys, l2Geth)
	// TODO(#11886): proposer does not work with the generated world as there is no DisputeGameFactoryProxy
	//proposer := s.newProposerForL2(id, operatorKeys, opNode)
	batcher := s.newBatcherForL2(id, operatorKeys, l2Geth, opNode)

	return l2Set{
		chainID:      l2Out.Genesis.Config.ChainID,
		opNode:       opNode,
		l2Geth:       l2Geth,
		proposer:     nil,
		batcher:      batcher,
		operatorKeys: operatorKeys,
		userKeys:     make(map[string]ecdsa.PrivateKey),
		contracts:    make(map[string]interface{}),
	}
}

func (s *interopE2ESystem) ChainID(network string) *big.Int {
	return s.l2s[network].chainID
}

// prepareSupervisor creates a new supervisor for the system
func (s *interopE2ESystem) prepareSupervisor() *supervisor.SupervisorService {
	// Be verbose with op-supervisor, it's in early test phase
	logger := testlog.Logger(s.t, log.LevelDebug).New("role", "supervisor")
	cfg := &supervisorConfig.Config{
		MetricsConfig: metrics.CLIConfig{
			Enabled: false,
		},
		PprofConfig: oppprof.CLIConfig{
			ListenEnabled: false,
		},
		LogConfig: oplog.CLIConfig{
			Level:  log.LevelDebug,
			Format: oplog.FormatText,
		},
		RPC: oprpc.CLIConfig{
			ListenAddr:  "127.0.0.1",
			ListenPort:  0,
			EnableAdmin: true,
		},
		L2RPCs:  []string{},
		Datadir: path.Join(s.t.TempDir(), "supervisor"),
	}
	depSet := make(map[supervisortypes.ChainID]*depset.StaticConfigDependency)

	// Iterate over the L2 chain configs. The L2 nodes don't exist yet.
	for _, l2Out := range s.worldOutput.L2s {
		chainID := supervisortypes.ChainIDFromBig(l2Out.Genesis.Config.ChainID)
		index, err := chainID.ToUInt32()
		require.NoError(s.t, err)
		depSet[chainID] = &depset.StaticConfigDependency{
			ChainIndex:     supervisortypes.ChainIndex(index),
			ActivationTime: 0,
			HistoryMinTime: 0,
		}
	}
	stDepSet, err := depset.NewStaticConfigDependencySet(depSet)
	require.NoError(s.t, err)
	cfg.DependencySetSource = stDepSet

	// Create the supervisor with the configuration
	super, err := supervisor.SupervisorFromConfig(context.Background(), cfg, logger)
	require.NoError(s.t, err)
	// Start the supervisor
	err = super.Start(context.Background())
	require.NoError(s.t, err)
	s.t.Cleanup(func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // force-quit
		s.t.Logf("Closing supervisor")
		closeErr := super.Stop(ctx)
		s.t.Logf("Closed supervisor: %v", closeErr)
	})
	return super
}

// SupervisorClient returns the supervisor client for the system, creating it if it doesn't exist
func (s *interopE2ESystem) SupervisorClient() *sources.SupervisorClient {
	if s.superClient != nil {
		return s.superClient
	}
	cl, err := client.NewRPC(context.Background(), s.logger, s.supervisor.RPC())
	require.NoError(s.t, err, "failed to dial supervisor RPC")
	superClient := sources.NewSupervisorClient(cl)
	s.superClient = superClient
	return superClient
}

// prepare sets up the system for testing
// components are built iteratively, so that they can be reused or modified
// their creation can't be safely skipped or reordered at this time
func (s *interopE2ESystem) prepare(t *testing.T, w worldResourcePaths) {
	s.t = t
	s.logger = testlog.Logger(s.t, log.LevelDebug)
	s.hdWallet = s.prepareHDWallet()
	s.worldDeployment, s.worldOutput = s.prepareWorld(w)

	// the supervisor and client are created first so that the L2s can use the supervisor
	s.supervisor = s.prepareSupervisor()

	s.beacon, s.l1 = s.prepareL1()
	s.l2s = s.prepareL2s()

	s.prepareContracts()

	// add the L2 RPCs to the supervisor now that the L2s are created
	ctx := context.Background()
	for _, l2 := range s.l2s {
		err := s.SupervisorClient().AddL2RPC(ctx, l2.l2Geth.UserRPC().RPC())
		require.NoError(s.t, err, "failed to add L2 RPC to supervisor")
	}

	// Try to close the op-supervisor first
	s.t.Cleanup(func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // force-quit
		s.t.Logf("Closing supervisor")
		closeErr := s.supervisor.Stop(ctx)
		s.t.Logf("Closed supervisor: %v", closeErr)
	})
}

// AddUser adds a user to the system by creating a user key for each L2.
// each user key is stored in the L2's userKeys map.
// because all user maps start empty, a users index should be the same for all L2s,
// but if in the future these maps can diverge, the indexes for username would also diverge
// NOTE: The first 20 accounts are implicitly funded by the Recipe's World Deployment
// see: op-chain-ops/interopgen/recipe.go
// TODO(#11887): make the funded account quantity specified in the recipe so SuperSystems can know which accounts are funded
func (s *interopE2ESystem) AddUser(username string) {
	for id, l2 := range s.l2s {
		bigID, _ := big.NewInt(0).SetString(id, 10)
		userSecret, _ := s.hdWallet.Secret(
			devkeys.ChainUserKey{
				ChainID: bigID,
				Index:   uint64(len(l2.userKeys)),
			},
		)
		l2.userKeys[username] = *userSecret
	}
}

// UserKey returns the user key for a user on an L2
func (s *interopE2ESystem) UserKey(id, username string) ecdsa.PrivateKey {
	return s.l2s[id].userKeys[username]
}

// Address returns the address for a user on an L2
func (s *interopE2ESystem) Address(id, username string) common.Address {
	secret := s.UserKey(id, username)
	require.NotNil(s.t, secret, "no secret found for user %s", username)
	return crypto.PubkeyToAddress(secret.PublicKey)
}

// prepareL2s creates the L2s for the system, returning a map of L2s
func (s *interopE2ESystem) prepareL2s() map[string]l2Set {
	l2s := make(map[string]l2Set)
	for id, l2Out := range s.worldOutput.L2s {
		l2s[id] = s.newL2(id, l2Out)
	}
	return l2s
}

// prepareContracts prepares contract-bindings for the L2s
func (s *interopE2ESystem) prepareContracts() {
	// Add bindings to common contracts for each L2
	l1GethClient := s.L1GethClient()
	for id, l2Dep := range s.worldDeployment.L2s {
		{
			contract, err := inbox.NewInbox(predeploys.CrossL2InboxAddr, s.L2GethClient(id))
			require.NoError(s.t, err)
			s.l2s[id].contracts["inbox"] = contract
		}
		{
			contract, err := systemconfig.NewSystemconfig(l2Dep.SystemConfigProxy, l1GethClient)
			require.NoError(s.t, err)
			s.l2s[id].contracts["systemconfig"] = contract
		}
	}
}

func (s *interopE2ESystem) L1GethClient() *ethclient.Client {
	if s.l1GethClient != nil {
		return s.l1GethClient
	}
	rpcEndpoint := s.l1.UserRPC()
	rpcCl := endpoint.DialRPC(
		endpoint.PreferAnyRPC,
		rpcEndpoint,
		func(v string) *rpc.Client {
			logger := testlog.Logger(s.t, log.LevelInfo)
			cl, err := dial.DialRPCClientWithTimeout(context.Background(), 30*time.Second, logger, v)
			require.NoError(s.t, err, "failed to dial L1 eth node instance")
			return cl
		})
	nodeClient := ethclient.NewClient(rpcCl)
	// register the client so it can be reused
	s.l1GethClient = nodeClient
	return nodeClient
}

func (s *interopE2ESystem) L2GethClient(id string) *ethclient.Client {
	// guard: check if the client already exists and return it in that case
	nodeClient, ok := s.l2GethClients[id]
	if ok {
		return nodeClient
	}
	// create a new client for the L2 from the L2's geth instance
	var ethClient services.EthInstance = s.L2Geth(id)
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
	nodeClient = ethclient.NewClient(rpcCl)
	// register the client so it can be reused
	s.addL2GethClient(id, nodeClient)
	return nodeClient
}

func (sys *interopE2ESystem) addL2GethClient(name string, client *ethclient.Client) {
	if sys.l2GethClients == nil {
		sys.l2GethClients = make(map[string]*ethclient.Client)
	}
	sys.l2GethClients[name] = client
}

// getter functions for L1 entities
func (s *interopE2ESystem) Supervisor() *supervisor.SupervisorService {
	return s.supervisor
}

// gettter functions for the individual L2s
func (s *interopE2ESystem) Batcher(id string) *bss.BatcherService {
	return s.l2s[id].batcher
}
func (s *interopE2ESystem) Proposer(id string) *l2os.ProposerService {
	return s.l2s[id].proposer
}
func (s *interopE2ESystem) OpNode(id string) *opnode.Opnode {
	return s.l2s[id].opNode
}
func (s *interopE2ESystem) L2Geth(id string) *geth.GethInstance {
	return s.l2s[id].l2Geth
}
func (s *interopE2ESystem) L2OperatorKey(id string, role devkeys.ChainOperatorRole) ecdsa.PrivateKey {
	return s.l2s[id].operatorKeys[role]
}

// L2IDs returns the list of L2 IDs, which are the keys of the L2s map
func (s *interopE2ESystem) L2IDs() []string {
	ids := make([]string, 0, len(s.l2s))
	for id := range s.l2s {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// SendL2Tx sends an L2 transaction to the L2 with the given ID.
// it acts as a wrapper around op-e2e.SendL2TxWithID
// and uses the L2's chain ID, username key, and geth client.
func (s *interopE2ESystem) SendL2Tx(
	id string,
	sender string,
	applyTxOpts helpers.TxOptsFn,
) *types.Receipt {
	senderSecret := s.UserKey(id, sender)
	require.NotNil(s.t, senderSecret, "no secret found for sender %s", sender)
	nonce, err := s.L2GethClient(id).PendingNonceAt(context.Background(), crypto.PubkeyToAddress(senderSecret.PublicKey))
	require.NoError(s.t, err, "failed to get nonce")
	newApply := func(opts *helpers.TxOpts) {
		applyTxOpts(opts)
		opts.Nonce = nonce
	}
	return helpers.SendL2TxWithID(
		s.t,
		s.l2s[id].chainID,
		s.L2GethClient(id),
		&senderSecret,
		newApply)
}

// ExecuteMessage calls the CrossL2Inbox executeMessage function
// it uses the L2's chain ID, username key, and geth client.
// expectedError represents the error returned by `ExecuteMessage` if it is expected.
// the returned err is related to `WaitMined`
func (s *interopE2ESystem) ExecuteMessage(
	ctx context.Context,
	id string,
	sender string,
	msgIdentifier supervisortypes.Identifier,
	target common.Address,
	message []byte,
	expectedError error,
) (*types.Receipt, error) {
	secret := s.UserKey(id, sender)
	auth, err := bind.NewKeyedTransactorWithChainID(&secret, s.l2s[id].chainID)

	require.NoError(s.t, err)

	auth.GasLimit = uint64(3000_000)
	auth.GasPrice = big.NewInt(20_000_000_000)

	contract := s.Contract(id, "inbox").(*inbox.Inbox)
	identifier := inbox.Identifier{
		Origin:      msgIdentifier.Origin,
		BlockNumber: new(big.Int).SetUint64(msgIdentifier.BlockNumber),
		LogIndex:    new(big.Int).SetUint64(uint64(msgIdentifier.LogIndex)),
		Timestamp:   new(big.Int).SetUint64(msgIdentifier.Timestamp),
		ChainId:     msgIdentifier.ChainID.ToBig(),
	}
	tx, err := contract.InboxTransactor.ExecuteMessage(auth, identifier, target, message)
	if expectedError != nil {
		require.ErrorContains(s.t, err, expectedError.Error())
		return nil, err
	} else {
		require.NoError(s.t, err)
	}
	s.logger.Info("Executing message", "tx", tx.Hash(), "to", tx.To(), "target", target, "data", hexutil.Bytes(tx.Data()))
	return bind.WaitMined(ctx, s.L2GethClient(id), tx)
}

func (s *interopE2ESystem) AddDependency(id string, dep *big.Int) *types.Receipt {
	// There is a note in OPContractsManagerInterop that the proxy-admin is used for now,
	// even though it should be a separate dependency-set-manager address.
	secret, err := s.hdWallet.Secret(devkeys.ChainOperatorKey{
		ChainID: s.l2s[id].chainID,
		Role:    devkeys.SystemConfigOwner,
	})
	require.NoError(s.t, err)

	auth, err := bind.NewKeyedTransactorWithChainID(secret, s.worldOutput.L1.Genesis.Config.ChainID)
	require.NoError(s.t, err)

	balance, err := s.l1GethClient.BalanceAt(context.Background(), crypto.PubkeyToAddress(secret.PublicKey), nil)
	require.NoError(s.t, err)
	require.False(s.t, balance.Sign() == 0, "system config owner needs a balance")

	auth.GasLimit = uint64(3000000)
	auth.GasPrice = big.NewInt(20000000000)

	contract := s.Contract(id, "systemconfig").(*systemconfig.Systemconfig)
	tx, err := contract.SystemconfigTransactor.AddDependency(auth, dep)
	require.NoError(s.t, err)

	receipt, err := wait.ForReceiptOK(context.Background(), s.L1GethClient(), tx.Hash())
	require.NoError(s.t, err)
	return receipt
}

func (s *interopE2ESystem) DeployEmitterContract(
	id string,
	sender string,
) common.Address {
	secret := s.UserKey(id, sender)
	auth, err := bind.NewKeyedTransactorWithChainID(&secret, s.l2s[id].chainID)
	require.NoError(s.t, err)
	auth.GasLimit = uint64(3000000)
	auth.GasPrice = big.NewInt(20000000000)
	address, _, _, err := emit.DeployEmit(auth, s.L2GethClient(id))
	require.NoError(s.t, err)
	contract, err := emit.NewEmit(address, s.L2GethClient(id))
	require.NoError(s.t, err)
	s.l2s[id].contracts["emitter"] = contract
	return address
}

func (s *interopE2ESystem) EmitData(
	id string,
	sender string,
	data string,
) *types.Receipt {
	secret := s.UserKey(id, sender)
	auth, err := bind.NewKeyedTransactorWithChainID(&secret, s.l2s[id].chainID)

	require.NoError(s.t, err)

	auth.GasLimit = uint64(3000000)
	auth.GasPrice = big.NewInt(20000000000)

	contract := s.Contract(id, "emitter").(*emit.Emit)
	tx, err := contract.EmitTransactor.EmitData(auth, []byte(data))
	require.NoError(s.t, err)
	receipt, err := bind.WaitMined(context.Background(), s.L2GethClient(id), tx)
	require.NoError(s.t, err)
	return receipt
}

func (s *interopE2ESystem) Contract(id string, name string) interface{} {
	return s.l2s[id].contracts[name]
}

func mustDial(t *testing.T, logger log.Logger) func(v string) *rpc.Client {
	return func(v string) *rpc.Client {
		cl, err := dial.DialRPCClientWithTimeout(context.Background(), 30*time.Second, logger, v)
		require.NoError(t, err, "failed to dial")
		return cl
	}
}

var testingJWTSecret = [32]byte{123}

func writeDefaultJWT(t testing.TB) string {
	// Sadly the geth node config cannot load JWT secret from memory, it has to be a file
	jwtPath := path.Join(t.TempDir(), "jwt_secret")
	if err := os.WriteFile(jwtPath, []byte(hexutil.Encode(testingJWTSecret[:])), 0o600); err != nil {
		t.Fatalf("failed to prepare jwt file for geth: %v", err)
	}
	return jwtPath
}
