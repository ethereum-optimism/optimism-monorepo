package derive

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/solabi"
)

var (
	SystemConfigUpdateBatcher           = common.Hash{31: 0}
	SystemConfigUpdateFeeScalars        = common.Hash{31: 1}
	SystemConfigUpdateGasLimit          = common.Hash{31: 2}
	SystemConfigUpdateUnsafeBlockSigner = common.Hash{31: 3}
	SystemConfigUpdateEIP1559Params     = common.Hash{31: 4}
)

var (
	ConfigUpdateEventABI      = "ConfigUpdate(uint256,uint8,bytes)"
	ConfigUpdateEventABIHash  = crypto.Keccak256Hash([]byte(ConfigUpdateEventABI))
	ConfigUpdateEventVersion0 = common.Hash{}
)

// UpdateSystemConfigWithL1Receipts filters all L1 receipts to find config updates and applies the config updates to the given sysCfg
func UpdateSystemConfigWithL1Receipts(sysCfg *eth.SystemConfig, receipts []*types.Receipt, cfg *rollup.Config, l1Time uint64) error {
	var result error
	for i, rec := range receipts {
		if rec.Status != types.ReceiptStatusSuccessful {
			continue
		}
		for j, log := range rec.Logs {
			if log.Address == cfg.L1SystemConfigAddress && len(log.Topics) > 0 && log.Topics[0] == ConfigUpdateEventABIHash {
				if err := ProcessSystemConfigUpdateLogEvent(sysCfg, log, cfg, l1Time); err != nil {
					result = multierror.Append(result, fmt.Errorf("malformatted L1 system sysCfg log in receipt %d, log %d: %w", i, j, err))
				}
			}
		}
	}
	return result
}

// ProcessSystemConfigUpdateLogEvent decodes an EVM log entry emitted by the system config contract and applies it as a system config change.
//
// parse log data for:
//
//	event ConfigUpdate(
//	    uint256 indexed version,
//	    UpdateType indexed updateType,
//	    bytes data
//	);
func ProcessSystemConfigUpdateLogEvent(destSysCfg *eth.SystemConfig, ev *types.Log, rollupCfg *rollup.Config, l1Time uint64) error {
	if len(ev.Topics) != 3 {
		return fmt.Errorf("expected 3 event topics (event identity, indexed version, indexed updateType), got %d", len(ev.Topics))
	}
	if ev.Topics[0] != ConfigUpdateEventABIHash {
		return fmt.Errorf("invalid SystemConfig update event: %s, expected %s", ev.Topics[0], ConfigUpdateEventABIHash)
	}

	// indexed 0
	version := ev.Topics[1]
	if version != ConfigUpdateEventVersion0 {
		return fmt.Errorf("unrecognized SystemConfig update event version: %s", version)
	}
	// indexed 1
	updateType := ev.Topics[2]

	// Create a reader of the unindexed data
	reader := bytes.NewReader(ev.Data)

	// Attempt to read unindexed data
	switch updateType {
	case SystemConfigUpdateBatcher:
		if pointer, err := solabi.ReadUint64(reader); err != nil || pointer != 32 {
			return NewCriticalError(errors.New("invalid pointer field"))
		}
		if length, err := solabi.ReadUint64(reader); err != nil || length != 32 {
			return NewCriticalError(errors.New("invalid length field"))
		}
		address, err := solabi.ReadAddress(reader)
		if err != nil {
			return NewCriticalError(errors.New("could not read address"))
		}
		if !solabi.EmptyReader(reader) {
			return NewCriticalError(errors.New("too many bytes"))
		}
		destSysCfg.BatcherAddr = address
		return nil
	case SystemConfigUpdateFeeScalars:
		if pointer, err := solabi.ReadUint64(reader); err != nil || pointer != 32 {
			return NewCriticalError(errors.New("invalid pointer field"))
		}
		if length, err := solabi.ReadUint64(reader); err != nil || length != 64 {
			return NewCriticalError(errors.New("invalid length field"))
		}
		overhead, err := solabi.ReadEthBytes32(reader)
		if err != nil {
			return NewCriticalError(errors.New("could not read overhead"))
		}
		scalar, err := solabi.ReadEthBytes32(reader)
		if err != nil {
			return NewCriticalError(errors.New("could not read scalar"))
		}
		if !solabi.EmptyReader(reader) {
			return NewCriticalError(errors.New("too many bytes"))
		}
		if rollupCfg.IsEcotone(l1Time) {
			if err := eth.CheckEcotoneL1SystemConfigScalar(scalar); err != nil {
				return nil // ignore invalid scalars, retain the old system-config scalar
			}
			// retain the scalar data in encoded form
			destSysCfg.Scalar = scalar
			// zero out the overhead, it will not affect the state-transition after Ecotone
			destSysCfg.Overhead = eth.Bytes32{}
		} else {
			destSysCfg.Overhead = overhead
			destSysCfg.Scalar = scalar
		}
		return nil
	case SystemConfigUpdateGasLimit:
		if pointer, err := solabi.ReadUint64(reader); err != nil || pointer != 32 {
			return NewCriticalError(errors.New("invalid pointer field"))
		}
		if length, err := solabi.ReadUint64(reader); err != nil || length != 32 {
			return NewCriticalError(errors.New("invalid length field"))
		}
		gasLimit, err := solabi.ReadUint64(reader)
		if err != nil {
			return NewCriticalError(errors.New("could not read gas limit"))
		}
		if !solabi.EmptyReader(reader) {
			return NewCriticalError(errors.New("too many bytes"))
		}
		destSysCfg.GasLimit = gasLimit
		return nil
	case SystemConfigUpdateEIP1559Params:
		if pointer, err := solabi.ReadUint64(reader); err != nil || pointer != 32 {
			return NewCriticalError(errors.New("invalid pointer field"))
		}
		if length, err := solabi.ReadUint64(reader); err != nil || length != 32 {
			return NewCriticalError(errors.New("invalid length field"))
		}
		params, err := solabi.ReadEthBytes32(reader)
		if err != nil {
			return NewCriticalError(errors.New("could not read eip-1559 params"))
		}
		if !solabi.EmptyReader(reader) {
			return NewCriticalError(errors.New("too many bytes"))
		}
		copy(destSysCfg.EIP1559Params[:], params[24:32])
		return nil
	case SystemConfigUpdateUnsafeBlockSigner:
		// Ignored in derivation. This configurable applies to runtime configuration outside of the derivation.
		return nil
	default:
		return fmt.Errorf("unrecognized L1 sysCfg update type: %s", updateType)
	}
}
