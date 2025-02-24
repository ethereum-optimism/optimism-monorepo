package preimage

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/preimages"
	"github.com/ethereum-optimism/optimism/op-challenger/game/keccak/matrix"
	"github.com/ethereum-optimism/optimism/op-challenger/game/keccak/types"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/transactions"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

const MinPreimageSize = 10000

type Helper struct {
	t            *testing.T
	require      *require.Assertions
	client       *ethclient.Client
	privKey      *ecdsa.PrivateKey
	oracle       contracts.PreimageOracleContract
	uuidProvider atomic.Int64
}

func NewHelper(t *testing.T, privKey *ecdsa.PrivateKey, client *ethclient.Client, oracle contracts.PreimageOracleContract) *Helper {
	return &Helper{
		t:       t,
		require: require.New(t),
		client:  client,
		privKey: privKey,
		oracle:  oracle,
	}
}

type InputModifier func(startBlock uint64, input *types.InputData)

func WithReplacedCommitment(idx uint64, value common.Hash) InputModifier {
	return func(startBlock uint64, input *types.InputData) {
		if startBlock > idx {
			return
		}
		if startBlock+uint64(len(input.Commitments)) < idx {
			return
		}
		input.Commitments[idx-startBlock] = value
	}
}

func WithLastCommitment(value common.Hash) InputModifier {
	return func(startBlock uint64, input *types.InputData) {
		if input.Finalize {
			input.Commitments[len(input.Commitments)-1] = value
		}
	}
}

// UploadLargePreimage inits the preimage upload and uploads the leaves, starting the challenge period.
// Squeeze is not called by this method as the challenge period has not yet elapsed.
func (h *Helper) UploadLargePreimage(ctx context.Context, dataSize int, modifiers ...InputModifier) types.LargePreimageIdent {
	//data := testutils.RandomData(rand.New(rand.NewSource(1234)), dataSize)
	data := make([]byte, dataSize)
	for i := range data {
		data[i] = 0xFF
	}
	s := matrix.NewStateMatrix()
	uuid := big.NewInt(h.uuidProvider.Add(1))
	candidate, err := h.oracle.InitLargePreimage(uuid, 32, uint32(len(data)))
	h.require.NoError(err)
	transactions.RequireSendTx(h.t, ctx, h.client, candidate, h.privKey)

	startBlock := big.NewInt(0)
	totalBlocks := len(data) / types.BlockSize
	in := bytes.NewReader(data)
	for {
		inputData, err := s.AbsorbUpTo(in, preimages.MaxChunkSize)
		if !errors.Is(err, io.EOF) {
			h.require.NoError(err)
		}
		//for _, modifier := range modifiers {
		//	modifier(startBlock.Uint64(), &inputData)
		//}
		h.t.Logf("Uploading %v parts of preimage %v starting at block %v of about %v Finalize: %v", len(inputData.Commitments), uuid.Uint64(), startBlock.Uint64(), totalBlocks, inputData.Finalize)
		commitments := make([]common.Hash, len(inputData.Commitments))
		for i := range commitments {
			commitments[i] = common.Hash(bytes.Repeat([]byte{0xFF}, 32))
		}
		tx, err := h.oracle.AddLeaves(uuid, startBlock, inputData.Input, commitments, inputData.Finalize)
		h.require.NoError(err)
		transactions.RequireSendTx(h.t, ctx, h.client, tx, h.privKey)
		startBlock = new(big.Int).Add(startBlock, big.NewInt(int64(len(inputData.Commitments))))
		if inputData.Finalize {
			break
		}
	}

	return types.LargePreimageIdent{
		Claimant: crypto.PubkeyToAddress(h.privKey.PublicKey),
		UUID:     uuid,
	}
}
func (h *Helper) InitBadLargePreimage(ctx context.Context) {
	chunkSize := 500 * 136
	bytesToSubmit := 4_012_000
	chunk := make([]byte, chunkSize)
	for i := range chunk {
		chunk[i] = 0xFF
	}
	mockStateCommitments := make([]common.Hash, chunkSize/136)
	mockStateCommitmentsLast := make([]common.Hash, chunkSize/136+1)
	for i := 0; i < len(mockStateCommitments); i++ {
		mockStateCommitments[i] = common.Hash(bytes.Repeat([]byte{0xFF}, 32))
		mockStateCommitmentsLast[i] = common.Hash(bytes.Repeat([]byte{0xFF}, 32))
	}
	mockStateCommitmentsLast[len(mockStateCommitments)-1] = common.Hash(bytes.Repeat([]byte{0xFF}, 32))

	uuid := big.NewInt(h.uuidProvider.Add(1))
	candidate, err := h.oracle.InitLargePreimage(uuid, 32, uint32(bytesToSubmit))
	h.require.NoError(err)
	transactions.RequireSendTx(h.t, ctx, h.client, candidate, h.privKey)
}

func (h *Helper) UploadBadLargePreimage(ctx context.Context) types.LargePreimageIdent {
	chunkSize := 500 * 136
	bytesToSubmit := 4_012_000
	chunk := make([]byte, chunkSize)
	for i := range chunk {
		chunk[i] = 0xFF
	}
	mockStateCommitments := make([]common.Hash, chunkSize/136)
	mockStateCommitmentsLast := make([]common.Hash, chunkSize/136+1)
	for i := 0; i < len(mockStateCommitments); i++ {
		mockStateCommitments[i] = common.Hash(bytes.Repeat([]byte{0xFF}, 32))
		mockStateCommitmentsLast[i] = common.Hash(bytes.Repeat([]byte{0xFF}, 32))
	}
	mockStateCommitmentsLast[len(mockStateCommitments)-1] = common.Hash(bytes.Repeat([]byte{0xFF}, 32))

	uuid := big.NewInt(h.uuidProvider.Add(1))
	candidate, err := h.oracle.InitLargePreimage(uuid, 32, uint32(bytesToSubmit))
	h.require.NoError(err)
	transactions.RequireSendTx(h.t, ctx, h.client, candidate, h.privKey)

	// Submit LPP in 500 * 136 byte chunks
	for i := 0; i < bytesToSubmit; i += chunkSize {
		finalize := i+chunkSize >= bytesToSubmit
		commitments := mockStateCommitments
		if finalize {
			commitments = mockStateCommitmentsLast
		}
		h.t.Logf("Uploading leaf %v, finalize %v", i/136, finalize)
		tx, err := h.oracle.AddLeaves(uuid, big.NewInt(int64(i/136)), chunk, commitments, finalize)
		require.NoError(h.t, err)
		transactions.RequireSendTx(h.t, ctx, h.client, tx, h.privKey)
	}

	ident := types.LargePreimageIdent{
		Claimant: crypto.PubkeyToAddress(h.privKey.PublicKey),
		UUID:     uuid,
	}

	h.WaitForFinalized(ctx, ident)
	metadatas, err := h.oracle.GetProposalMetadata(ctx, rpcblock.Latest, ident)
	require.NoError(h.t, err)
	period, err := h.oracle.ChallengePeriod(ctx)
	require.NoError(h.t, err)
	now := time.Now()
	fmt.Printf("Should verify: %v Challenge period: %v Now: %v, timestamp: %v\n", metadatas[0].ShouldVerify(now, time.Duration(period)*time.Second), period, now.Unix(), metadatas[0].Timestamp)
	return ident
}

func (h *Helper) WaitForChallenged(ctx context.Context, ident types.LargePreimageIdent) {
	timedCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := wait.For(timedCtx, time.Second, func() (bool, error) {
		metadata, err := h.oracle.GetProposalMetadata(ctx, rpcblock.Latest, ident)
		if err != nil {
			return false, err
		}
		h.require.Len(metadata, 1)
		return metadata[0].Countered, nil
	})
	h.require.NoError(err, "Preimage was not challenged")
}

func (h *Helper) WaitForFinalized(ctx context.Context, ident types.LargePreimageIdent) {
	timedCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := wait.For(timedCtx, time.Second, func() (bool, error) {
		metadata, err := h.oracle.GetProposalMetadata(ctx, rpcblock.Latest, ident)
		if err != nil {
			return false, err
		}
		h.require.Len(metadata, 1)
		return metadata[0].Timestamp > 0, nil
	})
	h.require.NoError(err, "Preimage was not challenged")
}
