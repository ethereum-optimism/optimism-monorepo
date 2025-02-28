package preimage

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"errors"
	"io"
	"math/big"
	"math/rand"
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
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
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
	data := testutils.RandomData(rand.New(rand.NewSource(1234)), dataSize)
	s := matrix.NewStateMatrix()
	uuid := big.NewInt(h.uuidProvider.Add(1))
	candidate, err := h.oracle.InitLargePreimage(uuid, 32, uint32(len(data)))
	h.require.NoError(err)
	transactions.RequireSendTx(h.t, ctx, h.client, candidate, h.privKey)

	startBlock := big.NewInt(0)
	totalBlocks := len(data) / types.BlockSize
	in := bytes.NewReader(data)
	var txs []txmgr.TxCandidate
	for {
		inputData, err := s.AbsorbUpTo(in, preimages.MaxChunkSize)
		if !errors.Is(err, io.EOF) {
			h.require.NoError(err)
		}
		for _, modifier := range modifiers {
			modifier(startBlock.Uint64(), &inputData)
		}
		h.t.Logf("Uploading %v parts of preimage %v starting at block %v of about %v Finalize: %v", len(inputData.Commitments), uuid.Uint64(), startBlock.Uint64(), totalBlocks, inputData.Finalize)
		tx, err := h.oracle.AddLeaves(uuid, startBlock, inputData.Input, inputData.Commitments, inputData.Finalize)
		h.require.NoError(err)
		// Can't use EstimateGas because all the transactions will be sent in a batch, and the contract checks the
		// start block, which won't match subsequent transactions until the first one is processed. So set the gas limit
		// to what EstimateGas was returning when this was written plus a bunch of headroom.
		tx.GasLimit = 2000000 // 826536
		txs = append(txs, tx)
		startBlock = new(big.Int).Add(startBlock, big.NewInt(int64(len(inputData.Commitments))))
		if inputData.Finalize {
			break
		}
	}
	transactions.RequireSendTxs(h.t, ctx, h.client, txs, h.privKey)

	return types.LargePreimageIdent{
		Claimant: crypto.PubkeyToAddress(h.privKey.PublicKey),
		UUID:     uuid,
	}
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
