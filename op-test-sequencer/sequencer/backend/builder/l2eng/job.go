package l2eng

import (
	"context"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type Engine interface {
	ForkchoiceUpdate(ctx context.Context, fc *eth.ForkchoiceState, attributes *eth.PayloadAttributes) (*eth.ForkchoiceUpdatedResult, error)
	NewPayload(ctx context.Context, payload *eth.ExecutionPayload, parentBeaconBlockRoot *common.Hash) (*eth.PayloadStatusV1, error)
	GetPayload(ctx context.Context, payloadInfo eth.PayloadInfo) (*eth.ExecutionPayloadEnvelope, error)
}

type Job struct {
	id seqtypes.BuildJobID

	eng Engine

	payloadInfo eth.PayloadInfo
}

func (job *Job) ID() seqtypes.BuildJobID {
	return job.id
}

func (job *Job) Cancel(ctx context.Context) error {
	_, err := job.eng.GetPayload(ctx, job.payloadInfo)
	if err != nil {
		// TODO not-found error is acceptable
		return err
	}
	return nil
}

func (job *Job) Seal(ctx context.Context) (eth.BlockRef, error) {
	envelope, err := job.eng.GetPayload(ctx, job.payloadInfo)
	if err != nil {
		return eth.BlockRef{}, err
	}
	// TODO handle envelope
	_ = envelope
	return eth.BlockRef{}, nil
}

func (job *Job) String() string {
	return job.id.String()
}

var _ work.BuildJob = (*Job)(nil)
