package localkey

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/p2p"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/signer"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/backend/work"
	"github.com/ethereum-optimism/optimism/op-test-sequencer/sequencer/seqtypes"
)

type Signer struct {
	id  seqtypes.SignerID
	log log.Logger

	chainID eth.ChainID
	signer  *p2p.LocalSigner
}

var _ work.Signer = (*Signer)(nil)

func NewSigner(id seqtypes.SignerID, log log.Logger, priv *ecdsa.PrivateKey) *Signer {
	signer := p2p.NewLocalSigner(priv)
	return &Signer{id: id, log: log, signer: signer}
}

func (s *Signer) String() string {
	return "local-key-signer-" + s.id.String()
}

func (s *Signer) ID() seqtypes.SignerID {
	return s.id
}

func (s *Signer) Close() error {
	return nil
}

func (s *Signer) Sign(ctx context.Context, v work.Block) (work.SignedBlock, error) {
	envelope, ok := v.(*eth.ExecutionPayloadEnvelope)
	if !ok {
		return nil, fmt.Errorf("cannot sign unknown block kind %T: %w", v, seqtypes.ErrUnknownKind)
	}

	var buf bytes.Buffer
	if _, err := envelope.MarshalSSZ(&buf); err != nil {
		return nil, fmt.Errorf("failed to encode execution payload: %w", err)
	}

	payloadHash := signer.PayloadHash(buf.Bytes())
	sig, err := s.signer.Sign(ctx, p2p.SigningDomainBlocksV1, s.chainID, payloadHash)

	// TODO: signing wrapper
	_, _ = sig, err
	return nil, nil
}
