package metrics

import (
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	txmetrics "github.com/ethereum-optimism/optimism/op-service/txmgr/metrics"
)

type noopMetrics struct {
	opmetrics.NoopRefMetrics
	txmetrics.NoopTxMetrics
	opmetrics.NoopRPCMetrics
}

var NoopMetrics Metricer = new(noopMetrics)

func (*noopMetrics) RecordInfo(version string) {}
func (*noopMetrics) RecordUp()                 {}

func (*noopMetrics) RecordL2BlocksProposed(l2ref eth.L2BlockRef) {}

func (m *noopMetrics) RecordL2Proposal(_ uint64) {}

func (*noopMetrics) StartBalanceMetrics(log.Logger, *ethclient.Client, common.Address) io.Closer {
	return nil
}
