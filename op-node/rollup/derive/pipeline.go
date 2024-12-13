package derive

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type Metrics interface {
	RecordL1Ref(name string, ref eth.L1BlockRef)
	RecordL2Ref(name string, ref eth.L2BlockRef)
	RecordChannelInputBytes(inputCompressedBytes int)
	RecordHeadChannelOpened()
	RecordChannelTimedOut()
	RecordFrame()
	RecordDerivedBatches(batchType string)
	SetDerivationIdle(idle bool)
	RecordPipelineReset()
}

type L1Fetcher interface {
	L1BlockRefByLabel(ctx context.Context, label eth.BlockLabel) (eth.L1BlockRef, error)
	L1BlockRefByNumberFetcher
	L1BlockRefByHashFetcher
	L1ReceiptsFetcher
	L1TransactionFetcher
}

type ResettableStage interface {
	// Reset resets a pull stage. `base` refers to the L1 Block Reference to reset to, with corresponding configuration.
	Reset(ctx context.Context, base eth.L1BlockRef, baseCfg eth.SystemConfig) error
}

// A ChannelFlusher flushes all internal state related to the current channel and then
// calls FlushChannel on the stage it owns. Note that this is in contrast to Reset, which
// is called by the owning Pipeline in a loop over all stages.
type ChannelFlusher interface {
	FlushChannel()
}

type ForkTransformer interface {
	Transform(rollup.ForkName)
}

type L2Source interface {
	PayloadByHash(context.Context, common.Hash) (*eth.ExecutionPayloadEnvelope, error)
	PayloadByNumber(context.Context, uint64) (*eth.ExecutionPayloadEnvelope, error)
	L2BlockRefByLabel(ctx context.Context, label eth.BlockLabel) (eth.L2BlockRef, error)
	L2BlockRefByHash(ctx context.Context, l2Hash common.Hash) (eth.L2BlockRef, error)
	L2BlockRefByNumber(ctx context.Context, num uint64) (eth.L2BlockRef, error)
	SystemConfigL2Fetcher
}

// DerivationPipeline is updated with new L1 data, and the Step() function can be iterated on to generate attributes
type DerivationPipeline struct {
	log       log.Logger
	rollupCfg *rollup.Config
	l1Fetcher L1Fetcher
	altDA     AltDAInputFetcher

	l2 L2Source

	// Index of the stage that is currently being reset.
	// >= len(stages) if no additional resetting is required
	resetting int
	stages    []ResettableStage

	// Special stages to keep track of
	traversal *L1Traversal

	attrib *AttributesQueue

	// L1 block that the next returned attributes are derived from, i.e. at the L2-end of the pipeline.
	origin         eth.L1BlockRef
	resetL2Safe    eth.L2BlockRef
	resetSysConfig eth.SystemConfig
	engineIsReset  bool

	metrics Metrics
}

// NewDerivationPipeline creates a DerivationPipeline, to turn L1 data into L2 block-inputs.
func NewDerivationPipeline(log log.Logger, rollupCfg *rollup.Config, l1Fetcher L1Fetcher, l1Blobs L1BlobsFetcher,
	altDA AltDAInputFetcher, l2Source L2Source, metrics Metrics,
) *DerivationPipeline {
	spec := rollup.NewChainSpec(rollupCfg)
	// Pull stages
	l1Traversal := NewL1Traversal(log, rollupCfg, l1Fetcher)
	dataSrc := NewDataSourceFactory(log, rollupCfg, l1Fetcher, l1Blobs, altDA) // auxiliary stage for L1Retrieval
	l1Src := NewL1Retrieval(log, dataSrc, l1Traversal)
	frameQueue := NewFrameQueue(log, rollupCfg, l1Src)
	channelMux := NewChannelMux(log, spec, frameQueue, metrics)
	chInReader := NewChannelInReader(rollupCfg, log, channelMux, metrics)
	batchMux := NewBatchMux(log, rollupCfg, chInReader, l2Source)
	attrBuilder := NewFetchingAttributesBuilder(rollupCfg, l1Fetcher, l2Source)
	attributesQueue := NewAttributesQueue(log, rollupCfg, attrBuilder, batchMux)

	// Reset from ResetEngine then up from L1 Traversal. The stages do not talk to each other during
	// the ResetEngine, but after the ResetEngine, this is the order in which the stages could talk to each other.
	// Note: The ResetEngine is the only reset that can fail.
	stages := []ResettableStage{l1Traversal, l1Src, altDA, frameQueue, channelMux, chInReader, batchMux, attributesQueue}

	return &DerivationPipeline{
		log:       log,
		rollupCfg: rollupCfg,
		l1Fetcher: l1Fetcher,
		altDA:     altDA,
		resetting: 0,
		stages:    stages,
		metrics:   metrics,
		traversal: l1Traversal,
		attrib:    attributesQueue,
		l2:        l2Source,
	}
}

// DerivationReady returns true if the derivation pipeline is ready to be used.
// When it's being reset its state is inconsistent, and should not be used externally.
func (dp *DerivationPipeline) DerivationReady() bool {
	return dp.engineIsReset && dp.resetting > 0
}

func (dp *DerivationPipeline) Reset() {
	dp.resetting = 0
	dp.resetSysConfig = eth.SystemConfig{}
	dp.resetL2Safe = eth.L2BlockRef{}
	dp.engineIsReset = false
}

func (dp *DerivationPipeline) DepositsOnlyAttributes(parent eth.BlockID, derivedFrom eth.L1BlockRef) (*AttributesWithParent, error) {
	return dp.attrib.DepositsOnlyAttributes(parent, derivedFrom)
}

// Origin is the L1 block of the inner-most stage of the derivation pipeline,
// i.e. the L1 chain up to and including this point included and/or produced all the safe L2 blocks.
func (dp *DerivationPipeline) Origin() eth.L1BlockRef {
	return dp.origin
}

// Step tries to progress the buffer.
// An EOF is returned if the pipeline is blocked by waiting for new L1 data.
// If ctx errors no error is returned, but the step may exit early in a state that can still be continued.
// Any other error is critical and the derivation pipeline should be reset.
// An error is expected when the underlying source closes.
// When Step returns nil, it should be called again, to continue the derivation process.
func (dp *DerivationPipeline) Step(ctx context.Context, pendingSafeHead eth.L2BlockRef) (outAttrib *AttributesWithParent, outErr error) {
	defer dp.metrics.RecordL1Ref("l1_derived", dp.Origin())

	dp.metrics.SetDerivationIdle(false)
	defer func() {
		if outErr == io.EOF || errors.Is(outErr, EngineELSyncing) {
			dp.metrics.SetDerivationIdle(true)
		}
	}()

	// if any stages need to be reset, do that first.
	if dp.resetting < len(dp.stages) {
		if !dp.engineIsReset {
			return nil, NewResetError(errors.New("cannot continue derivation until Engine has been reset"))
		}

		// After the Engine has been reset to ensure it is derived from the canonical L1 chain,
		// we still need to internally rewind the L1 traversal further,
		// so we can read all the L2 data necessary for constructing the next batches that come after the safe head.
		if pendingSafeHead != dp.resetL2Safe {
			if err := dp.initialReset(ctx, pendingSafeHead); err != nil {
				return nil, fmt.Errorf("failed initial reset work: %w", err)
			}
		}

		if err := dp.stages[dp.resetting].Reset(ctx, dp.origin, dp.resetSysConfig); err == io.EOF {
			dp.log.Debug("reset of stage completed", "stage", dp.resetting, "origin", dp.origin)
			dp.resetting += 1
			return nil, nil
		} else if err != nil {
			return nil, fmt.Errorf("stage %d failed resetting: %w", dp.resetting, err)
		} else {
			return nil, nil
		}
	}

	prevOrigin := dp.origin
	newOrigin := dp.attrib.Origin()
	if prevOrigin != newOrigin {
		// Check if the L2 unsafe head origin is consistent with the new origin
		if err := VerifyNewL1Origin(ctx, prevOrigin, dp.l1Fetcher, newOrigin); err != nil {
			return nil, fmt.Errorf("failed to verify L1 origin transition: %w", err)
		}
		dp.transformStages(prevOrigin, newOrigin)
		dp.origin = newOrigin
	}

	if attrib, err := dp.attrib.NextAttributes(ctx, pendingSafeHead); err == nil {
		return attrib, nil
	} else if err == io.EOF {
		// If every stage has returned io.EOF, try to advance the L1 Origin
		return nil, dp.traversal.AdvanceL1Block(ctx)
	} else if errors.Is(err, EngineELSyncing) {
		return nil, err
	} else {
		return nil, fmt.Errorf("derivation failed: %w", err)
	}
}

// initialReset does the initial reset work of finding the L1 point to rewind back to
func (dp *DerivationPipeline) initialReset(ctx context.Context, resetL2Safe eth.L2BlockRef) error {
	dp.log.Info("Rewinding derivation-pipeline L1 traversal to handle reset")

	dp.metrics.RecordPipelineReset()
	spec := rollup.NewChainSpec(dp.rollupCfg)

	// Walk back L2 chain to find the L1 origin that is old enough to start buffering channel data from.
	pipelineL2 := resetL2Safe
	l1Origin := resetL2Safe.L1Origin

	pipelineOrigin, err := dp.l1Fetcher.L1BlockRefByHash(ctx, l1Origin.Hash)
	if err != nil {
		return NewTemporaryError(fmt.Errorf("failed to fetch the new L1 progress: origin: %s; err: %w", pipelineL2.L1Origin, err))
	}

	for {
		afterL2Genesis := pipelineL2.Number > dp.rollupCfg.Genesis.L2.Number
		afterL1Genesis := pipelineL2.L1Origin.Number > dp.rollupCfg.Genesis.L1.Number
		afterChannelTimeout := pipelineL2.L1Origin.Number+spec.ChannelTimeout(pipelineOrigin.Time) > l1Origin.Number
		if afterL2Genesis && afterL1Genesis && afterChannelTimeout {
			parent, err := dp.l2.L2BlockRefByHash(ctx, pipelineL2.ParentHash)
			if err != nil {
				return NewResetError(fmt.Errorf("failed to fetch L2 parent block %s", pipelineL2.ParentID()))
			}
			pipelineL2 = parent
			pipelineOrigin, err = dp.l1Fetcher.L1BlockRefByHash(ctx, pipelineL2.L1Origin.Hash)
			if err != nil {
				return NewTemporaryError(fmt.Errorf("failed to fetch the new L1 progress: origin: %s; err: %w", pipelineL2.L1Origin, err))
			}
		} else {
			break
		}
	}

	sysCfg, err := dp.l2.SystemConfigByL2Hash(ctx, pipelineL2.Hash)
	if err != nil {
		return NewTemporaryError(fmt.Errorf("failed to fetch L1 config of L2 block %s: %w", pipelineL2.ID(), err))
	}

	dp.origin = pipelineOrigin
	dp.resetSysConfig = sysCfg
	dp.resetL2Safe = resetL2Safe
	return nil
}

func (db *DerivationPipeline) transformStages(oldOrigin, newOrigin eth.L1BlockRef) {
	fork := db.rollupCfg.IsActivationBlock(oldOrigin.Time, newOrigin.Time)
	if fork == "" {
		return
	}

	db.log.Info("Transforming stages", "fork", fork)
	for _, stage := range db.stages {
		if tf, ok := stage.(ForkTransformer); ok {
			tf.Transform(fork)
		}
	}
}

func (dp *DerivationPipeline) ConfirmEngineReset() {
	dp.engineIsReset = true
}
