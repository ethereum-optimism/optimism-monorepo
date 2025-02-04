package l2

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-program/client/l2/test"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/ethereum/go-ethereum/triedb/hashdb"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

var (
	userAccount    = common.HexToAddress("0x1111")
	codeAccount    = common.HexToAddress("0x2222")
	unknownAccount = common.HexToAddress("0x3333")
)

// Should implement the KeyValueStore API
var _ ethdb.KeyValueStore = (*OracleKeyValueStore)(nil)

func TestGet(t *testing.T) {
	t.Run("IncorrectLengthKey", func(t *testing.T) {
		oracle := test.NewStubStateOracle(t)
		db := NewOracleBackedDB(memorydb.New(), oracle, eth.ChainIDFromUInt64(1234))
		val, err := db.Get([]byte{1, 2, 3})
		require.ErrorIs(t, err, ErrInvalidKeyLength)
		require.Nil(t, val)
	})

	t.Run("KeyWithCodePrefix", func(t *testing.T) {
		oracle := test.NewStubStateOracle(t)
		db := NewOracleBackedDB(memorydb.New(), oracle, eth.ChainIDFromUInt64(1234))
		key := common.HexToHash("0x12345678")
		prefixedKey := append(rawdb.CodePrefix, key.Bytes()...)

		expected := []byte{1, 2, 3}
		oracle.Code[key] = expected
		val, err := db.Get(prefixedKey)

		require.NoError(t, err)
		require.Equal(t, expected, val)
	})

	t.Run("NormalKeyThatHappensToStartWithCodePrefix", func(t *testing.T) {
		oracle := test.NewStubStateOracle(t)
		db := NewOracleBackedDB(memorydb.New(), oracle, eth.ChainIDFromUInt64(1234))
		key := make([]byte, common.HashLength)
		copy(rawdb.CodePrefix, key)
		fmt.Println(key[0])
		expected := []byte{1, 2, 3}
		oracle.Data[common.BytesToHash(key)] = expected
		val, err := db.Get(key)

		require.NoError(t, err)
		require.Equal(t, expected, val)
	})

	t.Run("KnownKey", func(t *testing.T) {
		key := common.HexToHash("0xAA4488")
		expected := []byte{2, 6, 3, 8}
		oracle := test.NewStubStateOracle(t)
		oracle.Data[key] = expected
		db := NewOracleBackedDB(memorydb.New(), oracle, eth.ChainIDFromUInt64(1234))
		val, err := db.Get(key.Bytes())
		require.NoError(t, err)
		require.Equal(t, expected, val)
	})
}

func TestPut(t *testing.T) {
	t.Run("NewKey", func(t *testing.T) {
		oracle := test.NewStubStateOracle(t)
		db := NewOracleBackedDB(memorydb.New(), oracle, eth.ChainIDFromUInt64(1234))
		key := common.HexToHash("0xAA4488")
		value := []byte{2, 6, 3, 8}
		err := db.Put(key.Bytes(), value)
		require.NoError(t, err)

		actual, err := db.Get(key.Bytes())
		require.NoError(t, err)
		require.Equal(t, value, actual)
	})
	t.Run("ReplaceKey", func(t *testing.T) {
		oracle := test.NewStubStateOracle(t)
		db := NewOracleBackedDB(memorydb.New(), oracle, eth.ChainIDFromUInt64(1234))
		key := common.HexToHash("0xAA4488")
		value1 := []byte{2, 6, 3, 8}
		value2 := []byte{1, 2, 3}
		err := db.Put(key.Bytes(), value1)
		require.NoError(t, err)
		err = db.Put(key.Bytes(), value2)
		require.NoError(t, err)

		actual, err := db.Get(key.Bytes())
		require.NoError(t, err)
		require.Equal(t, value2, actual)
	})
}

func TestSupportsStateDBOperations(t *testing.T) {
	l2Genesis := createGenesis()
	realDb := rawdb.NewDatabase(memorydb.New())
	trieDB := triedb.NewDatabase(realDb, &triedb.Config{HashDB: hashdb.Defaults})
	genesisBlock := l2Genesis.MustCommit(realDb, trieDB)

	loader := test.NewKvStateOracle(t, realDb)
	assertStateDataAvailable(t, NewOracleBackedDB(memorydb.New(), loader, eth.ChainIDFromUInt64(1234)), l2Genesis, genesisBlock)
}

func TestUpdateState(t *testing.T) {
	l2Genesis := createGenesis()
	oracle := test.NewStubStateOracle(t)
	db := rawdb.NewDatabase(NewOracleBackedDB(memorydb.New(), oracle, eth.ChainIDFromUInt64(1234)))

	trieDB := triedb.NewDatabase(db, &triedb.Config{HashDB: hashdb.Defaults})
	genesisBlock := l2Genesis.MustCommit(db, trieDB)
	assertStateDataAvailable(t, db, l2Genesis, genesisBlock)

	statedb, err := state.New(genesisBlock.Root(), state.NewDatabase(triedb.NewDatabase(rawdb.NewDatabase(db), nil), nil))
	require.NoError(t, err)
	statedb.MakeSinglethreaded()
	statedb.SetBalance(userAccount, uint256.NewInt(50), tracing.BalanceChangeUnspecified)
	require.Equal(t, uint256.NewInt(50), statedb.GetBalance(userAccount))
	statedb.SetNonce(userAccount, uint64(5))
	require.Equal(t, uint64(5), statedb.GetNonce(userAccount))

	statedb.SetBalance(unknownAccount, uint256.NewInt(60), tracing.BalanceChangeUnspecified)
	require.Equal(t, uint256.NewInt(60), statedb.GetBalance(unknownAccount))
	statedb.SetCode(codeAccount, []byte{1})
	require.Equal(t, []byte{1}, statedb.GetCode(codeAccount))

	// Changes should be available under the new state root after committing
	isCancun := l2Genesis.Config.IsCancun(genesisBlock.Number(), genesisBlock.Time())
	newRoot, err := statedb.Commit(genesisBlock.NumberU64()+1, false, isCancun)
	require.NoError(t, err)
	err = statedb.Database().TrieDB().Commit(newRoot, true)
	require.NoError(t, err)

	statedb, err = state.New(newRoot, state.NewDatabase(triedb.NewDatabase(rawdb.NewDatabase(db), nil), nil))
	require.NoError(t, err)
	statedb.MakeSinglethreaded()
	require.Equal(t, uint256.NewInt(50), statedb.GetBalance(userAccount))
	require.Equal(t, uint64(5), statedb.GetNonce(userAccount))
	require.Equal(t, uint256.NewInt(60), statedb.GetBalance(unknownAccount))
	require.Equal(t, []byte{1}, statedb.GetCode(codeAccount))
}

func createGenesis() *core.Genesis {
	l2Genesis := &core.Genesis{
		Config:     &params.ChainConfig{},
		Difficulty: common.Big0,
		ParentHash: common.Hash{},
		BaseFee:    big.NewInt(7),
		Alloc: map[common.Address]types.Account{
			userAccount: {
				Balance: big.NewInt(1_000_000_000_000_000_000),
				Nonce:   10,
			},
			codeAccount: {
				Balance: big.NewInt(100),
				Code:    []byte{5, 7, 8, 3, 4},
				Storage: map[common.Hash]common.Hash{
					common.HexToHash("0x01"): common.HexToHash("0x11"),
					common.HexToHash("0x02"): common.HexToHash("0x12"),
					common.HexToHash("0x03"): common.HexToHash("0x13"),
				},
			},
		},
	}
	return l2Genesis
}

func assertStateDataAvailable(t *testing.T, db ethdb.KeyValueStore, l2Genesis *core.Genesis, genesisBlock *types.Block) {
	statedb, err := state.New(genesisBlock.Root(), state.NewDatabase(triedb.NewDatabase(rawdb.NewDatabase(db), nil), nil))
	require.NoError(t, err)

	for address, account := range l2Genesis.Alloc {
		require.Equal(t, uint256.MustFromBig(account.Balance), statedb.GetBalance(address))
		require.Equal(t, account.Nonce, statedb.GetNonce(address))
		require.Equal(t, common.BytesToHash(crypto.Keccak256(account.Code)), statedb.GetCodeHash(address))
		require.Equal(t, account.Code, statedb.GetCode(address))
		for key, value := range account.Storage {
			require.Equal(t, value, statedb.GetState(address, key))
		}
	}
	require.Equal(t, common.Hash{}, statedb.GetState(codeAccount, common.HexToHash("0x99")), "retrieve unset storage key")
	require.Equal(t, common.U2560, statedb.GetBalance(unknownAccount), "unset account balance")
	require.Equal(t, uint64(0), statedb.GetNonce(unknownAccount), "unset account balance")
	require.Nil(t, statedb.GetCode(unknownAccount), "unset account code")
	require.Equal(t, common.Hash{}, statedb.GetCodeHash(unknownAccount), "unset account code hash")
}
