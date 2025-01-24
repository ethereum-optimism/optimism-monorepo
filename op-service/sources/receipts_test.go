package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type ethBackend struct {
	*mock.Mock
}

func (b *ethBackend) GetBlockByHash(id common.Hash, fullTxs bool) (*RPCBlock, error) {
	out := b.Mock.MethodCalled("eth_getBlockByHash", id, fullTxs)
	return out[0].(*RPCBlock), nil
}

func (b *ethBackend) GetTransactionReceipt(txHash common.Hash) (*types.Receipt, error) {
	out := b.Mock.MethodCalled("eth_getTransactionReceipt", txHash)
	return out[0].(*types.Receipt), *out[1].(*error)
}

func (b *ethBackend) GetBlockReceipts(id string) ([]*types.Receipt, error) {
	out := b.Mock.MethodCalled("eth_getBlockReceipts", id)
	return out[0].([]*types.Receipt), *out[1].(*error)
}

type erigonBackend struct {
	*mock.Mock
}

func (b *erigonBackend) GetBlockReceiptsByBlockHash(id string) ([]*types.Receipt, error) {
	out := b.Mock.MethodCalled("erigon_getBlockReceiptsByBlockHash", id)
	return out[0].([]*types.Receipt), *out[1].(*error)
}

type alchemyBackend struct {
	*mock.Mock
}

func (b *alchemyBackend) GetTransactionReceipts(p blockHashParameter) (*receiptsWrapper, error) {
	out := b.Mock.MethodCalled("alchemy_getTransactionReceipts", p.BlockHash.String())
	return &receiptsWrapper{Receipts: out[0].([]*types.Receipt)}, *out[1].(*error)
}

type debugBackend struct {
	*mock.Mock
}

func (b *debugBackend) GetRawReceipts(id string) ([]hexutil.Bytes, error) {
	out := b.Mock.MethodCalled("debug_getRawReceipts", id)
	return out[0].([]hexutil.Bytes), *out[1].(*error)
}

type parityBackend struct {
	*mock.Mock
}

func (b *parityBackend) GetBlockReceipts(id string) ([]*types.Receipt, error) {
	out := b.Mock.MethodCalled("parity_getBlockReceipts", id)
	return out[0].([]*types.Receipt), *out[1].(*error)
}

type ReceiptsRequest struct {
	method ReceiptsFetchingMethod
	result []*types.Receipt
	err    error
}

type methodNotFoundError struct{ method string }

func (e *methodNotFoundError) ErrorCode() int { return -32601 }

func (e *methodNotFoundError) Error() string {
	return fmt.Sprintf("the method %s does not exist/is not available", e.method)
}

// ReceiptsTestCase runs through a series of receipt fetching RPC requests with mocked results
// to test the prioritization/fallback logic of the receipt fetching in the EthClient.
type ReceiptsTestCase struct {
	name         string
	providerKind RPCProviderKind
	staticMethod bool
	setup        func(t *testing.T) (*RPCBlock, []ReceiptsRequest)
}

func (tc *ReceiptsTestCase) Run(t *testing.T) {
	srv := rpc.NewServer()
	defer srv.Stop()
	m := &mock.Mock{}

	require.NoError(t, srv.RegisterName("eth", &ethBackend{Mock: m}))
	require.NoError(t, srv.RegisterName("alchemy", &alchemyBackend{Mock: m}))
	require.NoError(t, srv.RegisterName("debug", &debugBackend{Mock: m}))
	require.NoError(t, srv.RegisterName("parity", &parityBackend{Mock: m}))
	require.NoError(t, srv.RegisterName("erigon", &erigonBackend{Mock: m}))

	block, requests := tc.setup(t)

	// always expect a block request to fetch txs and receipts root hash etc.
	m.On("eth_getBlockByHash", block.Hash, true).Once().Return(block)

	for _, reqData := range requests {
		req := reqData
		// depending on the method, expect to serve receipts by request(s)
		switch req.method {
		case EthGetTransactionReceiptBatch:
			for i, tx := range block.Transactions {
				m.On("eth_getTransactionReceipt", tx.Hash()).Once().Return(req.result[i], &req.err)
			}
		case AlchemyGetTransactionReceipts:
			m.On("alchemy_getTransactionReceipts", block.Hash.String()).Once().Return(req.result, &req.err)
		case DebugGetRawReceipts:
			var raw []hexutil.Bytes
			for _, r := range req.result {
				data, err := r.MarshalBinary()
				require.NoError(t, err)
				raw = append(raw, data)
			}
			m.On("debug_getRawReceipts", block.Hash.String()).Once().Return(raw, &req.err)
		case ParityGetBlockReceipts:
			m.On("parity_getBlockReceipts", block.Hash.String()).Once().Return(req.result, &req.err)
		case EthGetBlockReceipts:
			m.On("eth_getBlockReceipts", block.Hash.String()).Once().Return(req.result, &req.err)
		case ErigonGetBlockReceiptsByBlockHash:
			m.On("erigon_getBlockReceiptsByBlockHash", block.Hash.String()).Once().Return(req.result, &req.err)
		default:
			t.Fatalf("unrecognized request method: %d", uint64(req.method))
		}
	}

	cl := rpc.DialInProc(srv)
	testCfg := &EthClientConfig{
		// receipts and transactions are cached per block
		ReceiptsCacheSize:     1000,
		TransactionsCacheSize: 1000,
		HeadersCacheSize:      1000,
		PayloadsCacheSize:     1000,
		MaxRequestsPerBatch:   20,
		MaxConcurrentRequests: 10,
		TrustRPC:              false,
		MustBePostMerge:       false,
		RPCProviderKind:       tc.providerKind,
		MethodResetDuration:   time.Minute,
	}
	if tc.staticMethod { // if static, instantly reset, for fast clock-independent testing
		testCfg.MethodResetDuration = 0
	}
	logger := testlog.Logger(t, log.LevelError)
	ethCl, err := NewEthClient(client.NewBaseRPCClient(cl), logger, nil, testCfg)
	require.NoError(t, err)
	defer ethCl.Close()

	for i, req := range requests {
		info, result, err := ethCl.FetchReceipts(context.Background(), block.Hash)
		if err == nil {
			require.NoError(t, req.err, "error")
			require.Equal(t, block.Hash, info.Hash(), fmt.Sprintf("req %d blockhash", i))
			for j, rec := range req.result {
				requireEqualReceipt(t, rec, result[j], "req %d result %d", i, j)
			}
		} else {
			require.Error(t, req.err, "error")
			require.Equal(t, req.err.Error(), err.Error(), fmt.Sprintf("req %d err", i))
		}
	}

	m.AssertExpectations(t)
}

func randomRpcBlockAndReceipts(rng *rand.Rand, txCount uint64) (*RPCBlock, []*types.Receipt) {
	block, receipts := testutils.RandomBlock(rng, txCount)
	return &RPCBlock{
		RPCHeader: RPCHeader{
			ParentHash:  block.ParentHash(),
			UncleHash:   block.UncleHash(),
			Coinbase:    block.Coinbase(),
			Root:        block.Root(),
			TxHash:      block.TxHash(),
			ReceiptHash: block.ReceiptHash(),
			Bloom:       eth.Bytes256(block.Bloom()),
			Difficulty:  *(*hexutil.Big)(block.Difficulty()),
			Number:      hexutil.Uint64(block.NumberU64()),
			GasLimit:    hexutil.Uint64(block.GasLimit()),
			GasUsed:     hexutil.Uint64(block.GasUsed()),
			Time:        hexutil.Uint64(block.Time()),
			Extra:       hexutil.Bytes(block.Extra()),
			MixDigest:   block.MixDigest(),
			Nonce:       types.EncodeNonce(block.Nonce()),
			BaseFee:     (*hexutil.Big)(block.BaseFee()),
			Hash:        block.Hash(),
		},
		Transactions: block.Transactions(),
	}, receipts
}

func TestEthClient_FetchReceipts(t *testing.T) {
	// Helper to quickly define the test case requests scenario:
	// each method fails to fetch the receipts, except the last
	fallbackCase := func(txCount uint64, methods ...ReceiptsFetchingMethod) func(t *testing.T) (*RPCBlock, []ReceiptsRequest) {
		return func(t *testing.T) (*RPCBlock, []ReceiptsRequest) {
			block, receipts := randomRpcBlockAndReceipts(rand.New(rand.NewSource(123)), txCount)
			// zero out the data we don't want to verify
			for _, r := range receipts {
				r.ContractAddress = common.Address{}
			}
			var out []ReceiptsRequest
			for _, m := range methods {
				out = append(out, ReceiptsRequest{
					method: m,
				})
			}
			// all but the last request fail to fetch receipts
			for i := 0; i < len(out)-1; i++ {
				out[i].result = nil
				out[i].err = new(methodNotFoundError)
			}
			// last request fetches receipts
			out[len(out)-1].result = receipts
			return block, out
		}
	}

	testCases := []ReceiptsTestCase{
		{
			name:         "alchemy",
			providerKind: RPCKindAlchemy,
			setup:        fallbackCase(30, AlchemyGetTransactionReceipts),
		},
		{
			name:         "alchemy sticky",
			providerKind: RPCKindAlchemy,
			staticMethod: true,
			setup:        fallbackCase(30, AlchemyGetTransactionReceipts, AlchemyGetTransactionReceipts),
		},
		{
			name:         "alchemy fallback 1",
			providerKind: RPCKindAlchemy,
			setup:        fallbackCase(40, AlchemyGetTransactionReceipts, EthGetBlockReceipts),
		},
		{
			name:         "alchemy low tx count cost saving",
			providerKind: RPCKindAlchemy,
			// when it's cheaper to fetch individual receipts than the alchemy-bundled receipts we change methods.
			setup: fallbackCase(5, EthGetTransactionReceiptBatch),
		},
		{
			name:         "quicknode",
			providerKind: RPCKindQuickNode,
			setup:        fallbackCase(30, DebugGetRawReceipts),
		},
		{
			name:         "quicknode fallback 1",
			providerKind: RPCKindQuickNode,
			setup: fallbackCase(30,
				DebugGetRawReceipts,
				EthGetBlockReceipts,
			),
		},
		{
			name:         "quicknode low tx count cost saving",
			providerKind: RPCKindQuickNode,
			// when it's cheaper to fetch individual receipts than the alchemy-bundled receipts we change methods.
			setup: fallbackCase(5, DebugGetRawReceipts, EthGetTransactionReceiptBatch),
		},
		{
			name:         "infura",
			providerKind: RPCKindInfura,
			setup:        fallbackCase(4, EthGetBlockReceipts, EthGetTransactionReceiptBatch),
		},
		{
			name:         "nethermind",
			providerKind: RPCKindNethermind,
			setup:        fallbackCase(4, ParityGetBlockReceipts), // uses parity namespace method
		},
		{
			name:         "geth with debug rpc",
			providerKind: RPCKindDebugGeth,
			setup:        fallbackCase(4, DebugGetRawReceipts),
		},
		{
			name:         "erigon",
			providerKind: RPCKindErigon,
			setup:        fallbackCase(4, ErigonGetBlockReceiptsByBlockHash),
		},
		{
			name:         "basic",
			providerKind: RPCKindBasic,
			setup:        fallbackCase(4, EthGetTransactionReceiptBatch),
		},
		{
			name:         "standard",
			providerKind: RPCKindStandard,
			setup:        fallbackCase(4, EthGetBlockReceipts),
		},
		{
			name:         "standard fallback",
			providerKind: RPCKindStandard,
			setup:        fallbackCase(4, EthGetBlockReceipts, EthGetTransactionReceiptBatch),
		},
		{
			name:         "any discovers alchemy",
			providerKind: RPCKindAny,
			setup:        fallbackCase(4, AlchemyGetTransactionReceipts),
		},
		{
			name:         "any discovers parity",
			providerKind: RPCKindAny,
			// fallback through the least priority method: parity (nethermind supports this still)
			setup: fallbackCase(4,
				AlchemyGetTransactionReceipts,
				DebugGetRawReceipts,
				ErigonGetBlockReceiptsByBlockHash,
				EthGetBlockReceipts,
				ParityGetBlockReceipts,
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.Run)
	}
}

func TestVerifyReceipts(t *testing.T) {
	validData := func() (eth.BlockID, common.Hash, []common.Hash, []*types.Receipt) {
		block := eth.BlockID{
			Hash:   common.HexToHash("0x40fb7cc5fbc1ec594230a60648a442412116d50ae43d517ea458d8ea4e60bd1b"),
			Number: 9998910,
		}
		txHashes := []common.Hash{
			common.HexToHash("0x61e4872004a80843fcbede236527bf24707f4e8f44a2704ed9c4fb91c87b0f29"),
			common.HexToHash("0x0ae25ad9ff01fd74fa1b0c11f12fbb623a3f0553a0eed465a6dbf0962898c3b6"),
			common.HexToHash("0x2de33b18143039dcdf88cb62c3f3dd8f3f5d9f29807edfd3b0507246c55f9cb8"),
			common.HexToHash("0xb6a381d3c31df47da82ac807c3000ae4adf55e981715f56d13a27b220de20198"),
		}
		receipts := []*types.Receipt{
			{
				Type:              2,
				Status:            0,
				CumulativeGasUsed: 0x3035b,
				Bloom:             types.BytesToBloom(common.FromHex("00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")),
				Logs:              nil,
				TxHash:            txHashes[0],
				GasUsed:           0x3035b,
				EffectiveGasPrice: big.NewInt(0x12a05f20a),
				BlockHash:         block.Hash,
				BlockNumber:       new(big.Int).SetUint64(block.Number),
				TransactionIndex:  0,
			},
			{
				Type:              2,
				Status:            1,
				CumulativeGasUsed: 0xa9ae4,
				Bloom:             types.BytesToBloom(common.FromHex("00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")),
				Logs:              nil,
				TxHash:            txHashes[1],
				GasUsed:           0x79789,
				EffectiveGasPrice: big.NewInt(0xb2d05e0a),
				BlockHash:         block.Hash,
				BlockNumber:       new(big.Int).SetUint64(block.Number),
				TransactionIndex:  1,
			},
			{
				Type:              0,
				Status:            1,
				CumulativeGasUsed: 0x101f09,
				Bloom:             types.BytesToBloom(common.FromHex("00000000000000000000000000000200000400000000000000000000000000800000000000040000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002002000000000000000000000000000000000000000000000000000000000000000000000000000000000000200000000000800000000000000000000000000000000000000000000000000000400000000000000020000000000000000000000000002000000000000000000000000000000000000000000")),
				Logs: []*types.Log{
					{
						Address: common.HexToAddress("0x759c5e44a9e4be8b7e9bd25a790ceb662c924c45"),
						Topics: []common.Hash{
							common.HexToHash("0x33830f93f4b524630176cdcbc04070b30ada13c6ad27081289131b77f178ff89"),
							common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000426a80"),
							common.HexToHash("0x000000000000000000000000a6275ee214f80a532c3abee0a4cbbc2d1dc22a72"),
						},
						Data:        common.FromHex("00000000000000000000000000000000000000000000000000000000000005dc"),
						BlockNumber: block.Number,
						TxHash:      txHashes[2],
						TxIndex:     2,
						BlockHash:   block.Hash,
						Index:       0,
						Removed:     false,
					},
				},
				TxHash:            txHashes[2],
				GasUsed:           0x58425,
				EffectiveGasPrice: big.NewInt(0xb2d05e00),
				BlockHash:         block.Hash,
				BlockNumber:       new(big.Int).SetUint64(block.Number),
				TransactionIndex:  2,
			},
			{
				Type:              0,
				Status:            1,
				CumulativeGasUsed: 0x1227ab,
				Bloom:             types.BytesToBloom(common.FromHex("00000000000000000000000000000200000400000000000000000000000000800000000000040000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002002000000000000000000000000000000000000000000000000000000000000000000000000000000000000200000000000800000000000000000000000000000000000000000000000000000400000000000000020000000000000000000000000002000000000000000000000000000000000000000000")),
				Logs: []*types.Log{
					{
						Address: common.HexToAddress("0x759c5e44a9e4be8b7e9bd25a790ceb662c924c45"),
						Topics: []common.Hash{
							common.HexToHash("0x33830f93f4b524630176cdcbc04070b30ada13c6ad27081289131b77f178ff89"),
							common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000426a80"),
							common.HexToHash("0x000000000000000000000000a6275ee214f80a532c3abee0a4cbbc2d1dc22a72"),
						},
						Data:        common.FromHex("00000000000000000000000000000000000000000000000000000000000005dc"),
						BlockNumber: block.Number,
						TxHash:      txHashes[3],
						TxIndex:     3,
						BlockHash:   block.Hash,
						Index:       1,
						Removed:     false,
					},
				},
				TxHash:            txHashes[3],
				GasUsed:           0x208a2,
				EffectiveGasPrice: big.NewInt(0x59682f0a),
				BlockHash:         block.Hash,
				BlockNumber:       new(big.Int).SetUint64(block.Number),
				TransactionIndex:  3,
			},
		}
		receiptsHash := types.DeriveSha(types.Receipts(receipts), trie.NewStackTrie(nil))
		return block, receiptsHash, txHashes, receipts
	}

	t.Run("Valid", func(t *testing.T) {
		block, receiptHash, txHashes, receipts := validData()
		err := validateReceipts(block, receiptHash, txHashes, receipts)
		require.NoError(t, err)
	})

	t.Run("NotEnoughReceipts", func(t *testing.T) {
		block, receiptHash, txHashes, receipts := validData()
		err := validateReceipts(block, receiptHash, txHashes, receipts[1:])
		require.ErrorContains(t, err, fmt.Sprintf("got %d receipts but expected %d", len(receipts)-1, len(receipts)))
	})

	t.Run("TooManyReceipts", func(t *testing.T) {
		block, receiptHash, txHashes, receipts := validData()
		err := validateReceipts(block, receiptHash, txHashes, append(receipts, receipts[0]))
		require.ErrorContains(t, err, fmt.Sprintf("got %d receipts but expected %d", len(receipts)+1, len(receipts)))
	})

	t.Run("NoTxButNotEmptyTrieRoot", func(t *testing.T) {
		block, receiptHash, _, _ := validData()
		err := validateReceipts(block, receiptHash, nil, nil)
		require.ErrorContains(t, err, "no transactions, but got non-empty receipt trie root")
	})

	t.Run("NoTxWithEmptyTrieRoot", func(t *testing.T) {
		block, _, _, _ := validData()
		err := validateReceipts(block, types.EmptyRootHash, nil, nil)
		require.NoError(t, err)
	})

	t.Run("IncorrectReceiptRoot", func(t *testing.T) {
		block, _, txHashes, receipts := validData()
		err := validateReceipts(block, common.Hash{0x35}, txHashes, receipts)
		require.ErrorContains(t, err, "failed to fetch list of receipts: expected receipt root")
	})

	t.Run("NilReceipt", func(t *testing.T) {
		block, receiptHash, txHashes, receipts := validData()
		receipts[0] = nil
		err := validateReceipts(block, receiptHash, txHashes, receipts)
		require.ErrorContains(t, err, "returns nil on retrieval")
	})

	t.Run("IncorrectTxIndex", func(t *testing.T) {
		block, receiptHash, txHashes, receipts := validData()
		receipts[0].TransactionIndex = 2
		err := validateReceipts(block, receiptHash, txHashes, receipts)
		require.ErrorContains(t, err, "has unexpected tx index")
	})

	t.Run("Missing block number", func(t *testing.T) {
		block, receiptHash, txHashes, receipts := validData()
		receipts[0].BlockNumber = nil
		err := validateReceipts(block, receiptHash, txHashes, receipts)
		require.ErrorContains(t, err, "has unexpected nil block number")
	})

	t.Run("IncorrectBlockNumber", func(t *testing.T) {
		block, receiptHash, txHashes, receipts := validData()
		receipts[0].BlockNumber = big.NewInt(1234)
		err := validateReceipts(block, receiptHash, txHashes, receipts)
		require.ErrorContains(t, err, "has unexpected block number")
	})

	t.Run("IncorrectBlockHash", func(t *testing.T) {
		block, receiptHash, txHashes, receipts := validData()
		receipts[0].BlockHash = common.Hash{0x48}
		err := validateReceipts(block, receiptHash, txHashes, receipts)
		require.ErrorContains(t, err, "has unexpected block hash")
	})

	t.Run("IncorrectCumulativeUsed", func(t *testing.T) {
		block, receiptHash, txHashes, receipts := validData()
		receipts[1].CumulativeGasUsed = receipts[0].CumulativeGasUsed
		err := validateReceipts(block, receiptHash, txHashes, receipts)
		require.ErrorContains(t, err, "has invalid gas used metadata")
	})

	t.Run("IncorrectLogIndex", func(t *testing.T) {
		block, receiptHash, txHashes, receipts := validData()
		receipts[2].Logs[0].Index = 4
		err := validateReceipts(block, receiptHash, txHashes, receipts)
		require.ErrorContains(t, err, "has unexpected log index")
	})

	t.Run("LogIndexNotBlockBased", func(t *testing.T) {
		block, receiptHash, txHashes, receipts := validData()
		receipts[3].Logs[0].Index = 0
		err := validateReceipts(block, receiptHash, txHashes, receipts)
		require.ErrorContains(t, err, "has unexpected log index")
	})

	t.Run("IncorrectLogTxIndex", func(t *testing.T) {
		block, receiptHash, txHashes, receipts := validData()
		receipts[2].Logs[0].TxIndex = 1
		err := validateReceipts(block, receiptHash, txHashes, receipts)
		require.ErrorContains(t, err, "has unexpected tx index")
	})

	t.Run("IncorrectLogBlockHash", func(t *testing.T) {
		block, receiptHash, txHashes, receipts := validData()
		receipts[2].Logs[0].BlockHash = common.Hash{0x64}
		err := validateReceipts(block, receiptHash, txHashes, receipts)
		require.ErrorContains(t, err, "has unexpected block hash")
	})

	t.Run("IncorrectLogBlockNumber", func(t *testing.T) {
		block, receiptHash, txHashes, receipts := validData()
		receipts[2].Logs[0].BlockNumber = 4727923
		err := validateReceipts(block, receiptHash, txHashes, receipts)
		require.ErrorContains(t, err, "has unexpected block number")
	})

	t.Run("IncorrectLogTxHash", func(t *testing.T) {
		block, receiptHash, txHashes, receipts := validData()
		receipts[2].Logs[0].TxHash = common.Hash{0x87}
		err := validateReceipts(block, receiptHash, txHashes, receipts)
		require.ErrorContains(t, err, "has unexpected tx hash")
	})

	t.Run("Removed", func(t *testing.T) {
		block, receiptHash, txHashes, receipts := validData()
		receipts[2].Logs[0].Removed = true
		err := validateReceipts(block, receiptHash, txHashes, receipts)
		require.ErrorContains(t, err, "must never be removed due to reorg")
	})
}

func requireEqualReceipt(t *testing.T, exp, act *types.Receipt, msgAndArgs ...any) {
	t.Helper()
	expJson, err := json.MarshalIndent(exp, "", "  ")
	require.NoError(t, err, msgAndArgs...)
	actJson, err := json.MarshalIndent(act, "", "  ")
	require.NoError(t, err, msgAndArgs...)
	require.Equal(t, string(expJson), string(actJson), msgAndArgs...)
}
