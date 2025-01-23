package super

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	interopTypes "github.com/ethereum-optimism/optimism/op-program/client/interop/types"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
)

var (
	ErrGetStepData = errors.New("GetStepData not supported")
	ErrIndexTooBig = errors.New("trace index is greater than max uint64")

	InvalidTransition     = []byte("invalid")
	InvalidTransitionHash = crypto.Keccak256Hash(InvalidTransition)
)

const (
	StepsPerTimestamp = 1024
)

type PreimagePrestateProvider interface {
	types.PrestateProvider
	AbsolutePreState(ctx context.Context) (eth.Super, error)
}
type RootProvider interface {
	SuperRootAtTimestamp(ctx context.Context, timestamp hexutil.Uint64) (eth.SuperRootResponse, error)
}

type SuperTraceProvider struct {
	PreimagePrestateProvider
	logger             log.Logger
	rootProvider       RootProvider
	prestateTimestamp  uint64
	poststateTimestamp uint64
	l1Head             eth.BlockID
	gameDepth          types.Depth
}

func NewSuperTraceProvider(logger log.Logger, prestateProvider PreimagePrestateProvider, rootProvider RootProvider, l1Head eth.BlockID, gameDepth types.Depth, prestateTimestamp, poststateTimestamp uint64) *SuperTraceProvider {
	return &SuperTraceProvider{
		PreimagePrestateProvider: prestateProvider,
		logger:                   logger,
		rootProvider:             rootProvider,
		prestateTimestamp:        prestateTimestamp,
		poststateTimestamp:       poststateTimestamp,
		l1Head:                   l1Head,
		gameDepth:                gameDepth,
	}
}

func (s *SuperTraceProvider) Get(ctx context.Context, pos types.Position) (common.Hash, error) {
	preimage, err := s.GetPreimageBytes(ctx, pos)
	if err != nil {
		return common.Hash{}, err
	}
	return crypto.Keccak256Hash(preimage), nil
}

func (s *SuperTraceProvider) GetPreimageBytes(ctx context.Context, pos types.Position) ([]byte, error) {
	// Find the timestamp and step at position
	timestamp, step, err := s.ComputeStep(pos)
	if err != nil {
		return nil, err
	}
	s.logger.Info("Getting claim", "pos", pos.ToGIndex(), "timestamp", timestamp, "step", step)
	if step == 0 {
		root, err := s.rootProvider.SuperRootAtTimestamp(ctx, hexutil.Uint64(timestamp))
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve super root at timestamp %v: %w", timestamp, err)
		}
		return responseToSuper(root).Marshal(), nil
	}
	// Fetch the super root at the next timestamp since we are part way through the transition to it
	prevRoot, err := s.rootProvider.SuperRootAtTimestamp(ctx, hexutil.Uint64(timestamp))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve super root at timestamp %v: %w", timestamp, err)
	}
	nextTimestamp := timestamp + 1
	nextRoot, err := s.rootProvider.SuperRootAtTimestamp(ctx, hexutil.Uint64(nextTimestamp))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve super root at timestamp %v: %w", nextTimestamp, err)
	}
	superV1 := responseToSuper(prevRoot)
	expectedState := interopTypes.TransitionState{
		SuperRoot:       superV1.Marshal(),
		PendingProgress: make([]interopTypes.OptimisticBlock, 0, step),
		Step:            step,
	}
	for i := uint64(0); i < min(step, uint64(len(nextRoot.Chains))); i++ {
		rawOutput, err := eth.UnmarshalOutput(nextRoot.Chains[i].Pending)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal pending output %v at timestamp %v: %w", i, nextTimestamp, err)
		}
		output, ok := rawOutput.(*eth.OutputV0)
		if !ok {
			return nil, fmt.Errorf("unsupported output version %v at timestamp %v", output.Version(), nextTimestamp)
		}
		expectedState.PendingProgress = append(expectedState.PendingProgress, interopTypes.OptimisticBlock{
			BlockHash:  output.BlockHash,
			OutputRoot: eth.OutputRoot(output),
		})
	}
	return expectedState.Marshal(), nil
}

func (s *SuperTraceProvider) ComputeStep(pos types.Position) (timestamp uint64, step uint64, err error) {
	bigIdx := pos.TraceIndex(s.gameDepth)
	if !bigIdx.IsUint64() {
		err = fmt.Errorf("%w: %v", ErrIndexTooBig, bigIdx)
		return
	}

	traceIdx := bigIdx.Uint64() + 1
	timestampIncrements := traceIdx / StepsPerTimestamp
	timestamp = s.prestateTimestamp + timestampIncrements
	if timestamp >= s.poststateTimestamp { // Apply trace extension once the claimed timestamp is reached
		timestamp = s.poststateTimestamp
		step = 0
	} else {
		step = traceIdx % StepsPerTimestamp
	}
	return
}

func (s *SuperTraceProvider) GetStepData(_ context.Context, _ types.Position) (prestate []byte, proofData []byte, preimageData *types.PreimageOracleData, err error) {
	return nil, nil, nil, ErrGetStepData
}

func (s *SuperTraceProvider) GetL2BlockNumberChallenge(_ context.Context) (*types.InvalidL2BlockNumberChallenge, error) {
	// Never need to challenge L2 block number for super root games.
	return nil, types.ErrL2BlockNumberValid
}

var _ types.TraceProvider = (*SuperTraceProvider)(nil)
