package processors

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

func TestDecodeExecutingMessageLog(t *testing.T) {
	data := `
{
  "address": "0x4200000000000000000000000000000000000022",
  "topics": [
    "0x5c37832d2e8d10e346e55ad62071a6a2f9fa5130614ef2ec6617555c6f467ba7",
    "0xc3f57e1f0dd62a4f77787d834029bfeaab8894022c47edbe13b044fb658c9190"
  ],
  "data": "0x0000000000000000000000005fbdb2315678afecb367f032d93f642f64180aa3000000000000000000000000000000000000000000000000000000000000119d0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000006724d56300000000000000000000000000000000000000000000000000000000000dbc68",
  "blockHash": "0x355b82e9db9105fe3e7b7ed1897878ecbba8be7f30f94aca9dc55b6296a624cf",
  "blockNumber": "0x13a8",
  "transactionHash": "0x6eb22bb67562ac6a8fdbf60d6227c0b1f3f9d1d15ead1b0de055358f4fb9fa69",
  "transactionIndex": "0x2",
  "logIndex": "0x0",
  "removed": false
}
`
	var logEvent ethTypes.Log
	require.NoError(t, json.Unmarshal([]byte(data), &logEvent))

	msg, err := DecodeExecutingMessageLog(&logEvent)
	require.NoError(t, err)
	require.NotNil(t, msg)

	// struct Identifier {
	//     address origin;
	//     uint256 blockNumber;
	//     uint256 logIndex;
	//     uint256 timestamp;
	//     uint256 chainId;
	// }
	// function executeMessage(Identifier calldata _id,
	//     address _target, bytes calldata _message) external payable;

	originAddr := common.HexToAddress("0x5fbdb2315678afecb367f032d93f642f64180aa3")
	payloadHash := common.HexToHash("0xc3f57e1f0dd62a4f77787d834029bfeaab8894022c47edbe13b044fb658c9190")
	logHash := types.PayloadHashToLogHash(payloadHash, originAddr)

	require.Equal(t, logHash, msg.Hash)
	require.Equal(t, uint64(4509), msg.BlockNum)
	require.Equal(t, uint32(0), msg.LogIdx)
	require.Equal(t, uint64(1730467171), msg.Timestamp)
	require.Equal(t, types.ChainIndex(900200), msg.Chain)
}
