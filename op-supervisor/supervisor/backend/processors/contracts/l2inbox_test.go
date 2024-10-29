package contracts

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/snapshots"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestDecodeExecutingMessageEvent(t *testing.T) {
	inbox := NewCrossL2Inbox()
	payload := bytes.Repeat([]byte{0xaa, 0xbb}, 50)
	payloadHash := crypto.Keccak256Hash(payload)
	expected := types.ExecutingMessage{
		Chain:     42424, // TODO(#11105): translate chain ID to chain index
		BlockNum:  12345,
		LogIdx:    98,
		Timestamp: 9578295,
	}
	contractIdent := contractIdentifier{
		Origin:      common.Address{0xbb, 0xcc},
		ChainId:     new(big.Int).SetUint64(uint64(expected.Chain)), // TODO(#11105): translate chain ID to chain index
		BlockNumber: new(big.Int).SetUint64(expected.BlockNum),
		Timestamp:   new(big.Int).SetUint64(expected.Timestamp),
		LogIndex:    new(big.Int).SetUint64(uint64(expected.LogIdx)),
	}
	expected.Hash = types.PayloadHashToLogHash(payloadHash, contractIdent.Origin)
	abi := snapshots.LoadCrossL2InboxABI()
	validData, err := abi.Events[eventExecutingMessage].Inputs.Pack(payloadHash, contractIdent)
	require.NoError(t, err)
	createValidLog := func() *ethTypes.Log {
		//protoHack := bytes.Repeat([]byte{0x00}, 32*5)
		return &ethTypes.Log{
			Address: predeploys.CrossL2InboxAddr,
			Topics:  []common.Hash{abi.Events[eventExecutingMessage].ID, payloadHash},
			Data:    validData,
		}
	}

	t.Run("ParseValid", func(t *testing.T) {
		l := createValidLog()
		result, err := inbox.DecodeExecutingMessageLog(l)
		require.NoError(t, err)
		require.Equal(t, expected, result)
	})

	t.Run("IgnoreIncorrectContract", func(t *testing.T) {
		l := createValidLog()
		l.Address = common.Address{0xff}
		_, err := inbox.DecodeExecutingMessageLog(l)
		require.ErrorIs(t, err, ErrEventNotFound)
	})

	t.Run("IgnoreWrongEvent", func(t *testing.T) {
		l := createValidLog()
		l.Topics[0] = common.Hash{0xbb}
		_, err := inbox.DecodeExecutingMessageLog(l)
		require.ErrorIs(t, err, ErrEventNotFound)
	})

	t.Run("ErrorOnInvalidEvent", func(t *testing.T) {
		l := createValidLog()
		l.Data = []byte{0xbb, 0xcc}
		_, err := inbox.DecodeExecutingMessageLog(l)
		require.ErrorIs(t, err, batching.ErrInvalidEvent)
	})
}
