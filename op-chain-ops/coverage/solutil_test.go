package coverage

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/holiman/uint256"
	"log"
	"math/big"
	"testing"
)

type MyContract struct {
	addr common.Address
}

func (m MyContract) Address() common.Address {
	return m.addr
}

func NewEVMEnv() (*vm.EVM, *state.StateDB) {
	// Temporary hack until Cancun is activated on mainnet
	cpy := *params.MainnetChainConfig
	chainCfg := &cpy // don't modify the global chain config
	// Activate Cancun for EIP-4844 KZG point evaluation precompile
	cancunActivation := *chainCfg.ShanghaiTime + 10
	chainCfg.CancunTime = &cancunActivation
	offsetBlocks := uint64(1000) // blocks after cancun fork
	bc := &testChain{startTime: *chainCfg.CancunTime + offsetBlocks*12}
	header := bc.GetHeader(common.Hash{}, 17034870+offsetBlocks)
	db := rawdb.NewMemoryDatabase()
	statedb := state.NewDatabase(triedb.NewDatabase(db, nil), nil)
	state, err := state.New(types.EmptyRootHash, statedb)
	if err != nil {
		panic(fmt.Errorf("failed to create memory state db: %w", err))
	}
	blockContext := core.NewEVMBlockContext(header, bc, nil, chainCfg, state)
	vmCfg := vm.Config{}

	env := vm.NewEVM(blockContext, vm.TxContext{}, state, chainCfg, vmCfg)

	return env, state
}

type testChain struct {
	startTime uint64
}

func (d *testChain) Engine() consensus.Engine {
	return ethash.NewFullFaker()
}

func (d *testChain) GetHeader(h common.Hash, n uint64) *types.Header {
	parentHash := common.Hash{0: 0xff}
	binary.BigEndian.PutUint64(parentHash[1:], n-1)
	return &types.Header{
		ParentHash:      parentHash,
		UncleHash:       types.EmptyUncleHash,
		Coinbase:        common.Address{},
		Root:            common.Hash{},
		TxHash:          types.EmptyTxsHash,
		ReceiptHash:     types.EmptyReceiptsHash,
		Bloom:           types.Bloom{},
		Difficulty:      big.NewInt(0),
		Number:          new(big.Int).SetUint64(n),
		GasLimit:        30_000_000,
		GasUsed:         0,
		Time:            d.startTime + n*12,
		Extra:           nil,
		MixDigest:       common.Hash{},
		Nonce:           types.BlockNonce{},
		BaseFee:         big.NewInt(7),
		WithdrawalsHash: &types.EmptyWithdrawalsHash,
	}
}
func TestEverything(t *testing.T) {
	artifactFS := foundry.OpenArtifactsDir("../../packages/contracts-bedrock/forge-artifacts")

	artifact1, err := artifactFS.ReadArtifact("SimpleStorage.sol", "SimpleStorage")
	if err != nil {
		log.Fatalf("Failed to load artifact: %v", err)
	}

	artifacts := []*foundry.Artifact{artifact1}

	tracer, err := NewCoverageTracer(artifacts)
	if err != nil {
		log.Fatalf("Failed to initialize CoverageTracer: %v", err)
	}

	env, _ := NewEVMEnv()
	env.Config.Tracer = tracer.Hooks()
	env.StateDB.SetCode(common.Address{0: 0xff, 19: 1}, artifact1.DeployedBytecode.Object)

	contractABI, err := abi.JSON(bytes.NewReader([]byte(`[
		{
			"inputs": [{"internalType": "bytes32", "name": "_key", "type": "bytes32"}],
			"name": "get",
			"outputs": [{"internalType": "bytes32", "name": "", "type": "bytes32"}],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [{"internalType": "bytes32", "name": "_key", "type": "bytes32"}, {"internalType": "bytes32", "name": "_value", "type": "bytes32"}],
			"name": "set",
			"outputs": [],
			"stateMutability": "payable",
			"type": "function"
		}
	]`)))
	if err != nil {
		log.Fatalf("Failed to parse ABI: %v", err)
	}

	// Example key and value
	key := [32]byte{}
	copy(key[:], ("example_key"))
	value := [32]byte{}
	copy(value[:], ("example_value"))

	// Call the `set` function
	setData, err := contractABI.Pack("set", key, value)
	if err != nil {
		log.Fatalf("Failed to encode set data: %v", err)
	}
	fmt.Printf("Encoded set data: %s\n", hex.EncodeToString(setData))

	myContract := MyContract{addr: common.Address{0: 0xff, 19: 1}}

	_, _, err = env.Call(myContract, common.Address{0: 0xff, 19: 1}, setData, 90000, uint256.NewInt(0))
	if err != nil {
		log.Fatalf("EVM Call failed: %v", err)
	}

	// Call the `get` function
	getData, err := contractABI.Pack("get", key)
	if err != nil {
		log.Fatalf("Failed to encode set data: %v", err)
	}
	fmt.Printf("Encoded get data: %s\n", hex.EncodeToString(setData))

	_, _, err = env.Call(myContract, common.Address{0: 0xff, 19: 1}, getData, 90000, uint256.NewInt(0))
	if err != nil {
		log.Fatalf("EVM Call failed: %v", err)
	}

	if err := tracer.GenerateLCOV("coverage.lcov"); err != nil {
		log.Fatalf("Failed to generate LCOV: %v", err)
	}
}
