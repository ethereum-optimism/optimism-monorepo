package geth

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/txpool/blobpool"
	"github.com/ethereum/go-ethereum/core/txpool/legacypool"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/miner"
	"github.com/ethereum/go-ethereum/node"

	// Force-load the tracer engines to trigger registration
	_ "github.com/ethereum/go-ethereum/eth/tracers/js"
	_ "github.com/ethereum/go-ethereum/eth/tracers/native"

	"github.com/ethereum-optimism/optimism/op-service/clock"
)

func InitL1(blockTime uint64, finalizedDistance uint64, genesis *core.Genesis, c clock.Clock, blobPoolDir string, beaconSrv Beacon, opts ...GethOption) (*GethInstance, error) {
	ethConfig := &ethconfig.Config{
		NetworkId: genesis.Config.ChainID.Uint64(),
		Genesis:   genesis,
		BlobPool: blobpool.Config{
			Datadir:   blobPoolDir,
			Datacap:   blobpool.DefaultConfig.Datacap,
			PriceBump: blobpool.DefaultConfig.PriceBump,
		},
		StateScheme: rawdb.HashScheme,
		Miner: miner.Config{
			PendingFeeRecipient: common.Address{},
			ExtraData:           nil,
			GasCeil:             0,
			GasPrice:            nil,
			// enough to build blocks within 1 second, but high enough to avoid unnecessary test CPU cycles.
			Recommit: time.Millisecond * 400,
		},
	}
	nodeConfig := &node.Config{
		Name:        "l1-geth",
		HTTPHost:    "127.0.0.1",
		HTTPPort:    0,
		WSHost:      "127.0.0.1",
		WSPort:      0,
		WSModules:   []string{"debug", "admin", "eth", "txpool", "net", "rpc", "web3", "personal", "engine", "miner"},
		HTTPModules: []string{"debug", "admin", "eth", "txpool", "net", "rpc", "web3", "personal", "engine", "miner"},
	}

	gethInstance, err := createGethNode(false, nodeConfig, ethConfig, opts...)
	if err != nil {
		return nil, err
	}

	// Instead of running a whole beacon node, we run this fake-proof-of-stake sidecar that sequences L1 blocks using the Engine API.
	gethInstance.Node.RegisterLifecycle(&fakePoS{
		clock:             c,
		eth:               gethInstance.Backend,
		log:               log.Root(), // geth logger is global anyway. Would be nice to replace with a local logger though.
		blockTime:         blockTime,
		finalizedDistance: finalizedDistance,
		safeDistance:      4,
		engineAPI:         catalyst.NewConsensusAPI(gethInstance.Backend),
		beacon:            beaconSrv,
	})

	return gethInstance, nil
}

func defaultNodeConfig(name string, jwtPath string) *node.Config {
	return &node.Config{
		Name:        name,
		WSHost:      "127.0.0.1",
		WSPort:      0,
		AuthAddr:    "127.0.0.1",
		AuthPort:    0,
		HTTPHost:    "127.0.0.1",
		HTTPPort:    0,
		WSModules:   []string{"debug", "admin", "eth", "txpool", "net", "rpc", "web3", "personal", "engine", "miner"},
		HTTPModules: []string{"debug", "admin", "eth", "txpool", "net", "rpc", "web3", "personal", "engine", "miner"},
		JWTSecret:   jwtPath,
	}
}

type GethOption func(ethCfg *ethconfig.Config, nodeCfg *node.Config) error

// InitL2 inits a L2 geth node.
func InitL2(name string, genesis *core.Genesis, jwtPath string, opts ...GethOption) (*GethInstance, error) {
	ethConfig := &ethconfig.Config{
		NetworkId:   genesis.Config.ChainID.Uint64(),
		Genesis:     genesis,
		StateScheme: rawdb.HashScheme,
		Miner: miner.Config{
			PendingFeeRecipient: common.Address{},
			ExtraData:           nil,
			GasCeil:             0,
			GasPrice:            nil,
			// enough to build blocks within 1 second, but high enough to avoid unnecessary test CPU cycles.
			Recommit: time.Millisecond * 400,
		},
		TxPool: legacypool.Config{
			NoLocals: true,
		},
	}
	nodeConfig := defaultNodeConfig(fmt.Sprintf("l2-geth-%v", name), jwtPath)
	return createGethNode(true, nodeConfig, ethConfig, opts...)
}

// createGethNode creates an in-memory geth node based on the configuration.
// The private keys are added to the keystore and are unlocked.
// If the node is l2, catalyst is enabled.
// The node should be started and then closed when done.
func createGethNode(l2 bool, nodeCfg *node.Config, ethCfg *ethconfig.Config, opts ...GethOption) (*GethInstance, error) {
	for i, opt := range opts {
		if err := opt(ethCfg, nodeCfg); err != nil {
			return nil, fmt.Errorf("failed to apply geth option %d: %w", i, err)
		}
	}
	ethCfg.StateScheme = rawdb.HashScheme
	ethCfg.NoPruning = true // force everything to be an archive node
	n, err := node.New(nodeCfg)
	if err != nil {
		n.Close()
		return nil, err
	}

	backend, err := eth.New(n, ethCfg)
	if err != nil {
		n.Close()
		return nil, err

	}

	// PR 25459 changed this to only default in CLI, but not in default programmatic RPC selection.
	// PR 25642 fixed it for the mobile version only...
	utils.RegisterFilterAPI(n, backend.APIBackend, ethCfg)

	n.RegisterAPIs(tracers.APIs(backend.APIBackend))

	// Enable catalyst if l2
	if l2 {
		if err := catalyst.Register(n, backend); err != nil {
			n.Close()
			return nil, err
		}
	}
	return &GethInstance{
		Backend: backend,
		Node:    n,
	}, nil
}
