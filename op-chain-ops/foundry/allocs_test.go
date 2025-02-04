package foundry

import (
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/ethereum/go-ethereum/triedb/hashdb"
)

func TestForgeAllocs_FromState(t *testing.T) {
	// Internals of state-dumping of Geth have silent errors.
	cfg := oplog.DefaultCLIConfig()
	cfg.Level = log.LevelTrace
	oplog.SetGlobalLogHandler(oplog.NewLogHandler(os.Stdout, cfg))

	rawDB := rawdb.NewMemoryDatabase()
	stateDB := state.NewDatabase(triedb.NewDatabase(rawDB, &triedb.Config{
		Preimages: true,
		IsVerkle:  false,
		HashDB:    hashdb.Defaults,
		PathDB:    nil,
	}), nil)
	st, err := state.New(types.EmptyRootHash, stateDB)
	require.NoError(t, err)

	alice := common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	st.CreateAccount(alice)
	st.SetBalance(alice, uint256.NewInt(123), tracing.BalanceChangeUnspecified)
	st.SetNonce(alice, 42)

	bob := common.HexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	st.CreateAccount(bob)
	st.CreateContract(bob)
	st.SetBalance(bob, uint256.NewInt(100), tracing.BalanceChangeUnspecified)
	st.SetNonce(bob, 1)
	st.SetState(bob, common.Hash{0: 0x42}, common.Hash{0: 7})

	contract := common.HexToAddress("0xCcCCccccCCCCcCCCCCCcCcCccCcCCCcCcccccccC")
	st.CreateAccount(contract)
	st.CreateContract(contract)
	st.SetNonce(contract, 30)
	st.SetBalance(contract, uint256.NewInt(0), tracing.BalanceChangeUnspecified)
	st.SetCode(contract, []byte{10, 11, 12, 13, 14})

	// Commit and make a new state, we cannot reuse the state after Commit
	// (see doc-comment in Commit, absolute footgun)
	root, err := st.Commit(0, false, false)
	require.NoError(t, err)
	st, err = state.New(root, stateDB)
	require.NoError(t, err)

	st.SetState(contract, common.Hash{0: 0xa}, common.Hash{0: 1})
	st.SetState(contract, crypto.Keccak256Hash([]byte("hello")), crypto.Keccak256Hash([]byte("world")))

	root, err = st.Commit(0, false, false)
	require.NoError(t, err)
	st, err = state.New(root, stateDB)
	require.NoError(t, err)

	var allocs ForgeAllocs
	allocs.FromState(st)

	require.Len(t, allocs.Accounts, 3)

	require.Contains(t, allocs.Accounts, alice)
	require.Nil(t, allocs.Accounts[alice].Code)
	require.Nil(t, allocs.Accounts[alice].Storage)
	require.Equal(t, "123", allocs.Accounts[alice].Balance.String())
	require.Equal(t, uint64(42), allocs.Accounts[alice].Nonce)

	require.Contains(t, allocs.Accounts, bob)
	require.Nil(t, allocs.Accounts[bob].Code)
	require.Len(t, allocs.Accounts[bob].Storage, 1)
	require.Equal(t, common.Hash{0: 7}, allocs.Accounts[bob].Storage[common.Hash{0: 0x42}])
	require.Equal(t, "100", allocs.Accounts[bob].Balance.String())
	require.Equal(t, uint64(1), allocs.Accounts[bob].Nonce)

	require.Contains(t, allocs.Accounts, contract)
	require.Equal(t, []byte{10, 11, 12, 13, 14}, allocs.Accounts[contract].Code)
	require.Len(t, allocs.Accounts[contract].Storage, 2)
	require.Equal(t, common.Hash{0: 1}, allocs.Accounts[contract].Storage[common.Hash{0: 0xa}])
	require.Equal(t, crypto.Keccak256Hash([]byte("world")),
		allocs.Accounts[contract].Storage[crypto.Keccak256Hash([]byte("hello"))])
	require.Equal(t, "0", allocs.Accounts[contract].Balance.String())
	require.Equal(t, uint64(30), allocs.Accounts[contract].Nonce)
}

func TestForgeAllocs_Marshaling(t *testing.T) {
	data := &ForgeAllocs{
		Accounts: map[common.Address]types.Account{
			common.HexToAddress("0x12345"): {
				Balance: big.NewInt(12345),
				Code:    []byte{0x01, 0x02, 0x03},
				Nonce:   123,
				Storage: map[common.Hash]common.Hash{
					common.HexToHash("0x12345"): common.HexToHash("0x12345"),
				},
			},
		},
	}

	out, err := json.Marshal(data)
	require.NoError(t, err)

	var in ForgeAllocs
	require.NoError(t, json.Unmarshal(out, &in))
}
