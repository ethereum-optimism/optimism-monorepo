package keccak

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/ethereum-optimism/optimism/op-challenger/game/keccak/fetcher"
	"github.com/ethereum-optimism/optimism/op-challenger/game/keccak/matrix"
	keccakTypes "github.com/ethereum-optimism/optimism/op-challenger/game/keccak/types"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	lru "github.com/hashicorp/golang-lru/v2"
)

const validPreimageCacheSize = 500

type VerifierPreimageOracle interface {
	fetcher.Oracle
	GetProposalTreeRoot(ctx context.Context, block rpcblock.Block, ident keccakTypes.LargePreimageIdent) (common.Hash, error)
}

type Fetcher interface {
	FetchInputs(ctx context.Context, blockHash common.Hash, oracle fetcher.Oracle, ident keccakTypes.LargePreimageIdent) ([]keccakTypes.InputData, error)
}

type PreimageVerifier struct {
	log     log.Logger
	fetcher Fetcher

	// knownValid caches the merkle tree roots that have been confirmed as valid.
	// Invalid roots are not cached as those preimages will be ignored once the challenge is processed.
	knownValid *lru.Cache[common.Hash, bool]
}

func NewPreimageVerifier(logger log.Logger, fetcher Fetcher) *PreimageVerifier {
	// Can't error because size is hard coded
	cache, _ := lru.New[common.Hash, bool](validPreimageCacheSize)
	return &PreimageVerifier{
		log:        logger,
		fetcher:    fetcher,
		knownValid: cache,
	}
}

func (v *PreimageVerifier) CreateChallenge(ctx context.Context, blockHash common.Hash, oracle VerifierPreimageOracle, preimage keccakTypes.LargePreimageMetaData) (keccakTypes.Challenge, error) {
	root, err := oracle.GetProposalTreeRoot(ctx, rpcblock.ByHash(blockHash), preimage.LargePreimageIdent)
	if err != nil {
		return keccakTypes.Challenge{}, fmt.Errorf("failed to get proposal merkle root: %w", err)
	}
	if valid, ok := v.knownValid.Get(root); ok && valid {
		// We've already determined that the keccak transition is valid.
		// Note that the merkle tree may have been validated by a different proposal but since the tree root
		// commits to all the input data and the resulting keccak state matrix, any other proposal with the same
		// root must also have the same inputs and correctly applied keccak.
		// It is possible that this proposal can't be squeezed because the claimed data length doesn't match the
		// actual length but the contracts enforce that and it can't be challenged on that basis.
		return keccakTypes.Challenge{}, matrix.ErrValid
	}
	inputs, err := v.fetcher.FetchInputs(ctx, blockHash, oracle, preimage.LargePreimageIdent)
	if err != nil {
		return keccakTypes.Challenge{}, fmt.Errorf("failed to fetch leaves: %w", err)
	}
	readers := make([]io.Reader, 0, len(inputs))
	var commitments []common.Hash
	dataLen := 0
	for _, input := range inputs {
		readers = append(readers, bytes.NewReader(input.Input))
		commitments = append(commitments, input.Commitments...)
		dataLen += len(input.Input)
	}
	if dataLen != int(preimage.BytesProcessed) {
		return keccakTypes.Challenge{}, fmt.Errorf("retrieved incorrect input length, expected %v but got %v", preimage.BytesProcessed, dataLen)
	}
	challenge, err := matrix.Challenge(io.MultiReader(readers...), commitments)
	if errors.Is(err, matrix.ErrValid) {
		v.knownValid.Add(root, true)
		return keccakTypes.Challenge{}, err
	} else if err != nil {
		return keccakTypes.Challenge{}, fmt.Errorf("failed to create challenge: %w", err)
	}
	return challenge, nil
}
