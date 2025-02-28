package p2p

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	opsigner "github.com/ethereum-optimism/optimism/op-service/signer"
)

var SigningDomainBlocksV1 = [32]byte{}

type Signer interface {
	Sign(ctx context.Context, domain eth.Bytes32, chainID eth.ChainID, payloadHash common.Hash) (sig *[65]byte, err error)
	io.Closer
}

func BlockSigningHash(cfg *rollup.Config, payloadBytes []byte) (common.Hash, error) {
	msg := opsigner.BlockSigningMessage{
		Domain:      SigningDomainBlocksV1,
		ChainID:     eth.ChainIDFromBig(cfg.L2ChainID),
		PayloadHash: opsigner.PayloadHash(payloadBytes),
	}
	return msg.ToSigningHash(), nil
}

// LocalSigner is suitable for testing
type LocalSigner struct {
	priv *ecdsa.PrivateKey
}

func NewLocalSigner(priv *ecdsa.PrivateKey) *LocalSigner {
	return &LocalSigner{priv: priv}
}

func (s *LocalSigner) Sign(ctx context.Context, domain eth.Bytes32, chainID eth.ChainID, payloadHash common.Hash) (sig *[65]byte, err error) {
	if s.priv == nil {
		return nil, errors.New("signer is closed")
	}
	msg := opsigner.BlockSigningMessage{
		Domain:      domain,
		ChainID:     chainID,
		PayloadHash: payloadHash,
	}
	signingHash := msg.ToSigningHash()
	signature, err := crypto.Sign(signingHash[:], s.priv)
	if err != nil {
		return nil, err
	}
	return (*[65]byte)(signature), nil
}

func (s *LocalSigner) Close() error {
	s.priv = nil
	return nil
}

type RemoteSigner struct {
	client *opsigner.SignerClient
	sender *common.Address
}

func NewRemoteSigner(logger log.Logger, config opsigner.CLIConfig) (*RemoteSigner, error) {
	signerClient, err := opsigner.NewSignerClientFromConfig(logger, config)
	if err != nil {
		return nil, err
	}
	senderAddress := common.HexToAddress(config.Address)
	return &RemoteSigner{signerClient, &senderAddress}, nil
}

func (s *RemoteSigner) Sign(ctx context.Context, domain eth.Bytes32, chainID eth.ChainID, payloadHash common.Hash) (sig *[65]byte, err error) {
	if s.client == nil {
		return nil, errors.New("signer is closed")
	}

	// We use V1 for now, since the server may not support V2 yet
	blockPayloadArgs := &opsigner.BlockPayloadArgs{
		Domain:        domain,
		ChainID:       chainID.ToBig(),
		PayloadHash:   payloadHash[:],
		SenderAddress: s.sender,
	}
	signature, err := s.client.SignBlockPayload(ctx, blockPayloadArgs)
	if err != nil {
		return nil, err
	}
	return &signature, nil
}

func (s *RemoteSigner) Close() error {
	s.client = nil
	return nil
}

type PreparedSigner struct {
	Signer
}

func (p *PreparedSigner) SetupSigner(ctx context.Context) (Signer, error) {
	return p.Signer, nil
}

type SignerSetup interface {
	SetupSigner(ctx context.Context) (Signer, error)
}
