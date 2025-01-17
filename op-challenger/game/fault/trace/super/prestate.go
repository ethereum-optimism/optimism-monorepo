package super

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type SuperRootPrestateProvider struct {
	provider  RootProvider
	timestamp uint64
}

var _ PreimagePrestateProvider = (*SuperRootPrestateProvider)(nil)

func NewSuperRootPrestateProvider(provider RootProvider, prestateTimestamp uint64) *SuperRootPrestateProvider {
	return &SuperRootPrestateProvider{
		provider:  provider,
		timestamp: prestateTimestamp,
	}
}

func (s *SuperRootPrestateProvider) AbsolutePreStateCommitment(ctx context.Context) (common.Hash, error) {
	prestate, err := s.AbsolutePreState(ctx)
	if err != nil {
		return common.Hash{}, err
	}
	return common.Hash(eth.SuperRoot(prestate)), nil
}

func (s *SuperRootPrestateProvider) AbsolutePreState(ctx context.Context) (eth.Super, error) {
	response, err := s.provider.SuperRootAtTimestamp(ctx, hexutil.Uint64(s.timestamp))
	if err != nil {
		return nil, err
	}
	return responseToSuper(response), nil
}
