package p2p

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"io"
	"math/big"
	"testing"
	"time"

	"github.com/golang/snappy"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	opsigner "github.com/ethereum-optimism/optimism/op-service/signer"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestGuardGossipValidator(t *testing.T) {
	logger := testlog.Logger(t, log.LevelCrit)
	val := guardGossipValidator(logger, func(ctx context.Context, id peer.ID, message *pubsub.Message) pubsub.ValidationResult {
		if id == "mallory" {
			panic("mallory was here")
		}
		if id == "bob" {
			return pubsub.ValidationIgnore
		}
		return pubsub.ValidationAccept
	})
	// Test that panics from mallory are recovered and rejected,
	// and test that we can continue to ignore bob and accept alice.
	require.Equal(t, pubsub.ValidationAccept, val(context.Background(), "alice", nil))
	require.Equal(t, pubsub.ValidationReject, val(context.Background(), "mallory", nil))
	require.Equal(t, pubsub.ValidationIgnore, val(context.Background(), "bob", nil))
	require.Equal(t, pubsub.ValidationReject, val(context.Background(), "mallory", nil))
	require.Equal(t, pubsub.ValidationAccept, val(context.Background(), "alice", nil))
	require.Equal(t, pubsub.ValidationIgnore, val(context.Background(), "bob", nil))
}

func TestCombinePeers(t *testing.T) {
	res := combinePeers([]peer.ID{"foo", "bar"}, []peer.ID{"bar", "baz"})
	require.Equal(t, []peer.ID{"foo", "bar", "baz"}, res)
}

func TestVerifyBlockSignature(t *testing.T) {
	logger := testlog.Logger(t, log.LevelCrit)
	cfg := &rollup.Config{
		L2ChainID: big.NewInt(100),
	}
	peerId := peer.ID("foo")
	secrets, err := crypto.GenerateKey()
	require.NoError(t, err)
	msg := []byte("any msg")

	t.Run("Valid", func(t *testing.T) {
		runCfg := &testutils.MockRuntimeConfig{P2PSeqAddress: crypto.PubkeyToAddress(secrets.PublicKey)}
		signer := &PreparedSigner{Signer: NewLocalSigner(secrets)}
		sig, err := signer.Sign(context.Background(), SigningDomainBlocksV1, cfg.L2ChainID, msg)
		require.NoError(t, err)
		result := verifyBlockSignature(logger, cfg, runCfg, peerId, sig[:], msg)
		require.Equal(t, pubsub.ValidationAccept, result)
	})

	t.Run("WrongSigner", func(t *testing.T) {
		runCfg := &testutils.MockRuntimeConfig{P2PSeqAddress: common.HexToAddress("0x1234")}
		signer := &PreparedSigner{Signer: NewLocalSigner(secrets)}
		sig, err := signer.Sign(context.Background(), SigningDomainBlocksV1, cfg.L2ChainID, msg)
		require.NoError(t, err)
		result := verifyBlockSignature(logger, cfg, runCfg, peerId, sig[:], msg)
		require.Equal(t, pubsub.ValidationReject, result)
	})

	t.Run("InvalidSignature", func(t *testing.T) {
		runCfg := &testutils.MockRuntimeConfig{P2PSeqAddress: crypto.PubkeyToAddress(secrets.PublicKey)}
		sig := make([]byte, 65)
		result := verifyBlockSignature(logger, cfg, runCfg, peerId, sig, msg)
		require.Equal(t, pubsub.ValidationReject, result)
	})

	t.Run("NoSequencer", func(t *testing.T) {
		runCfg := &testutils.MockRuntimeConfig{}
		signer := &PreparedSigner{Signer: NewLocalSigner(secrets)}
		sig, err := signer.Sign(context.Background(), SigningDomainBlocksV1, cfg.L2ChainID, msg)
		require.NoError(t, err)
		result := verifyBlockSignature(logger, cfg, runCfg, peerId, sig[:], msg)
		require.Equal(t, pubsub.ValidationIgnore, result)
	})
}

type mockRemoteSigner struct {
	priv *ecdsa.PrivateKey
}

func (t *mockRemoteSigner) SignBlockPayload(args opsigner.BlockPayloadArgs) (hexutil.Bytes, error) {
	signingHash, err := args.ToSigningHash()
	if err != nil {
		return nil, err
	}
	signature, err := crypto.Sign(signingHash[:], t.priv)
	if err != nil {
		return nil, err
	}
	return signature, nil
}

func TestVerifyBlockSignatureWithRemoteSigner(t *testing.T) {
	secrets, err := crypto.GenerateKey()
	require.NoError(t, err)

	remoteSigner := &mockRemoteSigner{secrets}
	server := oprpc.NewServer(
		"127.0.0.1",
		0,
		"test",
		oprpc.WithAPIs([]rpc.API{
			{
				Namespace: "opsigner",
				Service:   remoteSigner,
			},
		}),
	)

	require.NoError(t, server.Start())
	defer func() {
		_ = server.Stop()
	}()

	logger := testlog.Logger(t, log.LevelCrit)
	cfg := &rollup.Config{
		L2ChainID: big.NewInt(100),
	}

	peerId := peer.ID("foo")
	msg := []byte("any msg")

	signerCfg := opsigner.NewCLIConfig()
	signerCfg.Endpoint = fmt.Sprintf("http://%s", server.Endpoint())
	signerCfg.TLSConfig.TLSKey = ""
	signerCfg.TLSConfig.TLSCert = ""
	signerCfg.TLSConfig.TLSCaCert = ""
	signerCfg.TLSConfig.Enabled = false

	t.Run("Valid", func(t *testing.T) {
		runCfg := &testutils.MockRuntimeConfig{P2PSeqAddress: crypto.PubkeyToAddress(secrets.PublicKey)}
		remoteSigner, err := NewRemoteSigner(logger, signerCfg)
		require.NoError(t, err)
		signer := &PreparedSigner{Signer: remoteSigner}
		sig, err := signer.Sign(context.Background(), SigningDomainBlocksV1, cfg.L2ChainID, msg)
		require.NoError(t, err)
		result := verifyBlockSignature(logger, cfg, runCfg, peerId, sig[:], msg)
		require.Equal(t, pubsub.ValidationAccept, result)
	})

	t.Run("WrongSigner", func(t *testing.T) {
		runCfg := &testutils.MockRuntimeConfig{P2PSeqAddress: common.HexToAddress("0x1234")}
		remoteSigner, err := NewRemoteSigner(logger, signerCfg)
		require.NoError(t, err)
		signer := &PreparedSigner{Signer: remoteSigner}
		sig, err := signer.Sign(context.Background(), SigningDomainBlocksV1, cfg.L2ChainID, msg)
		require.NoError(t, err)
		result := verifyBlockSignature(logger, cfg, runCfg, peerId, sig[:], msg)
		require.Equal(t, pubsub.ValidationReject, result)
	})

	t.Run("InvalidSignature", func(t *testing.T) {
		runCfg := &testutils.MockRuntimeConfig{P2PSeqAddress: crypto.PubkeyToAddress(secrets.PublicKey)}
		sig := make([]byte, 65)
		result := verifyBlockSignature(logger, cfg, runCfg, peerId, sig, msg)
		require.Equal(t, pubsub.ValidationReject, result)
	})

	t.Run("NoSequencer", func(t *testing.T) {
		runCfg := &testutils.MockRuntimeConfig{}
		remoteSigner, err := NewRemoteSigner(logger, signerCfg)
		require.NoError(t, err)
		signer := &PreparedSigner{Signer: remoteSigner}
		sig, err := signer.Sign(context.Background(), SigningDomainBlocksV1, cfg.L2ChainID, msg)
		require.NoError(t, err)
		result := verifyBlockSignature(logger, cfg, runCfg, peerId, sig[:], msg)
		require.Equal(t, pubsub.ValidationIgnore, result)
	})

	t.Run("RemoteSignerNoTLS", func(t *testing.T) {
		signerCfg := opsigner.NewCLIConfig()
		signerCfg.Endpoint = fmt.Sprintf("http://%s", server.Endpoint())
		signerCfg.TLSConfig.TLSKey = "invalid"
		signerCfg.TLSConfig.TLSCert = "invalid"
		signerCfg.TLSConfig.TLSCaCert = "invalid"
		signerCfg.TLSConfig.Enabled = true

		_, err := NewRemoteSigner(logger, signerCfg)
		require.Error(t, err)
	})

	t.Run("RemoteSignerInvalidEndpoint", func(t *testing.T) {
		signerCfg := opsigner.NewCLIConfig()
		signerCfg.Endpoint = "Invalid"
		signerCfg.TLSConfig.TLSKey = ""
		signerCfg.TLSConfig.TLSCert = ""
		signerCfg.TLSConfig.TLSCaCert = ""
		_, err := NewRemoteSigner(logger, signerCfg)
		require.Error(t, err)
	})
}

type MarshalSSZ interface {
	MarshalSSZ(w io.Writer) (n int, err error)
}

func createSignedP2Payload(payload MarshalSSZ, signer Signer, l2ChainID *big.Int) ([]byte, error) {
	var buf bytes.Buffer
	buf.Write(make([]byte, 65))
	if _, err := payload.MarshalSSZ(&buf); err != nil {
		return nil, fmt.Errorf("failed to encoded execution payload to publish: %w", err)
	}
	data := buf.Bytes()
	payloadData := data[65:]
	sig, err := signer.Sign(context.TODO(), SigningDomainBlocksV1, l2ChainID, payloadData)
	if err != nil {
		return nil, fmt.Errorf("failed to sign execution payload with signer: %w", err)
	}
	copy(data[:65], sig[:])

	// compress the full message
	// This also copies the data, freeing up the original buffer to go back into the pool
	return snappy.Encode(nil, data), nil
}

func createExecutionPayload(w types.Withdrawals, withdrawalsRoot *common.Hash, excessGas, gasUsed *uint64) *eth.ExecutionPayload {
	return &eth.ExecutionPayload{
		Timestamp:       hexutil.Uint64(time.Now().Unix()),
		Withdrawals:     &w,
		WithdrawalsRoot: withdrawalsRoot,
		ExcessBlobGas:   (*eth.Uint64Quantity)(excessGas),
		BlobGasUsed:     (*eth.Uint64Quantity)(gasUsed),
	}
}

func createEnvelope(h *common.Hash, w types.Withdrawals, withdrawalsRoot *common.Hash, excessGas, gasUsed *uint64) *eth.ExecutionPayloadEnvelope {
	return &eth.ExecutionPayloadEnvelope{
		ExecutionPayload:      createExecutionPayload(w, withdrawalsRoot, excessGas, gasUsed),
		ParentBeaconBlockRoot: h,
	}
}

// TestBlockValidator does some very basic tests of the p2p block validation logic
func TestBlockValidator(t *testing.T) {
	// Params Set 1: Create the validation function
	cfg := &rollup.Config{
		L2ChainID: big.NewInt(100),
	}
	secrets, err := crypto.GenerateKey()
	require.NoError(t, err)
	runCfg := &testutils.MockRuntimeConfig{P2PSeqAddress: crypto.PubkeyToAddress(secrets.PublicKey)}
	signer := &PreparedSigner{Signer: NewLocalSigner(secrets)}
	// Params Set 2: Call the validation function
	peerID := peer.ID("foo")

	v2Validator := BuildBlocksValidator(testlog.Logger(t, log.LevelCrit), cfg, runCfg, eth.BlockV2)
	v3Validator := BuildBlocksValidator(testlog.Logger(t, log.LevelCrit), cfg, runCfg, eth.BlockV3)
	v4Validator := BuildBlocksValidator(testlog.Logger(t, log.LevelCrit), cfg, runCfg, eth.BlockV4)

	zero, one := uint64(0), uint64(1)
	beaconHash, withdrawalsRoot := common.HexToHash("0x1234"), common.HexToHash("0x9876")

	payloadTests := []struct {
		name      string
		validator pubsub.ValidatorEx
		result    pubsub.ValidationResult
		payload   *eth.ExecutionPayload
	}{
		{"V2Valid", v2Validator, pubsub.ValidationAccept, createExecutionPayload(types.Withdrawals{}, nil, nil, nil)},
		{"V2NonZeroWithdrawals", v2Validator, pubsub.ValidationReject, createExecutionPayload(types.Withdrawals{&types.Withdrawal{Index: 1, Validator: 1}}, nil, nil, nil)},
		{"V2NonZeroBlobProperties", v2Validator, pubsub.ValidationReject, createExecutionPayload(types.Withdrawals{}, nil, &zero, &zero)},
		{"V3RejectExecutionPayload", v3Validator, pubsub.ValidationReject, createExecutionPayload(types.Withdrawals{}, nil, &zero, &zero)},
	}

	for _, tt := range payloadTests {
		test := tt
		t.Run(fmt.Sprintf("ExecutionPayload_%s", test.name), func(t *testing.T) {
			e := &eth.ExecutionPayloadEnvelope{ExecutionPayload: test.payload}
			test.payload.BlockHash, _ = e.CheckBlockHash() // hack to generate the block hash easily.
			data, err := createSignedP2Payload(test.payload, signer, cfg.L2ChainID)
			require.NoError(t, err)
			message := &pubsub.Message{Message: &pubsub_pb.Message{Data: data}}
			res := test.validator(context.TODO(), peerID, message)
			require.Equal(t, res, test.result)
		})
	}

	envelopeTests := []struct {
		name      string
		validator pubsub.ValidatorEx
		result    pubsub.ValidationResult
		payload   *eth.ExecutionPayloadEnvelope
	}{
		{"V3RejectNonZeroExcessGas", v3Validator, pubsub.ValidationReject, createEnvelope(&beaconHash, types.Withdrawals{}, nil, &one, &zero)},
		{"V3RejectNonZeroBlobGasUsed", v3Validator, pubsub.ValidationReject, createEnvelope(&beaconHash, types.Withdrawals{}, nil, &zero, &one)},
		{"V3RejectNonZeroBlobGasUsed", v3Validator, pubsub.ValidationReject, createEnvelope(&beaconHash, types.Withdrawals{}, nil, &zero, &one)},
		{"V3Valid", v3Validator, pubsub.ValidationAccept, createEnvelope(&beaconHash, types.Withdrawals{}, nil, &zero, &zero)},
		{"V4Valid", v4Validator, pubsub.ValidationAccept, createEnvelope(&beaconHash, types.Withdrawals{}, &withdrawalsRoot, &zero, &zero)},
		{"V4RejectNoWithdrawalRoot", v4Validator, pubsub.ValidationReject, createEnvelope(&beaconHash, types.Withdrawals{}, nil, &zero, &zero)},
	}

	for _, tt := range envelopeTests {
		test := tt
		t.Run(fmt.Sprintf("ExecutionPayloadEnvelope_%s", test.name), func(t *testing.T) {
			test.payload.ExecutionPayload.BlockHash, _ = test.payload.CheckBlockHash() // hack to generate the block hash easily.
			data, err := createSignedP2Payload(test.payload, signer, cfg.L2ChainID)
			require.NoError(t, err)
			message := &pubsub.Message{Message: &pubsub_pb.Message{Data: data}}
			res := test.validator(context.TODO(), peerID, message)
			require.Equal(t, res, test.result)
		})
	}
}
