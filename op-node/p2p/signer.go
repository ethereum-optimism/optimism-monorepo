package p2p

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	opsigner "github.com/ethereum-optimism/optimism/op-service/signer"
)

var SigningDomainBlocksV1 = [32]byte{}

type Signer interface {
	Sign(ctx context.Context, domain [32]byte, chainID *big.Int, encodedMsg []byte) (sig *[65]byte, err error)
	io.Closer
}

func BlockSigningHash(cfg *rollup.Config, payloadBytes []byte) (common.Hash, error) {
	return opsigner.NewBlockPayloadArgs(SigningDomainBlocksV1, cfg.L2ChainID, payloadBytes, nil).ToSigningHash()
}

// LocalSigner is suitable for testing
type LocalSigner struct {
	priv *ecdsa.PrivateKey
}

func NewLocalSigner(priv *ecdsa.PrivateKey) *LocalSigner {
	return &LocalSigner{priv: priv}
}

func (s *LocalSigner) Sign(ctx context.Context, domain [32]byte, chainID *big.Int, encodedMsg []byte) (sig *[65]byte, err error) {
	if s.priv == nil {
		return nil, errors.New("signer is closed")
	}

	blockPayloadArgs := opsigner.NewBlockPayloadArgs(domain, chainID, encodedMsg, nil)
	signingHash, err := blockPayloadArgs.ToSigningHash()

	if err != nil {
		return nil, err
	}
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

func (s *RemoteSigner) Sign(ctx context.Context, domain [32]byte, chainID *big.Int, encodedMsg []byte) (sig *[65]byte, err error) {
	if s.client == nil {
		return nil, errors.New("signer is closed")
	}

	blockPayloadArgs := opsigner.NewBlockPayloadArgs(domain, chainID, encodedMsg, s.sender)
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
