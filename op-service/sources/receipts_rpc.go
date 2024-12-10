package sources

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/sources/caching"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-service/eth"
)

func newRPCRecProviderFromConfig(client client.RPC, log log.Logger, metrics caching.Metrics, config *EthClientConfig) *CachingReceiptsProvider {
	recCfg := RPCReceiptsConfig{
		MaxBatchSize:        config.MaxRequestsPerBatch,
		ProviderKind:        config.RPCProviderKind,
		MethodResetDuration: config.MethodResetDuration,
	}
	return NewCachingRPCReceiptsProvider(client, log, recCfg, metrics, config.ReceiptsCacheSize)
}

type rpcClient interface {
	CallContext(ctx context.Context, result any, method string, args ...any) error
	BatchCallContext(ctx context.Context, b []rpc.BatchElem) error
}

type RPCReceiptsFetcher struct {
	client rpcClient
	basic  *BasicRPCReceiptsFetcher

	log log.Logger

	provKind RPCProviderKind

	// availableReceiptMethods tracks which receipt methods can be used for fetching receipts
	// This may be modified concurrently, but we don't lock since it's a single
	// uint64 that's not critical (fine to miss or mix up a modification)
	availableReceiptMethods ReceiptsFetchingMethod

	// lastMethodsReset tracks when availableReceiptMethods was last reset.
	// When receipt-fetching fails it falls back to available methods,
	// but periodically it will try to reset to the preferred optimal methods.
	lastMethodsReset time.Time

	// methodResetDuration defines how long we take till we reset lastMethodsReset
	methodResetDuration time.Duration
}

type RPCReceiptsConfig struct {
	MaxBatchSize        int
	ProviderKind        RPCProviderKind
	MethodResetDuration time.Duration
}

func NewRPCReceiptsFetcher(client rpcClient, log log.Logger, config RPCReceiptsConfig) *RPCReceiptsFetcher {
	return &RPCReceiptsFetcher{
		client:                  client,
		basic:                   NewBasicRPCReceiptsFetcher(client, config.MaxBatchSize),
		log:                     log,
		provKind:                config.ProviderKind,
		availableReceiptMethods: AvailableReceiptsFetchingMethods(config.ProviderKind),
		lastMethodsReset:        time.Now(),
		methodResetDuration:     config.MethodResetDuration,
	}
}

func (f *RPCReceiptsFetcher) FetchReceipts(ctx context.Context, blockInfo eth.BlockInfo, txHashes []common.Hash) (result types.Receipts, err error) {
	m := f.PickReceiptsMethod(len(txHashes))
	block := eth.ToBlockID(blockInfo)
	switch m {
	case EthGetTransactionReceiptBatch:
		result, err = f.basic.FetchReceipts(ctx, blockInfo, txHashes)
	case AlchemyGetTransactionReceipts:
		var tmp receiptsWrapper
		err = f.client.CallContext(ctx, &tmp, "alchemy_getTransactionReceipts", blockHashParameter{BlockHash: block.Hash})
		result = tmp.Receipts
	case DebugGetRawReceipts:
		var rawReceipts []hexutil.Bytes
		err = f.client.CallContext(ctx, &rawReceipts, "debug_getRawReceipts", block.Hash)
		if err == nil {
			if len(rawReceipts) == len(txHashes) {
				result, err = eth.DecodeRawReceipts(block, rawReceipts, txHashes)
			} else {
				err = fmt.Errorf("got %d raw receipts, but expected %d", len(rawReceipts), len(txHashes))
			}
		}
	case ParityGetBlockReceipts:
		err = f.client.CallContext(ctx, &result, "parity_getBlockReceipts", block.Hash)
	case EthGetBlockReceipts:
		err = f.client.CallContext(ctx, &result, "eth_getBlockReceipts", block.Hash)
	case ErigonGetBlockReceiptsByBlockHash:
		err = f.client.CallContext(ctx, &result, "erigon_getBlockReceiptsByBlockHash", block.Hash)
	default:
		err = fmt.Errorf("unknown receipt fetching method: %d", uint64(m))
	}

	if err != nil {
		f.OnReceiptsMethodErr(m, err)
		return nil, err
	}

	if err = validateReceipts(block, blockInfo.ReceiptHash(), txHashes, result); err != nil {
		return nil, err
	}

	return
}

// receiptsWrapper is a decoding type util. Alchemy in particular wraps the receipts array result.
type receiptsWrapper struct {
	Receipts []*types.Receipt `json:"receipts"`
}

func (f *RPCReceiptsFetcher) PickReceiptsMethod(txCount int) ReceiptsFetchingMethod {
	txc := uint64(txCount)
	if now := time.Now(); now.Sub(f.lastMethodsReset) > f.methodResetDuration {
		m := AvailableReceiptsFetchingMethods(f.provKind)
		if f.availableReceiptMethods != m {
			f.log.Warn("resetting back RPC preferences, please review RPC provider kind setting", "kind", f.provKind.String())
		}
		f.availableReceiptMethods = m
		f.lastMethodsReset = now
	}
	return PickBestReceiptsFetchingMethod(f.provKind, f.availableReceiptMethods, txc)
}

func (f *RPCReceiptsFetcher) OnReceiptsMethodErr(m ReceiptsFetchingMethod, err error) {
	if unusableMethod(err) {
		// clear the bit of the method that errored
		f.availableReceiptMethods &^= m
		f.log.Warn("failed to use selected RPC method for receipt fetching, temporarily falling back to alternatives",
			"provider_kind", f.provKind, "failed_method", m, "fallback", f.availableReceiptMethods, "err", err)
	} else {
		f.log.Debug("failed to use selected RPC method for receipt fetching, but method does appear to be available, so we continue to use it",
			"provider_kind", f.provKind, "failed_method", m, "fallback", f.availableReceiptMethods&^m, "err", err)
	}
}

// Cost break-down sources:
// Alchemy: https://docs.alchemy.com/reference/compute-units
// QuickNode: https://www.quicknode.com/docs/ethereum/api_credits
// Infura: no pricing table available.
//
// Receipts are encoded the same everywhere:
//
//     blockHash, blockNumber, transactionIndex, transactionHash, from, to, cumulativeGasUsed, gasUsed,
//     contractAddress, logs, logsBloom, status, effectiveGasPrice, type.
//
// Note that Alchemy/Geth still have a "root" field for legacy reasons,
// but ethereum does not compute state-roots per tx anymore, so quicknode and others do not serve this data.

// RPCProviderKind identifies an RPC provider, used to hint at the optimal receipt fetching approach.
type RPCProviderKind string

const (
	RPCKindAlchemy    RPCProviderKind = "alchemy"
	RPCKindQuickNode  RPCProviderKind = "quicknode"
	RPCKindInfura     RPCProviderKind = "infura"
	RPCKindParity     RPCProviderKind = "parity"
	RPCKindNethermind RPCProviderKind = "nethermind"
	RPCKindDebugGeth  RPCProviderKind = "debug_geth"
	RPCKindErigon     RPCProviderKind = "erigon"
	RPCKindBasic      RPCProviderKind = "basic"    // try only the standard most basic receipt fetching
	RPCKindAny        RPCProviderKind = "any"      // try any method available
	RPCKindStandard   RPCProviderKind = "standard" // try standard methods, including newer optimized standard RPC methods
)

var RPCProviderKinds = []RPCProviderKind{
	RPCKindAlchemy,
	RPCKindQuickNode,
	RPCKindInfura,
	RPCKindParity,
	RPCKindNethermind,
	RPCKindDebugGeth,
	RPCKindErigon,
	RPCKindBasic,
	RPCKindAny,
	RPCKindStandard,
}

func (kind RPCProviderKind) String() string {
	return string(kind)
}

func (kind *RPCProviderKind) Set(value string) error {
	if !ValidRPCProviderKind(RPCProviderKind(value)) {
		return fmt.Errorf("unknown rpc kind: %q", value)
	}
	*kind = RPCProviderKind(value)
	return nil
}

func (kind *RPCProviderKind) Clone() any {
	cpy := *kind
	return &cpy
}

func ValidRPCProviderKind(value RPCProviderKind) bool {
	for _, k := range RPCProviderKinds {
		if k == value {
			return true
		}
	}
	return false
}

// ReceiptsFetchingMethod is a bitfield with 1 bit for each receipts fetching type.
// Depending on errors, tx counts and preferences the code may select different sets of fetching methods.
type ReceiptsFetchingMethod uint64

func (r ReceiptsFetchingMethod) String() string {
	out := ""
	x := r
	addMaybe := func(m ReceiptsFetchingMethod, v string) {
		if x&m != 0 {
			out += v
			x ^= x & m
		}
		if x != 0 { // add separator if there are entries left
			out += ", "
		}
	}
	addMaybe(EthGetTransactionReceiptBatch, "eth_getTransactionReceipt (batched)")
	addMaybe(AlchemyGetTransactionReceipts, "alchemy_getTransactionReceipts")
	addMaybe(DebugGetRawReceipts, "debug_getRawReceipts")
	addMaybe(ParityGetBlockReceipts, "parity_getBlockReceipts")
	addMaybe(EthGetBlockReceipts, "eth_getBlockReceipts")
	addMaybe(ErigonGetBlockReceiptsByBlockHash, "erigon_getBlockReceiptsByBlockHash")
	addMaybe(^ReceiptsFetchingMethod(0), "unknown") // if anything is left, describe it as unknown
	return out
}

const (
	// EthGetTransactionReceiptBatch is standard per-tx receipt fetching with JSON-RPC batches.
	// Available in: standard, everywhere.
	//   - Alchemy: 15 CU / tx
	//   - Quicknode: 2 credits / tx
	// Method: eth_getTransactionReceipt
	// See: https://ethereum.github.io/execution-apis/api-documentation/
	EthGetTransactionReceiptBatch ReceiptsFetchingMethod = 1 << iota
	// AlchemyGetTransactionReceipts is a special receipt fetching method provided by Alchemy.
	// Available in:
	//   - Alchemy: 250 CU total
	// Method: alchemy_getTransactionReceipts
	// Params:
	//   - object with "blockNumber" or "blockHash" field
	// Returns: "array of receipts" - docs lie, array is wrapped in a struct with single "receipts" field
	// See: https://docs.alchemy.com/reference/alchemy-gettransactionreceipts#alchemy_gettransactionreceipts
	AlchemyGetTransactionReceipts
	// DebugGetRawReceipts is a debug method from Geth, faster by avoiding serialization and metadata overhead.
	// Ideal for fast syncing from a local geth node.
	// Available in:
	//   - Geth: free
	//   - QuickNode: 22 credits maybe? Unknown price, undocumented ("debug_getblockreceipts" exists in table though?)
	// Method: debug_getRawReceipts
	// Params:
	//   - string presenting a block number or hash
	// Returns: list of strings, hex encoded RLP of receipts data. "consensus-encoding of all receipts in a single block"
	// See: https://geth.ethereum.org/docs/rpc/ns-debug#debug_getrawreceipts
	DebugGetRawReceipts
	// ParityGetBlockReceipts is an old parity method, which has been adopted by Nethermind and some RPC providers.
	// Available in:
	//   - Alchemy: 500 CU total
	//   - QuickNode: 59 credits - docs are wrong, not actually available anymore.
	//   - Any open-ethereum/parity legacy: free
	//   - Nethermind: free
	// Method: parity_getBlockReceipts
	// Params:
	//   Parity: "quantity or tag"
	//   Alchemy: string with block hash, number in hex, or block tag.
	//   Nethermind: very flexible: tag, number, hex or object with "requireCanonical"/"blockHash" fields.
	// Returns: array of receipts
	// See:
	//   - Parity: https://openethereum.github.io/JSONRPC-parity-module#parity_getblockreceipts
	//   - QuickNode: undocumented.
	//   - Alchemy: https://docs.alchemy.com/reference/eth-getblockreceipts
	//   - Nethermind: https://docs.nethermind.io/nethermind/ethereum-client/json-rpc/parity#parity_getblockreceipts
	ParityGetBlockReceipts
	// EthGetBlockReceipts is a previously non-standard receipt fetching method in the eth namespace,
	// supported by some RPC platforms.
	// This since has been standardized in https://github.com/ethereum/execution-apis/pull/438 and adopted in Geth:
	// https://github.com/ethereum/go-ethereum/pull/27702
	// Available in:
	//   - Alchemy: 500 CU total  (and deprecated)
	//   - QuickNode: 59 credits total       (does not seem to work with block hash arg, inaccurate docs)
	//   - Standard, incl. Geth, Besu and Reth, and Nethermind has a PR in review.
	// Method: eth_getBlockReceipts
	// Params:
	//   - QuickNode: string, "quantity or tag", docs say incl. block hash, but API does not actually accept it.
	//   - Alchemy: string, block hash / num (hex) / block tag
	// Returns: array of receipts
	// See:
	//   - QuickNode: https://www.quicknode.com/docs/ethereum/eth_getBlockReceipts
	//   - Alchemy: https://docs.alchemy.com/reference/eth-getblockreceipts
	// Erigon has this available, but does not support block-hash argument to the method:
	// https://github.com/ledgerwatch/erigon/blob/287a3d1d6c90fc6a7a088b5ae320f93600d5a167/cmd/rpcdaemon/commands/eth_receipts.go#L571
	EthGetBlockReceipts
	// ErigonGetBlockReceiptsByBlockHash is an Erigon-specific receipt fetching method,
	// the same as EthGetBlockReceipts but supporting a block-hash argument.
	// Available in:
	//   - Erigon
	// Method: erigon_getBlockReceiptsByBlockHash
	// Params:
	//  - Erigon: string, hex-encoded block hash
	// Returns:
	//  - Erigon: array of json-ified receipts
	// See:
	// https://github.com/ledgerwatch/erigon/blob/287a3d1d6c90fc6a7a088b5ae320f93600d5a167/cmd/rpcdaemon/commands/erigon_receipts.go#LL391C24-L391C51
	ErigonGetBlockReceiptsByBlockHash

	// Other:
	//  - 250 credits, not supported, strictly worse than other options. In quicknode price-table.
	// qn_getBlockWithReceipts - in price table, ? undocumented, but in quicknode "Single Flight RPC" description
	// qn_getReceipts          - in price table, ? undocumented, but in quicknode "Single Flight RPC" description
	// debug_getBlockReceipts  - ? undocumented, shows up in quicknode price table, not available.
)

// AvailableReceiptsFetchingMethods selects receipt fetching methods based on the RPC provider kind.
func AvailableReceiptsFetchingMethods(kind RPCProviderKind) ReceiptsFetchingMethod {
	switch kind {
	case RPCKindAlchemy:
		return AlchemyGetTransactionReceipts | EthGetBlockReceipts | EthGetTransactionReceiptBatch
	case RPCKindQuickNode:
		return DebugGetRawReceipts | EthGetBlockReceipts | EthGetTransactionReceiptBatch
	case RPCKindInfura:
		return EthGetBlockReceipts | EthGetTransactionReceiptBatch
	case RPCKindParity:
		return ParityGetBlockReceipts | EthGetTransactionReceiptBatch
	case RPCKindNethermind:
		return ParityGetBlockReceipts | EthGetTransactionReceiptBatch
	case RPCKindDebugGeth:
		return DebugGetRawReceipts | EthGetTransactionReceiptBatch
	case RPCKindErigon:
		return ErigonGetBlockReceiptsByBlockHash | EthGetTransactionReceiptBatch
	case RPCKindBasic:
		return EthGetTransactionReceiptBatch
	case RPCKindAny:
		// if it's any kind of RPC provider, then try all methods
		return AlchemyGetTransactionReceipts | EthGetBlockReceipts |
			DebugGetRawReceipts | ErigonGetBlockReceiptsByBlockHash |
			ParityGetBlockReceipts | EthGetTransactionReceiptBatch
	case RPCKindStandard:
		return EthGetBlockReceipts | EthGetTransactionReceiptBatch
	default:
		return EthGetTransactionReceiptBatch
	}
}

// PickBestReceiptsFetchingMethod selects an RPC method that is still available,
// and optimal for fetching the given number of tx receipts from the specified provider kind.
func PickBestReceiptsFetchingMethod(kind RPCProviderKind, available ReceiptsFetchingMethod, txCount uint64) ReceiptsFetchingMethod {
	// If we have optimized methods available, it makes sense to use them, but only if the cost is
	// lower than fetching transactions one by one with the standard receipts RPC method.
	if kind == RPCKindAlchemy {
		if available&AlchemyGetTransactionReceipts != 0 && txCount > 250/15 {
			return AlchemyGetTransactionReceipts
		}
		if available&EthGetBlockReceipts != 0 && txCount > 500/15 {
			return EthGetBlockReceipts
		}
		return EthGetTransactionReceiptBatch
	} else if kind == RPCKindQuickNode {
		if available&DebugGetRawReceipts != 0 {
			return DebugGetRawReceipts
		}
		if available&EthGetBlockReceipts != 0 && txCount > 59/2 {
			return EthGetBlockReceipts
		}
		return EthGetTransactionReceiptBatch
	}
	// in order of preference (based on cost): check available methods
	if available&AlchemyGetTransactionReceipts != 0 {
		return AlchemyGetTransactionReceipts
	}
	if available&DebugGetRawReceipts != 0 {
		return DebugGetRawReceipts
	}
	if available&ErigonGetBlockReceiptsByBlockHash != 0 {
		return ErigonGetBlockReceiptsByBlockHash
	}
	if available&EthGetBlockReceipts != 0 {
		return EthGetBlockReceipts
	}
	if available&ParityGetBlockReceipts != 0 {
		return ParityGetBlockReceipts
	}
	// otherwise fall back on per-tx fetching
	return EthGetTransactionReceiptBatch
}
