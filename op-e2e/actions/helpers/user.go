package helpers

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts/metrics"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-chain-ops/crossdomain"
	legacybindings "github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/config"
	e2ehelpers "github.com/ethereum-optimism/optimism/op-e2e/system/helpers"
	"github.com/ethereum-optimism/optimism/op-node/bindings"
	bindingspreview "github.com/ethereum-optimism/optimism/op-node/bindings/preview"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/withdrawals"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
)

type L1Bindings struct {
	// contract bindings
	OptimismPortal     *bindings.OptimismPortal
	L2OutputOracle     *bindings.L2OutputOracle
	OptimismPortal2    *bindingspreview.OptimismPortal2
	DisputeGameFactory *bindings.DisputeGameFactory
}

func NewL1Bindings(t Testing, l1Cl *ethclient.Client, allocType config.AllocType) *L1Bindings {
	l1Deployments := config.L1Deployments(allocType)
	optimismPortal, err := bindings.NewOptimismPortal(l1Deployments.OptimismPortalProxy, l1Cl)
	require.NoError(t, err)

	l2OutputOracle, err := bindings.NewL2OutputOracle(l1Deployments.L2OutputOracleProxy, l1Cl)
	require.NoError(t, err)

	optimismPortal2, err := bindingspreview.NewOptimismPortal2(l1Deployments.OptimismPortalProxy, l1Cl)
	require.NoError(t, err)

	disputeGameFactory, err := bindings.NewDisputeGameFactory(l1Deployments.DisputeGameFactoryProxy, l1Cl)
	require.NoError(t, err)

	return &L1Bindings{
		OptimismPortal:     optimismPortal,
		L2OutputOracle:     l2OutputOracle,
		OptimismPortal2:    optimismPortal2,
		DisputeGameFactory: disputeGameFactory,
	}
}

type L2Bindings struct {
	L2ToL1MessagePasser *bindings.L2ToL1MessagePasser

	ProofClient withdrawals.ProofClient
}

func NewL2Bindings(t Testing, l2Cl *ethclient.Client, proofCl withdrawals.ProofClient) *L2Bindings {
	l2ToL1MessagePasser, err := bindings.NewL2ToL1MessagePasser(predeploys.L2ToL1MessagePasserAddr, l2Cl)
	require.NoError(t, err)

	return &L2Bindings{
		L2ToL1MessagePasser: l2ToL1MessagePasser,
		ProofClient:         proofCl,
	}
}

// BasicUserEnv provides access to the eth RPC, signer, and contract bindings for a single ethereum layer.
// This environment can be shared between different BasicUser instances.
type BasicUserEnv[B any] struct {
	EthCl  *ethclient.Client
	Signer types.Signer

	AddressCorpora []common.Address

	Bindings B
}

// BasicUser is an actor on a single ethereum layer, with one account key.
// The user maintains a set of standard txOpts to build its transactions with,
// along with configurable txToAddr and txCallData.
// The user has an RNG source with actions to randomize its transaction building.
type BasicUser[B any] struct {
	log log.Logger
	rng *rand.Rand
	env *BasicUserEnv[B]

	account *ecdsa.PrivateKey
	address common.Address

	txOpts bind.TransactOpts

	txToAddr   *common.Address
	txCallData []byte

	// lastTxHash persists the last transaction,
	// so we can chain together tx sending and tx checking easily.
	// Sending and checking are detached, since txs may not be instantly confirmed.
	lastTxHash common.Hash
}

func NewBasicUser[B any](log log.Logger, priv *ecdsa.PrivateKey, rng *rand.Rand) *BasicUser[B] {
	return &BasicUser[B]{
		log:     log,
		rng:     rng,
		account: priv,
		address: crypto.PubkeyToAddress(priv.PublicKey),
	}
}

// SetUserEnv changes the user environment.
// This way a user can be initialized before being embedded in a genesis allocation,
// and change between different endpoints that may be initialized after the user.
func (s *BasicUser[B]) SetUserEnv(env *BasicUserEnv[B]) {
	s.env = env
}

func (s *BasicUser[B]) Signer() types.Signer {
	return s.env.Signer
}

func (s *BasicUser[B]) signerFn(address common.Address, tx *types.Transaction) (*types.Transaction, error) {
	if address != s.address {
		return nil, bind.ErrNotAuthorized
	}
	signature, err := crypto.Sign(s.env.Signer.Hash(tx).Bytes(), s.account)
	if err != nil {
		return nil, err
	}
	return tx.WithSignature(s.env.Signer, signature)
}

// ActResetTxOpts prepares the tx options to default values, based on the current pending block header.
func (s *BasicUser[B]) ActResetTxOpts(t Testing) {
	latestHeader, err := s.env.EthCl.HeaderByNumber(t.Ctx(), nil)
	require.NoError(t, err, "need l2 latest header for accurate basefee info")

	gasTipCap := big.NewInt(2 * params.GWei)
	gasFeeCap := new(big.Int).Add(gasTipCap, new(big.Int).Mul(latestHeader.BaseFee, big.NewInt(2)))

	s.txOpts = bind.TransactOpts{
		From:      s.address,
		Nonce:     nil, // pick nonce based on pending state
		Signer:    s.signerFn,
		Value:     big.NewInt(0),
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		GasLimit:  0,    // a.k.a. estimate
		NoSend:    true, // actions should be explicit about sending
	}
}

func (s *BasicUser[B]) ActRandomTxToAddr(t Testing) {
	i := s.rng.Intn(len(s.env.AddressCorpora))
	var to *common.Address
	if i > 0 { // 0 == nil
		to = &s.env.AddressCorpora[i]
	}
	s.txToAddr = to
}

func (s *BasicUser[B]) ActSetTxCalldata(calldata []byte) Action {
	return func(t Testing) {
		require.NotNil(t, calldata)
		s.txCallData = calldata
	}
}

func (s *BasicUser[B]) ActSetTxToAddr(to *common.Address) Action {
	return func(t Testing) {
		s.txToAddr = to
	}
}

func (s *BasicUser[B]) ActRandomTxValue(t Testing) {
	// compute a random portion of balance
	precision := int64(1000)
	bal, err := s.env.EthCl.BalanceAt(t.Ctx(), s.address, nil)
	require.NoError(t, err)
	part := big.NewInt(s.rng.Int63n(precision))
	new(big.Int).Div(new(big.Int).Mul(bal, part), big.NewInt(precision))
	s.txOpts.Value = big.NewInt(s.rng.Int63())
}

func (s *BasicUser[B]) ActSetTxValue(value *big.Int) Action {
	return func(t Testing) {
		s.txOpts.Value = value
	}
}

func (s *BasicUser[B]) ActRandomTxData(t Testing) {
	dataLen := s.rng.Intn(128_000)
	out := make([]byte, dataLen)
	_, err := s.rng.Read(out[:])
	require.NoError(t, err)
	s.txCallData = out
}

func (s *BasicUser[B]) PendingNonce(t Testing) uint64 {
	if s.txOpts.Nonce != nil {
		return s.txOpts.Nonce.Uint64()
	}
	// fetch from pending state
	nonce, err := s.env.EthCl.PendingNonceAt(t.Ctx(), s.address)
	require.NoError(t, err, "failed to get L1 nonce for account %s", s.address)
	return nonce
}

func (s *BasicUser[B]) TxValue() *big.Int {
	if s.txOpts.Value != nil {
		return s.txOpts.Value
	}
	return big.NewInt(0)
}

func (s *BasicUser[B]) LastTxReceipt(t Testing) *types.Receipt {
	require.NotEqual(t, s.lastTxHash, common.Hash{}, "must send tx before getting last receipt")
	receipt, err := s.env.EthCl.TransactionReceipt(t.Ctx(), s.lastTxHash)
	require.NoError(t, err)
	return receipt
}

func (s *BasicUser[B]) MakeTransaction(t Testing) *types.Transaction {
	gas, err := s.env.EthCl.EstimateGas(t.Ctx(), ethereum.CallMsg{
		From:      s.address,
		To:        s.txToAddr,
		GasFeeCap: s.txOpts.GasFeeCap,
		GasTipCap: s.txOpts.GasTipCap,
		Value:     s.TxValue(),
		Data:      s.txCallData,
	})
	require.NoError(t, err, "gas estimation should pass")
	return types.MustSignNewTx(s.account, s.env.Signer, &types.DynamicFeeTx{
		To:        s.txToAddr,
		GasFeeCap: s.txOpts.GasFeeCap,
		GasTipCap: s.txOpts.GasTipCap,
		Value:     s.TxValue(),
		ChainID:   s.env.Signer.ChainID(),
		Nonce:     s.PendingNonce(t),
		Gas:       gas,
		Data:      s.txCallData,
	})
}

// ActMakeTx makes a tx with the predetermined contents (see randomization and other actions)
// and sends it to the tx pool
func (s *BasicUser[B]) ActMakeTx(t Testing) {
	tx := s.MakeTransaction(t)
	err := s.env.EthCl.SendTransaction(t.Ctx(), tx)
	require.NoError(t, err, "must send tx")
	s.lastTxHash = tx.Hash()
	// reset the calldata
	s.txCallData = []byte{}
}

func (s *BasicUser[B]) ActCheckReceiptStatusOfLastTx(success bool) func(t Testing) {
	return func(t Testing) {
		s.CheckReceipt(t, success, s.lastTxHash)
	}
}

func (s *BasicUser[B]) CheckReceipt(t Testing, success bool, txHash common.Hash) *types.Receipt {
	receipt, err := s.env.EthCl.TransactionReceipt(t.Ctx(), txHash)
	if receipt != nil && err == nil {
		expected := types.ReceiptStatusFailed
		if success {
			expected = types.ReceiptStatusSuccessful
		}
		require.Equal(t, expected, receipt.Status, "expected receipt status to match")
		return receipt
	} else if err != nil && !errors.Is(err, ethereum.NotFound) {
		t.Fatalf("receipt for tx %s was not found", txHash)
	} else {
		t.Fatalf("receipt error: %v", err)
	}
	return nil
}

type L1User struct {
	BasicUser[*L1Bindings]
}

type L2User struct {
	BasicUser[*L2Bindings]
}

// CrossLayerUser represents the same user account on L1 and L2,
// and provides actions to make cross-layer transactions.
type CrossLayerUser struct {
	L1 L1User
	L2 L2User

	// track the last deposit, to easily chain together deposit actions
	lastL1DepositTxHash common.Hash

	lastL2WithdrawalTxHash common.Hash

	allocType config.AllocType
}

func NewCrossLayerUser(log log.Logger, priv *ecdsa.PrivateKey, rng *rand.Rand, allocType config.AllocType) *CrossLayerUser {
	addr := crypto.PubkeyToAddress(priv.PublicKey)
	return &CrossLayerUser{
		L1: L1User{
			BasicUser: BasicUser[*L1Bindings]{
				log:     log,
				rng:     rng,
				account: priv,
				address: addr,
			},
		},
		L2: L2User{
			BasicUser: BasicUser[*L2Bindings]{
				log:     log,
				rng:     rng,
				account: priv,
				address: addr,
			},
		},
		allocType: allocType,
	}
}

func (s *CrossLayerUser) ActDeposit(t Testing) {
	isCreation := false
	toAddr := common.Address{}
	if s.L2.txToAddr == nil {
		isCreation = true
	} else {
		toAddr = *s.L2.txToAddr
	}
	depositTransferValue := s.L2.TxValue()
	depositGas := s.L2.txOpts.GasLimit
	if s.L2.txOpts.GasLimit == 0 {
		// estimate gas used by deposit
		gas, err := s.L2.env.EthCl.EstimateGas(t.Ctx(), ethereum.CallMsg{
			From:       s.L2.address,
			To:         &toAddr,
			Value:      depositTransferValue, // TODO: estimate gas does not support minting yet
			Data:       s.L2.txCallData,
			AccessList: nil,
		})
		require.NoError(t, err)
		depositGas = gas
	}

	// Finally send TX
	s.L1.txOpts.GasLimit = 0
	tx, err := s.L1.env.Bindings.OptimismPortal.DepositTransaction(&s.L1.txOpts, toAddr, depositTransferValue, depositGas, isCreation, s.L2.txCallData)
	require.Nil(t, err, "with deposit tx")

	// Add 10% padding for the L1 gas limit because the estimation process can be affected by the 1559 style cost scale
	// for buying L2 gas in the portal contracts.
	s.L1.txOpts.GasLimit = tx.Gas() + (tx.Gas() / 10)

	tx, err = s.L1.env.Bindings.OptimismPortal.DepositTransaction(&s.L1.txOpts, toAddr, depositTransferValue, depositGas, isCreation, s.L2.txCallData)
	require.NoError(t, err, "failed to create deposit tx")

	s.L1.txOpts.GasLimit = 0

	fmt.Printf("Gas limit: %v\n", tx.Gas())
	// Send the actual tx (since tx opts don't send by default)
	err = s.L1.env.EthCl.SendTransaction(t.Ctx(), tx)
	require.NoError(t, err, "must send tx")
	s.lastL1DepositTxHash = tx.Hash()
}

func (s *CrossLayerUser) ActCheckDepositStatus(l1Success, l2Success bool) Action {
	return func(t Testing) {
		s.CheckDepositTx(t, s.lastL1DepositTxHash, 0, l1Success, l2Success)
	}
}

func (s *CrossLayerUser) CheckDepositTx(t Testing, l1TxHash common.Hash, index int, l1Success, l2Success bool) {
	depositReceipt := s.L1.CheckReceipt(t, l1Success, l1TxHash)
	if depositReceipt == nil {
		require.False(t, l1Success)
		require.False(t, l2Success)
	} else {
		require.Less(t, index, len(depositReceipt.Logs), "must have enough logs in receipt")
		reconstructedDep, err := derive.UnmarshalDepositLogEvent(depositReceipt.Logs[index])
		require.NoError(t, err, "Could not reconstruct L2 Deposit")
		l2Tx := types.NewTx(reconstructedDep)
		s.L2.CheckReceipt(t, l2Success, l2Tx.Hash())
	}
}

func (s *CrossLayerUser) ActStartWithdrawal(t Testing) {
	targetAddr := common.Address{}
	if s.L1.txToAddr != nil {
		targetAddr = *s.L2.txToAddr
	}
	tx, err := s.L2.env.Bindings.L2ToL1MessagePasser.InitiateWithdrawal(&s.L2.txOpts, targetAddr, new(big.Int).SetUint64(s.L1.txOpts.GasLimit), s.L1.txCallData)
	require.NoError(t, err, "create initiate withdraw tx")
	err = s.L2.env.EthCl.SendTransaction(t.Ctx(), tx)
	require.NoError(t, err, "must send tx")
	s.lastL2WithdrawalTxHash = tx.Hash()
}

// ActCheckStartWithdrawal checks that a previous witdrawal tx was either successful or failed.
func (s *CrossLayerUser) ActCheckStartWithdrawal(success bool) Action {
	return func(t Testing) {
		s.L2.CheckReceipt(t, success, s.lastL2WithdrawalTxHash)
	}
}

func (s *CrossLayerUser) Address() common.Address {
	return s.L1.address
}

func (s *CrossLayerUser) getLatestWithdrawalParams(t Testing) (*withdrawals.ProvenWithdrawalParameters, error) {
	receipt := s.L2.CheckReceipt(t, true, s.lastL2WithdrawalTxHash)
	l2WithdrawalBlock, err := s.L2.env.EthCl.BlockByNumber(t.Ctx(), receipt.BlockNumber)
	require.NoError(t, err)

	var l2OutputBlockNr *big.Int
	var l2OutputBlock *types.Block
	if s.allocType.UsesProofs() {
		latestGame, err := withdrawals.FindLatestGame(t.Ctx(), &s.L1.env.Bindings.DisputeGameFactory.DisputeGameFactoryCaller, &s.L1.env.Bindings.OptimismPortal2.OptimismPortal2Caller)
		require.NoError(t, err)
		l2OutputBlockNr = new(big.Int).SetBytes(latestGame.ExtraData[0:32])
		l2OutputBlock, err = s.L2.env.EthCl.BlockByNumber(t.Ctx(), l2OutputBlockNr)
		require.NoError(t, err)
	} else {
		l2OutputBlockNr, err = s.L1.env.Bindings.L2OutputOracle.LatestBlockNumber(&bind.CallOpts{})
		require.NoError(t, err)
		l2OutputBlock, err = s.L2.env.EthCl.BlockByNumber(t.Ctx(), l2OutputBlockNr)
		require.NoError(t, err)
	}

	if l2OutputBlock.NumberU64() < l2WithdrawalBlock.NumberU64() {
		return nil, fmt.Errorf("the latest L2 output is %d and is not past L2 block %d that includes the withdrawal yet, no withdrawal can be proved yet", l2OutputBlock.NumberU64(), l2WithdrawalBlock.NumberU64())
	}

	if !s.allocType.UsesProofs() {
		finalizationPeriod, err := s.L1.env.Bindings.L2OutputOracle.FINALIZATIONPERIODSECONDS(&bind.CallOpts{})
		require.NoError(t, err)
		l1Head, err := s.L1.env.EthCl.HeaderByNumber(t.Ctx(), nil)
		require.NoError(t, err)

		if l2OutputBlock.Time()+finalizationPeriod.Uint64() >= l1Head.Time {
			return nil, fmt.Errorf("L2 output block %d (time %d) is not past finalization period %d from L2 block %d (time %d) at head %d (time %d)", l2OutputBlock.NumberU64(), l2OutputBlock.Time(), finalizationPeriod.Uint64(), l2WithdrawalBlock.NumberU64(), l2WithdrawalBlock.Time(), l1Head.Number.Uint64(), l1Head.Time)
		}
	}

	header, err := s.L2.env.EthCl.HeaderByNumber(t.Ctx(), l2OutputBlockNr)
	require.NoError(t, err)
	params, err := e2ehelpers.ProveWithdrawalParameters(t.Ctx(), s.L2.env.Bindings.ProofClient, s.L2.env.EthCl, s.L2.env.EthCl, s.lastL2WithdrawalTxHash, header, &s.L1.env.Bindings.L2OutputOracle.L2OutputOracleCaller, &s.L1.env.Bindings.DisputeGameFactory.DisputeGameFactoryCaller, &s.L1.env.Bindings.OptimismPortal2.OptimismPortal2Caller, s.allocType)
	require.NoError(t, err)

	return &params, nil
}

func (s *CrossLayerUser) getDisputeGame(t Testing, params withdrawals.ProvenWithdrawalParameters) (*legacybindings.FaultDisputeGame, common.Address, error) {
	wd := crossdomain.Withdrawal{
		Nonce:    params.Nonce,
		Sender:   &params.Sender,
		Target:   &params.Target,
		Value:    params.Value,
		GasLimit: params.GasLimit,
		Data:     params.Data,
	}

	portal2, err := bindingspreview.NewOptimismPortal2(config.L1Deployments(s.allocType).OptimismPortalProxy, s.L1.env.EthCl)
	require.Nil(t, err)

	wdHash, err := wd.Hash()
	require.Nil(t, err)

	game, err := portal2.ProvenWithdrawals(&bind.CallOpts{}, wdHash, s.L1.address)
	require.Nil(t, err)
	require.NotNil(t, game, "withdrawal should be proven")

	proxy, err := legacybindings.NewFaultDisputeGame(game.DisputeGameProxy, s.L1.env.EthCl)
	require.Nil(t, err)

	return proxy, game.DisputeGameProxy, nil
}

// ActCompleteWithdrawal creates a L1 proveWithdrawal tx for latest withdrawal.
// The tx hash is remembered as the last L1 tx, to check as L1 actor.
func (s *CrossLayerUser) ActProveWithdrawal(t Testing) {
	s.L1.lastTxHash = s.ProveWithdrawal(t, s.lastL2WithdrawalTxHash)
}

// ProveWithdrawal creates a L1 proveWithdrawal tx for the given L2 withdrawal tx, returning the tx hash.
func (s *CrossLayerUser) ProveWithdrawal(t Testing, l2TxHash common.Hash) common.Hash {
	params, err := s.getLatestWithdrawalParams(t)
	if err != nil {
		t.InvalidAction("cannot prove withdrawal: %v", err)
		return common.Hash{}
	}

	// Create the prove tx
	tx, err := s.L1.env.Bindings.OptimismPortal.ProveWithdrawalTransaction(
		&s.L1.txOpts,
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

	// Send the actual tx (since tx opts don't send by default)
	err = s.L1.env.EthCl.SendTransaction(t.Ctx(), tx)
	require.NoError(t, err, "must send prove tx")
	return tx.Hash()
}

// ActCompleteWithdrawal creates a L1 withdrawal finalization tx for latest withdrawal.
// The tx hash is remembered as the last L1 tx, to check as L1 actor.
// The withdrawal functions like CompleteWithdrawal
func (s *CrossLayerUser) ActCompleteWithdrawal(t Testing) {
	s.L1.lastTxHash = s.CompleteWithdrawal(t, s.lastL2WithdrawalTxHash)
}

// CompleteWithdrawal creates a L1 withdrawal finalization tx for the given L2 withdrawal tx, returning the tx hash.
// It's an invalid action to attempt to complete a withdrawal that has not passed the L1 finalization period yet
func (s *CrossLayerUser) CompleteWithdrawal(t Testing, l2TxHash common.Hash) common.Hash {
	params, err := s.getLatestWithdrawalParams(t)
	if err != nil {
		t.InvalidAction("cannot complete withdrawal: %v", err)
		return common.Hash{}
	}

	// Create the withdrawal tx
	tx, err := s.L1.env.Bindings.OptimismPortal.FinalizeWithdrawalTransaction(
		&s.L1.txOpts,
		bindings.TypesWithdrawalTransaction{
			Nonce:    params.Nonce,
			Sender:   params.Sender,
			Target:   params.Target,
			Value:    params.Value,
			GasLimit: params.GasLimit,
			Data:     params.Data,
		},
	)
	require.NoError(t, err)

	// Send the actual tx (since tx opts don't send by default)
	err = s.L1.env.EthCl.SendTransaction(t.Ctx(), tx)
	require.NoError(t, err, "must send finalize tx")
	return tx.Hash()
}

// ActResolveClaim creates a L1 resolveClaim tx for the latest withdrawal.
func (s *CrossLayerUser) ActResolveClaim(t Testing) {
	s.L1.lastTxHash = s.ResolveClaim(t, s.lastL2WithdrawalTxHash)
}

// ResolveClaim creates a L1 resolveClaim tx for the given L2 withdrawal tx, returning the tx hash.
func (s *CrossLayerUser) ResolveClaim(t Testing, l2TxHash common.Hash) common.Hash {
	params, err := s.getLatestWithdrawalParams(t)
	if err != nil {
		t.InvalidAction("cannot resolve claim: %v", err)
		return common.Hash{}
	}

	game, gameAddr, err := s.getDisputeGame(t, *params)
	require.NoError(t, err)

	caller := batching.NewMultiCaller(s.L1.env.EthCl.Client(), batching.DefaultBatchSize)
	gameContract, err := contracts.NewFaultDisputeGameContract(context.Background(), metrics.NoopContractMetrics, gameAddr, caller)
	require.Nil(t, err)

	timedCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, wait.For(timedCtx, time.Second, func() (bool, error) {
		err := gameContract.CallResolveClaim(context.Background(), 0)
		t.Logf("Could not resolve dispute game claim: %v", err)
		return err == nil, nil
	}))

	resolveClaimTx, err := game.ResolveClaim(&s.L1.txOpts, common.Big0, common.Big0)
	require.Nil(t, err)

	err = s.L1.env.EthCl.SendTransaction(t.Ctx(), resolveClaimTx)
	require.Nil(t, err)
	return resolveClaimTx.Hash()
}

// ActResolve creates a L1 resolve tx for the latest withdrawal.
// Resolve is different than resolving a claim, the root claim must be resolved first and then
// the game itself can be resolved.
func (s *CrossLayerUser) ActResolve(t Testing) {
	s.L1.lastTxHash = s.Resolve(t, s.lastL2WithdrawalTxHash)
}

// Resolve creates a L1 resolve tx for the given L2 withdrawal tx, returning the tx hash.
func (s *CrossLayerUser) Resolve(t Testing, l2TxHash common.Hash) common.Hash {
	params, err := s.getLatestWithdrawalParams(t)
	if err != nil {
		t.InvalidAction("cannot resolve game: %v", err)
		return common.Hash{}
	}

	game, _, err := s.getDisputeGame(t, *params)
	require.NoError(t, err)

	resolveTx, err := game.Resolve(&s.L1.txOpts)
	require.Nil(t, err)

	err = s.L1.env.EthCl.SendTransaction(t.Ctx(), resolveTx)
	require.Nil(t, err)
	return resolveTx.Hash()
}
