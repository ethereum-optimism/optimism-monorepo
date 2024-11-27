package testutils

import (
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// TestDerivationMetrics implements the metrics used in the derivation pipeline as no-op operations.
// Optionally a test may hook into the metrics
type TestDerivationMetrics struct {
	FnRecordL1ReorgDepth      func(d uint64)
	FnRecordL1Ref             func(name string, ref eth.L1BlockRef)
	FnRecordL2Ref             func(name string, ref eth.L2BlockRef)
	FnRecordUnsafePayloads    func(length uint64, memSize uint64, next eth.BlockID)
	FnRecordChannelInputBytes func(inputCompressedBytes int)
	FnRecordChannelTimedOut   func()
}

func (t *TestDerivationMetrics) CountSequencedTxsInBlock(txns int, deposits int) {
}

func (t *TestDerivationMetrics) RecordSequencerBuildingDiffTime(duration time.Duration) {
}

func (t *TestDerivationMetrics) RecordSequencerSealingTime(duration time.Duration) {
}

func (t *TestDerivationMetrics) RecordL1ReorgDepth(d uint64) {
	if t.FnRecordL1ReorgDepth != nil {
		t.FnRecordL1ReorgDepth(d)
	}
}

func (t *TestDerivationMetrics) RecordL1Ref(name string, ref eth.L1BlockRef) {
	if t.FnRecordL1Ref != nil {
		t.FnRecordL1Ref(name, ref)
	}
}

func (t *TestDerivationMetrics) RecordL2Ref(name string, ref eth.L2BlockRef) {
	if t.FnRecordL2Ref != nil {
		t.FnRecordL2Ref(name, ref)
	}
}

func (t *TestDerivationMetrics) RecordUnsafePayloadsBuffer(length uint64, memSize uint64, next eth.BlockID) {
	if t.FnRecordUnsafePayloads != nil {
		t.FnRecordUnsafePayloads(length, memSize, next)
	}
}

func (t *TestDerivationMetrics) RecordChannelInputBytes(inputCompressedBytes int) {
	if t.FnRecordChannelInputBytes != nil {
		t.FnRecordChannelInputBytes(inputCompressedBytes)
	}
}

func (t *TestDerivationMetrics) RecordHeadChannelOpened() {
}

func (t *TestDerivationMetrics) RecordChannelTimedOut() {
	if t.FnRecordChannelTimedOut != nil {
		t.FnRecordChannelTimedOut()
	}
}

func (t *TestDerivationMetrics) RecordFrame() {
}

func (n *TestDerivationMetrics) RecordDerivedBatches(batchType string) {
}

type TestRPCMetrics struct{}

func (n *TestRPCMetrics) RecordRPCServerRequest(method string) func() {
	return func() {}
}

func (n *TestRPCMetrics) RecordRPCClientRequest(method string) func(err error) {
	return func(err error) {}
}

func (n *TestRPCMetrics) RecordRPCClientResponse(method string, err error) {}

func (t *TestDerivationMetrics) SetDerivationIdle(idle bool) {}

func (t *TestDerivationMetrics) RecordPipelineReset() {
}
