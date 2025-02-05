package helpers

import (
	"context"
	"crypto/ecdsa"
	"encoding/binary"
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/op-e2e/config"

	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-proposer/metrics"
	"github.com/ethereum-optimism/optimism/op-proposer/proposer"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
)

type ProposerCfg struct {
	OutputOracleAddr       *common.Address
	DisputeGameFactoryAddr *common.Address
	ProposalInterval       time.Duration
	ProposalRetryInterval  time.Duration
	DisputeGameType        uint32
	ProposerKey            *ecdsa.PrivateKey
	AllowNonFinalized      bool
	AllocType              config.AllocType
}

type L2Proposer struct {
	log                    log.Logger
	l1                     *ethclient.Client
	driver                 *proposer.L2OutputSubmitter
	l2OutputOracle         *bindings.L2OutputOracleCaller
	l2OutputOracleAddr     *common.Address
	disputeGameFactory     *bindings.DisputeGameFactoryCaller
	disputeGameFactoryAddr *common.Address
	address                common.Address
	privKey                *ecdsa.PrivateKey
	lastTx                 common.Hash
	allocType              config.AllocType
}

type fakeTxMgr struct {
	from common.Address
}

func (f fakeTxMgr) From() common.Address {
	return f.from
}

func (f fakeTxMgr) BlockNumber(_ context.Context) (uint64, error) {
	panic("unimplemented")
}

func (f fakeTxMgr) Send(_ context.Context, _ txmgr.TxCandidate) (*types.Receipt, error) {
	panic("unimplemented")
}

func (f fakeTxMgr) SendAsync(ctx context.Context, candidate txmgr.TxCandidate, ch chan txmgr.SendResponse) {
	panic("unimplemented")
}

func (f fakeTxMgr) Close() {
}

func (f fakeTxMgr) IsClosed() bool {
	return false
}

func (f fakeTxMgr) API() rpc.API {
	panic("unimplemented")
}

func (f fakeTxMgr) SuggestGasPriceCaps(context.Context) (*big.Int, *big.Int, *big.Int, error) {
	panic("unimplemented")
}

func NewL2Proposer(t Testing, log log.Logger, cfg *ProposerCfg, l1 *ethclient.Client, rollupCl *sources.RollupClient) *L2Proposer {
	proposerConfig := proposer.ProposerConfig{
		PollInterval:           time.Second,
		NetworkTimeout:         time.Second,
		ProposalInterval:       cfg.ProposalInterval,
		L2OutputOracleAddr:     cfg.OutputOracleAddr,
		DisputeGameFactoryAddr: cfg.DisputeGameFactoryAddr,
		DisputeGameType:        cfg.DisputeGameType,
		AllowNonFinalized:      cfg.AllowNonFinalized,
	}
	rollupProvider, err := dial.NewStaticL2RollupProviderFromExistingRollup(rollupCl)
	require.NoError(t, err)
	driverSetup := proposer.DriverSetup{
		Log:            log,
		Metr:           metrics.NoopMetrics,
		Cfg:            proposerConfig,
		Txmgr:          fakeTxMgr{from: crypto.PubkeyToAddress(cfg.ProposerKey.PublicKey)},
		L1Client:       l1,
		Multicaller:    batching.NewMultiCaller(l1.Client(), batching.DefaultBatchSize),
		RollupProvider: rollupProvider,
	}

	dr, err := proposer.NewL2OutputSubmitter(driverSetup)
	require.NoError(t, err)

	address := crypto.PubkeyToAddress(cfg.ProposerKey.PublicKey)

	var l2OutputOracle *bindings.L2OutputOracleCaller
	var disputeGameFactory *bindings.DisputeGameFactoryCaller
	if cfg.AllocType.UsesProofs() {
		disputeGameFactory, err = bindings.NewDisputeGameFactoryCaller(*cfg.DisputeGameFactoryAddr, l1)
		require.NoError(t, err)
	} else {
		l2OutputOracle, err := bindings.NewL2OutputOracleCaller(*cfg.OutputOracleAddr, l1)
		require.NoError(t, err)
		proposer, err := l2OutputOracle.PROPOSER(&bind.CallOpts{})
		require.NoError(t, err)
		require.Equal(t, proposer, address, "PROPOSER must be the proposer's address")
	}

	return &L2Proposer{
		log:                    log,
		l1:                     l1,
		driver:                 dr,
		l2OutputOracle:         l2OutputOracle,
		l2OutputOracleAddr:     cfg.OutputOracleAddr,
		disputeGameFactory:     disputeGameFactory,
		disputeGameFactoryAddr: cfg.DisputeGameFactoryAddr,
		address:                address,
		privKey:                cfg.ProposerKey,
		allocType:              cfg.AllocType,
	}
}

// sendTx reimplements creating & sending transactions because we need to do the final send as async in
// the action tests while we do it synchronously in the real system.
func (p *L2Proposer) sendTx(t Testing, data []byte) {
	gasTipCap := big.NewInt(2 * params.GWei)
	pendingHeader, err := p.l1.HeaderByNumber(t.Ctx(), big.NewInt(-1))
	require.NoError(t, err, "need l1 pending header for gas price estimation")
	gasFeeCap := new(big.Int).Add(gasTipCap, new(big.Int).Mul(pendingHeader.BaseFee, big.NewInt(2)))
	chainID, err := p.l1.ChainID(t.Ctx())
	require.NoError(t, err)
	nonce, err := p.l1.NonceAt(t.Ctx(), p.address, nil)
	require.NoError(t, err)

	var addr common.Address
	if p.allocType.UsesProofs() {
		addr = *p.disputeGameFactoryAddr
	} else {
		addr = *p.l2OutputOracleAddr
	}

	gasLimit, err := estimateGasPending(t.Ctx(), p.l1, ethereum.CallMsg{
		From:      p.address,
		To:        &addr,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		Data:      data,
	})
	require.NoError(t, err)

	rawTx := &types.DynamicFeeTx{
		Nonce:     nonce,
		To:        &addr,
		Data:      data,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		Gas:       gasLimit,
		ChainID:   chainID,
	}

	tx, err := types.SignNewTx(p.privKey, types.LatestSignerForChainID(chainID), rawTx)
	require.NoError(t, err, "need to sign tx")

	err = p.l1.SendTransaction(t.Ctx(), tx)
	require.NoError(t, err, "need to send tx")

	p.lastTx = tx.Hash()
}

// estimateGasPending calls eth_estimateGas specifying the pending block. This is required for transactions from the
// proposer because they include a reference to the latest block which isn't available via `BLOCKHASH` if `latest` is
// used. In production code, the proposer waits until another L1 block is published but in e2e tests no new L1 blocks
// will be created so pending must be used.
func estimateGasPending(ctx context.Context, ec *ethclient.Client, msg ethereum.CallMsg) (uint64, error) {
	var hex hexutil.Uint64
	err := ec.Client().CallContext(ctx, &hex, "eth_estimateGas", toCallArg(msg), "pending")
	if err != nil {
		return 0, err
	}
	return uint64(hex), nil
}

func toCallArg(msg ethereum.CallMsg) interface{} {
	arg := map[string]interface{}{
		"from": msg.From,
		"to":   msg.To,
	}
	if len(msg.Data) > 0 {
		arg["data"] = hexutil.Bytes(msg.Data)
	}
	if msg.Value != nil {
		arg["value"] = (*hexutil.Big)(msg.Value)
	}
	if msg.Gas != 0 {
		arg["gas"] = hexutil.Uint64(msg.Gas)
	}
	if msg.GasPrice != nil {
		arg["gasPrice"] = (*hexutil.Big)(msg.GasPrice)
	}
	return arg
}

func (p *L2Proposer) fetchNextOutput(t Testing) (*eth.OutputResponse, bool, error) {
	if p.allocType.UsesProofs() {
		output, shouldPropose, err := p.driver.FetchDGFOutput(t.Ctx())
		if err != nil || !shouldPropose {
			return nil, false, err
		}
		encodedBlockNumber := make([]byte, 32)
		binary.BigEndian.PutUint64(encodedBlockNumber[24:], output.BlockRef.Number)
		game, err := p.disputeGameFactory.Games(&bind.CallOpts{}, p.driver.Cfg.DisputeGameType, output.OutputRoot, encodedBlockNumber)
		if err != nil {
			return nil, false, err
		}
		if game.Timestamp != 0 {
			return nil, false, nil
		}

		return output, true, nil
	} else {
		return p.driver.FetchL2OOOutput(t.Ctx())
	}
}

func (p *L2Proposer) CanPropose(t Testing) bool {
	_, shouldPropose, err := p.fetchNextOutput(t)
	require.NoError(t, err)
	return shouldPropose
}

func (p *L2Proposer) ActMakeProposalTx(t Testing) {
	output, shouldPropose, err := p.fetchNextOutput(t)
	require.NoError(t, err)

	if !shouldPropose {
		return
	}

	var txData []byte
	if p.allocType.UsesProofs() {
		tx, err := p.driver.ProposeL2OutputDGFTxCandidate(context.Background(), output)
		require.NoError(t, err)
		txData = tx.TxData
	} else {
		txData, err = p.driver.ProposeL2OutputTxData(output)
		require.NoError(t, err)
	}

	// Note: Use L1 instead of the output submitter's transaction manager because
	// this is non-blocking while the txmgr is blocking & deadlocks the tests
	p.sendTx(t, txData)
}

func (p *L2Proposer) LastProposalTx() common.Hash {
	return p.lastTx
}
