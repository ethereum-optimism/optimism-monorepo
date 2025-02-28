package p2p

import (
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	opsigner "github.com/ethereum-optimism/optimism/op-service/signer"
	"github.com/stretchr/testify/require"
)

func TestSigningHash_DifferentDomain(t *testing.T) {
	cfg := &rollup.Config{
		L2ChainID: big.NewInt(100),
	}

	payloadBytes := []byte("arbitraryData")
	msg, err := opsigner.NewBlockPayloadArgs(SigningDomainBlocksV1, cfg.L2ChainID, payloadBytes, nil).Message()
	require.NoError(t, err, "creating first signing hash")

	msg2, err := opsigner.NewBlockPayloadArgs([32]byte{3}, cfg.L2ChainID, payloadBytes, nil).Message()
	require.NoError(t, err, "creating second signing hash")

	hash := msg.ToSigningHash()
	hash2 := msg2.ToSigningHash()
	require.NotEqual(t, hash, hash2, "signing hash should be different when domain is different")
}

func TestSigningHash_DifferentChainID(t *testing.T) {
	cfg1 := &rollup.Config{
		L2ChainID: big.NewInt(100),
	}
	cfg2 := &rollup.Config{
		L2ChainID: big.NewInt(101),
	}

	payloadBytes := []byte("arbitraryData")
	msg, err := opsigner.NewBlockPayloadArgs(SigningDomainBlocksV1, cfg1.L2ChainID, payloadBytes, nil).Message()
	require.NoError(t, err, "creating first signing hash")

	msg2, err := opsigner.NewBlockPayloadArgs(SigningDomainBlocksV1, cfg2.L2ChainID, payloadBytes, nil).Message()
	require.NoError(t, err, "creating second signing hash")

	hash := msg.ToSigningHash()
	hash2 := msg2.ToSigningHash()
	require.NotEqual(t, hash, hash2, "signing hash should be different when chain ID is different")
}

func TestSigningHash_DifferentPayload(t *testing.T) {
	cfg := &rollup.Config{
		L2ChainID: big.NewInt(100),
	}

	msg, err := opsigner.NewBlockPayloadArgs(SigningDomainBlocksV1, cfg.L2ChainID, []byte("payload1"), nil).Message()
	require.NoError(t, err, "creating first signing hash")

	msg2, err := opsigner.NewBlockPayloadArgs(SigningDomainBlocksV1, cfg.L2ChainID, []byte("payload2"), nil).Message()
	require.NoError(t, err, "creating second signing hash")

	hash := msg.ToSigningHash()
	hash2 := msg2.ToSigningHash()
	require.NotEqual(t, hash, hash2, "signing hash should be different when payload is different")
}

func TestSigningHash_LimitChainID(t *testing.T) {
	// ChainID with bitlen 257
	chainID := big.NewInt(1)
	chainID = chainID.SetBit(chainID, 256, 1)
	cfg := &rollup.Config{
		L2ChainID: chainID,
	}
	_, err := opsigner.NewBlockPayloadArgs(SigningDomainBlocksV1, cfg.L2ChainID, []byte("arbitraryData"), nil).Message()
	require.ErrorContains(t, err, "chain_id is too large")
}
