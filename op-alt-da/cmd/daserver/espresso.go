package main

import (
	"context"

	espressoClient "github.com/EspressoSystems/espresso-sequencer-go/client"
	tagged_base64 "github.com/EspressoSystems/espresso-sequencer-go/tagged-base64"
	"github.com/ethereum/go-ethereum/log"
)

type EspressoStore struct {
	client *espressoClient.Client
	logger log.Logger
}

func NewEspressoStore(endpt string, logger log.Logger) *EspressoStore {
	client := espressoClient.NewClient(endpt)

	return &EspressoStore{
		client: client,
		logger: logger,
	}
}

func (s *EspressoStore) Get(ctx context.Context, key []byte) ([]byte, error) {
	s.logger.Info("Get request", "key", key)
	tb64, err := tagged_base64.New("TX", key[1:])
	if err != nil {
		return nil, err
	}
	result, err := s.client.FetchTransactionByHash(ctx, tb64)
	if err != nil {
		return nil, err
	}
	return result.Transaction.Payload, nil
}

func (s *EspressoStore) Put(ctx context.Context, key []byte, value []byte) error {
	s.logger.Warn("Put request, ignoring", "key", key)
	return nil
}
