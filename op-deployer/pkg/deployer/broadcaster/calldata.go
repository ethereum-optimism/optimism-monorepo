package broadcaster

import (
	"context"
	"sync"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

const defaultGasLimit = 30_000_000

type CalldataDump struct {
	To    *common.Address `json:"to"`
	Data  hexutil.Bytes   `json:"data"`
	Value *hexutil.Big    `json:"value"`
}

type CalldataBroadcaster struct {
	txs []txmgr.TxCandidate
	mtx sync.Mutex
}

func (d *CalldataBroadcaster) Broadcast(ctx context.Context) ([]BroadcastResult, error) {
	return nil, nil
}

func (d *CalldataBroadcaster) Hook(bcast script.Broadcast) {
	candidate := asTxCandidate(bcast, defaultGasLimit)

	d.mtx.Lock()
	d.txs = append(d.txs, candidate)
	d.mtx.Unlock()
}

func (d *CalldataBroadcaster) Dump() ([]CalldataDump, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	var out []CalldataDump
	for _, tx := range d.txs {
		out = append(out, CalldataDump{
			To:    tx.To,
			Value: (*hexutil.Big)(tx.Value),
			Data:  tx.TxData,
		})
	}
	d.txs = nil
	return out, nil
}
