package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources"
)

const (
	methodEthGetBlockByNumber = "eth_getBlockByNumber"
	methodDebugChainConfig    = "debug_chainConfig"
	methodDebugSetHead        = "debug_setHead"
)

func GetChainConfig(ctx context.Context, open client.RPC) (cfg *params.ChainConfig, err error) {
	err = open.CallContext(ctx, &cfg, methodDebugChainConfig)
	return
}

type RPCBlock struct {
	types.Header
	Transactions []*types.Transaction `json:"transactions"`
}

func getBlock(ctx context.Context, client client.RPC, method string, tag string) (*types.Block, error) {
	var raw json.RawMessage
	err := client.CallContext(ctx, &raw, method, tag, true)
	if err != nil {
		return nil, err
	}

	var head *types.Header
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, err
	}

	type rpcBlock struct {
		Hash         common.Hash          `json:"hash"`
		Transactions []*types.Transaction `json:"transactions"`
	}

	var body *rpcBlock
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}

	return types.NewBlockWithHeader(head).WithBody(types.Body{Transactions: body.Transactions}), nil
}

func getHeader(ctx context.Context, client client.RPC, method string, tag string) (*types.Header, error) {
	var header *types.Header
	err := client.CallContext(ctx, &header, method, tag, false)
	if err != nil {
		return nil, err
	}
	return header, nil
}

func headSafeFinalized(ctx context.Context, client client.RPC) (head, safe, finalized *types.Header, err error) {
	head, err = getHeader(ctx, client, methodEthGetBlockByNumber, "latest")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get latest: %w", err)
	}
	safe, fin, err := safeFinalized(ctx, client)
	return head, safe, fin, err
}

func headBlockSafeFinalized(ctx context.Context, client client.RPC) (head *types.Block, safe, finalized *types.Header, err error) {
	head, err = getBlock(ctx, client, methodEthGetBlockByNumber, "latest")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get latest: %w", err)
	}
	safe, fin, err := safeFinalized(ctx, client)
	return head, safe, fin, err
}

func safeFinalized(ctx context.Context, client client.RPC) (safe, finalized *types.Header, err error) {
	safe, err = getHeader(ctx, client, methodEthGetBlockByNumber, "safe")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get safe: %w", err)
	}
	finalized, err = getHeader(ctx, client, methodEthGetBlockByNumber, "finalized")
	if err != nil {
		return safe, nil, fmt.Errorf("failed to get finalized: %w", err)
	}
	return safe, finalized, nil
}

func insertBlock(ctx context.Context, client *sources.EngineAPIClient, payloadEnv *eth.ExecutionPayloadEnvelope) error {
	payload := payloadEnv.ExecutionPayload
	payloadResult, err := client.NewPayload(ctx, payload, payloadEnv.ParentBeaconBlockRoot)
	if err != nil {
		return fmt.Errorf("failed to insert block %d: %w", payload.BlockNumber, err)
	}
	if payloadResult.Status != eth.ExecutionValid {
		return fmt.Errorf("block insertion was not valid: %v", payloadResult.ValidationError)
	}
	return nil
}

func updateForkchoice(ctx context.Context, client *sources.EngineAPIClient, head, safe, finalized common.Hash) error {
	res, err := client.ForkchoiceUpdate(ctx, &eth.ForkchoiceState{
		HeadBlockHash:      head,
		SafeBlockHash:      safe,
		FinalizedBlockHash: finalized,
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to update forkchoice with new head %s: %w", head, err)
	}
	if res.PayloadStatus.Status != eth.ExecutionValid {
		return fmt.Errorf("forkchoice update was not valid: %v", res.PayloadStatus.ValidationError)
	}
	return nil
}

func debugSetHead(ctx context.Context, open client.RPC, head uint64) error {
	return open.CallContext(ctx, nil, methodDebugSetHead, hexutil.Uint64(head))
}

type BlockBuildingSettings struct {
	BlockTime uint64
	// skip a block; timestamps will still increase in multiples of BlockTime like L1, but there may be gaps.
	AllowGaps    bool
	Random       common.Hash
	FeeRecipient common.Address
	BuildTime    time.Duration
}

func BuildBlock(ctx context.Context, client *sources.EngineAPIClient, status *StatusData, settings *BlockBuildingSettings) (*eth.ExecutionPayloadEnvelope, error) {
	timestamp := status.Head.Time + settings.BlockTime
	if settings.AllowGaps {
		now := uint64(time.Now().Unix())
		if now > timestamp {
			timestamp = now - ((now - timestamp) % settings.BlockTime)
		}
	}
	attrs := newPayloadAttributes(client.EngineVersionProvider(), timestamp, settings.Random, settings.FeeRecipient)
	pre, err := client.ForkchoiceUpdate(ctx,
		&eth.ForkchoiceState{
			HeadBlockHash:      status.Head.Hash,
			SafeBlockHash:      status.Safe.Hash,
			FinalizedBlockHash: status.Finalized.Hash,
		}, attrs)
	if err != nil {
		return nil, fmt.Errorf("failed to set forkchoice when building new block: %w", err)
	}
	if pre.PayloadStatus.Status != eth.ExecutionValid {
		return nil, fmt.Errorf("pre-block forkchoice update was not valid: %v", pre.PayloadStatus.ValidationError)
	}

	// wait some time for the block to get built
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(settings.BuildTime):
	}

	payload, err := client.GetPayload(ctx, eth.PayloadInfo{ID: *pre.PayloadID, Timestamp: timestamp})
	if err != nil {
		return nil, fmt.Errorf("failed to get payload %v, %d time after instructing engine to build it: %w", pre.PayloadID, settings.BuildTime, err)
	}

	if err := insertBlock(ctx, client, payload); err != nil {
		return nil, err
	}
	if err := updateForkchoice(ctx, client, payload.ExecutionPayload.BlockHash, status.Safe.Hash, status.Finalized.Hash); err != nil {
		return nil, err
	}

	return payload, nil
}

func newPayloadAttributes(evp sources.EngineVersionProvider, timestamp uint64, prevRandao common.Hash, feeRecipient common.Address) *eth.PayloadAttributes {
	pa := &eth.PayloadAttributes{
		Timestamp:             hexutil.Uint64(timestamp),
		PrevRandao:            eth.Bytes32(prevRandao),
		SuggestedFeeRecipient: feeRecipient,
	}

	ver := evp.ForkchoiceUpdatedVersion(pa)
	if ver == eth.FCUV2 || ver == eth.FCUV3 {
		withdrawals := make(types.Withdrawals, 0)
		pa.Withdrawals = &withdrawals
	}
	if ver == eth.FCUV3 {
		pa.ParentBeaconBlockRoot = new(common.Hash)
	}

	return pa
}

func Auto(
	ctx context.Context,
	metrics Metricer,
	client *sources.EngineAPIClient,
	log log.Logger,
	settings *BlockBuildingSettings,
) error {
	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()

	var lastPayload *eth.ExecutionPayload
	var buildErr error
	for {
		select {
		case <-ctx.Done():
			log.Info("context closed", "err", ctx.Err())
			return ctx.Err()
		case now := <-ticker.C:
			blockTime := time.Duration(settings.BlockTime) * time.Second
			lastTime := uint64(0)
			if lastPayload != nil {
				lastTime = uint64(lastPayload.Timestamp)
			}
			buildTriggerTime := time.Unix(int64(lastTime), 0).Add(blockTime - settings.BuildTime)

			if lastPayload == nil || now.After(buildTriggerTime) {
				buildTime := settings.BuildTime
				// don't waste time on trying to include txs if we are lagging behind at least a block,
				// but don't go ham if we are failing to build blocks already.
				if buildErr == nil && now.After(buildTriggerTime.Add(blockTime)) {
					buildTime = 10 * time.Millisecond
				}
				buildErr = nil
				status, err := Status(ctx, client.RPC)
				if err != nil {
					log.Error("failed to get pre-block engine status", "err", err)
					metrics.RecordBlockFail()
					buildErr = err
					continue
				}
				log.Info("status", "head", status.Head, "safe", status.Safe, "finalized", status.Finalized,
					"head_time", status.Head.Time, "txs", status.Txs, "gas", status.Gas, "basefee", status.Gas)

				// On a mocked "beacon epoch transition", update finalization and justification checkpoints.
				// There are no gap slots, so we just go back 32 blocks.
				if status.Head.Number%32 == 0 {
					if status.Safe.Number+32 <= status.Head.Number {
						safe, err := getHeader(ctx, client.RPC, methodEthGetBlockByNumber, hexutil.Uint64(status.Head.Number-32).String())
						if err != nil {
							buildErr = err
							log.Error("failed to find block for new safe block progress", "err", err)
							continue
						}
						status.Safe = eth.L1BlockRef{Hash: safe.Hash(), Number: safe.Number.Uint64(), Time: safe.Time, ParentHash: safe.ParentHash}
					}
					if status.Finalized.Number+32 <= status.Safe.Number {
						finalized, err := getHeader(ctx, client.RPC, methodEthGetBlockByNumber, hexutil.Uint64(status.Safe.Number-32).String())
						if err != nil {
							buildErr = err
							log.Error("failed to find block for new finalized block progress", "err", err)
							continue
						}
						status.Finalized = eth.L1BlockRef{Hash: finalized.Hash(), Number: finalized.Number.Uint64(), Time: finalized.Time, ParentHash: finalized.ParentHash}
					}
				}

				payloadEnv, err := BuildBlock(ctx, client, status, &BlockBuildingSettings{
					BlockTime:    settings.BlockTime,
					AllowGaps:    settings.AllowGaps,
					Random:       settings.Random,
					FeeRecipient: settings.FeeRecipient,
					BuildTime:    buildTime,
				})
				if err != nil {
					buildErr = err
					log.Error("failed to produce block", "err", err)
					metrics.RecordBlockFail()
				} else {
					payload := payloadEnv.ExecutionPayload
					lastPayload = payload
					log.Info("created block", "hash", payload.BlockHash, "number", payload.BlockNumber,
						"timestamp", payload.Timestamp, "txs", len(payload.Transactions),
						"gas", payload.GasUsed, "basefee", payload.BaseFeePerGas)
					basefee := (*uint256.Int)(&payload.BaseFeePerGas).Float64()
					metrics.RecordBlockStats(
						payload.BlockHash, uint64(payload.BlockNumber), uint64(payload.Timestamp),
						uint64(len(payload.Transactions)),
						uint64(payload.GasUsed), basefee)
				}
			}
		}
	}
}

type StatusData struct {
	Head      eth.L1BlockRef `json:"head"`
	Safe      eth.L1BlockRef `json:"safe"`
	Finalized eth.L1BlockRef `json:"finalized"`
	Txs       uint64         `json:"txs"`
	Gas       uint64         `json:"gas"`
	StateRoot common.Hash    `json:"stateRoot"`
	BaseFee   *big.Int       `json:"baseFee"`
}

func Status(ctx context.Context, client client.RPC) (*StatusData, error) {
	head, safe, finalized, err := headBlockSafeFinalized(ctx, client)
	if err != nil {
		return nil, err
	}
	return &StatusData{
		Head:      eth.L1BlockRef{Hash: head.Hash(), Number: head.NumberU64(), Time: head.Time(), ParentHash: head.ParentHash()},
		Safe:      eth.L1BlockRef{Hash: safe.Hash(), Number: safe.Number.Uint64(), Time: safe.Time, ParentHash: safe.ParentHash},
		Finalized: eth.L1BlockRef{Hash: finalized.Hash(), Number: finalized.Number.Uint64(), Time: finalized.Time, ParentHash: finalized.ParentHash},
		Txs:       uint64(len(head.Transactions())),
		Gas:       head.GasUsed(),
		StateRoot: head.Root(),
		BaseFee:   head.BaseFee(),
	}, nil
}

// Copy takes the forkchoice state of copyFrom, and applies it to copyTo, and inserts the head-block.
// The destination engine should then start syncing to this new chain if it has peers to do so.
func Copy(ctx context.Context, copyFrom client.RPC, copyTo *sources.EngineAPIClient) error {
	copyHead, copySafe, copyFinalized, err := headBlockSafeFinalized(ctx, copyFrom)
	if err != nil {
		return err
	}
	payloadEnv, err := blockAsPayloadEnv(copyHead, copyTo.EngineVersionProvider())
	if err != nil {
		return err
	}

	if err := updateForkchoice(ctx, copyTo, copyHead.ParentHash(), copySafe.Hash(), copyFinalized.Hash()); err != nil {
		return err
	}
	if err := insertBlock(ctx, copyTo, payloadEnv); err != nil {
		return err
	}
	if err := updateForkchoice(ctx, copyTo,
		payloadEnv.ExecutionPayload.BlockHash, copySafe.Hash(), copyFinalized.Hash()); err != nil {
		return err
	}
	return nil
}

// CopyPayload takes the execution payload at number & applies it via NewPayload to copyTo
func CopyPayload(ctx context.Context, number uint64, copyFrom client.RPC, copyTo *sources.EngineAPIClient) error {
	copyHead, err := getBlock(ctx, copyFrom, methodEthGetBlockByNumber, hexutil.EncodeUint64(number))
	if err != nil {
		return err
	}
	payloadEnv, err := blockAsPayloadEnv(copyHead, copyTo.EngineVersionProvider())
	if err != nil {
		return err
	}
	if err := insertBlock(ctx, copyTo, payloadEnv); err != nil {
		return err
	}
	return nil
}

func blockAsPayloadEnv(block *types.Block, evp sources.EngineVersionProvider) (*eth.ExecutionPayloadEnvelope, error) {
	var config params.ChainConfig
	// hack: if we're calling at least FCUV2, get empty withdrawals by setting Canyon before the block time
	if v := evp.ForkchoiceUpdatedVersion(&eth.PayloadAttributes{Timestamp: hexutil.Uint64(block.Time())}); v != eth.FCUV1 {
		config.CanyonTime = new(uint64)
	}
	return eth.BlockAsPayloadEnv(block, &config)
}

func SetForkchoice(ctx context.Context, client *sources.EngineAPIClient, finalizedNum, safeNum, unsafeNum uint64) error {
	if unsafeNum < safeNum {
		return fmt.Errorf("cannot set unsafe (%d) < safe (%d)", unsafeNum, safeNum)
	}
	if safeNum < finalizedNum {
		return fmt.Errorf("cannot set safe (%d) < finalized (%d)", safeNum, finalizedNum)
	}
	head, err := getHeader(ctx, client.RPC, methodEthGetBlockByNumber, "latest")
	if err != nil {
		return fmt.Errorf("failed to get latest block: %w", err)
	}
	if unsafeNum > head.Number.Uint64() {
		return fmt.Errorf("cannot set unsafe (%d) > latest (%d)", unsafeNum, head.Number.Uint64())
	}
	finalizedHeader, err := getHeader(ctx, client.RPC, methodEthGetBlockByNumber, hexutil.Uint64(finalizedNum).String())
	if err != nil {
		return fmt.Errorf("failed to get block %d to mark finalized: %w", finalizedNum, err)
	}
	safeHeader, err := getHeader(ctx, client.RPC, methodEthGetBlockByNumber, hexutil.Uint64(safeNum).String())
	if err != nil {
		return fmt.Errorf("failed to get block %d to mark safe: %w", safeNum, err)
	}
	if err := updateForkchoice(ctx, client, head.Hash(), safeHeader.Hash(), finalizedHeader.Hash()); err != nil {
		return fmt.Errorf("failed to update forkchoice: %w", err)
	}
	return nil
}

func SetForkchoiceByHash(ctx context.Context, client *sources.EngineAPIClient, finalized, safe, unsafe common.Hash) error {
	if err := updateForkchoice(ctx, client, unsafe, safe, finalized); err != nil {
		return fmt.Errorf("failed to update forkchoice: %w", err)
	}
	return nil
}

func Rewind(ctx context.Context, lgr log.Logger, client *sources.EngineAPIClient, open client.RPC, to uint64, setHead bool) error {
	unsafe, err := getHeader(ctx, open, methodEthGetBlockByNumber, hexutil.Uint64(to).String())
	if err != nil {
		return fmt.Errorf("failed to get header %d: %w", to, err)
	}
	toUnsafe := eth.HeaderBlockID(unsafe)

	latest, safe, finalized, err := headSafeFinalized(ctx, open)
	if err != nil {
		return fmt.Errorf("failed to get current heads: %w", err)
	}

	// when rewinding, don't increase unsafe/finalized tags
	toSafe, toFinalized := toUnsafe, toUnsafe
	if safe != nil && safe.Number.Uint64() < to {
		toSafe = eth.HeaderBlockID(safe)
	}
	if finalized != nil && finalized.Number.Uint64() < to {
		toFinalized = eth.HeaderBlockID(finalized)
	}

	lgr.Info("Rewinding chain",
		"setHead", setHead,
		"latest", eth.HeaderBlockID(latest),
		"unsafe", toUnsafe,
		"safe", toSafe,
		"finalized", toFinalized,
	)
	if setHead {
		lgr.Debug("Calling "+methodDebugSetHead, "head", to)
		if err := debugSetHead(ctx, open, to); err != nil {
			return fmt.Errorf("failed to setHead %d: %w", to, err)
		}
	}
	return SetForkchoiceByHash(ctx, client, toFinalized.Hash, toSafe.Hash, toUnsafe.Hash)
}

func RawJSONInteraction(ctx context.Context, client client.RPC, method string, args []string, input io.Reader, output io.Writer) error {
	var params []any
	if input != nil {
		r := json.NewDecoder(input)
		for {
			var param json.RawMessage
			if err := r.Decode(&param); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return fmt.Errorf("unexpected error while reading json params: %w", err)
			}
			params = append(params, param)
		}
	} else {
		for _, arg := range args {
			// add quotes to unquoted strings, but not to other json data
			if isUnquotedJsonString(arg) {
				arg = fmt.Sprintf("%q", arg)
			}
			params = append(params, json.RawMessage(arg))
		}
	}
	var result json.RawMessage
	if err := client.CallContext(ctx, &result, method, params...); err != nil {
		return fmt.Errorf("failed RPC call: %w", err)
	}
	if _, err := output.Write(result); err != nil {
		return fmt.Errorf("failed to write RPC output: %w", err)
	}
	return nil
}

func isUnquotedJsonString(v string) bool {
	v = strings.TrimSpace(v)
	// check if empty string (must get quotes)
	if len(v) == 0 {
		return true
	}
	// check if special value
	switch v {
	case "null", "true", "false":
		return false
	}
	// check if it looks like a json structure
	switch v[0] {
	case '[', '{', '"':
		return false
	}
	// check if a number
	var n json.Number
	if err := json.Unmarshal([]byte(v), &n); err == nil {
		return false
	}
	return true
}
