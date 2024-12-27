package derive

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-service/eth"
)

var (
	DepositEventABI      = "TransactionDeposited(address,address,uint256,bytes)"
	DepositEventABIHash  = crypto.Keccak256Hash([]byte(DepositEventABI))
	DepositEventVersion0 = uint64(0)
	DepositEventVersion1 = uint64(1)
)

// UnmarshalDepositLogEventIgnoreNonce decodes an EVM log entry emitted by the deposit contract into typed deposit data.
// Same as UnmarshalDepositLogEvent, but it force-ensures that the deposit nonce is correct.
// This is useful for testing or non-consensus applications.
func UnmarshalDepositLogEventIgnoreNonce(ev *types.Log) (*types.DepositTx, error) {
	previousNonce := uint64(0)
	if len(ev.Topics) == 4 {
		nonce, version := UnpackNonceAndVersion(ev.Topics[3])
		switch version {
		case DepositEventVersion1:
			previousNonce = nonce - 1
		}
	}
	dep, _, err := UnmarshalDepositLogEvent(ev, previousNonce)
	return dep, err
}

// UnmarshalDepositLogEvent decodes an EVM log entry emitted by the deposit contract into typed deposit data.
//
// parse log data for:
//
//	event TransactionDeposited(
//	    address indexed from,
//	    address indexed to,
//	    uint256 indexed nonceAndVersion,
//	    bytes opaqueData
//	);
//
// Additionally, the event log-index and
func UnmarshalDepositLogEvent(ev *types.Log, currentNonce uint64) (*types.DepositTx, uint64, error) {
	if len(ev.Topics) != 4 {
		return nil, currentNonce, fmt.Errorf("expected 4 event topics (event identity, indexed from, indexed to, indexed version), got %d", len(ev.Topics))
	}
	if ev.Topics[0] != DepositEventABIHash {
		return nil, currentNonce, fmt.Errorf("invalid deposit event selector: %s, expected %s", ev.Topics[0], DepositEventABIHash)
	}
	if len(ev.Data) < 64 {
		return nil, currentNonce, fmt.Errorf("incomplate opaqueData slice header (%d bytes): %x", len(ev.Data), ev.Data)
	}
	if len(ev.Data)%32 != 0 {
		return nil, currentNonce, fmt.Errorf("expected log data to be multiple of 32 bytes: got %d bytes", len(ev.Data))
	}

	// indexed 0
	from := common.BytesToAddress(ev.Topics[1][12:])
	// indexed 1
	to := common.BytesToAddress(ev.Topics[2][12:])
	// indexed 2
	nonceAndVersion := ev.Topics[3]
	// unindexed data
	// Solidity serializes the event's Data field as follows:
	// abi.encode(abi.encodPacked(uint256 mint, uint256 value, uint64 gasLimit, uint8 isCreation, bytes data))
	// Thus the first 32 bytes of the Data will give us the offset of the opaqueData,
	// which should always be 0x20.
	var opaqueContentOffset uint256.Int
	opaqueContentOffset.SetBytes(ev.Data[0:32])
	if !opaqueContentOffset.IsUint64() || opaqueContentOffset.Uint64() != 32 {
		return nil, currentNonce, fmt.Errorf("invalid opaqueData slice header offset: %d", opaqueContentOffset.Uint64())
	}
	// The next 32 bytes indicate the length of the opaqueData content.
	var opaqueContentLength uint256.Int
	opaqueContentLength.SetBytes(ev.Data[32:64])
	// Make sure the length is an uint64, it's not larger than the remaining data, and the log is using minimal padding (i.e. can't add 32 bytes without exceeding data)
	if !opaqueContentLength.IsUint64() || opaqueContentLength.Uint64() > uint64(len(ev.Data)-64) || opaqueContentLength.Uint64()+32 <= uint64(len(ev.Data)-64) {
		return nil, currentNonce, fmt.Errorf("invalid opaqueData slice header length: %d", opaqueContentLength.Uint64())
	}
	// The remaining data is the opaqueData which is tightly packed
	// and then padded to 32 bytes by the EVM.
	opaqueData := ev.Data[64 : 64+opaqueContentLength.Uint64()]

	var dep types.DepositTx

	source := UserDepositSource{
		L1BlockHash: ev.BlockHash,
		LogIndex:    uint64(ev.Index),
	}
	dep.SourceHash = source.SourceHash()
	dep.From = from
	dep.IsSystemTransaction = false

	newNonce, version := UnpackNonceAndVersion(nonceAndVersion)

	var err error
	switch version {
	case DepositEventVersion0:
		err = unmarshalDepositVersion0(&dep, to, opaqueData)
	case DepositEventVersion1:
		if newNonce != currentNonce+1 {
			return nil, currentNonce, fmt.Errorf("invalid deposit nonce, expected %d got %d", currentNonce+1, newNonce)
		}
		err = unmarshalDepositVersion1(&dep, to, opaqueData)
	default:
		return nil, currentNonce, fmt.Errorf("invalid deposit version, got %d", version)
	}
	if err != nil {
		return nil, currentNonce, fmt.Errorf("failed to decode deposit (version %d): %w", version, err)
	}
	return &dep, newNonce, nil
}

func UnpackNonceAndVersion(nonceAndVersion common.Hash) (nonce uint64, version uint64) {
	i := new(big.Int).SetBytes(nonceAndVersion[:])
	mask64 := new(big.Int).SetBytes(common.Hex2Bytes("ffffffffffffffff"))
	version = mask64.And(mask64, i).Uint64()
	nonce = i.Rsh(i, 128).Uint64()
	return
}

func PackNonceAndVersion(nonce uint64, version uint64) common.Hash {
	i := new(big.Int).SetUint64(nonce)
	i.Lsh(i, 128)
	i.Or(i, new(big.Int).SetUint64(version))
	return common.BytesToHash(i.Bytes())
}

func unmarshalDepositVersion0(dep *types.DepositTx, to common.Address, opaqueData []byte) error {
	if len(opaqueData) < 32+32+8+1 {
		return fmt.Errorf("unexpected opaqueData length: %d", len(opaqueData))
	}
	offset := uint64(0)

	// uint256 mint
	dep.Mint = new(big.Int).SetBytes(opaqueData[offset : offset+32])
	// 0 mint is represented as nil to skip minting code
	if dep.Mint.Cmp(new(big.Int)) == 0 {
		dep.Mint = nil
	}
	offset += 32

	// uint256 value
	dep.Value = new(big.Int).SetBytes(opaqueData[offset : offset+32])
	offset += 32

	// uint64 gas
	gas := new(big.Int).SetBytes(opaqueData[offset : offset+8])
	if !gas.IsUint64() {
		return fmt.Errorf("bad gas value: %x", opaqueData[offset:offset+8])
	}
	dep.Gas = gas.Uint64()
	offset += 8

	// uint8 isCreation
	// isCreation: If the boolean byte is 1 then dep.To will stay nil,
	// and it will create a contract using L2 account nonce to determine the created address.
	if opaqueData[offset] == 0 {
		dep.To = &to
	}
	offset += 1

	// The remainder of the opaqueData is the transaction data (without length prefix).
	// The data may be padded to a multiple of 32 bytes
	txDataLen := uint64(len(opaqueData)) - offset

	// remaining bytes fill the data
	dep.Data = opaqueData[offset : offset+txDataLen]

	return nil
}

func unmarshalDepositVersion1(dep *types.DepositTx, to common.Address, opaqueData []byte) error {
	// version 1 simply adds a nonce; the rest is the same
	return unmarshalDepositVersion0(dep, to, opaqueData)
}

// MarshalDepositLogEvent returns an EVM log entry that encodes a TransactionDeposited event from the deposit contract.
// This is the reverse of the deposit transaction derivation.
func MarshalDepositLogEvent(depositContractAddr common.Address, deposit *types.DepositTx) (*types.Log, error) {
	toBytes := common.Hash{}
	if deposit.To != nil {
		toBytes = eth.AddressAsLeftPaddedHash(*deposit.To)
	}
	topics := []common.Hash{
		DepositEventABIHash,
		eth.AddressAsLeftPaddedHash(deposit.From),
		toBytes,
		PackNonceAndVersion(0, DepositEventVersion0),
	}

	data := make([]byte, 64, 64+3*32)

	// opaqueData slice content offset: value will always be 0x20.
	binary.BigEndian.PutUint64(data[32-8:32], 32)

	opaqueData, err := marshalDepositVersion0(deposit)
	if err != nil {
		return &types.Log{}, err
	}

	// opaqueData slice length
	binary.BigEndian.PutUint64(data[64-8:64], uint64(len(opaqueData)))

	// opaqueData slice content
	data = append(data, opaqueData...)

	// pad to multiple of 32
	if len(data)%32 != 0 {
		data = append(data, make([]byte, 32-(len(data)%32))...)
	}

	return &types.Log{
		Address: depositContractAddr,
		Topics:  topics,
		Data:    data,
		Removed: false,

		// ignored (zeroed):
		BlockNumber: 0,
		TxHash:      common.Hash{},
		TxIndex:     0,
		BlockHash:   common.Hash{},
		Index:       0,
	}, nil
}

func marshalDepositVersion0(deposit *types.DepositTx) ([]byte, error) {
	opaqueData := make([]byte, 32+32+8+1, 32+32+8+1+len(deposit.Data))
	offset := 0

	// uint256 mint
	if deposit.Mint != nil {
		if deposit.Mint.BitLen() > 256 {
			return nil, fmt.Errorf("mint value exceeds 256 bits: %d", deposit.Mint)
		}
		deposit.Mint.FillBytes(opaqueData[offset : offset+32])
	}
	offset += 32

	// uint256 value
	if deposit.Value.BitLen() > 256 {
		return nil, fmt.Errorf("value value exceeds 256 bits: %d", deposit.Value)
	}
	deposit.Value.FillBytes(opaqueData[offset : offset+32])
	offset += 32

	// uint64 gas
	binary.BigEndian.PutUint64(opaqueData[offset:offset+8], deposit.Gas)
	offset += 8

	// uint8 isCreation
	if deposit.To == nil { // isCreation
		opaqueData[offset] = 1
	}

	// Deposit data then fills the remaining event data
	opaqueData = append(opaqueData, deposit.Data...)

	return opaqueData, nil
}
