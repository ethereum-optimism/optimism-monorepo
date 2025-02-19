package eth

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type BlockVersion int

const ( // iota is reset to 0
	BlockV1 BlockVersion = iota
	BlockV2
	BlockV3
	BlockV4
)

// ExecutionPayload and ExecutionPayloadEnvelope are the only SSZ types we have to marshal/unmarshal,
// so instead of importing a SSZ lib we implement the bare minimum.
// This is more efficient than RLP, and matches the L1 consensus-layer encoding of ExecutionPayload.

var (
	// The payloads are small enough to read and write at once.
	// But this happens often enough that we want to avoid re-allocating buffers for this.
	payloadBufPool = sync.Pool{New: func() any {
		x := make([]byte, 0, 100_000)
		return &x
	}}

	// ErrExtraDataTooLarge occurs when the ExecutionPayload's ExtraData field
	// is too large to be properly represented in SSZ.
	ErrExtraDataTooLarge = errors.New("extra data too large")

	ErrBadTransactionOffset = errors.New("transactions offset is smaller than extra data offset, aborting")
	ErrBadWithdrawalsOffset = errors.New("withdrawals offset is smaller than transaction offset, aborting")
	ErrBadExtraDataOffset   = errors.New("unexpected extra data offset")
	ErrScopeTooSmall        = errors.New("scope too small to decode execution payload")

	ErrMissingData = errors.New("execution payload envelope is missing data")
)

const (
	// All fields (4s are offsets to dynamic data)
	blockV1FixedPart = 32 + 20 + 32 + 32 + 256 + 32 + 8 + 8 + 8 + 8 + 4 + 32 + 32 + 4

	// V1 + Withdrawals offset
	blockV2FixedPart = blockV1FixedPart + 4

	// V2 + BlobGasUsed + ExcessBlobGas
	blockV3FixedPart = blockV2FixedPart + 8 + 8

	// V3 + WithdrawalsRoot
	blockV4FixedPart = blockV3FixedPart + 32

	withdrawalSize = 8 + 8 + 20 + 8

	// MAX_TRANSACTIONS_PER_PAYLOAD in consensus spec
	// https://github.com/ethereum/consensus-specs/blob/ef434e87165e9a4c82a99f54ffd4974ae113f732/specs/bellatrix/beacon-chain.md#execution
	maxTransactionsPerPayload = 1 << 20

	// MAX_WITHDRAWALS_PER_PAYLOAD	 in consensus spec
	// https://github.com/ethereum/consensus-specs/blob/dev/specs/capella/beacon-chain.md#execution
	maxWithdrawalsPerPayload = 1 << 4
)

func (v BlockVersion) HasBlobProperties() bool {
	return v == BlockV3 || v == BlockV4
}

func (v BlockVersion) HasWithdrawals() bool {
	return v == BlockV2 || v == BlockV3 || v == BlockV4
}

func (v BlockVersion) HasParentBeaconBlockRoot() bool {
	return v == BlockV3 || v == BlockV4
}

func (v BlockVersion) HasWithdrawalsRoot() bool {
	return v == BlockV4
}

// ImpliesRequestsRoot returns whether an empty requests-root should be assumed to be there, but not encoded.
func (v BlockVersion) ImpliesRequestsRoot() bool {
	return v == BlockV4
}

func executionPayloadFixedPart(version BlockVersion) uint32 {
	if version == BlockV4 {
		return blockV4FixedPart
	} else if version == BlockV3 {
		return blockV3FixedPart
	} else if version == BlockV2 {
		return blockV2FixedPart
	} else {
		return blockV1FixedPart
	}
}

func (payload *ExecutionPayload) inferVersion() BlockVersion {
	if payload.WithdrawalsRoot != nil && *payload.WithdrawalsRoot != types.EmptyWithdrawalsHash {
		return BlockV4
	} else if payload.ExcessBlobGas != nil && payload.BlobGasUsed != nil {
		return BlockV3
	} else if payload.Withdrawals != nil {
		return BlockV2
	} else {
		return BlockV1
	}
}

func (payload *ExecutionPayload) SizeSSZ() (full uint32) {
	return executionPayloadFixedPart(payload.inferVersion()) + uint32(len(payload.ExtraData)) + payload.transactionSize() + payload.withdrawalSize()
}

func (payload *ExecutionPayload) withdrawalSize() uint32 {
	if payload.Withdrawals == nil {
		return 0
	}

	return uint32(len(*payload.Withdrawals) * withdrawalSize)
}

func (payload *ExecutionPayload) transactionSize() uint32 {
	// One offset to each transaction
	result := uint32(len(payload.Transactions)) * 4
	// Each transaction
	for _, tx := range payload.Transactions {
		result += uint32(len(tx))
	}
	return result
}

// marshalBytes32LE returns the value of z as a 32-byte little-endian array.
func marshalBytes32LE(out []byte, z *Uint256Quantity) {
	_ = out[31] // bounds check hint to compiler
	binary.LittleEndian.PutUint64(out[0:8], z[0])
	binary.LittleEndian.PutUint64(out[8:16], z[1])
	binary.LittleEndian.PutUint64(out[16:24], z[2])
	binary.LittleEndian.PutUint64(out[24:32], z[3])
}

func unmarshalBytes32LE(in []byte, z *Uint256Quantity) {
	_ = in[31] // bounds check hint to compiler
	z[0] = binary.LittleEndian.Uint64(in[0:8])
	z[1] = binary.LittleEndian.Uint64(in[8:16])
	z[2] = binary.LittleEndian.Uint64(in[16:24])
	z[3] = binary.LittleEndian.Uint64(in[24:32])
}

// MarshalSSZ encodes the ExecutionPayload as SSZ type
func (payload *ExecutionPayload) MarshalSSZ(w io.Writer) (n int, err error) {
	fixedSize := executionPayloadFixedPart(payload.inferVersion())
	transactionSize := payload.transactionSize()

	// Cast to uint32 to enable 32-bit MIPS support where math.MaxUint32-executionPayloadFixedPart is too big for int
	// In that case, len(payload.ExtraData) can't be longer than an int so this is always false anyway.
	extraDataSize := uint32(len(payload.ExtraData))
	if extraDataSize > math.MaxUint32-fixedSize {
		return 0, ErrExtraDataTooLarge
	}

	scope := payload.SizeSSZ()

	buf := *payloadBufPool.Get().(*[]byte)
	if uint32(cap(buf)) < scope {
		buf = make([]byte, scope)
	} else {
		buf = buf[:scope]
	}
	defer payloadBufPool.Put(&buf)

	offset := uint32(0)
	copy(buf[offset:offset+32], payload.ParentHash[:])
	offset += 32
	copy(buf[offset:offset+20], payload.FeeRecipient[:])
	offset += 20
	copy(buf[offset:offset+32], payload.StateRoot[:])
	offset += 32
	copy(buf[offset:offset+32], payload.ReceiptsRoot[:])
	offset += 32
	copy(buf[offset:offset+256], payload.LogsBloom[:])
	offset += 256
	copy(buf[offset:offset+32], payload.PrevRandao[:])
	offset += 32
	binary.LittleEndian.PutUint64(buf[offset:offset+8], uint64(payload.BlockNumber))
	offset += 8
	binary.LittleEndian.PutUint64(buf[offset:offset+8], uint64(payload.GasLimit))
	offset += 8
	binary.LittleEndian.PutUint64(buf[offset:offset+8], uint64(payload.GasUsed))
	offset += 8
	binary.LittleEndian.PutUint64(buf[offset:offset+8], uint64(payload.Timestamp))
	offset += 8
	// offset to ExtraData
	binary.LittleEndian.PutUint32(buf[offset:offset+4], fixedSize)
	offset += 4
	marshalBytes32LE(buf[offset:offset+32], &payload.BaseFeePerGas)
	offset += 32
	copy(buf[offset:offset+32], payload.BlockHash[:])
	offset += 32
	// offset to Transactions
	binary.LittleEndian.PutUint32(buf[offset:offset+4], fixedSize+extraDataSize)
	offset += 4

	if payload.Withdrawals == nil && offset != fixedSize {
		panic("transactions - fixed part size is inconsistent")
	}

	if payload.Withdrawals != nil {
		binary.LittleEndian.PutUint32(buf[offset:offset+4], fixedSize+extraDataSize+transactionSize)
		offset += 4
	}

	payloadVersion := payload.inferVersion()
	if payloadVersion.HasBlobProperties() {
		if payload.BlobGasUsed == nil || payload.ExcessBlobGas == nil {
			return 0, errors.New("cannot encode ecotone payload without dencun header attributes")
		}
		binary.LittleEndian.PutUint64(buf[offset:offset+8], uint64(*payload.BlobGasUsed))
		offset += 8
		binary.LittleEndian.PutUint64(buf[offset:offset+8], uint64(*payload.ExcessBlobGas))
		offset += 8
	}

	if payloadVersion.HasWithdrawalsRoot() {
		if payload.WithdrawalsRoot == nil {
			return 0, errors.New("cannot encode Isthmus payload without withdrawals root")
		}
		copy(buf[offset:offset+32], (*payload.WithdrawalsRoot)[:])
		offset += 32
	}

	if payload.Withdrawals != nil && offset != fixedSize {
		panic("withdrawals - fixed part size is inconsistent")
	}

	// dynamic value 1: ExtraData
	copy(buf[offset:offset+extraDataSize], payload.ExtraData[:])
	offset += extraDataSize
	// dynamic value 2: Transactions
	marshalTransactions(buf[offset:offset+transactionSize], payload.Transactions)
	offset += transactionSize
	// dynamic value 3: Withdrawals
	if payload.Withdrawals != nil {
		marshalWithdrawals(buf[offset:], *payload.Withdrawals)
	}

	return w.Write(buf)
}

func marshalWithdrawals(out []byte, withdrawals types.Withdrawals) {
	offset := uint32(0)

	for _, withdrawal := range withdrawals {
		binary.LittleEndian.PutUint64(out[offset:offset+8], withdrawal.Index)
		offset += 8
		binary.LittleEndian.PutUint64(out[offset:offset+8], withdrawal.Validator)
		offset += 8
		copy(out[offset:offset+20], withdrawal.Address[:])
		offset += 20
		binary.LittleEndian.PutUint64(out[offset:offset+8], withdrawal.Amount)
		offset += 8
	}
}

func marshalTransactions(out []byte, txs []Data) {
	offset := uint32(0)
	txOffset := uint32(len(txs)) * 4
	for _, tx := range txs {
		binary.LittleEndian.PutUint32(out[offset:offset+4], txOffset)
		offset += 4
		nextTxOffset := txOffset + uint32(len(tx))
		copy(out[txOffset:nextTxOffset], tx)
		txOffset = nextTxOffset
	}
}

// UnmarshalSSZ decodes the ExecutionPayload as SSZ type
func (payload *ExecutionPayload) UnmarshalSSZ(version BlockVersion, scope uint32, r io.Reader) error {
	fixedSize := executionPayloadFixedPart(version)

	if scope < fixedSize {
		return fmt.Errorf("scope too small to decode execution payload: %d, version is: %v", scope, version)
	}

	buf := *payloadBufPool.Get().(*[]byte)
	if uint32(cap(buf)) < scope {
		buf = make([]byte, scope)
	} else {
		buf = buf[:scope]
	}
	defer payloadBufPool.Put(&buf)

	if _, err := io.ReadFull(r, buf); err != nil {
		return fmt.Errorf("failed to read fixed-size part of ExecutionPayload: %w", err)
	}
	offset := uint32(0)
	copy(payload.ParentHash[:], buf[offset:offset+32])
	offset += 32
	copy(payload.FeeRecipient[:], buf[offset:offset+20])
	offset += 20
	copy(payload.StateRoot[:], buf[offset:offset+32])
	offset += 32
	copy(payload.ReceiptsRoot[:], buf[offset:offset+32])
	offset += 32
	copy(payload.LogsBloom[:], buf[offset:offset+256])
	offset += 256
	copy(payload.PrevRandao[:], buf[offset:offset+32])
	offset += 32
	payload.BlockNumber = Uint64Quantity(binary.LittleEndian.Uint64(buf[offset : offset+8]))
	offset += 8
	payload.GasLimit = Uint64Quantity(binary.LittleEndian.Uint64(buf[offset : offset+8]))
	offset += 8
	payload.GasUsed = Uint64Quantity(binary.LittleEndian.Uint64(buf[offset : offset+8]))
	offset += 8
	payload.Timestamp = Uint64Quantity(binary.LittleEndian.Uint64(buf[offset : offset+8]))
	offset += 8
	extraDataOffset := binary.LittleEndian.Uint32(buf[offset : offset+4])
	if extraDataOffset != fixedSize {
		return fmt.Errorf("%w: %d <> %d", ErrBadExtraDataOffset, extraDataOffset, fixedSize)
	}
	offset += 4
	unmarshalBytes32LE(buf[offset:offset+32], &payload.BaseFeePerGas)
	offset += 32
	copy(payload.BlockHash[:], buf[offset:offset+32])
	offset += 32

	transactionsOffset := binary.LittleEndian.Uint32(buf[offset : offset+4])
	if transactionsOffset < extraDataOffset {
		return ErrBadTransactionOffset
	}
	offset += 4
	if version == BlockV1 && offset != fixedSize {
		panic("fixed part size is inconsistent")
	}

	withdrawalsOffset := scope
	if version.HasWithdrawals() {
		withdrawalsOffset = binary.LittleEndian.Uint32(buf[offset : offset+4])
		offset += 4

		if withdrawalsOffset < transactionsOffset {
			return ErrBadWithdrawalsOffset
		}
		if withdrawalsOffset > scope {
			return fmt.Errorf("withdrawals offset is too large: %d", withdrawalsOffset)
		}
	}

	if version.HasBlobProperties() {
		blobGasUsed := binary.LittleEndian.Uint64(buf[offset : offset+8])
		payload.BlobGasUsed = (*Uint64Quantity)(&blobGasUsed)
		offset += 8
		excessBlobGas := binary.LittleEndian.Uint64(buf[offset : offset+8])
		payload.ExcessBlobGas = (*Uint64Quantity)(&excessBlobGas)
		offset += 8
	}

	if version.HasWithdrawalsRoot() {
		withdrawalsRoot := common.Hash{}
		copy(withdrawalsRoot[:], buf[offset:offset+32])
		payload.WithdrawalsRoot = &withdrawalsRoot
		offset += 32
	}

	_ = offset // for future extensions: we keep the offset accurate for extensions

	if transactionsOffset > extraDataOffset+32 || transactionsOffset > scope {
		return fmt.Errorf("extra-data is too large: %d", transactionsOffset-extraDataOffset)
	}

	extraDataSize := transactionsOffset - extraDataOffset
	payload.ExtraData = make(BytesMax32, extraDataSize)
	copy(payload.ExtraData, buf[extraDataOffset:transactionsOffset])

	txs, err := unmarshalTransactions(buf[transactionsOffset:withdrawalsOffset])
	if err != nil {
		return fmt.Errorf("failed to unmarshal transactions list: %w", err)
	}
	payload.Transactions = txs

	if version.HasWithdrawals() {
		withdrawals, err := unmarshalWithdrawals(buf[withdrawalsOffset:])
		if err != nil {
			return fmt.Errorf("failed to unmarshal withdrawals list: %w", err)
		}
		payload.Withdrawals = &withdrawals
	}

	return nil
}

func unmarshalWithdrawals(in []byte) (types.Withdrawals, error) {
	result := types.Withdrawals{} // empty list by default, intentionally non-nil

	if len(in)%withdrawalSize != 0 {
		return nil, errors.New("invalid withdrawals data")
	}

	withdrawalCount := len(in) / withdrawalSize

	if withdrawalCount > maxWithdrawalsPerPayload {
		return nil, fmt.Errorf("too many withdrawals: %d > %d", withdrawalCount, maxWithdrawalsPerPayload)
	}

	offset := 0

	for i := 0; i < withdrawalCount; i++ {
		withdrawal := &types.Withdrawal{}

		withdrawal.Index = binary.LittleEndian.Uint64(in[offset : offset+8])
		offset += 8

		withdrawal.Validator = binary.LittleEndian.Uint64(in[offset : offset+8])
		offset += 8

		copy(withdrawal.Address[:], in[offset:offset+20])
		offset += 20

		withdrawal.Amount = binary.LittleEndian.Uint64(in[offset : offset+8])
		offset += 8

		result = append(result, withdrawal)
	}

	return result, nil
}

func unmarshalTransactions(in []byte) (txs []Data, err error) {
	scope := uint32(len(in))
	if scope == 0 { // empty txs list
		return make([]Data, 0), nil
	}
	if scope < 4 {
		return nil, fmt.Errorf("not enough scope to read first tx offset: %d", scope)
	}
	offset := uint32(0)
	firstTxOffset := binary.LittleEndian.Uint32(in[offset : offset+4])
	offset += 4
	if firstTxOffset%4 != 0 {
		return nil, fmt.Errorf("invalid first tx offset: %d, not a multiple of offset size", firstTxOffset)
	}
	if firstTxOffset > scope {
		return nil, fmt.Errorf("invalid first tx offset: %d, out of scope %d", firstTxOffset, scope)
	}
	txCount := firstTxOffset / 4
	if txCount == 0 && scope > 0 {
		return nil, fmt.Errorf("invalid first tx offset: %d, no transactions in scope %d", firstTxOffset, scope)
	}
	if txCount > maxTransactionsPerPayload {
		return nil, fmt.Errorf("too many transactions: %d > %d", txCount, maxTransactionsPerPayload)
	}
	txs = make([]Data, txCount)
	currentTxOffset := firstTxOffset
	for i := uint32(0); i < txCount; i++ {
		nextTxOffset := scope
		if i+1 < txCount {
			nextTxOffset = binary.LittleEndian.Uint32(in[offset : offset+4])
			offset += 4
		}
		if nextTxOffset < currentTxOffset || nextTxOffset > scope {
			return nil, fmt.Errorf("tx %d has bad next offset: %d, current is %d, scope is %d", i, nextTxOffset, currentTxOffset, scope)
		}
		currentTxSize := nextTxOffset - currentTxOffset
		txs[i] = make(Data, currentTxSize)
		copy(txs[i], in[currentTxOffset:nextTxOffset])
		currentTxOffset = nextTxOffset
	}
	return txs, nil
}

// UnmarshalSSZ decodes the ExecutionPayloadEnvelope as SSZ type
func (envelope *ExecutionPayloadEnvelope) UnmarshalSSZ(version BlockVersion, scope uint32, r io.Reader) error {
	if scope < common.HashLength {
		return fmt.Errorf("%w: %d", ErrScopeTooSmall, scope)
	}

	data := make([]byte, common.HashLength)
	n, err := r.Read(data)
	if err != nil || n != common.HashLength {
		return err
	}

	envelope.ParentBeaconBlockRoot = &common.Hash{}
	copy(envelope.ParentBeaconBlockRoot[:], data)

	var payload ExecutionPayload
	err = payload.UnmarshalSSZ(version, scope-32, r)
	if err != nil {
		return err
	}

	if version.ImpliesRequestsRoot() {
		h := types.EmptyRequestsHash
		envelope.RequestsHash = &h
	}

	envelope.ExecutionPayload = &payload
	return nil
}

// MarshalSSZ encodes the ExecutionPayload as SSZ type
func (envelope *ExecutionPayloadEnvelope) MarshalSSZ(w io.Writer) (n int, err error) {
	if envelope.ExecutionPayload == nil || envelope.ParentBeaconBlockRoot == nil {
		return 0, ErrMissingData
	}

	// write parent beacon block root
	hashSize, err := w.Write(envelope.ParentBeaconBlockRoot[:])
	if err != nil || hashSize != common.HashLength {
		return 0, errors.New("unable to write parent beacon block hash")
	}

	payloadSize, err := envelope.ExecutionPayload.MarshalSSZ(w)
	if err != nil {
		return 0, err
	}

	return hashSize + payloadSize, nil
}
