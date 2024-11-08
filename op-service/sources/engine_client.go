package sources

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources/caching"
)

type EngineClientConfig struct {
	L2ClientConfig
}

func EngineClientDefaultConfig(config *rollup.Config) *EngineClientConfig {
	return &EngineClientConfig{
		// engine is trusted, no need to recompute responses etc.
		L2ClientConfig: *L2ClientDefaultConfig(config, true),
	}
}

// EngineClient extends L2Client with engine API bindings.
type EngineClient struct {
	*L2Client
	*EngineAPIClient
}

func NewEngineClient(client client.RPC, log log.Logger, metrics caching.Metrics, config *EngineClientConfig) (*EngineClient, error) {
	l2Client, err := NewL2Client(client, log, metrics, &config.L2ClientConfig)
	if err != nil {
		return nil, err
	}

	engineAPIClient := NewEngineAPIClient(client, log, config.RollupCfg)

	return &EngineClient{
		L2Client:        l2Client,
		EngineAPIClient: engineAPIClient,
	}, nil
}

// EngineAPIClient is an RPC client for the Engine API functions.
type EngineAPIClient struct {
	RPC     client.RPC
	log     log.Logger
	evp     EngineVersionProvider
	timeout time.Duration
}

type EngineVersionProvider interface {
	ForkchoiceUpdatedVersion(attr *eth.PayloadAttributes) eth.EngineAPIMethod
	NewPayloadVersion(timestamp uint64) eth.EngineAPIMethod
	GetPayloadVersion(timestamp uint64) eth.EngineAPIMethod
}

func NewEngineAPIClient(rpc client.RPC, l log.Logger, evp EngineVersionProvider) *EngineAPIClient {
	return &EngineAPIClient{
		RPC:     rpc,
		log:     l,
		evp:     evp,
		timeout: time.Second * 5,
	}
}

func NewEngineAPIClientWithTimeout(rpc client.RPC, l log.Logger, evp EngineVersionProvider, timeout time.Duration) *EngineAPIClient {
	return &EngineAPIClient{
		RPC:     rpc,
		log:     l,
		evp:     evp,
		timeout: timeout,
	}
}

// EngineVersionProvider returns the underlying engine version provider used for
// resolving the correct Engine API versions.
func (s *EngineAPIClient) EngineVersionProvider() EngineVersionProvider { return s.evp }

// ForkchoiceUpdate updates the forkchoice on the execution client. If attributes is not nil, the engine client will also begin building a block
// based on attributes after the new head block and return the payload ID.
// It's the caller's responsibility to check the error type, and in case of an rpc.Error, check the ErrorCode.
func (s *EngineAPIClient) ForkchoiceUpdate(ctx context.Context, fc *eth.ForkchoiceState, attributes *eth.PayloadAttributes) (*eth.ForkchoiceUpdatedResult, error) {
	llog := s.log.New("state", fc)       // local logger
	tlog := llog.New("attr", attributes) // trace logger
	tlog.Trace("Sharing forkchoice-updated signal")
	fcCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	var result eth.ForkchoiceUpdatedResult
	method := s.evp.ForkchoiceUpdatedVersion(attributes)
	err := s.RPC.CallContext(fcCtx, &result, string(method), fc, attributes)
	if err != nil {
		llog.Warn("Failed to share forkchoice-updated signal", "err", err)
		return nil, err
	}
	tlog.Trace("Shared forkchoice-updated signal")
	if attributes != nil { // block building is optional, we only get a payload ID if we are building a block
		tlog.Trace("Received payload id", "payloadId", result.PayloadID)
	}
	return &result, nil
}

// NewPayload executes a full block on the execution engine.
// This returns a PayloadStatusV1 which encodes any validation/processing error,
// and this type of error is kept separate from the returned `error` used for RPC errors, like timeouts.
func (s *EngineAPIClient) NewPayload(ctx context.Context, payload *eth.ExecutionPayload, parentBeaconBlockRoot *common.Hash) (*eth.PayloadStatusV1, error) {
	e := s.log.New("block_hash", payload.BlockHash)
	e.Trace("sending payload for execution")

	execCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	var result eth.PayloadStatusV1

	var err error
	switch method := s.evp.NewPayloadVersion(uint64(payload.Timestamp)); method {
	case eth.NewPayloadV3:
		err = s.RPC.CallContext(execCtx, &result, string(method), payload, []common.Hash{}, parentBeaconBlockRoot)
	case eth.NewPayloadV2:
		err = s.RPC.CallContext(execCtx, &result, string(method), payload)
	default:
		return nil, fmt.Errorf("unsupported NewPayload version: %s", method)
	}

	e.Trace("Received payload execution result", "status", result.Status, "latestValidHash", result.LatestValidHash, "message", result.ValidationError)
	if err != nil {
		e.Error("Payload execution failed", "err", err)
		return nil, fmt.Errorf("failed to execute payload: %w", err)
	}
	return &result, nil
}

// GetPayload gets the execution payload associated with the PayloadId.
// It's the caller's responsibility to check the error type, and in case of an rpc.Error, check the ErrorCode.
func (s *EngineAPIClient) GetPayload(ctx context.Context, payloadInfo eth.PayloadInfo) (*eth.ExecutionPayloadEnvelope, error) {
	e := s.log.New("payload_id", payloadInfo.ID)
	e.Trace("getting payload")
	var result eth.ExecutionPayloadEnvelope
	method := s.evp.GetPayloadVersion(payloadInfo.Timestamp)
	err := s.RPC.CallContext(ctx, &result, string(method), payloadInfo.ID)
	if err != nil {
		e.Warn("Failed to get payload", "payload_id", payloadInfo.ID, "err", err)
		return nil, err
	}
	e.Trace("Received payload")
	return &result, nil
}

func (s *EngineAPIClient) SignalSuperchainV1(ctx context.Context, recommended, required params.ProtocolVersion) (params.ProtocolVersion, error) {
	var result params.ProtocolVersion
	err := s.RPC.CallContext(ctx, &result, "engine_signalSuperchainV1", &catalyst.SuperchainSignal{
		Recommended: recommended,
		Required:    required,
	})
	return result, err
}
