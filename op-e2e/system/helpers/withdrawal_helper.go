package helpers

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"

	"github.com/ethereum-optimism/optimism/op-chain-ops/crossdomain"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts/metrics"
	gameTypes "github.com/ethereum-optimism/optimism/op-challenger/game/types"

	"github.com/ethereum-optimism/optimism/op-e2e/config"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/transactions"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-node/bindings"
	bindingspreview "github.com/ethereum-optimism/optimism/op-node/bindings/preview"
	"github.com/ethereum-optimism/optimism/op-node/withdrawals"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/gethclient"
	"github.com/stretchr/testify/require"
)

const SolErrClaimAlreadyResolved = "0xf1a94581"

type ClientProvider interface {
	NodeClient(name string) *ethclient.Client
}

func SendWithdrawal(t *testing.T, cfg e2esys.SystemConfig, l2Client *ethclient.Client, privKey *ecdsa.PrivateKey, applyOpts WithdrawalTxOptsFn) (*types.Transaction, *types.Receipt) {
	opts := defaultWithdrawalTxOpts()
	applyOpts(opts)

	// Bind L2 Withdrawer Contract
	l2withdrawer, err := bindings.NewL2ToL1MessagePasser(predeploys.L2ToL1MessagePasserAddr, l2Client)
	require.Nil(t, err, "binding withdrawer on L2")

	// Initiate Withdrawal
	l2opts, err := bind.NewKeyedTransactorWithChainID(privKey, cfg.L2ChainIDBig())
	require.Nil(t, err)
	l2opts.Value = opts.Value
	tx, err := l2withdrawer.InitiateWithdrawal(l2opts, l2opts.From, big.NewInt(int64(opts.Gas)), opts.Data)
	require.Nil(t, err, "sending initiate withdraw tx")

	receipt, err := geth.WaitForTransaction(tx.Hash(), l2Client, 10*time.Duration(cfg.DeployConfig.L1BlockTime)*time.Second)
	require.Nil(t, err, "withdrawal initiated on L2 sequencer")
	require.Equal(t, opts.ExpectedStatus, receipt.Status, "transaction had incorrect status")

	for i, client := range opts.VerifyClients {
		t.Logf("Waiting for tx %v on verification client %d", tx.Hash(), i)
		receiptVerif, err := geth.WaitForTransaction(tx.Hash(), client, 10*time.Duration(cfg.DeployConfig.L2BlockTime)*time.Second)
		require.Nilf(t, err, "Waiting for L2 tx on verification client %d", i)
		require.Equalf(t, receipt, receiptVerif, "Receipts should be the same on sequencer and verification client %d", i)
	}
	return tx, receipt
}

type WithdrawalTxOptsFn func(opts *WithdrawalTxOpts)

type WithdrawalTxOpts struct {
	ToAddr         *common.Address
	Nonce          uint64
	Value          *big.Int
	Gas            uint64
	Data           []byte
	ExpectedStatus uint64
	VerifyClients  []*ethclient.Client
}

// VerifyOnClients adds additional l2 clients that should sync the block the tx is included in
// Checks that the receipt received from these clients is equal to the receipt received from the sequencer
func (o *WithdrawalTxOpts) VerifyOnClients(clients ...*ethclient.Client) {
	o.VerifyClients = append(o.VerifyClients, clients...)
}

func defaultWithdrawalTxOpts() *WithdrawalTxOpts {
	return &WithdrawalTxOpts{
		ToAddr:         nil,
		Nonce:          0,
		Value:          common.Big0,
		Gas:            21_000,
		Data:           nil,
		ExpectedStatus: types.ReceiptStatusSuccessful,
	}
}

func ProveAndFinalizeWithdrawal(
	t *testing.T,
	cfg e2esys.SystemConfig,
	clients ClientProvider,
	l2NodeName string,
	ethPrivKey *ecdsa.PrivateKey,
	l2WithdrawalReceipt *types.Receipt,
) (*types.Receipt, *types.Receipt, *types.Receipt, *types.Receipt) {
	params, proveReceipt := ProveWithdrawal(t, cfg, clients, l2NodeName, ethPrivKey, l2WithdrawalReceipt)
	finalizeReceipt, resolveClaimReceipt, resolveReceipt := FinalizeWithdrawal(t, cfg, clients.NodeClient("l1"), ethPrivKey, proveReceipt, params)
	return proveReceipt, finalizeReceipt, resolveClaimReceipt, resolveReceipt
}

func ProveWithdrawal(t *testing.T, cfg e2esys.SystemConfig, clients ClientProvider, l2NodeName string, ethPrivKey *ecdsa.PrivateKey, l2WithdrawalReceipt *types.Receipt) (withdrawals.ProvenWithdrawalParameters, *types.Receipt) {
	// Get l2BlockNumber for proof generation
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Duration(cfg.DeployConfig.L1BlockTime)*time.Second)
	defer cancel()

	allocType := cfg.AllocType

	l1Client := clients.NodeClient(e2esys.RoleL1)
	var blockNumber uint64
	var err error
	l1Deployments := config.L1Deployments(allocType)
	if allocType.UsesProofs() {
		blockNumber, err = wait.ForGamePublished(ctx, l1Client, l1Deployments.OptimismPortalProxy, l1Deployments.DisputeGameFactoryProxy, l2WithdrawalReceipt.BlockNumber)
		require.NoError(t, err)
	} else {
		blockNumber, err = wait.ForOutputRootPublished(ctx, l1Client, l1Deployments.L2OutputOracleProxy, l2WithdrawalReceipt.BlockNumber)
		require.NoError(t, err)
	}

	receiptCl := clients.NodeClient(l2NodeName)
	headerCl := clients.NodeClient(l2NodeName)
	proofCl := gethclient.New(receiptCl.Client())

	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Get the latest header
	header, err := receiptCl.HeaderByNumber(ctx, new(big.Int).SetUint64(blockNumber))
	require.NoError(t, err)

	oracle, err := bindings.NewL2OutputOracleCaller(l1Deployments.L2OutputOracleProxy, l1Client)
	require.NoError(t, err)

	factory, err := bindings.NewDisputeGameFactoryCaller(l1Deployments.DisputeGameFactoryProxy, l1Client)
	require.NoError(t, err)

	portal2, err := bindingspreview.NewOptimismPortal2Caller(l1Deployments.OptimismPortalProxy, l1Client)
	require.NoError(t, err)

	params, err := ProveWithdrawalParameters(context.Background(), proofCl, receiptCl, headerCl, l2WithdrawalReceipt.TxHash, header, oracle, factory, portal2, allocType)
	require.NoError(t, err)

	portal, err := bindings.NewOptimismPortal(l1Deployments.OptimismPortalProxy, l1Client)
	require.NoError(t, err)

	opts, err := bind.NewKeyedTransactorWithChainID(ethPrivKey, cfg.L1ChainIDBig())
	require.NoError(t, err)

	// Prove withdrawal
	tx, err := portal.ProveWithdrawalTransaction(
		opts,
		bindings.TypesWithdrawalTransaction{
			Nonce:    params.Nonce,
			Sender:   params.Sender,
			Target:   params.Target,
			Value:    params.Value,
			GasLimit: params.GasLimit,
			Data:     params.Data,
		},
		params.L2OutputIndex,
		params.OutputRootProof,
		params.WithdrawalProof,
	)
	require.NoError(t, err)

	// Ensure that our withdrawal was proved successfully
	proveReceipt, err := geth.WaitForTransaction(tx.Hash(), l1Client, 3*time.Duration(cfg.DeployConfig.L1BlockTime)*time.Second)
	require.NoError(t, err, "prove withdrawal")
	require.Equal(t, types.ReceiptStatusSuccessful, proveReceipt.Status)
	return params, proveReceipt
}

func ProveWithdrawalParameters(ctx context.Context, proofCl withdrawals.ProofClient, l2ReceiptCl withdrawals.ReceiptClient, l2HeaderCl withdrawals.HeaderClient, txHash common.Hash, header *types.Header, l2OutputOracleContract *bindings.L2OutputOracleCaller, disputeGameFactoryContract *bindings.DisputeGameFactoryCaller, optimismPortal2Contract *bindingspreview.OptimismPortal2Caller, allocType config.AllocType) (withdrawals.ProvenWithdrawalParameters, error) {
	if allocType.UsesProofs() {
		return withdrawals.ProveWithdrawalParametersFaultProofs(ctx, proofCl, l2ReceiptCl, l2HeaderCl, txHash, disputeGameFactoryContract, optimismPortal2Contract)
	} else {
		return withdrawals.ProveWithdrawalParameters(ctx, proofCl, l2ReceiptCl, txHash, header, l2OutputOracleContract)
	}
}

func FinalizeWithdrawal(t *testing.T, cfg e2esys.SystemConfig, l1Client *ethclient.Client, privKey *ecdsa.PrivateKey, withdrawalProofReceipt *types.Receipt, params withdrawals.ProvenWithdrawalParameters) (*types.Receipt, *types.Receipt, *types.Receipt) {
	// Wait for finalization and then create the Finalized Withdrawal Transaction
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Duration(cfg.DeployConfig.L1BlockTime)*time.Second)
	defer cancel()

	wd := crossdomain.Withdrawal{
		Nonce:    params.Nonce,
		Sender:   &params.Sender,
		Target:   &params.Target,
		Value:    params.Value,
		GasLimit: params.GasLimit,
		Data:     params.Data,
	}

	allocType := cfg.AllocType

	opts, err := bind.NewKeyedTransactorWithChainID(privKey, cfg.L1ChainIDBig())
	require.NoError(t, err)

	var resolveClaimReceipt *types.Receipt
	var resolveReceipt *types.Receipt
	l1Deployments := config.L1Deployments(allocType)
	if allocType.UsesProofs() {
		portal2, err := bindingspreview.NewOptimismPortal2(l1Deployments.OptimismPortalProxy, l1Client)
		require.NoError(t, err)

		wdHash, err := wd.Hash()
		require.NoError(t, err)

		game, err := portal2.ProvenWithdrawals(&bind.CallOpts{}, wdHash, opts.From)
		require.NoError(t, err)
		require.NotNil(t, game, "withdrawal should be proven")

		caller := batching.NewMultiCaller(l1Client.Client(), batching.DefaultBatchSize)
		gameContract, err := contracts.NewFaultDisputeGameContract(context.Background(), metrics.NoopContractMetrics, game.DisputeGameProxy, caller)
		require.NoError(t, err)

		timedCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		require.NoError(t, wait.For(timedCtx, time.Second, func() (bool, error) {
			err := gameContract.CallResolveClaim(context.Background(), 0)
			if err != nil {
				t.Logf("Could not resolve dispute game claim: %v", err)
			}
			return err == nil, nil
		}))

		t.Log("FinalizeWithdrawal: resolveClaim...")
		tx, err := gameContract.ResolveClaimTx(0)
		require.NoError(t, err, "create resolveClaim tx")
		_, resolveClaimReceipt, err = transactions.SendTx(ctx, l1Client, tx, privKey)
		var rsErr *wait.ReceiptStatusError
		if errors.As(err, &rsErr) && rsErr.TxTrace.Output.String() == SolErrClaimAlreadyResolved {
			t.Logf("resolveClaim failed (tx: %s) because claim got already resolved", resolveClaimReceipt.TxHash)
		} else {
			require.NoError(t, err)
		}

		t.Log("FinalizeWithdrawal: resolve...")
		tx, err = gameContract.ResolveTx()
		require.NoError(t, err, "create resolve tx")
		_, resolveReceipt = transactions.RequireSendTx(t, ctx, l1Client, tx, privKey, transactions.WithReceiptStatusIgnore())
		if resolveReceipt.Status == types.ReceiptStatusFailed {
			t.Logf("resolve failed (tx: %s)! But game may have resolved already. Checking now...", resolveReceipt.TxHash)
			// it may have failed because someone else front-ran this by calling `resolve()` first.
			status, err := gameContract.GetStatus(ctx)
			require.NoError(t, err)
			require.Equal(t, gameTypes.GameStatusDefenderWon, status, "game must have resolved with defender won")
			t.Logf("resolve was not needed, the game was already resolved")
		}

		t.Log("FinalizeWithdrawal: waiting for successful withdrawal check...")
		err = wait.ForWithdrawalCheck(ctx, l1Client, wd, l1Deployments.OptimismPortalProxy, opts.From)
		require.NoError(t, err)
	} else {
		t.Log("FinalizeWithdrawal: waiting for finalization...")
		err := wait.ForFinalizationPeriod(ctx, l1Client, withdrawalProofReceipt.BlockNumber, l1Deployments.L2OutputOracleProxy)
		require.NoError(t, err)
	}

	portal, err := bindings.NewOptimismPortal(l1Deployments.OptimismPortalProxy, l1Client)
	require.NoError(t, err)

	// Finalize withdrawal
	t.Log("FinalizeWithdrawal: finalizing withdrawal...")
	tx, err := portal.FinalizeWithdrawalTransaction(opts, wd.WithdrawalTransaction())
	require.NoError(t, err)

	// Ensure that our withdrawal was finalized successfully
	finalizeReceipt, err := wait.ForReceiptOK(ctx, l1Client, tx.Hash())
	require.NoError(t, err, "finalize withdrawal")
	require.Equal(t, types.ReceiptStatusSuccessful, finalizeReceipt.Status)
	return finalizeReceipt, resolveClaimReceipt, resolveReceipt
}
