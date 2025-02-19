package test

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-program/client/l2/engineapi"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	geth "github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

type Miner struct {
	backend   *core.BlockChain
	engineAPI *engineapi.L2EngineAPI
}

func NewMiner(t *testing.T, logger log.Logger, isthmusTime uint64) (*Miner, *core.BlockChain) {
	config := *params.MergedTestChainConfig
	var zero uint64
	// activate recent OP-stack forks
	config.RegolithTime = &zero
	config.CanyonTime = &zero
	config.EcotoneTime = &zero
	config.FjordTime = &zero
	config.GraniteTime = &zero
	config.HoloceneTime = &zero
	config.IsthmusTime = &isthmusTime
	config.PragueTime = &isthmusTime
	denomCanyon := uint64(250)
	config.Optimism = &params.OptimismConfig{
		EIP1559Denominator:       50,
		EIP1559Elasticity:        10,
		EIP1559DenominatorCanyon: &denomCanyon,
	}
	genesis := &core.Genesis{
		Config:     &config,
		Difficulty: common.Big0,
		ParentHash: common.Hash{},
		BaseFee:    big.NewInt(7),
		Alloc: map[common.Address]types.Account{
			params.HistoryStorageAddress: {Nonce: 1, Code: params.HistoryStorageCode, Balance: common.Big0}, // for Isthmus eip-2935
		},
		ExtraData: []byte{0x0, 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8}, // for Holocene eip-1559 params
	}
	ethCfg := &ethconfig.Config{
		NetworkId:   genesis.Config.ChainID.Uint64(),
		Genesis:     genesis,
		StateScheme: rawdb.HashScheme,
		NoPruning:   true,
	}
	nodeCfg := &node.Config{
		Name:    "l2-geth",
		DataDir: t.TempDir(),
	}
	n, err := node.New(nodeCfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = n.Close()
	})
	backend, err := geth.New(n, ethCfg)
	require.NoError(t, err)
	chain := backend.BlockChain()

	engineAPI := engineapi.NewL2EngineAPI(logger, chain, nil)
	require.NotNil(t, engineAPI)
	return &Miner{backend: chain, engineAPI: engineAPI}, chain
}

// Mine builds a block on top of the current head and adds it to the chain.
func (m *Miner) Mine(t *testing.T, attrs *eth.PayloadAttributes) {
	head := m.backend.CurrentHeader()
	m.MineAt(t, head, attrs)
}

func (m *Miner) Fork(t *testing.T, blockNumber uint64, attrs *eth.PayloadAttributes) {
	head := m.backend.GetHeaderByNumber(blockNumber)
	if attrs == nil {
		gasLimit := eth.Uint64Quantity(head.GasLimit)
		eip1559Params := eth.Bytes8([]byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88})
		attrs = &eth.PayloadAttributes{
			Timestamp:             eth.Uint64Quantity(head.Time + 2),
			PrevRandao:            eth.Bytes32{0x11},
			SuggestedFeeRecipient: common.Address{0x33},
			Withdrawals:           &types.Withdrawals{},
			ParentBeaconBlockRoot: &common.Hash{0x22},
			NoTxPool:              true,
			GasLimit:              &gasLimit,
			EIP1559Params:         &eip1559Params,
		}
	}
	m.MineAt(t, head, attrs)
}

func (m *Miner) MineAt(t *testing.T, head *types.Header, attrs *eth.PayloadAttributes) {
	hash := head.Hash()
	genesis := m.backend.Genesis()
	if attrs == nil {
		eip1559Params := eth.Bytes8([]byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8})
		gasLimit := eth.Uint64Quantity(4712388)
		attrs = &eth.PayloadAttributes{
			Timestamp:             eth.Uint64Quantity(head.Time + 2),
			PrevRandao:            eth.Bytes32{0x11},
			SuggestedFeeRecipient: common.Address{0x33},
			Withdrawals:           &types.Withdrawals{},
			ParentBeaconBlockRoot: &common.Hash{0x22},
			NoTxPool:              true,
			GasLimit:              &gasLimit,
			EIP1559Params:         &eip1559Params,
		}
	}
	result, err := m.engineAPI.ForkchoiceUpdatedV3(context.Background(), &eth.ForkchoiceState{
		HeadBlockHash:      hash,
		SafeBlockHash:      hash,
		FinalizedBlockHash: genesis.Hash(),
	}, attrs)
	require.NoError(t, err)
	require.EqualValues(t, engine.VALID, result.PayloadStatus.Status)
	require.NotNil(t, result.PayloadID)

	var envelope *eth.ExecutionPayloadEnvelope
	if m.backend.Config().IsIsthmus(uint64(attrs.Timestamp)) {
		envelope, err = m.engineAPI.GetPayloadV4(context.Background(), *result.PayloadID)
	} else {
		envelope, err = m.engineAPI.GetPayloadV3(context.Background(), *result.PayloadID)
	}
	require.NoError(t, err)
	require.NotNil(t, envelope)

	var newPayloadResult *eth.PayloadStatusV1
	if m.backend.Config().IsIsthmus(uint64(attrs.Timestamp)) {
		newPayloadResult, err = m.engineAPI.NewPayloadV4(context.Background(), envelope.ExecutionPayload, []common.Hash{}, envelope.ParentBeaconBlockRoot, []hexutil.Bytes{})
	} else {
		newPayloadResult, err = m.engineAPI.NewPayloadV3(context.Background(), envelope.ExecutionPayload, []common.Hash{}, envelope.ParentBeaconBlockRoot)
	}
	require.NoError(t, err)
	require.EqualValues(t, engine.VALID, newPayloadResult.Status)

	result, err = m.engineAPI.ForkchoiceUpdatedV3(context.Background(), &eth.ForkchoiceState{
		HeadBlockHash:      envelope.ExecutionPayload.BlockHash,
		SafeBlockHash:      envelope.ExecutionPayload.BlockHash,
		FinalizedBlockHash: envelope.ExecutionPayload.BlockHash,
	}, nil)
	require.NoError(t, err)
	require.EqualValues(t, engine.VALID, result.PayloadStatus.Status)
}
