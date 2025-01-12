package derive

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/solabi"
)

const (
	L1InfoFuncBedrockSignature = "setL1BlockValues(uint64,uint64,uint256,bytes32,uint64,bytes32,uint256,uint256)"
	L1InfoFuncEcotoneSignature = "setL1BlockValuesEcotone()"
	L1InfoFuncJovianSignature  = "setL1BlockValuesJovian()"
	L1InfoFuncInteropSignature = "setL1BlockValuesInterop()"
	DepositsCompleteSignature  = "depositsComplete()"
	L1InfoArguments            = 8
	L1InfoBedrockLen           = 4 + 32*L1InfoArguments
	L1InfoEcotoneLen           = 4 + 32*5      // after Ecotone upgrade, args are packed into 5 32-byte slots
	L1InfoJovianLen            = 4 + 32*5 + 16 // after Jovian upgrade, args are packed into 5 32-byte slots and 1 16-byte slot
	DepositsCompleteLen        = 4             // only the selector
	// DepositsCompleteGas allocates 21k gas for intrinsic tx costs, and
	// an additional 15k to ensure that the DepositsComplete call does not run out of gas.
	// GasBenchMark_L1BlockInterop_DepositsComplete:test_depositsComplete_benchmark() (gas: 7768)
	// GasBenchMark_L1BlockInterop_DepositsComplete_Warm:test_depositsComplete_benchmark() (gas: 5768)
	// see `test_depositsComplete_benchmark` at: `/packages/contracts-bedrock/test/BenchmarkTest.t.sol`
	DepositsCompleteGas = uint64(21_000 + 15_000)
)

var (
	L1InfoFuncBedrockBytes4 = crypto.Keccak256([]byte(L1InfoFuncBedrockSignature))[:4]
	L1InfoFuncEcotoneBytes4 = crypto.Keccak256([]byte(L1InfoFuncEcotoneSignature))[:4]
	L1InfoFuncJovianBytes4  = crypto.Keccak256([]byte(L1InfoFuncJovianSignature))[:4]
	L1InfoFuncInteropBytes4 = crypto.Keccak256([]byte(L1InfoFuncInteropSignature))[:4]
	DepositsCompleteBytes4  = crypto.Keccak256([]byte(DepositsCompleteSignature))[:4]
	L1InfoDepositerAddress  = common.HexToAddress("0xdeaddeaddeaddeaddeaddeaddeaddeaddead0001")
	L1BlockAddress          = predeploys.L1BlockAddr
	ErrInvalidFormat        = errors.New("invalid ecotone l1 block info format")
)

const (
	RegolithSystemTxGas = 1_000_000
)

// L1BlockInfo presents the information stored in a L1Block.setL1BlockValues call
type L1BlockInfo struct {
	Number    uint64
	Time      uint64
	BaseFee   *big.Int
	BlockHash common.Hash
	// Not strictly a piece of L1 information. Represents the number of L2 blocks since the start of the epoch,
	// i.e. when the actual L1 info was first introduced.
	SequenceNumber uint64
	// BatcherAddr version 0 is just the address with 0 padding to the left.
	BatcherAddr common.Address

	L1FeeOverhead eth.Bytes32 // ignored after Ecotone upgrade
	L1FeeScalar   eth.Bytes32 // ignored after Ecotone upgrade

	BlobBaseFee       *big.Int // added by Ecotone upgrade
	BaseFeeScalar     uint32   // added by Ecotone upgrade
	BlobBaseFeeScalar uint32   // added by Ecotone upgrade

	DepositNonce      uint64 // added by the Jovian upgrade
	ConfigUpdateNonce uint64 // added by the Jovian upgrade
}

// Bedrock Binary Format
// +---------+--------------------------+
// | Bytes   | Field                    |
// +---------+--------------------------+
// | 4       | Function signature       |
// | 32      | Number                   |
// | 32      | Time                     |
// | 32      | BaseFee                  |
// | 32      | BlockHash                |
// | 32      | SequenceNumber           |
// | 32      | BatcherHash              |
// | 32      | L1FeeOverhead            |
// | 32      | L1FeeScalar              |
// +---------+--------------------------+

func (info *L1BlockInfo) marshalBinaryBedrock() ([]byte, error) {
	w := bytes.NewBuffer(make([]byte, 0, L1InfoBedrockLen))
	if err := solabi.WriteSignature(w, L1InfoFuncBedrockBytes4); err != nil {
		return nil, err
	}
	if err := solabi.WriteUint64(w, info.Number); err != nil {
		return nil, err
	}
	if err := solabi.WriteUint64(w, info.Time); err != nil {
		return nil, err
	}
	if err := solabi.WriteUint256(w, info.BaseFee); err != nil {
		return nil, err
	}
	if err := solabi.WriteHash(w, info.BlockHash); err != nil {
		return nil, err
	}
	if err := solabi.WriteUint64(w, info.SequenceNumber); err != nil {
		return nil, err
	}
	if err := solabi.WriteAddress(w, info.BatcherAddr); err != nil {
		return nil, err
	}
	if err := solabi.WriteEthBytes32(w, info.L1FeeOverhead); err != nil {
		return nil, err
	}
	if err := solabi.WriteEthBytes32(w, info.L1FeeScalar); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func (info *L1BlockInfo) unmarshalBinaryBedrock(data []byte) error {
	if len(data) != L1InfoBedrockLen {
		return fmt.Errorf("data is unexpected length: %d", len(data))
	}
	reader := bytes.NewReader(data)

	var err error
	if _, err := solabi.ReadAndValidateSignature(reader, L1InfoFuncBedrockBytes4); err != nil {
		return err
	}
	if info.Number, err = solabi.ReadUint64(reader); err != nil {
		return err
	}
	if info.Time, err = solabi.ReadUint64(reader); err != nil {
		return err
	}
	if info.BaseFee, err = solabi.ReadUint256(reader); err != nil {
		return err
	}
	if info.BlockHash, err = solabi.ReadHash(reader); err != nil {
		return err
	}
	if info.SequenceNumber, err = solabi.ReadUint64(reader); err != nil {
		return err
	}
	if info.BatcherAddr, err = solabi.ReadAddress(reader); err != nil {
		return err
	}
	if info.L1FeeOverhead, err = solabi.ReadEthBytes32(reader); err != nil {
		return err
	}
	if info.L1FeeScalar, err = solabi.ReadEthBytes32(reader); err != nil {
		return err
	}
	if !solabi.EmptyReader(reader) {
		return errors.New("too many bytes")
	}
	return nil
}

// Ecotone Binary Format
// +---------+--------------------------+
// | Bytes   | Field                    |
// +---------+--------------------------+
// | 4       | Function signature       |
// | 4       | BaseFeeScalar            |
// | 4       | BlobBaseFeeScalar        |
// | 8       | SequenceNumber           |
// | 8       | Timestamp                |
// | 8       | L1BlockNumber            |
// | 32      | BaseFee                  |
// | 32      | BlobBaseFee              |
// | 32      | BlockHash                |
// | 32      | BatcherHash              |
// +---------+--------------------------+

// Jovian & Interop Binary Format
// +---------+--------------------------+
// | Bytes   | Field                    |
// +---------+--------------------------+
// | 4       | Function signature       |
// | 4       | BaseFeeScalar            |
// | 4       | BlobBaseFeeScalar        |
// | 8       | SequenceNumber           |
// | 8       | Timestamp                |
// | 8       | L1BlockNumber            |
// | 32      | BaseFee                  |
// | 32      | BlobBaseFee              |
// | 32      | BlockHash                |
// | 32      | BatcherHash              |
// | 8       | DepositNonce             |
// | 8       | ConfigUpdateNonce        |
// +---------+--------------------------+

func (info *L1BlockInfo) marshalBinaryEcotone() ([]byte, error) {
	out, err := marshalBinaryWithSignature(info, L1InfoFuncEcotoneBytes4, false)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Ecotone l1 block info: %w", err)
	}
	return out, nil
}

func (info *L1BlockInfo) marshalBinaryJovian() ([]byte, error) {
	out, err := marshalBinaryWithSignature(info, L1InfoFuncJovianBytes4, true)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Jovian l1 block info: %w", err)
	}
	return out, nil
}

func (info *L1BlockInfo) marshalBinaryInterop() ([]byte, error) {
	out, err := marshalBinaryWithSignature(info, L1InfoFuncInteropBytes4, true)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Interop l1 block info: %w", err)
	}
	return out, nil
}

func marshalBinaryWithSignature(info *L1BlockInfo, signature []byte, includeNonces bool) ([]byte, error) {
	var w *bytes.Buffer
	if includeNonces {
		w = bytes.NewBuffer(make([]byte, 0, L1InfoJovianLen)) // Jovian and Interop have the same length
	} else {
		w = bytes.NewBuffer(make([]byte, 0, L1InfoEcotoneLen))
	}
	if err := solabi.WriteSignature(w, signature); err != nil {
		return nil, err
	}
	if err := binary.Write(w, binary.BigEndian, info.BaseFeeScalar); err != nil {
		return nil, err
	}
	if err := binary.Write(w, binary.BigEndian, info.BlobBaseFeeScalar); err != nil {
		return nil, err
	}
	if err := binary.Write(w, binary.BigEndian, info.SequenceNumber); err != nil {
		return nil, err
	}
	if err := binary.Write(w, binary.BigEndian, info.Time); err != nil {
		return nil, err
	}
	if err := binary.Write(w, binary.BigEndian, info.Number); err != nil {
		return nil, err
	}
	if err := solabi.WriteUint256(w, info.BaseFee); err != nil {
		return nil, err
	}
	blobBasefee := info.BlobBaseFee
	if blobBasefee == nil {
		blobBasefee = big.NewInt(1) // set to 1, to match the min blob basefee as defined in EIP-4844
	}
	if err := solabi.WriteUint256(w, blobBasefee); err != nil {
		return nil, err
	}
	if err := solabi.WriteHash(w, info.BlockHash); err != nil {
		return nil, err
	}
	// ABI encoding will perform the left-padding with zeroes to 32 bytes, matching the "batcherHash" SystemConfig format and version 0 byte.
	if err := solabi.WriteAddress(w, info.BatcherAddr); err != nil {
		return nil, err
	}
	if includeNonces {
		if err := binary.Write(w, binary.BigEndian, info.DepositNonce); err != nil {
			return nil, err
		}
		if err := binary.Write(w, binary.BigEndian, info.ConfigUpdateNonce); err != nil {
			return nil, err
		}
	}
	return w.Bytes(), nil
}

func (info *L1BlockInfo) unmarshalBinaryEcotone(data []byte) error {
	return unmarshalBinaryWithSignatureAndData(info, L1InfoFuncEcotoneBytes4, data, false)
}

func (info *L1BlockInfo) unmarshalBinaryJovian(data []byte) error {
	return unmarshalBinaryWithSignatureAndData(info, L1InfoFuncJovianBytes4, data, true)
}

func (info *L1BlockInfo) unmarshalBinaryInterop(data []byte) error {
	return unmarshalBinaryWithSignatureAndData(info, L1InfoFuncInteropBytes4, data, true)
}

func unmarshalBinaryWithSignatureAndData(info *L1BlockInfo, signature []byte, data []byte, includeNonces bool) error {
	expectedLength := L1InfoEcotoneLen
	if includeNonces {
		expectedLength = L1InfoJovianLen
	}
	if len(data) != expectedLength {
		return fmt.Errorf("data is unexpected length: %d", len(data))
	}
	r := bytes.NewReader(data)

	var err error
	if _, err := solabi.ReadAndValidateSignature(r, signature); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &info.BaseFeeScalar); err != nil {
		return ErrInvalidFormat
	}
	if err := binary.Read(r, binary.BigEndian, &info.BlobBaseFeeScalar); err != nil {
		return ErrInvalidFormat
	}
	if err := binary.Read(r, binary.BigEndian, &info.SequenceNumber); err != nil {
		return ErrInvalidFormat
	}
	if err := binary.Read(r, binary.BigEndian, &info.Time); err != nil {
		return ErrInvalidFormat
	}
	if err := binary.Read(r, binary.BigEndian, &info.Number); err != nil {
		return ErrInvalidFormat
	}
	if info.BaseFee, err = solabi.ReadUint256(r); err != nil {
		return err
	}
	if info.BlobBaseFee, err = solabi.ReadUint256(r); err != nil {
		return err
	}
	if info.BlockHash, err = solabi.ReadHash(r); err != nil {
		return err
	}
	// The "batcherHash" will be correctly parsed as address, since the version 0 and left-padding matches the ABI encoding format.
	if info.BatcherAddr, err = solabi.ReadAddress(r); err != nil {
		return err
	}
	if includeNonces {
		if err := binary.Read(r, binary.BigEndian, &info.DepositNonce); err != nil {
			return ErrInvalidFormat
		}
		if err := binary.Read(r, binary.BigEndian, &info.ConfigUpdateNonce); err != nil {
			return ErrInvalidFormat
		}
	}
	if !solabi.EmptyReader(r) {
		return errors.New("too many bytes")
	}
	return nil
}

// isEcotoneButNotFirstBlock returns whether the specified block is subject to the Ecotone upgrade,
// but is not the activation block itself.
func isEcotoneButNotFirstBlock(rollupCfg *rollup.Config, l2Timestamp uint64) bool {
	return rollupCfg.IsEcotone(l2Timestamp) && !rollupCfg.IsEcotoneActivationBlock(l2Timestamp)
}

// isJovianButNotFirstBlock returns whether the specified block is subject to the Jovian upgrade,
// but is not the activation block itself.
func isJovianButNotFirstBlock(rollupCfg *rollup.Config, l2Timestamp uint64) bool {
	return rollupCfg.IsJovian(l2Timestamp) && !rollupCfg.IsJovianActivationBlock(l2Timestamp)
}

// isInteropButNotFirstBlock returns whether the specified block is subject to the Interop upgrade,
// but is not the activation block itself.
func isInteropButNotFirstBlock(rollupCfg *rollup.Config, l2Timestamp uint64) bool {
	// Since we use the pre-interop L1 tx one last time during the upgrade block,
	// we must disallow the deposit-txs from using the CrossL2Inbox during this block.
	// If the CrossL2Inbox does not exist yet, then it is safe,
	// but we have to ensure that the spec and code puts any Interop upgrade-txs after the user deposits.
	return rollupCfg.IsInterop(l2Timestamp) && !rollupCfg.IsInteropActivationBlock(l2Timestamp)
}

// L1BlockInfoFromBytes is the inverse of L1InfoDeposit, to see where the L2 chain is derived from
func L1BlockInfoFromBytes(rollupCfg *rollup.Config, l2BlockTime uint64, data []byte) (*L1BlockInfo, error) {
	var info L1BlockInfo
	// Important, this should be ordered from most recent to oldest
	if isInteropButNotFirstBlock(rollupCfg, l2BlockTime) {
		return &info, info.unmarshalBinaryInterop(data)
	}
	if isJovianButNotFirstBlock(rollupCfg, l2BlockTime) {
		return &info, info.unmarshalBinaryJovian(data)
	}
	if isEcotoneButNotFirstBlock(rollupCfg, l2BlockTime) {
		return &info, info.unmarshalBinaryEcotone(data)
	}
	return &info, info.unmarshalBinaryBedrock(data)
}

// L1InfoDeposit creates a L1 Info deposit transaction based on the L1 block,
// and the L2 block-height difference with the start of the epoch.
func L1InfoDeposit(rollupCfg *rollup.Config, sysCfg eth.SystemConfig, seqNumber uint64, block eth.BlockInfo, l2Timestamp uint64) (*types.DepositTx, error) {
	l1BlockInfo := L1BlockInfo{
		Number:         block.NumberU64(),
		Time:           block.Time(),
		BaseFee:        block.BaseFee(),
		BlockHash:      block.Hash(),
		SequenceNumber: seqNumber,
		BatcherAddr:    sysCfg.BatcherAddr,
	}
	jovian := isJovianButNotFirstBlock(rollupCfg, l2Timestamp)
	if !jovian {
		if sysCfg.DepositNonce != 0 {
			return nil, fmt.Errorf("found non-zero deposit nonce for non-Jovian block")
		}
		if sysCfg.ConfigUpdateNonce != 0 {
			return nil, fmt.Errorf("found non-zero config update nonce for non-Jovian block")
		}
	}
	var data []byte
	if isEcotoneButNotFirstBlock(rollupCfg, l2Timestamp) {
		l1BlockInfo.BlobBaseFee = block.BlobBaseFee()
		if l1BlockInfo.BlobBaseFee == nil {
			// The L2 spec states to use the MIN_BLOB_GASPRICE from EIP-4844 if not yet active on L1.
			l1BlockInfo.BlobBaseFee = big.NewInt(1)
		}
		scalars, err := sysCfg.EcotoneScalars()
		if err != nil {
			return nil, err
		}
		l1BlockInfo.BlobBaseFeeScalar = scalars.BlobBaseFeeScalar
		l1BlockInfo.BaseFeeScalar = scalars.BaseFeeScalar
		if jovian {
			l1BlockInfo.DepositNonce = sysCfg.DepositNonce
			l1BlockInfo.ConfigUpdateNonce = sysCfg.ConfigUpdateNonce
			if isInteropButNotFirstBlock(rollupCfg, l2Timestamp) {
				data, err = l1BlockInfo.marshalBinaryInterop()
				if err != nil {
					return nil, fmt.Errorf("failed to marshal Interop l1 block info: %w", err)
				}
			} else {
				data, err = l1BlockInfo.marshalBinaryJovian()
				if err != nil {
					return nil, fmt.Errorf("failed to marshal Jovian l1 block info: %w", err)
				}
			}
		} else {
			data, err = l1BlockInfo.marshalBinaryEcotone()
			if err != nil {
				return nil, fmt.Errorf("failed to marshal Ecotone l1 block info: %w", err)
			}
		}
	} else {
		l1BlockInfo.L1FeeOverhead = sysCfg.Overhead
		l1BlockInfo.L1FeeScalar = sysCfg.Scalar
		var err error
		data, err = l1BlockInfo.marshalBinaryBedrock()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Bedrock l1 block info: %w", err)
		}
	}

	source := L1InfoDepositSource{
		L1BlockHash: block.Hash(),
		SeqNumber:   seqNumber,
	}
	// Set a very large gas limit with `IsSystemTransaction` to ensure
	// that the L1 Attributes Transaction does not run out of gas.
	out := &types.DepositTx{
		SourceHash:          source.SourceHash(),
		From:                L1InfoDepositerAddress,
		To:                  &L1BlockAddress,
		Mint:                nil,
		Value:               big.NewInt(0),
		Gas:                 150_000_000,
		IsSystemTransaction: true,
		Data:                data,
	}
	// With the regolith fork we disable the IsSystemTx functionality, and allocate real gas
	if rollupCfg.IsRegolith(l2Timestamp) {
		out.IsSystemTransaction = false
		out.Gas = RegolithSystemTxGas
	}
	return out, nil
}

// L1InfoDepositBytes returns a serialized L1-info attributes transaction.
func L1InfoDepositBytes(rollupCfg *rollup.Config, sysCfg eth.SystemConfig, seqNumber uint64, l1Info eth.BlockInfo, l2Timestamp uint64) ([]byte, error) {
	dep, err := L1InfoDeposit(rollupCfg, sysCfg, seqNumber, l1Info, l2Timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to create L1 info tx: %w", err)
	}
	l1Tx := types.NewTx(dep)
	opaqueL1Tx, err := l1Tx.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to encode L1 info tx: %w", err)
	}
	return opaqueL1Tx, nil
}

func DepositsCompleteDeposit(seqNumber uint64, block eth.BlockInfo) (*types.DepositTx, error) {
	source := AfterForceIncludeSource{
		L1BlockHash: block.Hash(),
		SeqNumber:   seqNumber,
	}
	out := &types.DepositTx{
		SourceHash:          source.SourceHash(),
		From:                L1InfoDepositerAddress,
		To:                  &L1BlockAddress,
		Mint:                nil,
		Value:               big.NewInt(0),
		Gas:                 DepositsCompleteGas,
		IsSystemTransaction: false,
		Data:                DepositsCompleteBytes4,
	}
	return out, nil
}

func DepositsCompleteBytes(seqNumber uint64, l1Info eth.BlockInfo) ([]byte, error) {
	dep, err := DepositsCompleteDeposit(seqNumber, l1Info)
	if err != nil {
		return nil, fmt.Errorf("failed to create DepositsComplete tx: %w", err)
	}
	depositsCompleteTx := types.NewTx(dep)
	opaqueDepositsCompleteTx, err := depositsCompleteTx.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to encode DepositsComplete tx: %w", err)
	}
	return opaqueDepositsCompleteTx, nil
}
