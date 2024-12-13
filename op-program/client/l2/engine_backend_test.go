package l2

import (
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	"github.com/ethereum-optimism/optimism/op-program/client/l2/engineapi"
	"github.com/ethereum-optimism/optimism/op-program/client/l2/engineapi/test"
	l2test "github.com/ethereum-optimism/optimism/op-program/client/l2/test"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/ethereum/go-ethereum/triedb/hashdb"
	"github.com/stretchr/testify/require"
)

var fundedKey, _ = crypto.GenerateKey()
var fundedAddress = crypto.PubkeyToAddress(fundedKey.PublicKey)
var targetAddress = common.HexToAddress("0x001122334455")

var (
	kzgInputData          = common.FromHex("01e798154708fe7789429634053cbf9f99b619f9f084048927333fce637f549b564c0a11a0f704f4fc3e8acfe0f8245f0ad1347b378fbf96e206da11a5d3630624d25032e67a7e6a4910df5834b8fe70e6bcfeeac0352434196bdf4b2485d5a18f59a8d2a1a625a17f3fea0fe5eb8c896db3764f3185481bc22f91b4aaffcca25f26936857bc3a7c2539ea8ec3a952b7873033e038326e87ed3e1276fd140253fa08e9fc25fb2d9a98527fc22a2c9612fbeafdad446cbc7bcdbdcd780af2c16a")
	ecRecoverInputData    = common.FromHex("18c547e4f7b0f325ad1e56f57e26c745b09a3e503d86e00e5255ff7f715d3d1c000000000000000000000000000000000000000000000000000000000000001c73b1693892219d736caba55bdb67216e485557ea6b6af75f37096c9aa6a5a75feeb940b1d03b21e36b0e47e79769f095fe2ab855bd91e3a38756b7d75a9c4549")
	bn256PairingInputData = common.FromHex("1c76476f4def4bb94541d57ebba1193381ffa7aa76ada664dd31c16024c43f593034dd2920f673e204fee2811c678745fc819b55d3e9d294e45c9b03a76aef41209dd15ebff5d46c4bd888e51a93cf99a7329636c63514396b4a452003a35bf704bf11ca01483bfa8b34b43561848d28905960114c8ac04049af4b6315a416782bb8324af6cfc93537a2ad1a445cfd0ca2a71acd7ac41fadbf933c2a51be344d120a2a4cf30c1bf9845f20c6fe39e07ea2cce61f0c9bb048165fe5e4de877550111e129f1cf1097710d41c4ac70fcdfa5ba2023c6ff1cbeac322de49d1b6df7c2032c61a830e3c17286de9462bf242fca2883585b93870a73853face6a6bf411198e9393920d483a7260bfb731fb5d25f1aa493335a9e71297e485b7aef312c21800deef121f1e76426a00665e5c4479674322d4f75edadd46debd5cd992f6ed090689d0585ff075ec9e99ad690c3395bc4b313370b38ef355acdadcd122975b12c85ea5db8c6deb4aab71808dcb408fe3d1e7690c43d37b4ce6cc0166fa7daa")
)

var (
	ecRecoverReturnValue      = []byte{0x1, 0x2}
	bn256PairingReturnValue   = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	blobPrecompileReturnValue = common.FromHex("000000000000000000000000000000000000000000000000000000000000100073eda753299d7d483339d80809a1d80553bda402fffe5bfeffffffff00000001")
)

var (
	ecRecoverRequiredGas    uint64 = 3000
	bn256PairingRequiredGas uint64 = 113000
	kzgRequiredGas          uint64 = 50_000
)

func TestInitialState(t *testing.T) {
	blocks, chain := setupOracleBackedChain(t, 5)
	head := blocks[5]
	require.Equal(t, head.Header(), chain.CurrentHeader())
	require.Equal(t, head.Header(), chain.CurrentSafeBlock())
	require.Equal(t, head.Header(), chain.CurrentFinalBlock())
}

func TestGetBlocks(t *testing.T) {
	blocks, chain := setupOracleBackedChain(t, 5)

	for i, block := range blocks {
		blockNumber := uint64(i)
		assertBlockDataAvailable(t, chain, block, blockNumber)
		require.Equal(t, block.Hash(), chain.GetCanonicalHash(blockNumber), "get canonical hash for block %v", blockNumber)
	}
}

func TestCanonicalHashNotFoundPastChainHead(t *testing.T) {
	blocks, chain := setupOracleBackedChainWithLowerHead(t, 5, 3)

	for i := 0; i <= 3; i++ {
		require.Equal(t, blocks[i].Hash(), chain.GetCanonicalHash(uint64(i)))
		require.Equal(t, blocks[i].Header(), chain.GetHeaderByNumber(uint64(i)))
	}
	for i := 4; i <= 5; i++ {
		require.Equal(t, common.Hash{}, chain.GetCanonicalHash(uint64(i)))
		require.Nil(t, chain.GetHeaderByNumber(uint64(i)))
	}
}

func TestAppendToChain(t *testing.T) {
	blocks, chain := setupOracleBackedChainWithLowerHead(t, 4, 3)
	newBlock := blocks[4]
	require.Nil(t, chain.GetBlock(newBlock.Hash(), newBlock.NumberU64()), "block unknown before being added")

	_, err := chain.InsertBlockWithoutSetHead(newBlock, false)
	require.NoError(t, err)
	require.Equal(t, blocks[3].Header(), chain.CurrentHeader(), "should not update chain head yet")
	require.Equal(t, common.Hash{}, chain.GetCanonicalHash(uint64(4)), "not yet a canonical hash")
	require.Nil(t, chain.GetHeaderByNumber(uint64(4)), "not yet a canonical header")
	assertBlockDataAvailable(t, chain, newBlock, 4)

	canonical, err := chain.SetCanonical(newBlock)
	require.NoError(t, err)
	require.Equal(t, newBlock.Hash(), canonical)
	require.Equal(t, newBlock.Hash(), chain.GetCanonicalHash(uint64(4)), "get canonical hash for new head")
	require.Equal(t, newBlock.Header(), chain.GetHeaderByNumber(uint64(4)), "get canonical header for new head")
}

func TestSetFinalized(t *testing.T) {
	blocks, chain := setupOracleBackedChainWithLowerHead(t, 5, 0)
	for _, block := range blocks[1:] {
		_, err := chain.InsertBlockWithoutSetHead(block, false)
		require.NoError(t, err)
	}
	chain.SetFinalized(blocks[2].Header())
	require.Equal(t, blocks[2].Header(), chain.CurrentFinalBlock())
}

func TestSetSafe(t *testing.T) {
	blocks, chain := setupOracleBackedChainWithLowerHead(t, 5, 0)
	for _, block := range blocks[1:] {
		_, err := chain.InsertBlockWithoutSetHead(block, false)
		require.NoError(t, err)
	}
	chain.SetSafe(blocks[2].Header())
	require.Equal(t, blocks[2].Header(), chain.CurrentSafeBlock())
}

func TestUpdateStateDatabaseWhenImportingBlock(t *testing.T) {
	blocks, chain := setupOracleBackedChain(t, 3)
	newBlock := createBlock(t, chain)

	db, err := chain.StateAt(blocks[1].Root())
	require.NoError(t, err)
	balance := db.GetBalance(fundedAddress)
	require.NotEqual(t, big.NewInt(0), balance, "should have balance at imported block")

	require.NotEqual(t, blocks[1].Root(), newBlock.Root(), "block should have modified world state")

	require.False(t, chain.HasBlockAndState(newBlock.Root(), newBlock.NumberU64()), "state from non-imported block should not be available")

	_, err = chain.InsertBlockWithoutSetHead(newBlock, false)
	require.NoError(t, err)
	db, err = chain.StateAt(newBlock.Root())
	require.NoError(t, err, "state should be available after importing")
	balance = db.GetBalance(fundedAddress)
	require.NotEqual(t, big.NewInt(0), balance, "should have balance from imported block")
}

func TestRejectBlockWithStateRootMismatch(t *testing.T) {
	_, chain := setupOracleBackedChain(t, 1)
	newBlock := createBlock(t, chain)
	// Create invalid block by keeping the modified state root but exclude the transaction
	invalidBlock := types.NewBlockWithHeader(newBlock.Header())

	_, err := chain.InsertBlockWithoutSetHead(invalidBlock, false)
	require.ErrorContains(t, err, "block root mismatch")
}

func TestGetHeaderByNumber(t *testing.T) {
	t.Run("Forwards", func(t *testing.T) {
		blocks, chain := setupOracleBackedChain(t, 10)
		for _, block := range blocks {
			result := chain.GetHeaderByNumber(block.NumberU64())
			require.Equal(t, block.Header(), result)
		}
	})
	t.Run("Reverse", func(t *testing.T) {
		blocks, chain := setupOracleBackedChain(t, 10)
		for i := len(blocks) - 1; i >= 0; i-- {
			block := blocks[i]
			result := chain.GetHeaderByNumber(block.NumberU64())
			require.Equal(t, block.Header(), result)
		}
	})
	t.Run("AppendedBlock", func(t *testing.T) {
		_, chain := setupOracleBackedChain(t, 10)

		// Append a block
		newBlock := createBlock(t, chain)
		_, err := chain.InsertBlockWithoutSetHead(newBlock, false)
		require.NoError(t, err)
		_, err = chain.SetCanonical(newBlock)
		require.NoError(t, err)

		require.Equal(t, newBlock.Header(), chain.GetHeaderByNumber(newBlock.NumberU64()))
	})
	t.Run("AppendedBlockAfterLookup", func(t *testing.T) {
		blocks, chain := setupOracleBackedChain(t, 10)
		// Look up an early block to prime the block cache
		require.Equal(t, blocks[0].Header(), chain.GetHeaderByNumber(blocks[0].NumberU64()))

		// Append a block
		newBlock := createBlock(t, chain)
		_, err := chain.InsertBlockWithoutSetHead(newBlock, false)
		require.NoError(t, err)
		_, err = chain.SetCanonical(newBlock)
		require.NoError(t, err)

		require.Equal(t, newBlock.Header(), chain.GetHeaderByNumber(newBlock.NumberU64()))
	})
	t.Run("AppendedMultipleBlocks", func(t *testing.T) {
		blocks, chain := setupOracleBackedChainWithLowerHead(t, 5, 2)

		// Append a few blocks
		newBlock1 := blocks[3]
		newBlock2 := blocks[4]
		newBlock3 := blocks[5]
		_, err := chain.InsertBlockWithoutSetHead(newBlock1, false)
		require.NoError(t, err)
		_, err = chain.InsertBlockWithoutSetHead(newBlock2, false)
		require.NoError(t, err)
		_, err = chain.InsertBlockWithoutSetHead(newBlock3, false)
		require.NoError(t, err)

		_, err = chain.SetCanonical(newBlock3)
		require.NoError(t, err)

		require.Equal(t, newBlock3.Header(), chain.GetHeaderByNumber(newBlock3.NumberU64()), "Lookup block3")
		require.Equal(t, newBlock2.Header(), chain.GetHeaderByNumber(newBlock2.NumberU64()), "Lookup block2")
		require.Equal(t, newBlock1.Header(), chain.GetHeaderByNumber(newBlock1.NumberU64()), "Lookup block1")
	})
}

func TestPrecompileOracle(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		target      common.Address
		requiredGas uint64
		result      []byte
	}{
		{
			name:        "EcRecover",
			input:       ecRecoverInputData,
			target:      common.BytesToAddress([]byte{0x1}),
			requiredGas: ecRecoverRequiredGas,
			result:      ecRecoverReturnValue,
		},
		{
			name:        "Bn256Pairing",
			input:       bn256PairingInputData,
			target:      common.BytesToAddress([]byte{0x8}),
			requiredGas: bn256PairingRequiredGas,
			result:      bn256PairingReturnValue,
		},
		{
			name:        "KZGPointEvaluation",
			input:       kzgInputData,
			target:      common.BytesToAddress([]byte{0xa}),
			requiredGas: kzgRequiredGas,
			result:      blobPrecompileReturnValue,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			blockCount := 3
			headBlockNumber := 3
			logger := testlog.Logger(t, log.LevelDebug)
			chainCfg, blocks, oracle := setupOracle(t, blockCount, headBlockNumber, true)
			head := blocks[headBlockNumber].Hash()
			stubOutput := eth.OutputV0{BlockHash: head}
			precompileOracle := l2test.NewStubPrecompileOracle(t)
			arg := append(test.target.Bytes(), binary.BigEndian.AppendUint64(nil, test.requiredGas)...)
			arg = append(arg, test.input...)
			precompileOracle.Results = map[common.Hash]l2test.PrecompileResult{
				crypto.Keccak256Hash(arg): {Result: test.result, Ok: true},
			}
			chain, err := NewOracleBackedL2Chain(logger, oracle, precompileOracle, chainCfg, common.Hash(eth.OutputRoot(&stubOutput)))
			require.NoError(t, err)

			newBlock := createBlock(t, chain, WithInput(test.input), WithTargetAddress(test.target))
			_, err = chain.InsertBlockWithoutSetHead(newBlock, false)
			require.NoError(t, err)
			require.Equal(t, 1, precompileOracle.Calls)
		})
	}
}

func assertBlockDataAvailable(t *testing.T, chain *OracleBackedL2Chain, block *types.Block, blockNumber uint64) {
	require.Equal(t, block.Hash(), chain.GetBlockByHash(block.Hash()).Hash(), "get block %v by hash", blockNumber)
	require.Equal(t, block.Header(), chain.GetHeaderByHash(block.Hash()), "get header %v by hash", blockNumber)
	require.Equal(t, block.Hash(), chain.GetBlock(block.Hash(), blockNumber).Hash(), "get block %v by hash and number", blockNumber)
	require.Equal(t, block.Header(), chain.GetHeader(block.Hash(), blockNumber), "get header %v by hash and number", blockNumber)
	require.True(t, chain.HasBlockAndState(block.Hash(), blockNumber), "has block and state for block %v", blockNumber)
}

func setupOracleBackedChain(t *testing.T, blockCount int) ([]*types.Block, *OracleBackedL2Chain) {
	return setupOracleBackedChainWithLowerHead(t, blockCount, blockCount)
}

func setupOracleBackedChainWithLowerHead(t *testing.T, blockCount int, headBlockNumber int) ([]*types.Block, *OracleBackedL2Chain) {
	logger := testlog.Logger(t, log.LevelDebug)
	chainCfg, blocks, oracle := setupOracle(t, blockCount, headBlockNumber, false)
	head := blocks[headBlockNumber].Hash()
	stubOutput := eth.OutputV0{BlockHash: head}
	precompileOracle := l2test.NewStubPrecompileOracle(t)
	chain, err := NewOracleBackedL2Chain(logger, oracle, precompileOracle, chainCfg, common.Hash(eth.OutputRoot(&stubOutput)))
	require.NoError(t, err)
	return blocks, chain
}

func setupOracle(t *testing.T, blockCount int, headBlockNumber int, enableEcotone bool) (*params.ChainConfig, []*types.Block, *l2test.StubBlockOracle) {
	deployConfig := &genesis.DeployConfig{
		L2InitializationConfig: genesis.L2InitializationConfig{
			DevDeployConfig: genesis.DevDeployConfig{
				FundDevAccounts: true,
			},
			L2GenesisBlockDeployConfig: genesis.L2GenesisBlockDeployConfig{
				L2GenesisBlockGasLimit: 30_000_000,
				// Arbitrary non-zero difficulty in genesis.
				// This is slightly weird for a chain starting post-merge but it happens so need to make sure it works
				L2GenesisBlockDifficulty: (*hexutil.Big)(big.NewInt(100)),
			},
			L2CoreDeployConfig: genesis.L2CoreDeployConfig{
				L1ChainID:   900,
				L2ChainID:   901,
				L2BlockTime: 2,
			},
		},
	}
	if enableEcotone {
		ts := hexutil.Uint64(0)
		deployConfig.L2GenesisRegolithTimeOffset = &ts
		deployConfig.L2GenesisCanyonTimeOffset = &ts
		deployConfig.L2GenesisDeltaTimeOffset = &ts
		deployConfig.L2GenesisEcotoneTimeOffset = &ts
	}
	l1Genesis, err := genesis.NewL1Genesis(deployConfig)
	require.NoError(t, err)
	l2Genesis, err := genesis.NewL2Genesis(deployConfig, l1Genesis.ToBlock().Header())
	require.NoError(t, err)

	l2Genesis.Alloc[fundedAddress] = types.Account{
		Balance: big.NewInt(1_000_000_000_000_000_000),
		Nonce:   0,
	}
	chainCfg := l2Genesis.Config
	consensus := beacon.New(nil)
	db := rawdb.NewMemoryDatabase()
	trieDB := triedb.NewDatabase(db, &triedb.Config{HashDB: hashdb.Defaults})

	// Set minimal amount of stuff to avoid nil references later
	genesisBlock := l2Genesis.MustCommit(db, trieDB)
	blocks, _ := core.GenerateChain(chainCfg, genesisBlock, consensus, db, blockCount, func(i int, gen *core.BlockGen) {})
	blocks = append([]*types.Block{genesisBlock}, blocks...)

	var outputs []eth.Output
	for _, block := range blocks {
		outputs = append(outputs, &eth.OutputV0{BlockHash: block.Hash()})
	}
	oracle := l2test.NewStubOracleWithBlocks(t, blocks[:headBlockNumber+1], outputs, db)
	return chainCfg, blocks, oracle
}

type blockCreateConfig struct {
	input  []byte
	target *common.Address
}

type blockCreateOption func(*blockCreateConfig)

func WithInput(input []byte) blockCreateOption {
	return func(opts *blockCreateConfig) {
		opts.input = input
	}
}

func WithTargetAddress(target common.Address) blockCreateOption {
	return func(opts *blockCreateConfig) {
		opts.target = &target
	}
}

func createBlock(t *testing.T, chain *OracleBackedL2Chain, opts ...blockCreateOption) *types.Block {
	cfg := blockCreateConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.target == nil {
		cfg.target = &targetAddress
	}
	parent := chain.GetBlockByHash(chain.CurrentHeader().Hash())
	parentDB, err := chain.StateAt(parent.Root())
	require.NoError(t, err)
	nonce := parentDB.GetNonce(fundedAddress)
	config := chain.Config()
	db := rawdb.NewDatabase(NewOracleBackedDB(chain.oracle))
	blocks, _ := core.GenerateChain(config, parent, chain.Engine(), db, 1, func(i int, gen *core.BlockGen) {
		rawTx := &types.DynamicFeeTx{
			ChainID:   config.ChainID,
			Nonce:     nonce,
			To:        cfg.target,
			GasTipCap: big.NewInt(0),
			GasFeeCap: parent.BaseFee(),
			Gas:       2_000_000,
			Data:      cfg.input,
			Value:     big.NewInt(1),
		}
		tx := types.MustSignNewTx(fundedKey, types.NewLondonSigner(config.ChainID), rawTx)
		gen.AddTx(tx)
	})
	return blocks[0]
}

func TestEngineAPITests(t *testing.T) {
	test.RunEngineAPITests(t, func(t *testing.T) engineapi.EngineBackend {
		_, chain := setupOracleBackedChain(t, 0)
		return chain
	})
}
