package fees

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"

	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc/eip4844"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

func L1InfoFromState(ctx context.Context, contract *bindings.L1Block, l2Number *big.Int, ecotone bool) (*derive.L1BlockInfo, error) {
	var err error
	out := &derive.L1BlockInfo{}
	opts := bind.CallOpts{
		BlockNumber: l2Number,
		Context:     ctx,
	}

	out.Number, err = contract.Number(&opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get number: %w", err)
	}

	out.Time, err = contract.Timestamp(&opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get timestamp: %w", err)
	}

	out.BaseFee, err = contract.Basefee(&opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get base fee: %w", err)
	}

	blockHashBytes, err := contract.Hash(&opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get block hash: %w", err)
	}
	out.BlockHash = common.BytesToHash(blockHashBytes[:])

	out.SequenceNumber, err = contract.SequenceNumber(&opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get sequence number: %w", err)
	}

	if !ecotone {
		overhead, err := contract.L1FeeOverhead(&opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get l1 fee overhead: %w", err)
		}
		out.L1FeeOverhead = eth.Bytes32(common.BigToHash(overhead))

		scalar, err := contract.L1FeeScalar(&opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get l1 fee scalar: %w", err)
		}
		out.L1FeeScalar = eth.Bytes32(common.BigToHash(scalar))
	}

	batcherHash, err := contract.BatcherHash(&opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get batch sender: %w", err)
	}
	out.BatcherAddr = common.BytesToAddress(batcherHash[:])

	if ecotone {
		blobBaseFeeScalar, err := contract.BlobBaseFeeScalar(&opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get blob basefee scalar: %w", err)
		}
		out.BlobBaseFeeScalar = blobBaseFeeScalar

		baseFeeScalar, err := contract.BaseFeeScalar(&opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get basefee scalar: %w", err)
		}
		out.BaseFeeScalar = baseFeeScalar

		blobBaseFee, err := contract.BlobBaseFee(&opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get blob basefee: %w", err)
		}
		out.BlobBaseFee = blobBaseFee
	}

	return out, nil
}

func TestL1InfoContract(t *testing.T) {
	op_e2e.InitParallel(t)

	cfg := e2esys.DefaultSystemConfig(t)

	sys, err := cfg.Start(t)
	require.Nil(t, err, "Error starting up system")

	l1Client := sys.NodeClient("l1")
	l2Seq := sys.NodeClient("sequencer")
	l2Verif := sys.NodeClient("verifier")

	endVerifBlockNumber := big.NewInt(4)
	endSeqBlockNumber := big.NewInt(6)
	endVerifBlock, err := geth.WaitForBlock(endVerifBlockNumber, l2Verif)
	require.Nil(t, err)
	endSeqBlock, err := geth.WaitForBlock(endSeqBlockNumber, l2Seq)
	require.Nil(t, err)

	seqL1Info, err := bindings.NewL1Block(cfg.L1InfoPredeployAddress, l2Seq)
	require.Nil(t, err)

	verifL1Info, err := bindings.NewL1Block(cfg.L1InfoPredeployAddress, l2Verif)
	require.Nil(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fillInfoLists := func(start *types.Block, contract *bindings.L1Block, client *ethclient.Client) ([]*derive.L1BlockInfo, []*derive.L1BlockInfo) {
		var txList, stateList []*derive.L1BlockInfo
		for b := start; ; {
			var infoFromTx *derive.L1BlockInfo
			infoFromTx, err := derive.L1BlockInfoFromBytes(sys.RollupConfig, b.Time(), b.Transactions()[0].Data())
			require.NoError(t, err)
			txList = append(txList, infoFromTx)

			ecotone := sys.RollupConfig.IsEcotone(b.Time()) && !sys.RollupConfig.IsEcotoneActivationBlock(b.Time())
			infoFromState, err := L1InfoFromState(ctx, contract, b.Number(), ecotone)
			require.Nil(t, err)
			stateList = append(stateList, infoFromState)

			// Genesis L2 block contains no L1 Deposit TX
			if b.NumberU64() == 1 {
				return txList, stateList
			}
			b, err = client.BlockByHash(ctx, b.ParentHash())
			require.Nil(t, err)
		}
	}

	l1InfosFromSequencerTransactions, l1InfosFromSequencerState := fillInfoLists(endSeqBlock, seqL1Info, l2Seq)
	l1InfosFromVerifierTransactions, l1InfosFromVerifierState := fillInfoLists(endVerifBlock, verifL1Info, l2Verif)

	l1blocks := make(map[common.Hash]*derive.L1BlockInfo)
	maxL1Hash := l1InfosFromSequencerTransactions[0].BlockHash
	for h := maxL1Hash; ; {
		b, err := l1Client.BlockByHash(ctx, h)
		require.Nil(t, err)

		l1blocks[h] = &derive.L1BlockInfo{
			Number:         b.NumberU64(),
			Time:           b.Time(),
			BaseFee:        b.BaseFee(),
			BlockHash:      h,
			SequenceNumber: 0, // ignored, will be overwritten
			BatcherAddr:    sys.RollupConfig.Genesis.SystemConfig.BatcherAddr,
		}
		if sys.RollupConfig.IsEcotone(b.Time()) && !sys.RollupConfig.IsEcotoneActivationBlock(b.Time()) {
			scalars, err := sys.RollupConfig.Genesis.SystemConfig.EcotoneScalars()
			require.NoError(t, err)
			l1blocks[h].BlobBaseFeeScalar = scalars.BlobBaseFeeScalar
			l1blocks[h].BaseFeeScalar = scalars.BaseFeeScalar
			if excess := b.ExcessBlobGas(); excess != nil {
				l1blocks[h].BlobBaseFee = eip4844.CalcBlobFee(*excess)
			} else {
				l1blocks[h].BlobBaseFee = big.NewInt(1)
			}
		} else {
			l1blocks[h].L1FeeOverhead = sys.RollupConfig.Genesis.SystemConfig.Overhead
			l1blocks[h].L1FeeScalar = sys.RollupConfig.Genesis.SystemConfig.Scalar
		}

		h = b.ParentHash()
		if b.NumberU64() == 0 {
			break
		}
	}

	checkInfoList := func(name string, list []*derive.L1BlockInfo) {
		for _, info := range list {
			if expected, ok := l1blocks[info.BlockHash]; ok {
				expected.SequenceNumber = info.SequenceNumber // the seq nr is not part of the L1 info we know in advance, so we ignore it.
				require.Equal(t, expected, info)
			} else {
				t.Fatalf("Did not find block hash for L1 Info: %v in test %s", info, name)
			}
		}
	}

	checkInfoList("On sequencer with tx", l1InfosFromSequencerTransactions)
	checkInfoList("On sequencer with state", l1InfosFromSequencerState)
	checkInfoList("On verifier with tx", l1InfosFromVerifierTransactions)
	checkInfoList("On verifier with state", l1InfosFromVerifierState)
}
