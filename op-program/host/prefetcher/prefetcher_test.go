package prefetcher

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	hostTypes "github.com/ethereum-optimism/optimism/op-program/host/types"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/require"

	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum-optimism/optimism/op-program/client/l1"
	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	"github.com/ethereum-optimism/optimism/op-program/client/mpt"
	hostcommon "github.com/ethereum-optimism/optimism/op-program/host/common"
	"github.com/ethereum-optimism/optimism/op-program/host/kvstore"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

var (
	ecRecoverInput    = common.FromHex("18c547e4f7b0f325ad1e56f57e26c745b09a3e503d86e00e5255ff7f715d3d1c000000000000000000000000000000000000000000000000000000000000001c73b1693892219d736caba55bdb67216e485557ea6b6af75f37096c9aa6a5a75feeb940b1d03b21e36b0e47e79769f095fe2ab855bd91e3a38756b7d75a9c4549")
	kzgPointEvalInput = common.FromHex("01e798154708fe7789429634053cbf9f99b619f9f084048927333fce637f549b564c0a11a0f704f4fc3e8acfe0f8245f0ad1347b378fbf96e206da11a5d3630624d25032e67a7e6a4910df5834b8fe70e6bcfeeac0352434196bdf4b2485d5a18f59a8d2a1a625a17f3fea0fe5eb8c896db3764f3185481bc22f91b4aaffcca25f26936857bc3a7c2539ea8ec3a952b7873033e038326e87ed3e1276fd140253fa08e9fc25fb2d9a98527fc22a2c9612fbeafdad446cbc7bcdbdcd780af2c16a")
	defaultChainID    = eth.ChainIDFromUInt64(14)
)

func TestNoHint(t *testing.T) {
	t.Run("NotFound", func(t *testing.T) {
		prefetcher, _, _, _, _ := createPrefetcher(t)
		res, err := prefetcher.GetPreimage(context.Background(), common.Hash{0xab})
		require.ErrorIs(t, err, kvstore.ErrNotFound)
		require.Nil(t, res)
	})

	t.Run("Exists", func(t *testing.T) {
		prefetcher, _, _, _, kv := createPrefetcher(t)
		data := []byte{1, 2, 3}
		hash := crypto.Keccak256Hash(data)
		require.NoError(t, kv.Put(hash, data))

		res, err := prefetcher.GetPreimage(context.Background(), hash)
		require.NoError(t, err)
		require.Equal(t, res, data)
	})
}

func TestFetchL1BlockHeader(t *testing.T) {
	rng := rand.New(rand.NewSource(123))
	block, rcpts := testutils.RandomBlock(rng, 2)
	hash := block.Hash()
	key := preimage.Keccak256Key(hash).PreimageKey()
	pre, err := rlp.EncodeToBytes(block.Header())
	require.NoError(t, err)

	t.Run("AlreadyKnown", func(t *testing.T) {
		prefetcher, _, _, _, kv := createPrefetcher(t)
		storeBlock(t, kv, block, rcpts)

		oracle := l1.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher))
		result := oracle.HeaderByBlockHash(hash)
		require.Equal(t, eth.HeaderBlockInfo(block.Header()), result)
	})

	t.Run("Unknown", func(t *testing.T) {
		prefetcher, l1Cl, _, _, _ := createPrefetcher(t)
		l1Cl.ExpectInfoByHash(hash, eth.HeaderBlockInfo(block.Header()), nil)
		defer l1Cl.AssertExpectations(t)

		require.NoError(t, prefetcher.Hint(l1.BlockHeaderHint(hash).Hint()))
		result, err := prefetcher.GetPreimage(context.Background(), key)
		require.NoError(t, err)
		require.Equal(t, pre, result)
	})
}

func TestFetchL1Transactions(t *testing.T) {
	rng := rand.New(rand.NewSource(123))
	block, rcpts := testutils.RandomBlock(rng, 10)
	hash := block.Hash()

	t.Run("AlreadyKnown", func(t *testing.T) {
		prefetcher, _, _, _, kv := createPrefetcher(t)

		storeBlock(t, kv, block, rcpts)

		// Check the data is available (note the oracle does not know about the block, only the kvstore does)
		oracle := l1.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher))
		header, txs := oracle.TransactionsByBlockHash(hash)
		require.EqualValues(t, hash, header.Hash())
		assertTransactionsEqual(t, block.Transactions(), txs)
	})

	t.Run("Unknown", func(t *testing.T) {
		prefetcher, l1Cl, _, _, _ := createPrefetcher(t)
		l1Cl.ExpectInfoByHash(hash, eth.BlockToInfo(block), nil)
		l1Cl.ExpectInfoAndTxsByHash(hash, eth.BlockToInfo(block), block.Transactions(), nil)
		defer l1Cl.AssertExpectations(t)

		oracle := l1.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher))
		header, txs := oracle.TransactionsByBlockHash(hash)
		require.EqualValues(t, hash, header.Hash())
		assertTransactionsEqual(t, block.Transactions(), txs)
	})
}

func TestFetchL1Receipts(t *testing.T) {
	rng := rand.New(rand.NewSource(123))
	block, receipts := testutils.RandomBlock(rng, 10)
	hash := block.Hash()

	t.Run("AlreadyKnown", func(t *testing.T) {
		prefetcher, _, _, _, kv := createPrefetcher(t)
		storeBlock(t, kv, block, receipts)

		// Check the data is available (note the oracle does not know about the block, only the kvstore does)
		oracle := l1.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher))
		header, actualReceipts := oracle.ReceiptsByBlockHash(hash)
		require.EqualValues(t, hash, header.Hash())
		assertReceiptsEqual(t, receipts, actualReceipts)
	})

	t.Run("Unknown", func(t *testing.T) {
		prefetcher, l1Cl, _, _, _ := createPrefetcher(t)
		l1Cl.ExpectInfoByHash(hash, eth.BlockToInfo(block), nil)
		l1Cl.ExpectInfoAndTxsByHash(hash, eth.BlockToInfo(block), block.Transactions(), nil)
		l1Cl.ExpectFetchReceipts(hash, eth.BlockToInfo(block), receipts, nil)
		defer l1Cl.AssertExpectations(t)

		oracle := l1.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher))
		header, actualReceipts := oracle.ReceiptsByBlockHash(hash)
		require.EqualValues(t, hash, header.Hash())
		assertReceiptsEqual(t, receipts, actualReceipts)
	})

	// Blocks may have identical RLP receipts for different transactions.
	// Check that the node already existing is handled
	t.Run("CommonTrieNodes", func(t *testing.T) {
		prefetcher, l1Cl, _, _, kv := createPrefetcher(t)
		l1Cl.ExpectInfoByHash(hash, eth.BlockToInfo(block), nil)
		l1Cl.ExpectInfoAndTxsByHash(hash, eth.BlockToInfo(block), block.Transactions(), nil)
		l1Cl.ExpectFetchReceipts(hash, eth.BlockToInfo(block), receipts, nil)
		defer l1Cl.AssertExpectations(t)

		// Pre-store one receipt node (but not the whole trie leading to it)
		// This would happen if an identical receipt was in an earlier block
		opaqueRcpts, err := eth.EncodeReceipts(receipts)
		require.NoError(t, err)
		_, nodes := mpt.WriteTrie(opaqueRcpts)
		require.NoError(t, kv.Put(preimage.Keccak256Key(crypto.Keccak256Hash(nodes[0])).PreimageKey(), nodes[0]))

		oracle := l1.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher))
		header, actualReceipts := oracle.ReceiptsByBlockHash(hash)
		require.EqualValues(t, hash, header.Hash())
		assertReceiptsEqual(t, receipts, actualReceipts)
	})
}

func GetRandBlob(t *testing.T, seed int64) eth.Blob {
	r := rand.New(rand.NewSource(seed))
	bigData := eth.Data(make([]byte, eth.MaxBlobDataSize))
	for i := range bigData {
		bigData[i] = byte(r.Intn(256))
	}
	var b eth.Blob
	err := b.FromData(bigData)
	require.NoError(t, err)
	return b
}

func TestFetchL1Blob(t *testing.T) {
	blob := GetRandBlob(t, 0xf00f00)
	commitment, err := blob.ComputeKZGCommitment()
	require.NoError(t, err)
	versionedHash := eth.KZGToVersionedHash(commitment)
	blobHash := eth.IndexedBlobHash{Hash: versionedHash, Index: 0xFACADE}
	l1Ref := eth.L1BlockRef{Time: 0}

	t.Run("AlreadyKnown", func(t *testing.T) {
		prefetcher, _, blobFetcher, _, kv := createPrefetcher(t)
		storeBlob(t, kv, (eth.Bytes48)(commitment), &blob)

		oracle := l1.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher))
		defer blobFetcher.AssertExpectations(t)

		blobs := oracle.GetBlob(l1Ref, blobHash)
		require.EqualValues(t, blobs[:], blob[:])
	})

	t.Run("Unknown", func(t *testing.T) {
		prefetcher, _, blobFetcher, _, _ := createPrefetcher(t)

		oracle := l1.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher))
		blobFetcher.ExpectOnGetBlobSidecars(
			context.Background(),
			l1Ref,
			[]eth.IndexedBlobHash{blobHash},
			(eth.Bytes48)(commitment),
			[]*eth.Blob{&blob},
			nil,
		)
		defer blobFetcher.AssertExpectations(t)

		blobs := oracle.GetBlob(l1Ref, blobHash)
		require.EqualValues(t, blobs[:], blob[:])

		// Check that the preimages of field element keys are also stored
		// This makes it possible for the challenger to extract the commitment and required field from the
		// oracle key rather than needing the hint data.

		fieldElemKey := make([]byte, 80)
		copy(fieldElemKey[:48], commitment[:])
		for i := 0; i < params.BlobTxFieldElementsPerBlob; i++ {
			binary.BigEndian.PutUint64(fieldElemKey[72:], uint64(i))
			key := preimage.Keccak256Key(crypto.Keccak256(fieldElemKey)).PreimageKey()
			actual, err := prefetcher.kvStore.Get(key)
			require.NoError(t, err)
			require.Equal(t, fieldElemKey, actual)
		}
	})
}

func TestFetchPrecompileResult(t *testing.T) {
	failure := []byte{0}
	success := []byte{1}

	tests := []struct {
		name   string
		addr   common.Address
		input  []byte
		result []byte
	}{
		{
			name:   "EcRecover-Valid",
			addr:   common.BytesToAddress([]byte{0x1}),
			input:  ecRecoverInput,
			result: append(success, common.FromHex("000000000000000000000000a94f5374fce5edbc8e2a8697c15331677e6ebf0b")...),
		},
		{
			name:   "KzgPointEvaluation-Valid",
			addr:   common.BytesToAddress([]byte{0xa}),
			input:  kzgPointEvalInput,
			result: append(success, common.FromHex("000000000000000000000000000000000000000000000000000000000000100073eda753299d7d483339d80809a1d80553bda402fffe5bfeffffffff00000001")...),
		},
		{
			name:   "KzgPointEvaluation-Invalid",
			addr:   common.BytesToAddress([]byte{0xa}),
			input:  []byte{0x0},
			result: failure,
		},
		{
			name:   "Bn256Pairing-Valid",
			addr:   common.BytesToAddress([]byte{0x8}),
			input:  []byte{}, // empty is valid
			result: append(success, common.FromHex("0000000000000000000000000000000000000000000000000000000000000001")...),
		},
		{
			name:   "Bn256Pairing-Invalid",
			addr:   common.BytesToAddress([]byte{0x8}),
			input:  []byte{0x1},
			result: failure,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			prefetcher, _, _, _, _ := createPrefetcher(t)
			oracle := newLegacyPrecompileOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher))

			result, ok := oracle.Precompile(test.addr, test.input)
			require.Equal(t, test.result[0] == 1, ok)
			require.EqualValues(t, test.result[1:], result)

			key := crypto.Keccak256Hash(append(test.addr.Bytes(), test.input...))
			val, err := prefetcher.kvStore.Get(preimage.Keccak256Key(key).PreimageKey())
			require.NoError(t, err)
			require.NotEmpty(t, val)

			val, err = prefetcher.kvStore.Get(preimage.PrecompileKey(key).PreimageKey())
			require.NoError(t, err)
			require.EqualValues(t, test.result, val)
		})
	}

	t.Run("Already Known", func(t *testing.T) {
		input := []byte("test input")
		addr := common.BytesToAddress([]byte{0x1})
		result := []byte{0x1}
		prefetcher, _, _, _, kv := createPrefetcher(t)
		err := kv.Put(preimage.PrecompileKey(crypto.Keccak256Hash(append(addr.Bytes(), input...))).PreimageKey(), append([]byte{1}, result...))
		require.NoError(t, err)

		oracle := newLegacyPrecompileOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher))
		actualResult, status := oracle.Precompile(addr, input)
		require.EqualValues(t, result, actualResult)
		require.True(t, status)
	})
}

func TestFetchPrecompileResultV2(t *testing.T) {
	failure := []byte{0}
	success := []byte{1}

	tests := []struct {
		name        string
		addr        common.Address
		input       []byte
		requiredGas uint64
		result      []byte
	}{
		{
			name:        "EcRecover-Valid",
			addr:        common.BytesToAddress([]byte{0x1}),
			input:       ecRecoverInput,
			requiredGas: 3000,
			result:      append(success, common.FromHex("000000000000000000000000a94f5374fce5edbc8e2a8697c15331677e6ebf0b")...),
		},
		{
			name:        "Bn256Pairing-Valid",
			addr:        common.BytesToAddress([]byte{0x8}),
			input:       []byte{}, // empty is valid
			requiredGas: 6000,
			result:      append(success, common.FromHex("0000000000000000000000000000000000000000000000000000000000000001")...),
		},
		{
			name:        "Bn256Pairing-Invalid",
			addr:        common.BytesToAddress([]byte{0x8}),
			input:       []byte{0x1},
			requiredGas: 6000,
			result:      failure,
		},
		{
			name:        "KzgPointEvaluation-Valid",
			addr:        common.BytesToAddress([]byte{0xa}),
			input:       kzgPointEvalInput,
			requiredGas: 50_000,
			result:      append(success, common.FromHex("000000000000000000000000000000000000000000000000000000000000100073eda753299d7d483339d80809a1d80553bda402fffe5bfeffffffff00000001")...),
		},
		{
			name:        "KzgPointEvaluation-Invalid",
			addr:        common.BytesToAddress([]byte{0xa}),
			input:       []byte{0x0},
			requiredGas: 50_000,
			result:      failure,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			prefetcher, _, _, _, _ := createPrefetcher(t)
			oracle := l1.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher))

			result, ok := oracle.Precompile(test.addr, test.input, test.requiredGas)
			require.Equal(t, test.result[0] == 1, ok)
			require.EqualValues(t, test.result[1:], result)

			key := crypto.Keccak256Hash(append(append(test.addr.Bytes(), binary.BigEndian.AppendUint64(nil, test.requiredGas)...), test.input...))
			val, err := prefetcher.kvStore.Get(preimage.Keccak256Key(key).PreimageKey())
			require.NoError(t, err)
			require.NotEmpty(t, val)

			val, err = prefetcher.kvStore.Get(preimage.PrecompileKey(key).PreimageKey())
			require.NoError(t, err)
			require.EqualValues(t, test.result, val)
		})
	}

	t.Run("Already Known", func(t *testing.T) {
		input := []byte("test input")
		requiredGas := uint64(3000)
		addr := common.BytesToAddress([]byte{0x1})
		result := []byte{0x1}
		prefetcher, _, _, _, kv := createPrefetcher(t)
		keyArg := append(addr.Bytes(), binary.BigEndian.AppendUint64(nil, requiredGas)...)
		keyArg = append(keyArg, input...)
		err := kv.Put(preimage.PrecompileKey(crypto.Keccak256Hash(keyArg)).PreimageKey(), append([]byte{1}, result...))
		require.NoError(t, err)

		oracle := l1.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher))
		actualResult, status := oracle.Precompile(addr, input, requiredGas)
		require.EqualValues(t, actualResult, result)
		require.True(t, status)
	})
}

func TestUnsupportedPrecompile(t *testing.T) {
	prefetcher, _, _, _, _ := createPrefetcher(t)
	oracleFn := func(t *testing.T, prefetcher *Prefetcher) preimage.OracleFn {
		return func(key preimage.Key) []byte {
			_, err := prefetcher.GetPreimage(context.Background(), key.PreimageKey())
			require.ErrorContains(t, err, "unsupported precompile address")
			return []byte{1}
		}
	}
	oracle := newLegacyPrecompileOracle(oracleFn(t, prefetcher), asHinter(t, prefetcher))
	oracle.Precompile(common.HexToAddress("0xdead"), nil)
}

func TestRestrictedPrecompileContracts(t *testing.T) {
	for _, addr := range acceleratedPrecompiles {
		require.NotNil(t, getPrecompiledContract(addr))
	}
}

func TestFetchL2Block(t *testing.T) {
	rng := rand.New(rand.NewSource(123))
	chainID := eth.ChainIDFromUInt64(482948)
	block, rcpts := testutils.RandomBlock(rng, 10)
	hash := block.Hash()

	t.Run("AlreadyKnown", func(t *testing.T) {
		prefetcher, _, _, _, kv := createPrefetcher(t)
		storeBlock(t, kv, block, rcpts)

		oracle := l2.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher), false)
		result := oracle.BlockByHash(hash, chainID)
		require.EqualValues(t, block.Header(), result.Header())
		assertTransactionsEqual(t, block.Transactions(), result.Transactions())
	})

	t.Run("Unknown", func(t *testing.T) {
		prefetcher, _, _, l2Cls, _ := createPrefetcher(t)
		l2Cl := l2Cls.sources[defaultChainID]
		l2Cl.ExpectInfoAndTxsByHash(hash, eth.BlockToInfo(block), block.Transactions(), nil)
		defer l2Cl.MockL2Client.AssertExpectations(t)

		oracle := l2.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher), false)
		result := oracle.BlockByHash(hash, chainID)
		require.EqualValues(t, block.Header(), result.Header())
		assertTransactionsEqual(t, block.Transactions(), result.Transactions())
	})

	t.Run("WithChainID", func(t *testing.T) {
		prefetcher, _, _, l2Cls, _ := createPrefetcher(t, eth.ChainIDFromUInt64(5), eth.ChainIDFromUInt64(7), eth.ChainIDFromUInt64(10))
		l2Cl := l2Cls.sources[eth.ChainIDFromUInt64(7)]
		l2Cl.ExpectInfoAndTxsByHash(hash, eth.BlockToInfo(block), block.Transactions(), nil)
		defer assertAllClientExpectations(t, l2Cls)

		oracle := l2.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher), true)
		result := oracle.BlockByHash(hash, eth.ChainIDFromUInt64(7))
		require.EqualValues(t, block.Header(), result.Header())
		assertTransactionsEqual(t, block.Transactions(), result.Transactions())
	})
}

func TestFetchL2Transactions(t *testing.T) {
	rng := rand.New(rand.NewSource(123))
	block, rcpts := testutils.RandomBlock(rng, 10)
	hash := block.Hash()
	chainID := eth.ChainIDFromUInt64(rng.Uint64())

	t.Run("AlreadyKnown", func(t *testing.T) {
		prefetcher, _, _, _, kv := createPrefetcher(t)
		storeBlock(t, kv, block, rcpts)

		oracle := l2.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher), false)
		result := oracle.LoadTransactions(hash, block.TxHash(), chainID)
		assertTransactionsEqual(t, block.Transactions(), result)
	})

	t.Run("Unknown", func(t *testing.T) {
		prefetcher, _, _, l2Cls, _ := createPrefetcher(t)
		l2Cl := l2Cls.sources[defaultChainID]
		l2Cl.ExpectInfoAndTxsByHash(hash, eth.BlockToInfo(block), block.Transactions(), nil)
		defer l2Cl.MockL2Client.AssertExpectations(t)

		oracle := l2.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher), false)
		result := oracle.LoadTransactions(hash, block.TxHash(), chainID)
		assertTransactionsEqual(t, block.Transactions(), result)
	})

	t.Run("WithChainID", func(t *testing.T) {
		prefetcher, _, _, l2Cls, _ := createPrefetcher(t, eth.ChainIDFromUInt64(5), eth.ChainIDFromUInt64(7), eth.ChainIDFromUInt64(10))
		l2Cl := l2Cls.sources[eth.ChainIDFromUInt64(7)]
		l2Cl.ExpectInfoAndTxsByHash(hash, eth.BlockToInfo(block), block.Transactions(), nil)
		defer assertAllClientExpectations(t, l2Cls)

		oracle := l2.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher), true)
		result := oracle.LoadTransactions(hash, block.TxHash(), eth.ChainIDFromUInt64(7))
		assertTransactionsEqual(t, block.Transactions(), result)
	})
}

func TestFetchL2Node(t *testing.T) {
	rng := rand.New(rand.NewSource(123))
	node := testutils.RandomData(rng, 30)
	hash := crypto.Keccak256Hash(node)
	key := preimage.Keccak256Key(hash).PreimageKey()
	chainID := eth.ChainIDFromUInt64(rng.Uint64())

	t.Run("AlreadyKnown", func(t *testing.T) {
		prefetcher, _, _, _, kv := createPrefetcher(t)
		require.NoError(t, kv.Put(key, node))

		oracle := l2.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher), false)
		result := oracle.NodeByHash(hash, chainID)
		require.EqualValues(t, node, result)
	})

	t.Run("Unknown", func(t *testing.T) {
		prefetcher, _, _, l2Cls, _ := createPrefetcher(t)
		l2Cl := l2Cls.sources[defaultChainID]
		l2Cl.ExpectNodeByHash(hash, node, nil)
		defer assertAllClientExpectations(t, l2Cls)

		oracle := l2.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher), false)
		result := oracle.NodeByHash(hash, chainID)
		require.EqualValues(t, node, result)
	})

	t.Run("WithChainID", func(t *testing.T) {
		prefetcher, _, _, l2Cls, _ := createPrefetcher(t, eth.ChainIDFromUInt64(5), eth.ChainIDFromUInt64(9), eth.ChainIDFromUInt64(99))
		l2Cl := l2Cls.sources[eth.ChainIDFromUInt64(9)]
		l2Cl.ExpectNodeByHash(hash, node, nil)
		defer assertAllClientExpectations(t, l2Cls)

		oracle := l2.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher), true)
		result := oracle.NodeByHash(hash, eth.ChainIDFromUInt64(9))
		require.EqualValues(t, node, result)
	})
}

func TestFetchL2Code(t *testing.T) {
	rng := rand.New(rand.NewSource(123))
	code := testutils.RandomData(rng, 30)
	hash := crypto.Keccak256Hash(code)
	key := preimage.Keccak256Key(hash).PreimageKey()
	chainID := eth.ChainIDFromUInt64(rng.Uint64())

	t.Run("AlreadyKnown", func(t *testing.T) {
		prefetcher, _, _, _, kv := createPrefetcher(t)
		require.NoError(t, kv.Put(key, code))

		oracle := l2.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher), false)
		result := oracle.CodeByHash(hash, chainID)
		require.EqualValues(t, code, result)
	})

	t.Run("Unknown", func(t *testing.T) {
		prefetcher, _, _, l2Cls, _ := createPrefetcher(t)
		l2Cl := l2Cls.sources[defaultChainID]
		l2Cl.ExpectCodeByHash(hash, code, nil)
		defer l2Cl.MockDebugClient.AssertExpectations(t)

		oracle := l2.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher), false)
		result := oracle.CodeByHash(hash, chainID)
		require.EqualValues(t, code, result)
	})

	t.Run("WithChainID", func(t *testing.T) {
		prefetcher, _, _, l2Cls, _ := createPrefetcher(t, eth.ChainIDFromUInt64(8), eth.ChainIDFromUInt64(45), eth.ChainIDFromUInt64(98), eth.ChainIDFromUInt64(55))
		l2Cl := l2Cls.sources[eth.ChainIDFromUInt64(98)]
		l2Cl.ExpectCodeByHash(hash, code, nil)
		defer assertAllClientExpectations(t, l2Cls)

		oracle := l2.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher), true)
		result := oracle.CodeByHash(hash, eth.ChainIDFromUInt64(98))
		require.EqualValues(t, code, result)
	})
}

func TestFetchL2Output(t *testing.T) {
	rng := rand.New(rand.NewSource(123))
	output := testutils.RandomOutputV0(rng)
	hash := common.Hash(eth.OutputRoot(output))
	key := preimage.Keccak256Key(hash).PreimageKey()
	t.Run("AlreadyKnown", func(t *testing.T) {
		prefetcher, _, _, _, kv := createPrefetcher(t)
		require.NoError(t, kv.Put(key, output.Marshal()))

		oracle := l2.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher), false)
		result := oracle.OutputByRoot(hash, eth.ChainIDFromUInt64(rng.Uint64()))
		require.EqualValues(t, output, result)
	})

	t.Run("Unknown", func(t *testing.T) {
		prefetcher, _, _, l2Cls, _ := createPrefetcher(t)

		l2Cl := l2Cls.sources[defaultChainID]
		l2Cl.ExpectOutputByRoot(prefetcher.l2Head, output, nil)
		defer assertAllClientExpectations(t, l2Cls)
		oracle := l2.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher), false)
		result := oracle.OutputByRoot(hash, eth.ChainIDFromUInt64(rng.Uint64()))
		require.EqualValues(t, output, result)
	})

	t.Run("WithChainID", func(t *testing.T) {
		chain6Output := testutils.RandomOutputV0(rng)
		chain99Output := testutils.RandomOutputV0(rng)
		timestamp := uint64(4567882)
		superV1 := eth.SuperV1{
			Timestamp: timestamp,
			Chains: []eth.ChainIDAndOutput{
				{ChainID: eth.ChainIDFromUInt64(6), Output: eth.OutputRoot(chain6Output)},
				{ChainID: eth.ChainIDFromUInt64(78), Output: eth.OutputRoot(output)},
				{ChainID: eth.ChainIDFromUInt64(99), Output: eth.OutputRoot(chain99Output)},
			},
		}
		prefetcher, _, _, l2Cls, _ := createPrefetcherWithAgreedPrestate(t, superV1.Marshal(), eth.ChainIDFromUInt64(6), eth.ChainIDFromUInt64(78), eth.ChainIDFromUInt64(99))

		l2Cl := l2Cls.sources[eth.ChainIDFromUInt64(78)]
		blockNum, err := l2Cls.sources[eth.ChainIDFromUInt64(78)].RollupConfig().TargetBlockNumber(timestamp)
		require.NoError(t, err)
		l2Cl.ExpectOutputByNumber(blockNum, output, nil)
		defer assertAllClientExpectations(t, l2Cls)
		oracle := l2.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher), true)
		result := oracle.OutputByRoot(hash, eth.ChainIDFromUInt64(78))
		require.EqualValues(t, output, result)
	})
}

func TestFetchL2BlockData(t *testing.T) {
	chainID := eth.ChainIDFromUInt64(14)

	testBlockExec := func(t *testing.T, clientErrs []error) {
		require.NotEmpty(t, clientErrs)
		prefetcher, _, _, l2Clients, _ := createPrefetcher(t)
		l2Client := l2Clients.sources[defaultChainID]
		rng := rand.New(rand.NewSource(123))
		block, _ := testutils.RandomBlock(rng, 10)
		disputedBlock, _ := testutils.RandomBlock(rng, 10)

		isCanonical := clientErrs[len(clientErrs)-1] == nil

		for _, clientErr := range clientErrs {
			l2Client.ExpectInfoAndTxsByHash(disputedBlock.Hash(), eth.BlockToInfo(nil), nil, clientErr)
		}
		if !isCanonical {
			l2Client.ExpectInfoAndTxsByHash(block.Hash(), eth.BlockToInfo(block), block.Transactions(), nil)
		}

		defer l2Client.MockDebugClient.AssertExpectations(t)
		prefetcher.executor = &mockExecutor{}
		hint := l2.L2BlockDataHint{
			AgreedBlockHash: block.Hash(),
			BlockHash:       disputedBlock.Hash(),
			ChainID:         chainID,
		}.Hint()

		if !isCanonical {
			// Simulate program execution by writing block preimage to the kv store
			disputedBlockRLP, err := rlp.EncodeToBytes(disputedBlock.Header())
			require.NoError(t, err)
			err = prefetcher.kvStore.Put(preimage.Keccak256Key(disputedBlock.Hash()).PreimageKey(), disputedBlockRLP)
			require.NoError(t, err)
		}

		require.NoError(t, prefetcher.Hint(hint))
		if isCanonical {
			require.False(t, prefetcher.executor.(*mockExecutor).invoked)
		} else {
			require.True(t, prefetcher.executor.(*mockExecutor).invoked)
			require.Equal(t, prefetcher.executor.(*mockExecutor).blockNumber, block.NumberU64()+1)
			require.Equal(t, prefetcher.executor.(*mockExecutor).chainID, chainID)
		}

		data, err := prefetcher.kvStore.Get(BlockDataKey(disputedBlock.Hash()).Key())
		require.NoError(t, err)
		require.Equal(t, data, []byte{1})

		// ensure executor isn't used on a cache hit
		prefetcher.executor.(*mockExecutor).invoked = false
		require.NoError(t, prefetcher.Hint(hint))
		require.False(t, prefetcher.executor.(*mockExecutor).invoked)
	}
	t.Run("exec block is canonical", func(t *testing.T) {
		testBlockExec(t, []error{nil})
	})
	t.Run("exec block is canonical with errors", func(t *testing.T) {
		testBlockExec(t, []error{errors.New("fetch error"), nil})
	})
	t.Run("exec block is not canonical", func(t *testing.T) {
		testBlockExec(t, []error{ethereum.NotFound})
	})
	t.Run("exec block is not canonical with fetch error", func(t *testing.T) {
		testBlockExec(t, []error{errors.New("fetch error"), ethereum.NotFound})
	})

	t.Run("no exec", func(t *testing.T) {
		prefetcher, _, _, _, _ := createPrefetcher(t)
		hint := l2.L2BlockDataHint{
			AgreedBlockHash: common.Hash{0xaa},
			BlockHash:       common.Hash{0xab},
			ChainID:         chainID,
		}.Hint()
		err := prefetcher.Hint(hint)
		require.ErrorContains(t, err, "this prefetcher does not support native block execution")
	})
}

func TestFetchAgreedPrestate(t *testing.T) {
	t.Run("unavailable", func(t *testing.T) {
		prefetcher, _, _, _, _ := createPrefetcher(t)
		hash := common.Hash{0xaa}
		hint := l2.AgreedPrestateHint(hash).Hint()
		require.NoError(t, prefetcher.Hint(hint))
		_, err := prefetcher.GetPreimage(context.Background(), hash)
		require.ErrorIs(t, err, ErrAgreedPrestateUnavailable)
	})

	t.Run("available", func(t *testing.T) {
		prestate := []byte{1, 2, 3, 6}
		prefetcher, _, _, _, _ := createPrefetcherWithAgreedPrestate(t, prestate)
		hash := crypto.Keccak256Hash(prestate)
		hint := l2.AgreedPrestateHint(hash).Hint()
		require.NoError(t, prefetcher.Hint(hint))
		actual, err := prefetcher.GetPreimage(context.Background(), preimage.Keccak256Key(hash).PreimageKey())
		require.NoError(t, err)
		require.Equal(t, prestate, actual)
	})
}

func TestBadHints(t *testing.T) {
	prefetcher, _, _, _, kv := createPrefetcher(t)
	hash := common.Hash{0xad}

	t.Run("NoSpace", func(t *testing.T) {
		// Accept the hint
		require.NoError(t, prefetcher.Hint(l1.HintL1BlockHeader))

		// But it will fail to prefetch when the pre-image isn't available
		pre, err := prefetcher.GetPreimage(context.Background(), hash)
		require.ErrorContains(t, err, "unsupported hint")
		require.Nil(t, pre)
	})

	t.Run("InvalidHash", func(t *testing.T) {
		// Accept the hint
		require.NoError(t, prefetcher.Hint(l1.HintL1BlockHeader+" asdfsadf"))

		// But it will fail to prefetch when the pre-image isn't available
		pre, err := prefetcher.GetPreimage(context.Background(), hash)
		require.ErrorContains(t, err, "invalid bytes")
		require.Nil(t, pre)
	})

	t.Run("UnknownType", func(t *testing.T) {
		// Accept the hint
		require.NoError(t, prefetcher.Hint("unknown "+hash.Hex()))

		// But it will fail to prefetch when the pre-image isn't available
		pre, err := prefetcher.GetPreimage(context.Background(), hash)
		require.ErrorContains(t, err, "unknown hint type")
		require.Nil(t, pre)
	})

	// Should not return hint errors if the preimage is already available
	t.Run("KeyExists", func(t *testing.T) {
		// Prepopulate the requested preimage
		value := []byte{1, 2, 3, 4}
		require.NoError(t, kv.Put(hash, value))

		// Hint is invalid
		require.NoError(t, prefetcher.Hint("asdfsadf"))
		// But fetching the key fails because prefetching isn't required
		pre, err := prefetcher.GetPreimage(context.Background(), hash)
		require.NoError(t, err)
		require.Equal(t, value, pre)
	})
}

func TestRetryWhenNotAvailableAfterPrefetching(t *testing.T) {
	rng := rand.New(rand.NewSource(123))
	node := testutils.RandomData(rng, 30)
	hash := crypto.Keccak256Hash(node)
	chainID := eth.ChainIDFromUInt64(rng.Uint64())

	_, l1Source, l1BlobSource, l2Cls, kv := createPrefetcher(t)
	putsToIgnore := 2
	kv = &unreliableKvStore{KV: kv, putsToIgnore: putsToIgnore}
	sources := &l2Clients{sources: map[eth.ChainID]*l2Client{eth.ChainIDFromUInt64(6): l2Cls.sources[defaultChainID]}}
	prefetcher := NewPrefetcher(testlog.Logger(t, log.LevelInfo), l1Source, l1BlobSource, eth.ChainIDFromUInt64(6), sources, kv, nil, common.Hash{}, nil)

	l2Cl := sources.sources[eth.ChainIDFromUInt64(6)]
	// Expect one call for each ignored put, plus one more request for when the put succeeds
	for i := 0; i < putsToIgnore+1; i++ {
		l2Cl.ExpectNodeByHash(hash, node, nil)
	}
	defer l2Cl.MockDebugClient.AssertExpectations(t)

	oracle := l2.NewPreimageOracle(asOracleFn(t, prefetcher), asHinter(t, prefetcher), false)
	result := oracle.NodeByHash(hash, chainID)
	require.EqualValues(t, node, result)
}

type unreliableKvStore struct {
	kvstore.KV
	putsToIgnore int
}

func (s *unreliableKvStore) Put(k common.Hash, v []byte) error {
	if s.putsToIgnore > 0 {
		s.putsToIgnore--
		return nil
	}
	println("storing")
	return s.KV.Put(k, v)
}

type l2Clients struct {
	sources map[eth.ChainID]*l2Client
}

func (l *l2Clients) ForChainID(id eth.ChainID) (hostTypes.L2Source, error) {
	source, ok := l.sources[id]
	if !ok {
		return nil, fmt.Errorf("no such source for chain %d", id)
	}
	return source, nil
}

func (l *l2Clients) ForChainIDWithoutRetries(id eth.ChainID) (hostTypes.L2Source, error) {
	return l.ForChainID(id)
}

type l2Client struct {
	*testutils.MockL2Client
	*testutils.MockDebugClient
	rollupCfg *rollup.Config
}

func (m *l2Client) RollupConfig() *rollup.Config {
	return m.rollupCfg
}

func (m *l2Client) ExperimentalEnabled() bool {
	panic("implement me")
}

func (m *l2Client) OutputByRoot(ctx context.Context, blockHash common.Hash) (eth.Output, error) {
	out := m.Mock.MethodCalled("OutputByRoot", blockHash)
	return out[0].(eth.Output), *out[1].(*error)
}

func (m *l2Client) ExpectOutputByRoot(blockRoot common.Hash, output eth.Output, err error) {
	m.Mock.On("OutputByRoot", blockRoot).Once().Return(output, &err)
}

func (m *l2Client) OutputByNumber(ctx context.Context, blockNum uint64) (eth.Output, error) {
	out := m.Mock.MethodCalled("OutputByNumber", blockNum)
	return out[0].(eth.Output), *out[1].(*error)
}

func (m *l2Client) ExpectOutputByNumber(blockNum uint64, output eth.Output, err error) {
	m.Mock.On("OutputByNumber", blockNum).Once().Return(output, &err)
}

func createPrefetcher(t *testing.T, chainIDs ...eth.ChainID) (*Prefetcher, *testutils.MockL1Source, *testutils.MockBlobsFetcher, *l2Clients, kvstore.KV) {
	return createPrefetcherWithAgreedPrestate(t, nil, chainIDs...)
}

func createPrefetcherWithAgreedPrestate(t *testing.T, agreedPrestate []byte, chainIDs ...eth.ChainID) (*Prefetcher, *testutils.MockL1Source, *testutils.MockBlobsFetcher, *l2Clients, kvstore.KV) {
	logger := testlog.Logger(t, log.LevelDebug)
	kv := kvstore.NewMemKV()

	l1Source := new(testutils.MockL1Source)
	l1BlobSource := new(testutils.MockBlobsFetcher)

	// Provide a default chain if none specified.
	if len(chainIDs) == 0 {
		chainIDs = []eth.ChainID{defaultChainID}
	}

	l2Sources := &l2Clients{sources: make(map[eth.ChainID]*l2Client)}
	for i, chainID := range chainIDs {
		l2Source := &l2Client{
			rollupCfg: &rollup.Config{
				// Make the block numbers for each chain differ at each timestamp
				Genesis:   rollup.Genesis{L2Time: 500 + uint64(2*i)},
				BlockTime: 1,
			},
			MockL2Client:    new(testutils.MockL2Client),
			MockDebugClient: new(testutils.MockDebugClient),
		}
		l2Sources.sources[chainID] = l2Source
	}

	prefetcher := NewPrefetcher(logger, l1Source, l1BlobSource, chainIDs[0], l2Sources, kv, nil, common.Hash{0xdd}, agreedPrestate)
	return prefetcher, l1Source, l1BlobSource, l2Sources, kv
}

func storeBlock(t *testing.T, kv kvstore.KV, block *types.Block, receipts types.Receipts) {
	// Pre-store receipts
	opaqueRcpts, err := eth.EncodeReceipts(receipts)
	require.NoError(t, err)
	_, nodes := mpt.WriteTrie(opaqueRcpts)
	for _, p := range nodes {
		require.NoError(t, kv.Put(preimage.Keccak256Key(crypto.Keccak256Hash(p)).PreimageKey(), p))
	}

	// Pre-store transactions
	opaqueTxs, err := eth.EncodeTransactions(block.Transactions())
	require.NoError(t, err)
	_, txsNodes := mpt.WriteTrie(opaqueTxs)
	for _, p := range txsNodes {
		require.NoError(t, kv.Put(preimage.Keccak256Key(crypto.Keccak256Hash(p)).PreimageKey(), p))
	}

	// Pre-store block
	headerRlp, err := rlp.EncodeToBytes(block.Header())
	require.NoError(t, err)
	require.NoError(t, kv.Put(preimage.Keccak256Key(block.Hash()).PreimageKey(), headerRlp))
}

func storeBlob(t *testing.T, kv kvstore.KV, commitment eth.Bytes48, blob *eth.Blob) {
	// Pre-store versioned hash preimage (commitment)
	err := kv.Put(preimage.Sha256Key(sha256.Sum256(commitment[:])).PreimageKey(), commitment[:])
	require.NoError(t, err, "Failed to store versioned hash preimage in kvstore")

	// Pre-store blob field elements
	blobKeyBuf := make([]byte, 80)
	copy(blobKeyBuf[:48], commitment[:])
	for i := 0; i < params.BlobTxFieldElementsPerBlob; i++ {
		binary.BigEndian.PutUint64(blobKeyBuf[72:], uint64(i))
		feKey := crypto.Keccak256Hash(blobKeyBuf)

		err = kv.Put(preimage.BlobKey(feKey).PreimageKey(), blob[i<<5:(i+1)<<5])
		require.NoError(t, err, "Failed to store field element preimage in kvstore")
	}
}

func asOracleFn(t *testing.T, prefetcher *Prefetcher) preimage.OracleFn {
	return func(key preimage.Key) []byte {
		pre, err := prefetcher.GetPreimage(context.Background(), key.PreimageKey())
		require.NoError(t, err)
		return pre
	}
}

func asHinter(t *testing.T, prefetcher *Prefetcher) preimage.HinterFn {
	return func(v preimage.Hint) {
		err := prefetcher.Hint(v.Hint())
		require.NoError(t, err)
	}
}

func assertTransactionsEqual(t *testing.T, blockTx types.Transactions, txs types.Transactions) {
	require.Equal(t, len(blockTx), len(txs))
	for i, tx := range txs {
		require.Equal(t, blockTx[i].Hash(), tx.Hash())
	}
}

func assertReceiptsEqual(t *testing.T, expectedRcpt types.Receipts, actualRcpt types.Receipts) {
	require.Equal(t, len(expectedRcpt), len(actualRcpt))
	for i, rcpt := range actualRcpt {
		// Make a copy of each to zero out fields we expect to be different
		expected := *expectedRcpt[i]
		actual := *rcpt
		expected.ContractAddress = common.Address{}
		actual.ContractAddress = common.Address{}
		require.Equal(t, expected, actual)
	}
}

// legacyOracleImpl is a wrapper around the new preimage.Oracle interface that uses the legacy preimage hint API.
// It's used to test backwards-compatibility with clients using legacy preimage hints.
type legacyPrecompileOracle struct {
	oracle preimage.Oracle
	hint   preimage.Hinter
}

func newLegacyPrecompileOracle(raw preimage.Oracle, hint preimage.Hinter) *legacyPrecompileOracle {
	return &legacyPrecompileOracle{
		oracle: raw,
		hint:   hint,
	}
}

func (o *legacyPrecompileOracle) Precompile(address common.Address, input []byte) ([]byte, bool) {
	hintBytes := append(address.Bytes(), input...)
	o.hint.Hint(l1.PrecompileHint(hintBytes))
	key := preimage.PrecompileKey(crypto.Keccak256Hash(hintBytes))
	result := o.oracle.Get(key)
	if len(result) == 0 { // must contain at least the status code
		panic(fmt.Errorf("unexpected precompile oracle behavior, got result: %x", result))
	}
	return result[1:], result[0] == 1
}

type mockExecutor struct {
	invoked     bool
	blockNumber uint64
	chainID     eth.ChainID
}

func (m *mockExecutor) RunProgram(
	ctx context.Context, prefetcher hostcommon.Prefetcher, blockNumber uint64, chainID eth.ChainID, db l2.KeyValueStore) error {
	m.invoked = true
	m.blockNumber = blockNumber
	m.chainID = chainID
	return nil
}

func assertAllClientExpectations(t *testing.T, l2Cls *l2Clients) {
	for _, source := range l2Cls.sources {
		source.Mock.AssertExpectations(t)
		source.MockDebugClient.Mock.AssertExpectations(t)
	}
}
