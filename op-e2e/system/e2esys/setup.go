package e2esys

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"

	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"

	ds "github.com/ipfs/go-datastore"
	dsSync "github.com/ipfs/go-datastore/sync"
	ic "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoremem"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	bss "github.com/ethereum-optimism/optimism/op-batcher/batcher"
	batcherFlags "github.com/ethereum-optimism/optimism/op-batcher/flags"
	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	"github.com/ethereum-optimism/optimism/op-e2e/config"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/batcher"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/fakebeacon"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/opnode"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/services"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/setuputils"
	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	rollupNode "github.com/ethereum-optimism/optimism/op-node/node"
	"github.com/ethereum-optimism/optimism/op-node/p2p"
	"github.com/ethereum-optimism/optimism/op-node/p2p/store"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/driver"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	l2os "github.com/ethereum-optimism/optimism/op-proposer/proposer"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/endpoint"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

const (
	RoleSeq   = "sequencer"
	RoleVerif = "verifier"
	RoleL1    = "l1"
)

var (
	testingJWTSecret = [32]byte{123}
	genesisTime      = hexutil.Uint64(0)
)

type SystemConfigOpts struct {
	AllocType config.AllocType
}

type SystemConfigOpt func(s *SystemConfigOpts)

func WithAllocType(allocType config.AllocType) SystemConfigOpt {
	return func(s *SystemConfigOpts) {
		s.AllocType = allocType
	}
}

func DefaultSystemConfig(t testing.TB, opts ...SystemConfigOpt) SystemConfig {
	sco := &SystemConfigOpts{
		AllocType: config.DefaultAllocType,
	}
	for _, opt := range opts {
		opt(sco)
	}

	secrets, err := e2eutils.DefaultMnemonicConfig.Secrets()
	require.NoError(t, err)
	deployConfig := config.DeployConfig(sco.AllocType)
	deployConfig.L1GenesisBlockTimestamp = hexutil.Uint64(time.Now().Unix())
	e2eutils.ApplyDeployConfigForks(deployConfig)
	require.NoError(t, deployConfig.Check(testlog.Logger(t, log.LevelInfo)),
		"Deploy config is invalid, do you need to run make devnet-allocs?")
	l1Deployments := config.L1Deployments(sco.AllocType)
	require.NoError(t, l1Deployments.Check(deployConfig))

	require.Equal(t, secrets.Addresses().Batcher, deployConfig.BatchSenderAddress)
	require.Equal(t, secrets.Addresses().SequencerP2P, deployConfig.P2PSequencerAddress)
	require.Equal(t, secrets.Addresses().Proposer, deployConfig.L2OutputOracleProposer)

	// Tests depend on premine being filled with secrets addresses
	premine := make(map[common.Address]*big.Int)
	for _, addr := range secrets.Addresses().All() {
		premine[addr] = new(big.Int).Mul(big.NewInt(1000), big.NewInt(params.Ether))
	}

	return SystemConfig{
		Secrets:                secrets,
		Premine:                premine,
		DeployConfig:           deployConfig,
		L1Deployments:          l1Deployments,
		L1InfoPredeployAddress: predeploys.L1BlockAddr,
		JWTFilePath:            writeDefaultJWT(t),
		JWTSecret:              testingJWTSecret,
		L1FinalizedDistance:    8, // Short, for faster tests.
		BlobsPath:              t.TempDir(),
		AllocType:              sco.AllocType,
		Nodes: map[string]*rollupNode.Config{
			RoleSeq: {
				Driver: driver.Config{
					VerifierConfDepth:  0,
					SequencerConfDepth: 0,
					SequencerEnabled:   true,
				},
				// Submitter PrivKey is set in system start for rollup nodes where sequencer = true
				RPC: rollupNode.RPCConfig{
					ListenAddr:  "127.0.0.1",
					ListenPort:  0,
					EnableAdmin: true,
				},
				L1EpochPollInterval:         time.Second * 2,
				RuntimeConfigReloadInterval: time.Minute * 10,
				ConfigPersistence:           &rollupNode.DisabledConfigPersistence{},
				Sync:                        sync.Config{SyncMode: sync.CLSync},
			},
			RoleVerif: {
				Driver: driver.Config{
					VerifierConfDepth:  0,
					SequencerConfDepth: 0,
					SequencerEnabled:   false,
				},
				RPC: rollupNode.RPCConfig{
					ListenAddr:  "127.0.0.1",
					ListenPort:  0,
					EnableAdmin: true,
				},
				L1EpochPollInterval:         time.Second * 4,
				RuntimeConfigReloadInterval: time.Minute * 10,
				ConfigPersistence:           &rollupNode.DisabledConfigPersistence{},
				Sync:                        sync.Config{SyncMode: sync.CLSync},
			},
		},
		Loggers: map[string]log.Logger{
			RoleVerif:   testlog.Logger(t, log.LevelInfo).New("role", RoleVerif),
			RoleSeq:     testlog.Logger(t, log.LevelInfo).New("role", RoleSeq),
			"batcher":   testlog.Logger(t, log.LevelInfo).New("role", "batcher"),
			"proposer":  testlog.Logger(t, log.LevelInfo).New("role", "proposer"),
			"da-server": testlog.Logger(t, log.LevelInfo).New("role", "da-server"),
		},
		GethOptions:                   map[string][]geth.GethOption{},
		P2PTopology:                   nil, // no P2P connectivity by default
		NonFinalizedProposals:         false,
		DataAvailabilityType:          batcherFlags.CalldataType,
		BatcherMaxPendingTransactions: 1,
		BatcherTargetNumFrames:        1,
	}
}

func RegolithSystemConfig(t *testing.T, regolithTimeOffset *hexutil.Uint64, opts ...SystemConfigOpt) SystemConfig {
	cfg := DefaultSystemConfig(t, opts...)
	cfg.DeployConfig.L2GenesisRegolithTimeOffset = regolithTimeOffset
	cfg.DeployConfig.L2GenesisCanyonTimeOffset = nil
	cfg.DeployConfig.L2GenesisDeltaTimeOffset = nil
	cfg.DeployConfig.L2GenesisEcotoneTimeOffset = nil
	cfg.DeployConfig.L2GenesisFjordTimeOffset = nil
	cfg.DeployConfig.L2GenesisGraniteTimeOffset = nil
	cfg.DeployConfig.L2GenesisHoloceneTimeOffset = nil
	// ADD NEW FORKS HERE!
	return cfg
}

func CanyonSystemConfig(t *testing.T, canyonTimeOffset *hexutil.Uint64, opts ...SystemConfigOpt) SystemConfig {
	cfg := RegolithSystemConfig(t, &genesisTime, opts...)
	cfg.DeployConfig.L2GenesisCanyonTimeOffset = canyonTimeOffset
	return cfg
}

func DeltaSystemConfig(t *testing.T, deltaTimeOffset *hexutil.Uint64, opts ...SystemConfigOpt) SystemConfig {
	cfg := CanyonSystemConfig(t, &genesisTime, opts...)
	cfg.DeployConfig.L2GenesisDeltaTimeOffset = deltaTimeOffset
	return cfg
}

func EcotoneSystemConfig(t *testing.T, ecotoneTimeOffset *hexutil.Uint64, opts ...SystemConfigOpt) SystemConfig {
	cfg := DeltaSystemConfig(t, &genesisTime, opts...)
	//  from Ecotone onwards, activate L1 Cancun at genesis
	cfg.DeployConfig.L1CancunTimeOffset = &genesisTime
	cfg.DeployConfig.L2GenesisEcotoneTimeOffset = ecotoneTimeOffset
	return cfg
}

func FjordSystemConfig(t *testing.T, fjordTimeOffset *hexutil.Uint64, opts ...SystemConfigOpt) SystemConfig {
	cfg := EcotoneSystemConfig(t, &genesisTime, opts...)
	cfg.DeployConfig.L2GenesisFjordTimeOffset = fjordTimeOffset
	return cfg
}

func GraniteSystemConfig(t *testing.T, graniteTimeOffset *hexutil.Uint64, opts ...SystemConfigOpt) SystemConfig {
	cfg := FjordSystemConfig(t, &genesisTime, opts...)
	cfg.DeployConfig.L2GenesisGraniteTimeOffset = graniteTimeOffset
	return cfg
}

func HoloceneSystemConfig(t *testing.T, holoceneTimeOffset *hexutil.Uint64, opts ...SystemConfigOpt) SystemConfig {
	cfg := GraniteSystemConfig(t, &genesisTime, opts...)
	cfg.DeployConfig.L2GenesisHoloceneTimeOffset = holoceneTimeOffset
	return cfg
}

func writeDefaultJWT(t testing.TB) string {
	// Sadly the geth node config cannot load JWT secret from memory, it has to be a file
	jwtPath := path.Join(t.TempDir(), "jwt_secret")
	if err := os.WriteFile(jwtPath, []byte(hexutil.Encode(testingJWTSecret[:])), 0o600); err != nil {
		t.Fatalf("failed to prepare jwt file for geth: %v", err)
	}
	return jwtPath
}

type DepositContractConfig struct {
	L2Oracle           common.Address
	FinalizationPeriod *big.Int
}

type SystemConfig struct {
	Secrets                *e2eutils.Secrets
	L1InfoPredeployAddress common.Address

	DeployConfig  *genesis.DeployConfig
	L1Deployments *genesis.L1Deployments

	JWTFilePath string
	JWTSecret   [32]byte

	BlobsPath string

	// L1FinalizedDistance is the distance from the L1 head that L1 blocks will be artificially finalized on.
	L1FinalizedDistance uint64

	Premine        map[common.Address]*big.Int
	Nodes          map[string]*rollupNode.Config // Per node config. Don't use populate rollup.Config
	Loggers        map[string]log.Logger
	GethOptions    map[string][]geth.GethOption
	ProposerLogger log.Logger
	BatcherLogger  log.Logger

	ExternalL2Shim string

	// map of outbound connections to other nodes. Node names prefixed with "~" are unconnected but linked.
	// A nil map disables P2P completely.
	// Any node name not in the topology will not have p2p enabled.
	P2PTopology map[string][]string

	// Enables req-resp sync in the P2P nodes
	P2PReqRespSync bool

	// If the proposer can make proposals for L2 blocks derived from L1 blocks which are not finalized on L1 yet.
	NonFinalizedProposals bool

	// Explicitly disable proposer, for tests that don't want dispute games automatically created
	DisableProposer bool

	// Explicitly disable batcher, for tests that rely on unsafe L2 payloads
	DisableBatcher bool

	// Explicitly disable setting `RollupSequencerHTTP` to forward txs from sentry nodes
	DisableTxForwarder bool

	// Configure data-availability type that is used by the batcher.
	DataAvailabilityType batcherFlags.DataAvailabilityType

	// Max L1 tx size for the batcher transactions
	BatcherMaxL1TxSizeBytes uint64

	// Target number of frames to create per channel. Can be used to create
	// multi-blob transactions.
	// Default is 1 if unset.
	BatcherTargetNumFrames int

	// whether to actually use BatcherMaxL1TxSizeBytes for blobs, insteaf of max blob size
	BatcherUseMaxTxSizeForBlobs bool

	// Singular (0) or span batches (1)
	BatcherBatchType uint

	// If >0, limits the number of blocks per span batch
	BatcherMaxBlocksPerSpanBatch int

	// BatcherMaxPendingTransactions determines how many transactions the batcher will try to send
	// concurrently. 0 means unlimited.
	BatcherMaxPendingTransactions uint64

	// BatcherMaxConcurrentDARequest determines how many DAserver requests the batcher is allowed to
	// make concurrently. 0 means unlimited.
	BatcherMaxConcurrentDARequest uint64

	// SupportL1TimeTravel determines if the L1 node supports quickly skipping forward in time
	SupportL1TimeTravel bool

	AllocType config.AllocType
}

type System struct {
	Cfg SystemConfig

	RollupConfig *rollup.Config

	L2GenesisCfg *core.Genesis

	// Connections to running nodes
	EthInstances      map[string]services.EthInstance
	RollupNodes       map[string]services.RollupNode
	L2OutputSubmitter *l2os.ProposerService
	BatchSubmitter    *bss.BatcherService
	Mocknet           mocknet.Mocknet
	FakeAltDAServer   *altda.FakeDAServer

	L1BeaconAPIAddr endpoint.RestHTTP

	// TimeTravelClock is nil unless SystemConfig.SupportL1TimeTravel was set to true
	// It provides access to the clock instance used by the L1 node. Calling TimeTravelClock.AdvanceBy
	// allows tests to quickly time travel L1 into the future.
	// Note that this time travel may occur in a single block, creating a very large difference in the Time
	// on sequential blocks.
	TimeTravelClock *clock.AdvancingClock

	t      *testing.T
	closed atomic.Bool

	// rollupClients caches the lazily created RollupClient
	// instances so they can be reused and closed
	rollupClients map[string]*sources.RollupClient

	// clients caches lazily created L1/L2 ethclient.Client
	// instances so they can be reused and closed
	clients map[string]*ethclient.Client
}

func (sys *System) Config() SystemConfig { return sys.Cfg }

// AdvanceTime advances the system clock by the given duration.
// If the [System.TimeTravelClock] is nil, this is a no-op.
func (sys *System) AdvanceTime(d time.Duration) {
	if sys.TimeTravelClock != nil {
		sys.TimeTravelClock.AdvanceTime(d)
	}
}

func (sys *System) L1BeaconEndpoint() endpoint.RestHTTP {
	return sys.L1BeaconAPIAddr
}

func (sys *System) L1BeaconHTTPClient() *sources.BeaconHTTPClient {
	logger := testlog.Logger(sys.t, log.LevelInfo).New("component", "beaconClient")
	return sources.NewBeaconHTTPClient(client.NewBasicHTTPClient(sys.L1BeaconEndpoint().RestHTTP(), logger))
}

func (sys *System) NodeEndpoint(name string) endpoint.RPC {
	ethInst, ok := sys.EthInstances[name]
	if !ok {
		sys.t.Fatalf("unknown eth instance: %s", name)
	}
	return ethInst.UserRPC()
}

func (sys *System) RollupEndpoint(name string) endpoint.RPC {
	rNode, ok := sys.RollupNodes[name]
	if !ok {
		sys.t.Fatalf("unknown rollup node instance: %s", name)
	}
	return rNode.UserRPC()
}

func (sys *System) L1Deployments() *genesis.L1Deployments {
	return sys.Cfg.L1Deployments
}

func (sys *System) RollupCfg() *rollup.Config {
	return sys.RollupConfig
}

func (sys *System) L2Genesis() *core.Genesis {
	return sys.L2GenesisCfg
}

func (sys *System) AllocType() config.AllocType {
	return sys.Cfg.AllocType
}

func (sys *System) L1Slot(l1Timestamp uint64) uint64 {
	return (l1Timestamp - uint64(sys.Cfg.DeployConfig.L1GenesisBlockTimestamp)) /
		sys.Cfg.DeployConfig.L1BlockTime
}

func (sys *System) Close() {
	sys.t.Log("CLOSING")
	if !sys.closed.CompareAndSwap(false, true) {
		// Already closed.
		return
	}
	postCtx, postCancel := context.WithCancel(context.Background())
	postCancel() // immediate shutdown, no allowance for idling

	var combinedErr error
	if sys.L2OutputSubmitter != nil {
		if err := sys.L2OutputSubmitter.Kill(); err != nil && !errors.Is(err, l2os.ErrAlreadyStopped) {
			combinedErr = errors.Join(combinedErr, fmt.Errorf("stop L2OutputSubmitter: %w", err))
		}
	}
	if sys.BatchSubmitter != nil {
		if err := sys.BatchSubmitter.Kill(); err != nil && !errors.Is(err, bss.ErrAlreadyStopped) {
			combinedErr = errors.Join(combinedErr, fmt.Errorf("stop BatchSubmitter: %w", err))
		}
	}

	for name, node := range sys.RollupNodes {
		if err := node.Stop(postCtx); err != nil && !errors.Is(err, rollupNode.ErrAlreadyClosed) && !errors.Is(err, postCtx.Err()) {
			combinedErr = errors.Join(combinedErr, fmt.Errorf("stop rollup node %v: %w", name, err))
		}
	}
	for name, ei := range sys.EthInstances {
		if err := ei.Close(); err != nil && !errors.Is(err, node.ErrNodeStopped) {
			combinedErr = errors.Join(combinedErr, fmt.Errorf("stop EthInstance %v: %w", name, err))
		}
	}
	for _, client := range sys.clients {
		client.Close()
	}
	for _, client := range sys.rollupClients {
		client.Close()
	}
	if sys.Mocknet != nil {
		if err := sys.Mocknet.Close(); err != nil {
			combinedErr = errors.Join(combinedErr, fmt.Errorf("stop Mocknet: %w", err))
		}
	}
	require.NoError(sys.t, combinedErr, "Failed to stop system")
}

type SystemConfigHook func(sCfg *SystemConfig, s *System)

type StartOption struct {
	Key    string
	Role   string
	Action SystemConfigHook

	// Batcher CLIConfig modifications to apply before starting the batcher.
	BatcherMod func(*bss.CLIConfig)
}

type startOptions struct {
	opts map[string]SystemConfigHook
}

func parseStartOptions(_opts []StartOption) (startOptions, error) {
	opts := make(map[string]SystemConfigHook)
	for _, opt := range _opts {
		if _, ok := opts[opt.Key+":"+opt.Role]; ok {
			return startOptions{}, fmt.Errorf("duplicate option for key %s and role %s", opt.Key, opt.Role)
		}
		opts[opt.Key+":"+opt.Role] = opt.Action
	}

	return startOptions{
		opts: opts,
	}, nil
}

func WithBatcherCompressionAlgo(ca derive.CompressionAlgo) StartOption {
	return StartOption{
		BatcherMod: func(cfg *bss.CLIConfig) {
			cfg.CompressionAlgo = ca
		},
	}
}

func WithBatcherThrottling(interval time.Duration, threshold, txSize, blockSize uint64) StartOption {
	return StartOption{
		BatcherMod: func(cfg *bss.CLIConfig) {
			cfg.ThrottleInterval = interval
			cfg.ThrottleThreshold = threshold
			cfg.ThrottleTxSize = txSize
			cfg.ThrottleBlockSize = blockSize
		},
	}
}

func (s *startOptions) Get(key, role string) (SystemConfigHook, bool) {
	v, ok := s.opts[key+":"+role]
	return v, ok
}

func (cfg SystemConfig) Start(t *testing.T, startOpts ...StartOption) (*System, error) {
	parsedStartOpts, err := parseStartOptions(startOpts)
	if err != nil {
		return nil, err
	}

	sys := &System{
		t:             t,
		Cfg:           cfg,
		EthInstances:  make(map[string]services.EthInstance),
		RollupNodes:   make(map[string]services.RollupNode),
		clients:       make(map[string]*ethclient.Client),
		rollupClients: make(map[string]*sources.RollupClient),
	}
	// Automatically stop the system at the end of the test
	t.Cleanup(sys.Close)

	c := clock.SystemClock
	if cfg.SupportL1TimeTravel {
		sys.TimeTravelClock = clock.NewAdvancingClock(100 * time.Millisecond)
		c = sys.TimeTravelClock
	}

	if err := cfg.DeployConfig.Check(testlog.Logger(t, log.LevelInfo)); err != nil {
		return nil, err
	}

	l1Genesis, err := genesis.BuildL1DeveloperGenesis(
		cfg.DeployConfig,
		config.L1Allocs(cfg.AllocType),
		config.L1Deployments(cfg.AllocType),
	)
	if err != nil {
		return nil, err
	}

	for addr, amount := range cfg.Premine {
		if existing, ok := l1Genesis.Alloc[addr]; ok {
			l1Genesis.Alloc[addr] = types.Account{
				Code:    existing.Code,
				Storage: existing.Storage,
				Balance: amount,
				Nonce:   existing.Nonce,
			}
		} else {
			l1Genesis.Alloc[addr] = types.Account{
				Balance: amount,
				Nonce:   0,
			}
		}
	}

	l1Block := l1Genesis.ToBlock()
	allocsMode := cfg.DeployConfig.AllocMode(l1Block.Time())

	t.Log("Generating L2 genesis", "l2_allocs_mode", string(allocsMode))
	l2Allocs := config.L2Allocs(cfg.AllocType, allocsMode)
	l2Genesis, err := genesis.BuildL2Genesis(cfg.DeployConfig, l2Allocs, l1Block.Header())
	if err != nil {
		return nil, err
	}
	sys.L2GenesisCfg = l2Genesis
	for addr, amount := range cfg.Premine {
		if existing, ok := l2Genesis.Alloc[addr]; ok {
			l2Genesis.Alloc[addr] = types.Account{
				Code:    existing.Code,
				Storage: existing.Storage,
				Balance: amount,
				Nonce:   existing.Nonce,
			}
		} else {
			l2Genesis.Alloc[addr] = types.Account{
				Balance: amount,
				Nonce:   0,
			}
		}
	}

	var rollupAltDAConfig *rollup.AltDAConfig
	if cfg.DeployConfig.UseAltDA {
		rollupAltDAConfig = &rollup.AltDAConfig{
			DAChallengeAddress: cfg.L1Deployments.DataAvailabilityChallengeProxy,
			DAChallengeWindow:  cfg.DeployConfig.DAChallengeWindow,
			DAResolveWindow:    cfg.DeployConfig.DAResolveWindow,
			CommitmentType:     altda.GenericCommitmentString,
		}
	}

	makeRollupConfig := func() rollup.Config {
		return rollup.Config{
			Genesis: rollup.Genesis{
				L1: eth.BlockID{
					Hash:   l1Block.Hash(),
					Number: 0,
				},
				L2: eth.BlockID{
					Hash:   l2Genesis.ToBlock().Hash(),
					Number: 0,
				},
				L2Time:       uint64(cfg.DeployConfig.L1GenesisBlockTimestamp),
				SystemConfig: e2eutils.SystemConfigFromDeployConfig(cfg.DeployConfig),
			},
			BlockTime:               cfg.DeployConfig.L2BlockTime,
			MaxSequencerDrift:       cfg.DeployConfig.MaxSequencerDrift,
			SeqWindowSize:           cfg.DeployConfig.SequencerWindowSize,
			ChannelTimeoutBedrock:   cfg.DeployConfig.ChannelTimeoutBedrock,
			L1ChainID:               cfg.L1ChainIDBig(),
			L2ChainID:               cfg.L2ChainIDBig(),
			BatchInboxAddress:       cfg.DeployConfig.BatchInboxAddress,
			DepositContractAddress:  cfg.DeployConfig.OptimismPortalProxy,
			L1SystemConfigAddress:   cfg.DeployConfig.SystemConfigProxy,
			RegolithTime:            cfg.DeployConfig.RegolithTime(uint64(cfg.DeployConfig.L1GenesisBlockTimestamp)),
			CanyonTime:              cfg.DeployConfig.CanyonTime(uint64(cfg.DeployConfig.L1GenesisBlockTimestamp)),
			DeltaTime:               cfg.DeployConfig.DeltaTime(uint64(cfg.DeployConfig.L1GenesisBlockTimestamp)),
			EcotoneTime:             cfg.DeployConfig.EcotoneTime(uint64(cfg.DeployConfig.L1GenesisBlockTimestamp)),
			FjordTime:               cfg.DeployConfig.FjordTime(uint64(cfg.DeployConfig.L1GenesisBlockTimestamp)),
			GraniteTime:             cfg.DeployConfig.GraniteTime(uint64(cfg.DeployConfig.L1GenesisBlockTimestamp)),
			HoloceneTime:            cfg.DeployConfig.HoloceneTime(uint64(cfg.DeployConfig.L1GenesisBlockTimestamp)),
			InteropTime:             cfg.DeployConfig.InteropTime(uint64(cfg.DeployConfig.L1GenesisBlockTimestamp)),
			ProtocolVersionsAddress: cfg.L1Deployments.ProtocolVersionsProxy,
			AltDAConfig:             rollupAltDAConfig,
		}
	}
	defaultConfig := makeRollupConfig()
	if err := defaultConfig.Check(); err != nil {
		return nil, err
	}
	sys.RollupConfig = &defaultConfig

	// Create a fake Beacon node to hold on to blobs created by the L1 miner, and to serve them to L2
	bcn := fakebeacon.NewBeacon(testlog.Logger(t, log.LevelInfo).New("role", "l1_cl"),
		e2eutils.NewBlobStore(), l1Genesis.Timestamp, cfg.DeployConfig.L1BlockTime)
	t.Cleanup(func() {
		_ = bcn.Close()
	})
	require.NoError(t, bcn.Start("127.0.0.1:0"))
	beaconApiAddr := bcn.BeaconAddr()
	require.NotEmpty(t, beaconApiAddr, "beacon API listener must be up")
	sys.L1BeaconAPIAddr = endpoint.RestHTTPURL(beaconApiAddr)

	// Initialize nodes
	l1Geth, err := geth.InitL1(
		cfg.DeployConfig.L1BlockTime, cfg.L1FinalizedDistance, l1Genesis, c,
		path.Join(cfg.BlobsPath, "l1_el"), bcn, cfg.GethOptions[RoleL1]...)
	if err != nil {
		return nil, err
	}
	sys.EthInstances[RoleL1] = l1Geth
	err = l1Geth.Node.Start()
	if err != nil {
		return nil, err
	}

	sysLogger := testlog.Logger(t, log.LevelInfo).New("role", "system")

	l1UpCtx, l1UpCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer l1UpCancel()
	if err := wait.ForNodeUp(l1UpCtx, sys.NodeClient(RoleL1), sysLogger); err != nil {
		return nil, fmt.Errorf("l1 never came up: %w", err)
	}

	// Ordered such that the Sequencer is initialized first. Setup this way so that
	// the `RollupSequencerHTTP` GethOption can be supplied to any sentry nodes.
	l2Nodes := []string{RoleSeq}
	for name := range cfg.Nodes {
		if name == RoleSeq {
			continue
		}
		l2Nodes = append(l2Nodes, name)
	}

	for _, name := range l2Nodes {
		var ethClient services.EthInstance
		if name != RoleSeq && !cfg.DisableTxForwarder {
			cfg.GethOptions[name] = append(cfg.GethOptions[name], func(ethCfg *ethconfig.Config, nodeCfg *node.Config) error {
				ethCfg.RollupSequencerHTTP = sys.EthInstances[RoleSeq].UserRPC().RPC()
				return nil
			})
		}

		l2Geth, err := geth.InitL2(name, l2Genesis, cfg.JWTFilePath, cfg.GethOptions[name]...)
		if err != nil {
			return nil, err
		}
		if err := l2Geth.Node.Start(); err != nil {
			return nil, err
		}

		ethClient = l2Geth

		sys.EthInstances[name] = ethClient
	}

	// Configure connections to L1 and L2 for rollup nodes.
	// TODO: refactor testing to allow use of in-process rpc connections instead
	// of only websockets (which are required for external eth client tests).
	for name, nodeCfg := range cfg.Nodes {
		ConfigureL1(nodeCfg, sys.EthInstances[RoleL1], sys.L1BeaconEndpoint())
		ConfigureL2(nodeCfg, sys.EthInstances[name], cfg.JWTSecret)
	}

	l1Client := sys.NodeClient(RoleL1)
	_, err = geth.WaitForBlock(big.NewInt(2), l1Client)
	if err != nil {
		return nil, fmt.Errorf("waiting for blocks: %w", err)
	}

	sys.Mocknet = mocknet.New()

	p2pNodes := make(map[string]*p2p.Prepared)
	if cfg.P2PTopology != nil {
		// create the peer if it doesn't exist yet.
		initHostMaybe := func(name string) (*p2p.Prepared, error) {
			if p, ok := p2pNodes[name]; ok {
				return p, nil
			}
			h, err := sys.NewMockNetPeer()
			if err != nil {
				return nil, fmt.Errorf("failed to init p2p host for node %s", name)
			}
			h.Network()
			_, ok := cfg.Nodes[name]
			if !ok {
				return nil, fmt.Errorf("node %s from p2p topology not found in actual nodes map", name)
			}
			// TODO we can enable discv5 in the testnodes to test discovery of new peers.
			// Would need to mock though, and the discv5 implementation does not provide nice mocks here.
			p := &p2p.Prepared{
				HostP2P:           h,
				LocalNode:         nil,
				UDPv5:             nil,
				EnableReqRespSync: cfg.P2PReqRespSync,
			}
			p2pNodes[name] = p
			return p, nil
		}
		for k, vs := range cfg.P2PTopology {
			peerA, err := initHostMaybe(k)
			if err != nil {
				return nil, fmt.Errorf("failed to setup mocknet peer %s", k)
			}
			for _, v := range vs {
				v = strings.TrimPrefix(v, "~")
				peerB, err := initHostMaybe(v)
				if err != nil {
					return nil, fmt.Errorf("failed to setup mocknet peer %s (peer of %s)", v, k)
				}
				if _, err := sys.Mocknet.LinkPeers(peerA.HostP2P.ID(), peerB.HostP2P.ID()); err != nil {
					return nil, fmt.Errorf("failed to setup mocknet link between %s and %s", k, v)
				}
				// connect the peers after starting the full rollup node
			}
		}
	}

	// Rollup nodes

	// Ensure we are looping through the nodes in alphabetical order
	ks := maps.Keys(cfg.Nodes)
	// Sort strings in ascending alphabetical order
	sort.Strings(ks)

	for _, name := range ks {
		nodeConfig := cfg.Nodes[name]
		c := *nodeConfig // copy
		c.Rollup = makeRollupConfig()
		if err := c.LoadPersisted(cfg.Loggers[name]); err != nil {
			return nil, err
		}

		if p, ok := p2pNodes[name]; ok {
			c.P2P = p

			if c.Driver.SequencerEnabled && c.P2PSigner == nil {
				c.P2PSigner = &p2p.PreparedSigner{Signer: p2p.NewLocalSigner(cfg.Secrets.SequencerP2P)}
			}
		}

		c.Rollup.LogDescription(cfg.Loggers[name], chaincfg.L2ChainIDToNetworkDisplayName)
		l := cfg.Loggers[name]

		n, err := opnode.NewOpnode(l, &c, func(err error) {
			t.Error(err)
		})
		require.NoError(t, err)

		sys.RollupNodes[name] = n

		if action, ok := parsedStartOpts.Get("afterRollupNodeStart", name); ok {
			action(&cfg, sys)
		}
	}

	if cfg.P2PTopology != nil {
		// We only set up the connections after starting the actual nodes,
		// so GossipSub and other p2p protocols can be started before the connections go live.
		// This way protocol negotiation happens correctly.
		for k, vs := range cfg.P2PTopology {
			peerA := p2pNodes[k]
			for _, v := range vs {
				unconnected := strings.HasPrefix(v, "~")
				if unconnected {
					v = v[1:]
				}
				if !unconnected {
					peerB := p2pNodes[v]
					if _, err := sys.Mocknet.ConnectPeers(peerA.HostP2P.ID(), peerB.HostP2P.ID()); err != nil {
						return nil, fmt.Errorf("failed to setup mocknet connection between %s and %s", k, v)
					}
				}
			}
		}
	}

	// Don't start batch submitter and proposer if there's no sequencer.
	if sys.RollupNodes[RoleSeq] == nil {
		return sys, nil
	}

	// L2Output Submitter
	var proposerCLIConfig *l2os.CLIConfig
	if cfg.AllocType.UsesProofs() {
		proposerCLIConfig = &l2os.CLIConfig{
			L1EthRpc:          sys.EthInstances[RoleL1].UserRPC().RPC(),
			RollupRpc:         sys.RollupNodes[RoleSeq].UserRPC().RPC(),
			DGFAddress:        config.L1Deployments(cfg.AllocType).DisputeGameFactoryProxy.Hex(),
			ProposalInterval:  6 * time.Second,
			DisputeGameType:   254, // Fast game type
			PollInterval:      500 * time.Millisecond,
			TxMgrConfig:       setuputils.NewTxMgrConfig(sys.EthInstances[RoleL1].UserRPC(), cfg.Secrets.Proposer),
			AllowNonFinalized: cfg.NonFinalizedProposals,
			LogConfig: oplog.CLIConfig{
				Level:  log.LvlInfo,
				Format: oplog.FormatText,
			},
		}
	} else {
		proposerCLIConfig = &l2os.CLIConfig{
			L1EthRpc:          sys.EthInstances[RoleL1].UserRPC().RPC(),
			RollupRpc:         sys.RollupNodes[RoleSeq].UserRPC().RPC(),
			L2OOAddress:       config.L1Deployments(cfg.AllocType).L2OutputOracleProxy.Hex(),
			PollInterval:      500 * time.Millisecond,
			TxMgrConfig:       setuputils.NewTxMgrConfig(sys.EthInstances[RoleL1].UserRPC(), cfg.Secrets.Proposer),
			AllowNonFinalized: cfg.NonFinalizedProposals,
			LogConfig: oplog.CLIConfig{
				Level:  log.LvlInfo,
				Format: oplog.FormatText,
			},
		}
	}
	proposer, err := l2os.ProposerServiceFromCLIConfig(context.Background(), "0.0.1", proposerCLIConfig, sys.Cfg.Loggers["proposer"])
	if err != nil {
		return nil, fmt.Errorf("unable to setup l2 output submitter: %w", err)
	}
	if !cfg.DisableProposer {
		if err := proposer.Start(context.Background()); err != nil {
			return nil, fmt.Errorf("unable to start l2 output submitter: %w", err)
		}
	}
	sys.L2OutputSubmitter = proposer

	// batcher defaults if unset
	batcherMaxL1TxSizeBytes := cfg.BatcherMaxL1TxSizeBytes
	if batcherMaxL1TxSizeBytes == 0 {
		batcherMaxL1TxSizeBytes = 120_000
	}
	batcherTargetNumFrames := cfg.BatcherTargetNumFrames
	if batcherTargetNumFrames == 0 {
		batcherTargetNumFrames = 1
	}

	var batcherAltDACLIConfig altda.CLIConfig
	if cfg.DeployConfig.UseAltDA {
		fakeAltDAServer := altda.NewFakeDAServer("127.0.0.1", 0, sys.Cfg.Loggers["da-server"])
		if err := fakeAltDAServer.Start(); err != nil {
			return nil, fmt.Errorf("failed to start fake altDA server: %w", err)
		}
		sys.FakeAltDAServer = fakeAltDAServer

		batcherAltDACLIConfig = altda.CLIConfig{
			Enabled:               cfg.DeployConfig.UseAltDA,
			DAServerURL:           fakeAltDAServer.HttpEndpoint(),
			VerifyOnRead:          true,
			GenericDA:             true,
			MaxConcurrentRequests: cfg.BatcherMaxConcurrentDARequest,
		}
	}
	batcherCLIConfig := &bss.CLIConfig{
		L1EthRpc:                 sys.EthInstances[RoleL1].UserRPC().RPC(),
		L2EthRpc:                 sys.EthInstances[RoleSeq].UserRPC().RPC(),
		RollupRpc:                sys.RollupNodes[RoleSeq].UserRPC().RPC(),
		MaxPendingTransactions:   cfg.BatcherMaxPendingTransactions,
		MaxChannelDuration:       1,
		MaxL1TxSize:              batcherMaxL1TxSizeBytes,
		TestUseMaxTxSizeForBlobs: cfg.BatcherUseMaxTxSizeForBlobs,
		TargetNumFrames:          int(batcherTargetNumFrames),
		ApproxComprRatio:         0.4,
		SubSafetyMargin:          4,
		PollInterval:             50 * time.Millisecond,
		TxMgrConfig:              setuputils.NewTxMgrConfig(sys.EthInstances[RoleL1].UserRPC(), cfg.Secrets.Batcher),
		LogConfig: oplog.CLIConfig{
			Level:  log.LevelInfo,
			Format: oplog.FormatText,
		},
		Stopped:               sys.Cfg.DisableBatcher, // Batch submitter may be enabled later
		BatchType:             cfg.BatcherBatchType,
		MaxBlocksPerSpanBatch: cfg.BatcherMaxBlocksPerSpanBatch,
		DataAvailabilityType:  sys.Cfg.DataAvailabilityType,
		CompressionAlgo:       derive.Zlib,
		AltDA:                 batcherAltDACLIConfig,
	}

	// Apply batcher cli modifications
	for _, opt := range startOpts {
		if opt.BatcherMod != nil {
			opt.BatcherMod(batcherCLIConfig)
		}
	}

	// Batch Submitter
	batcher, err := bss.BatcherServiceFromCLIConfig(context.Background(), "0.0.1", batcherCLIConfig, sys.Cfg.Loggers["batcher"])
	if err != nil {
		return nil, fmt.Errorf("failed to setup batch submitter: %w", err)
	}
	sys.BatchSubmitter = batcher
	if action, ok := parsedStartOpts.Get("beforeBatcherStart", ""); ok {
		action(&cfg, sys)
	}
	if err := batcher.Start(context.Background()); err != nil {
		return nil, errors.Join(fmt.Errorf("failed to start batch submitter: %w", err), batcher.Stop(context.Background()))
	}
	return sys, nil
}

// IP6 range that gets blackholed (in case our traffic ever makes it out onto
// the internet).
var blackholeIP6 = net.ParseIP("100::")

// mocknet doesn't allow us to add a peerstore without fully creating the peer ourselves
func (sys *System) NewMockNetPeer() (host.Host, error) {
	sk, _, err := ic.GenerateECDSAKeyPair(rand.Reader)
	if err != nil {
		return nil, err
	}
	id, err := peer.IDFromPrivateKey(sk)
	if err != nil {
		return nil, err
	}
	suffix := id
	if len(id) > 8 {
		suffix = id[len(id)-8:]
	}
	ip := append(net.IP{}, blackholeIP6...)
	copy(ip[net.IPv6len-len(suffix):], suffix)
	a, err := ma.NewMultiaddr(fmt.Sprintf("/ip6/%s/tcp/4242", ip))
	if err != nil {
		return nil, fmt.Errorf("failed to create test multiaddr: %w", err)
	}
	p, err := peer.IDFromPublicKey(sk.GetPublic())
	if err != nil {
		return nil, err
	}

	ps, err := pstoremem.NewPeerstore()
	if err != nil {
		return nil, err
	}
	ps.AddAddr(p, a, peerstore.PermanentAddrTTL)
	_ = ps.AddPrivKey(p, sk)
	_ = ps.AddPubKey(p, sk.GetPublic())

	ds := dsSync.MutexWrap(ds.NewMapDatastore())
	eps, err := store.NewExtendedPeerstore(context.Background(), log.Root(), clock.SystemClock, ps, ds, 24*time.Hour)
	if err != nil {
		return nil, err
	}
	return sys.Mocknet.AddPeerWithPeerstore(p, eps)
}

func (sys *System) BatcherHelper() *batcher.Helper {
	return batcher.NewHelper(sys.t, sys.Cfg.Secrets.Batcher, sys.RollupConfig, sys.NodeClient(RoleL1))
}

func (sys *System) TestAccount(idx int) *ecdsa.PrivateKey {
	switch idx {
	case 0:
		return sys.Cfg.Secrets.Alice
	case 1:
		return sys.Cfg.Secrets.Bob
	case 2:
		return sys.Cfg.Secrets.Mallory
	default:
		panic(fmt.Sprintf("System: no test account for index %d", idx))
	}
}

func ConfigureL1(rollupNodeCfg *rollupNode.Config, l1Node services.EthInstance, beaconEndpoint endpoint.RestHTTP) {
	rollupNodeCfg.L1 = &rollupNode.L1EndpointConfig{
		L1NodeAddr:       endpoint.SelectRPC(EnvRPCPreference(), l1Node.UserRPC()),
		L1TrustRPC:       false,
		L1RPCKind:        sources.RPCKindStandard,
		RateLimit:        0,
		BatchSize:        20,
		HttpPollInterval: time.Millisecond * 100,
		MaxConcurrency:   10,
	}
	rollupNodeCfg.Beacon = &rollupNode.L1BeaconEndpointConfig{
		BeaconAddr: beaconEndpoint.RestHTTP(),
	}
}

func ConfigureL2(rollupNodeCfg *rollupNode.Config, l2Node services.EthInstance, jwtSecret [32]byte) {
	rollupNodeCfg.L2 = &rollupNode.L2EndpointConfig{
		L2EngineAddr:      endpoint.SelectRPC(EnvRPCPreference(), l2Node.AuthRPC()),
		L2EngineJWTSecret: jwtSecret,
	}
}

func (cfg SystemConfig) L1ChainIDBig() *big.Int {
	return new(big.Int).SetUint64(cfg.DeployConfig.L1ChainID)
}

func (cfg SystemConfig) L2ChainIDBig() *big.Int {
	return new(big.Int).SetUint64(cfg.DeployConfig.L2ChainID)
}

func (sys *System) RollupClient(name string) *sources.RollupClient {
	rollupClient, ok := sys.rollupClients[name]
	if ok {
		return rollupClient
	}
	rpcClient := endpoint.DialRPC(endpoint.PreferAnyRPC, sys.RollupEndpoint(name), func(v string) *rpc.Client {
		logger := testlog.Logger(sys.t, log.LevelInfo).New("rollupClient", name)
		cl, err := dial.DialRPCClientWithTimeout(context.Background(), 30*time.Second, logger, v)
		require.NoError(sys.t, err, "failed to dial rollup instance %s", name)
		return cl
	})
	rollupClient = sources.NewRollupClient(client.NewBaseRPCClient(rpcClient,
		// Increase timeouts because CI servers can be under a lot of load
		client.WithCallTimeout(30*time.Second),
		client.WithBatchCallTimeout(30*time.Second)))
	sys.rollupClients[name] = rollupClient
	return rollupClient
}

func (sys *System) NodeClient(name string) *ethclient.Client {
	nodeClient, ok := sys.clients[name]
	if ok {
		return nodeClient
	}
	rpcCl := endpoint.DialRPC(endpoint.PreferAnyRPC, sys.NodeEndpoint(name), func(v string) *rpc.Client {
		logger := testlog.Logger(sys.t, log.LevelInfo).New("node", name)
		cl, err := dial.DialRPCClientWithTimeout(context.Background(), 30*time.Second, logger, v)
		require.NoError(sys.t, err, "failed to dial eth node instance %s", name)
		return cl
	})
	nodeClient = ethclient.NewClient(rpcCl)
	sys.clients[name] = nodeClient
	return nodeClient
}

// EnvRPCPreference reads the type of RPC that should be used.
// Some E2E tests are forced to run with HTTP,
// since HTTP does not support subscriptions, which thus could affect functionality.
// The alternative E2E tests are labeled "ws", but really just any transport here is the same.
func EnvRPCPreference() endpoint.RPCPreference {
	// L1 is a legacy exception; the System setup itself depends on RPC subscriptions.
	if os.Getenv("OP_E2E_USE_HTTP") == "true" {
		return endpoint.PreferHttpRPC
	}
	return endpoint.PreferAnyRPC
}
