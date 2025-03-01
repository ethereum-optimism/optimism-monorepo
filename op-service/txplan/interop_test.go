package txplan

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/plan"
	suptypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type InitTrigger struct {
	Emitter    common.Address // address of the EventLogger contract
	Topics     []common.Hash
	OpaqueData []byte
}

func (v *InitTrigger) To() (*common.Address, error) {
	return &v.Emitter, nil
}

func (v *InitTrigger) Data() ([]byte, error) {
	// TODO format call
	return nil, nil
}

type ExecTrigger struct {
	Executor    common.Address // address of the EventLogger contract
	Identifier  suptypes.Identifier
	PayloadHash common.Hash
}

func (v *ExecTrigger) To() (*common.Address, error) {
	return &v.Executor, nil
}

func (v *ExecTrigger) Data() ([]byte, error) {
	// TODO format call
	return nil, nil
}

type Call interface {
	To() (*common.Address, error)
	Data() ([]byte, error)
}

type MultiTrigger struct {
	Calls []Call
}

func (v *MultiTrigger) Data() ([]byte, error) {
	// TODO format multi-call
	return nil, nil
}

type IntentTx[V Call] struct {
	PlannedTx *PlannedTx
	Content   plan.Lazy[V]
}

func NewIntent[V Call](opts ...Option) *IntentTx[V] {
	v := &IntentTx[V]{
		PlannedTx: NewPlannedTx(opts...),
	}
	v.PlannedTx.To.DependOn(&v.Content)
	v.PlannedTx.To.Fn(func(ctx context.Context) (*common.Address, error) {
		return v.Content.Value().To()
	})
	v.PlannedTx.Data.DependOn(&v.Content)
	v.PlannedTx.Data.Fn(func(ctx context.Context) (hexutil.Bytes, error) {
		return v.Content.Value().Data()
	})
	return v
}

func getTimestamp(ctx context.Context, blockHash common.Hash) (uint64, error) {
	return 123, nil // TODO
}

func initMsgToExecTrigger(ctx context.Context, executor common.Address, logEvent *types.Log) (*ExecTrigger, error) {
	payload := suptypes.LogToMessagePayload(logEvent)
	timestamp, err := getTimestamp(ctx, logEvent.BlockHash)
	if err != nil {
		return nil, err
	}
	return &ExecTrigger{
		Executor: executor,
		Identifier: suptypes.Identifier{
			Origin:      logEvent.Address,
			BlockNumber: logEvent.BlockNumber,
			LogIndex:    uint32(logEvent.Index),
			Timestamp:   timestamp,
			ChainID:     eth.ChainID{},
		},
		PayloadHash: crypto.Keccak256Hash(payload),
	}, nil
}

func TestInteropTx(t *testing.T) {
	t.Skip() // TODO

	eventLogger := common.Address{} // TODO deploy tx

	priv, err := crypto.GenerateKey()
	require.NoError(t, err)
	opts := []Option{
		WithPrivateKey(priv),
		// TODO: add options that submit and confirm the tx etc.
	}

	txA := NewIntent[*InitTrigger](opts...)
	txA.Content.Set(&InitTrigger{
		Emitter:    eventLogger,
		Topics:     []common.Hash{},
		OpaqueData: []byte("hello world!"),
	})

	txB := NewIntent[*ExecTrigger]()
	txB.Content.DependOn(&txA.PlannedTx.Included, &txA.PlannedTx.Success)
	txB.Content.Fn(func(ctx context.Context) (*ExecTrigger, error) {
		initMsgReceipt := txA.PlannedTx.Included.Value()
		logEvent := initMsgReceipt.Logs[0]
		return initMsgToExecTrigger(ctx, eventLogger, logEvent)
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	recA, err := txB.PlannedTx.Included.Eval(ctx)
	require.NoError(t, err)
	t.Logf("included initiating tx in block %s", recA.BlockHash)

	recB, err := txB.PlannedTx.Included.Eval(ctx)
	require.NoError(t, err)
	t.Logf("included executing tx in block %s", recB.BlockHash)
}
