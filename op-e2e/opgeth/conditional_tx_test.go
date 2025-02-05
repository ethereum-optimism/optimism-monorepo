package opgeth

import (
	"context"
	"math/big"
	"testing"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/stretchr/testify/require"
)

const sendTxCondMethodName = "eth_sendRawTransactionConditional"

var (
	uint64Ptr              = func(num uint64) *uint64 { return &num }
	enableTxCondGethOption = func(ethCfg *ethconfig.Config, nodeCfg *node.Config) error {
		ethCfg.RollupSequencerTxConditionalEnabled = true
		ethCfg.RollupSequencerTxConditionalCostRateLimit = 1000 // not parsed from default CLI values so explicily set
		return nil
	}
)

func mkTransferTx(t *testing.T, cfg *e2esys.SystemConfig, clnt *ethclient.Client) *types.Transaction {
	gasLimit := uint64(21000) // Gas limit for a standard ETH transfer
	gasPrice, err := clnt.SuggestGasPrice(context.Background())
	require.NoError(t, err)

	from, to := cfg.Secrets.Addresses().Alice, cfg.Secrets.Addresses().Bob
	nonce, err := clnt.PendingNonceAt(context.Background(), from)
	require.NoError(t, err)

	tx := types.NewTransaction(nonce, to, big.NewInt(params.Ether), gasLimit, gasPrice, nil)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(cfg.L2ChainIDBig()), cfg.Secrets.Alice)
	require.NoError(t, err)
	return signedTx
}

func TestSendRawTransactionConditionalDisabled(t *testing.T) {
	op_e2e.InitParallel(t)
	cfg := e2esys.DefaultSystemConfig(t)
	cfg.GethOptions[e2esys.RoleSeq] = []geth.GethOption{func(ethCfg *ethconfig.Config, nodeCfg *node.Config) error {
		ethCfg.RollupSequencerTxConditionalEnabled = false
		return nil
	}}

	sys, err := cfg.Start(t)
	require.NoError(t, err, "Error starting up system")

	err = sys.NodeClient(e2esys.RoleSeq).Client().Call(nil, sendTxCondMethodName)
	require.Error(t, err)

	// method not found json error
	require.Equal(t, -32601, err.(*rpc.JsonError).Code)
}

func TestSendRawTransactionConditionalEnabled(t *testing.T) {
	op_e2e.InitParallel(t)
	cfg := e2esys.DefaultSystemConfig(t)
	cfg.GethOptions[e2esys.RoleSeq] = []geth.GethOption{enableTxCondGethOption}

	sys, err := cfg.Start(t)
	require.NoError(t, err, "Error starting up system")

	// wait for a couple l2 blocks to be created as conditionals are checked against older state
	l2Client := sys.NodeClient(e2esys.RoleSeq)
	require.NoError(t, wait.ForBlock(context.Background(), l2Client, 5))

	tx := mkTransferTx(t, &cfg, l2Client)
	txBytes, err := tx.MarshalBinary()
	require.NoError(t, err)

	// rejected conditional
	err = l2Client.Client().Call(nil, sendTxCondMethodName, hexutil.Encode(txBytes), &types.TransactionConditional{TimestampMax: uint64Ptr(0)})
	require.Error(t, err)
	require.Equal(t, params.TransactionConditionalRejectedErrCode, err.(*rpc.JsonError).Code)

	// accepted conditional
	var hash common.Hash
	err = l2Client.Client().Call(&hash, sendTxCondMethodName, hexutil.Encode(txBytes), &types.TransactionConditional{TimestampMin: uint64Ptr(0)})
	require.NoError(t, err)
	require.Equal(t, tx.Hash(), hash)
	_, err = wait.ForReceiptOK(context.Background(), l2Client, tx.Hash())
	require.NoError(t, err)
}

func TestSendRawTransactionConditionalTxForwarding(t *testing.T) {
	op_e2e.InitParallel(t)
	cfg := e2esys.DefaultSystemConfig(t)
	cfg.GethOptions[e2esys.RoleSeq] = []geth.GethOption{enableTxCondGethOption}

	// Tx will be submitted to the verifier sentry node, so we need to enable this endpoint
	cfg.GethOptions[e2esys.RoleVerif] = []geth.GethOption{enableTxCondGethOption}

	sys, err := cfg.Start(t)
	require.NoError(t, err, "Error starting up system")

	// wait for a couple l2 blocks to be created as conditionals are checked against older state
	verifClient := sys.NodeClient(e2esys.RoleVerif)
	require.NoError(t, wait.ForBlock(context.Background(), verifClient, 5))

	tx := mkTransferTx(t, &cfg, verifClient)
	txBytes, err := tx.MarshalBinary()
	require.NoError(t, err)

	// send  the tx through the verifier
	var hash common.Hash
	err = verifClient.Client().Call(&hash, sendTxCondMethodName, hexutil.Encode(txBytes), &types.TransactionConditional{TimestampMin: uint64Ptr(0)})
	require.NoError(t, err)
	require.Equal(t, tx.Hash(), hash)

	// wait for a receipt on the sequencer to speed up the test
	_, err = wait.ForReceiptOK(context.Background(), sys.NodeClient(e2esys.RoleSeq), tx.Hash())
	require.NoError(t, err)
}
