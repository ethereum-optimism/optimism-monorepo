package helpers

import (
	"errors"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/txpool/blobpool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

// L1CanonSrc is used to sync L1 from another node.
// The other node always has the canonical chain.
// May be nil if there is nothing to sync from
type L1CanonSrc func(num uint64) *types.Block

// L1Replica is an instrumented in-memory L1 geth node that:
// - can sync from the given canonical L1 blocks source
// - can rewind the chain back (for reorgs)
// - can provide an RPC with mock errors
type L1Replica struct {
	log log.Logger

	node *node.Node
	Eth  *eth.Ethereum

	// L1 evm / chain
	l1Chain    *core.BlockChain
	l1Database ethdb.Database
	l1Cfg      *core.Genesis
	l1Signer   types.Signer

	failL1RPC func(call []rpc.BatchElem) error // mock error
}

// NewL1Replica constructs a L1Replica starting at the given genesis.
func NewL1Replica(t Testing, log log.Logger, genesis *core.Genesis) *L1Replica {
	ethCfg := &ethconfig.Config{
		NetworkId:                 genesis.Config.ChainID.Uint64(),
		Genesis:                   genesis,
		RollupDisableTxPoolGossip: true,
		StateScheme:               rawdb.HashScheme,
		NoPruning:                 true,
		BlobPool: blobpool.Config{
			Datadir:   t.TempDir(),
			Datacap:   blobpool.DefaultConfig.Datacap,
			PriceBump: blobpool.DefaultConfig.PriceBump,
		},
	}
	nodeCfg := &node.Config{
		Name:        "l1-geth",
		WSHost:      "127.0.0.1",
		WSPort:      0,
		HTTPHost:    "127.0.0.1",
		HTTPPort:    0,
		WSModules:   []string{"debug", "admin", "eth", "txpool", "net", "rpc", "web3", "personal"},
		HTTPModules: []string{"debug", "admin", "eth", "txpool", "net", "rpc", "web3", "personal"},
		DataDir:     "", // in-memory
		P2P: p2p.Config{
			NoDiscovery: true,
			NoDial:      true,
		},
	}
	n, err := node.New(nodeCfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = n.Close()
	})

	backend, err := eth.New(n, ethCfg)
	require.NoError(t, err)

	n.RegisterAPIs(tracers.APIs(backend.APIBackend))

	require.NoError(t, n.Start(), "failed to start L1 geth node")
	return &L1Replica{
		log:        log,
		node:       n,
		Eth:        backend,
		l1Chain:    backend.BlockChain(),
		l1Database: backend.ChainDb(),
		l1Cfg:      genesis,
		l1Signer:   types.LatestSigner(genesis.Config),
		failL1RPC:  nil,
	}
}

// ActL1RewindToParent rewinds the L1 chain to parent block of head
func (s *L1Replica) ActL1RewindToParent(t Testing) {
	s.ActL1RewindDepth(1)(t)
}

func (s *L1Replica) ActL1RewindDepth(depth uint64) Action {
	return func(t Testing) {
		if depth == 0 {
			return
		}
		head := s.l1Chain.CurrentHeader().Number.Uint64()
		if head < depth {
			t.InvalidAction("cannot rewind L1 past genesis (current: %d, rewind depth: %d)", head, depth)
			return
		}
		finalized := s.l1Chain.CurrentFinalBlock()
		if finalized != nil && head < finalized.Number.Uint64()+depth {
			t.InvalidAction("cannot rewind head of chain past finalized block %d with rewind depth %d", finalized.Number.Uint64(), depth)
			return
		}
		if err := s.l1Chain.SetHead(head - depth); err != nil {
			t.Fatalf("failed to rewind L1 chain to nr %d: %v", head-depth, err)
		}
	}
}

// ActL1Sync processes the next canonical L1 block,
// or rewinds one block if the canonical block cannot be applied to the head.
func (s *L1Replica) ActL1Sync(canonL1 func(num uint64) *types.Block) Action {
	return func(t Testing) {
		selfHead := s.l1Chain.CurrentHeader()
		n := selfHead.Number.Uint64()
		expected := canonL1(n)
		if expected == nil || selfHead.Hash() != expected.Hash() {
			s.ActL1RewindToParent(t)
			return
		}
		next := canonL1(n + 1)
		if next == nil {
			t.InvalidAction("already fully synced to head %s (%d), n+1 is not there", selfHead.Hash(), n)
			return
		}
		if next.ParentHash() != selfHead.Hash() {
			// canonical chain must be set up wrong - with actions one by one it is not supposed to reorg during a single sync step.
			t.Fatalf("canonical L1 source reorged unexpectedly from %s (num %d) to next block %s (parent %s)", n, selfHead.Hash(), next.Hash(), next.ParentHash())
		}
		_, err := s.l1Chain.InsertChain([]*types.Block{next})
		require.NoError(t, err, "L1 replica could not sync next canonical L1 block %s (%d)", next.Hash(), next.NumberU64())
	}
}

func (s *L1Replica) CanonL1Chain() func(num uint64) *types.Block {
	return s.l1Chain.GetBlockByNumber
}

// ActL1RPCFail makes the next L1 RPC request to this node fail
func (s *L1Replica) ActL1RPCFail(t Testing) {
	s.failL1RPC = func(call []rpc.BatchElem) error {
		s.failL1RPC = nil
		return errors.New("mock L1 RPC error")
	}
}

func (s *L1Replica) MockL1RPCErrors(fn func() error) {
	s.failL1RPC = func(call []rpc.BatchElem) error {
		return fn()
	}
}

func (s *L1Replica) HTTPEndpoint() string {
	return s.node.HTTPEndpoint()
}

func (s *L1Replica) EthClient() *ethclient.Client {
	cl := s.node.Attach()
	return ethclient.NewClient(cl)
}

func (s *L1Replica) RPCClient() client.RPC {
	cl := s.node.Attach()
	return testutils.RPCErrFaker{
		RPC: client.NewBaseRPCClient(cl),
		ErrFn: func(call []rpc.BatchElem) error {
			if s.failL1RPC == nil {
				return nil
			}
			return s.failL1RPC(call)
		},
	}
}

func (s *L1Replica) L1Client(t Testing, cfg *rollup.Config) *sources.L1Client {
	l1F, err := sources.NewL1Client(s.RPCClient(), s.log, nil, sources.L1ClientDefaultConfig(cfg, false, sources.RPCKindStandard))
	require.NoError(t, err)
	return l1F
}

func (s *L1Replica) L1ClientSimple(t Testing) *sources.L1Client {
	l1F, err := sources.NewL1Client(s.RPCClient(), s.log, nil, sources.L1ClientSimpleConfig(false, sources.RPCKindStandard, 100))
	require.NoError(t, err)
	return l1F
}

func (s *L1Replica) L1Chain() *core.BlockChain {
	return s.l1Chain
}

func (s *L1Replica) UnsafeNum() uint64 {
	head := s.l1Chain.CurrentBlock()
	headNum := uint64(0)
	if head != nil {
		headNum = head.Number.Uint64()
	}
	return headNum
}

func (s *L1Replica) SafeNum() uint64 {
	safe := s.l1Chain.CurrentSafeBlock()
	safeNum := uint64(0)
	if safe != nil {
		safeNum = safe.Number.Uint64()
	}
	return safeNum
}

func (s *L1Replica) FinalizedNum() uint64 {
	finalized := s.l1Chain.CurrentFinalBlock()
	finalizedNum := uint64(0)
	if finalized != nil {
		finalizedNum = finalized.Number.Uint64()
	}
	return finalizedNum
}

// ActL1Finalize finalizes a later block, which must be marked as safe before doing so (see ActL1SafeNext).
func (s *L1Replica) ActL1Finalize(t Testing, num uint64) {
	safeNum := s.SafeNum()
	finalizedNum := s.FinalizedNum()
	if safeNum < num {
		t.InvalidAction("need to move forward safe block before moving finalized block")
		return
	}
	newFinalized := s.l1Chain.GetHeaderByNumber(num)
	if newFinalized == nil {
		t.Fatalf("expected block at %d after finalized L1 block %d, safe head is ahead", num, finalizedNum)
	}
	s.l1Chain.SetFinalized(newFinalized)
}

// ActL1FinalizeNext finalizes the next block, which must be marked as safe before doing so (see ActL1SafeNext).
func (s *L1Replica) ActL1FinalizeNext(t Testing) {
	n := s.FinalizedNum() + 1
	s.ActL1Finalize(t, n)
}

// ActL1Safe marks the given unsafe block as safe.
func (s *L1Replica) ActL1Safe(t Testing, num uint64) {
	newSafe := s.l1Chain.GetHeaderByNumber(num)
	if newSafe == nil {
		t.InvalidAction("could not find L1 block %d, cannot label it as safe", num)
		return
	}
	s.l1Chain.SetSafe(newSafe)
}

// ActL1SafeNext marks the next unsafe block as safe.
func (s *L1Replica) ActL1SafeNext(t Testing) {
	n := s.SafeNum() + 1
	s.ActL1Safe(t, n)
}

func (s *L1Replica) Close() error {
	return s.node.Close()
}
