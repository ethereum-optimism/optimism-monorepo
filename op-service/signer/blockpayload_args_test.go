package signer

import (
	"encoding/json"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"math/big"
	"testing"
)

func TestBlockPayloadArgs(t *testing.T) {
	cfg := &rollup.Config{
		L2ChainID: big.NewInt(100),
	}
	payloadBytes := []byte("arbitraryData")
	addr := common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	exampleDomain := [32]byte{0: 123, 1: 42}
	args := NewBlockPayloadArgs(exampleDomain, cfg.L2ChainID, payloadBytes, &addr)
	out, err := json.MarshalIndent(args, "  ", "  ")
	require.NoError(t, err)
	content := string(out)
	// previously erroneously included in every request. Not used on server-side. Should be dropped now.
	require.NotContains(t, content, "PayloadBytes")
	// mistyped as list of ints in v0
	require.Contains(t, content, `"domain": [`)
	require.Contains(t, content, ` 123,`)
	require.Contains(t, content, ` 42,`)
	require.Contains(t, content, `"chainId": 100`)
	// mistyped as standard Go bytes, hence base64
	require.Contains(t, content, `"payloadHash": "7qa7ZZHSC1LytldPsgv3J5zQPVgWE9jqHojIK4QAFEs="`)
	require.Contains(t, content, `"senderAddress": "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"`)
}
