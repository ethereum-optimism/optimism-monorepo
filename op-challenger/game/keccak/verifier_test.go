package keccak

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math/big"
	"math/rand"
	"sync/atomic"
	"testing"

	"github.com/ethereum-optimism/optimism/op-challenger/game/keccak/fetcher"
	"github.com/ethereum-optimism/optimism/op-challenger/game/keccak/matrix"
	keccakTypes "github.com/ethereum-optimism/optimism/op-challenger/game/keccak/types"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

func TestVerify(t *testing.T) {
	logger := testlog.Logger(t, log.LevelInfo)
	tests := []struct {
		name        string
		inputs      func() []keccakTypes.InputData
		expectedErr error
	}{
		{
			name:        "Valid-SingleInput",
			inputs:      func() []keccakTypes.InputData { return validInputs(t, 1) },
			expectedErr: matrix.ErrValid,
		},
		{
			name:        "Valid-MultipleInputs",
			inputs:      func() []keccakTypes.InputData { return validInputs(t, 3) },
			expectedErr: matrix.ErrValid,
		},
		{
			name: "Invalid-FirstCommitment",
			inputs: func() []keccakTypes.InputData {
				inputs := validInputs(t, 1)
				inputs[0].Commitments[0] = common.Hash{0xaa}
				return inputs
			},
			expectedErr: nil,
		},
		{
			name: "Invalid-MiddleCommitment",
			inputs: func() []keccakTypes.InputData {
				inputs := validInputs(t, 1)
				inputs[0].Commitments[1] = common.Hash{0xaa}
				return inputs
			},
			expectedErr: nil,
		},
		{
			name: "Invalid-LastCommitment",
			inputs: func() []keccakTypes.InputData {
				inputs := validInputs(t, 3)
				inputs[2].Commitments[len(inputs[2].Commitments)-1] = common.Hash{0xaa}
				return inputs
			},
			expectedErr: nil,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			inputs := test.inputs()
			fetcher := &stubFetcher{
				inputs: inputs,
			}

			verifier := NewPreimageVerifier(logger, fetcher)
			preimage := keccakTypes.LargePreimageMetaData{
				BytesProcessed: inputLength(inputs),
			}
			oracle := &stubOracle{
				treeRoots: map[keccakTypes.LargePreimageIdent]common.Hash{
					preimage.LargePreimageIdent: {0xde},
				},
			}
			challenge, err := verifier.CreateChallenge(context.Background(), common.Hash{0xff}, oracle, preimage)
			require.ErrorIs(t, err, test.expectedErr)
			if err == nil {
				// Leave checking the validity of the challenge to the StateMatrix tests
				// Just confirm that we got a non-zero challenge
				require.NotEqual(t, keccakTypes.Challenge{}, challenge)
			} else {
				require.Equal(t, keccakTypes.Challenge{}, challenge)
			}
		})
	}
}

func TestCacheValidRoots(t *testing.T) {
	logger := testlog.Logger(t, log.LvlInfo)
	inputs := validInputs(t, 1)
	fetcher := &stubFetcher{
		inputs: inputs,
	}
	verifier := NewPreimageVerifier(logger, fetcher)
	preimage1 := keccakTypes.LargePreimageMetaData{
		LargePreimageIdent: keccakTypes.LargePreimageIdent{
			Claimant: common.Address{0x12},
			UUID:     big.NewInt(1),
		},
		BytesProcessed: inputLength(inputs),
	}
	preimage2 := keccakTypes.LargePreimageMetaData{
		LargePreimageIdent: keccakTypes.LargePreimageIdent{
			Claimant: common.Address{0x23},
			UUID:     big.NewInt(2),
		},
		BytesProcessed: inputLength(inputs),
	}
	oracle := &stubOracle{
		treeRoots: map[keccakTypes.LargePreimageIdent]common.Hash{
			preimage1.LargePreimageIdent: {0xde},
			preimage2.LargePreimageIdent: {0xde},
		},
	}
	challenge, err := verifier.CreateChallenge(context.Background(), common.Hash{0xff}, oracle, preimage1)
	require.ErrorIs(t, err, matrix.ErrValid)
	require.Equal(t, keccakTypes.Challenge{}, challenge, "Should be valid")
	require.EqualValues(t, 1, fetcher.fetchCount.Load(), "Should fetch data and validate")

	// Should cache the validity
	challenge, err = verifier.CreateChallenge(context.Background(), common.Hash{0xee}, oracle, preimage1)
	require.ErrorIs(t, err, matrix.ErrValid)
	require.Equal(t, keccakTypes.Challenge{}, challenge, "Should be valid")
	require.EqualValues(t, 1, fetcher.fetchCount.Load(), "Should use cached validity")

	// Should cache the validity across different challenges
	challenge, err = verifier.CreateChallenge(context.Background(), common.Hash{0xee}, oracle, preimage2)
	require.ErrorIs(t, err, matrix.ErrValid)
	require.Equal(t, keccakTypes.Challenge{}, challenge, "Should be valid")
	require.EqualValues(t, 1, fetcher.fetchCount.Load(), "Should use cached validity")
}

func validInputs(t *testing.T, inputCount int) []keccakTypes.InputData {
	chunkSize := 2 * keccakTypes.BlockSize
	data := testutils.RandomData(rand.New(rand.NewSource(4444)), inputCount*chunkSize)
	var calls []keccakTypes.InputData
	in := bytes.NewReader(data)
	s := matrix.NewStateMatrix()
	for {
		call, err := s.AbsorbUpTo(in, chunkSize)
		if !errors.Is(err, io.EOF) {
			require.NoError(t, err)
		}
		calls = append(calls, call)
		if errors.Is(err, io.EOF) {
			break
		}
	}
	return calls
}

type stubFetcher struct {
	inputs     []keccakTypes.InputData
	fetchCount atomic.Int64
}

func (s *stubFetcher) FetchInputs(_ context.Context, _ common.Hash, _ fetcher.Oracle, _ keccakTypes.LargePreimageIdent) ([]keccakTypes.InputData, error) {
	s.fetchCount.Add(1)
	return s.inputs, nil
}

func inputLength(inputs []keccakTypes.InputData) uint32 {
	inputLen := uint32(0)
	for _, input := range inputs {
		inputLen += uint32(len(input.Input))
	}
	return inputLen
}
