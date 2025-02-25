package fetcher

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	keccakTypes "github.com/ethereum-optimism/optimism/op-challenger/game/keccak/types"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

const (
	// Signal to indicate a receipt should be considered missing
	MissingReceiptStatus = math.MaxUint64
)

var (
	oracleAddr     = common.Address{0x99, 0x98}
	otherAddr      = common.Address{0x12, 0x34}
	claimantKey, _ = crypto.GenerateKey()
	otherKey, _    = crypto.GenerateKey()
	ident          = keccakTypes.LargePreimageIdent{
		Claimant: crypto.PubkeyToAddress(claimantKey.PublicKey),
		UUID:     big.NewInt(888),
	}
	chainID   = big.NewInt(123)
	blockHash = common.Hash{0xdd}
	input1    = keccakTypes.InputData{
		Input:       []byte{0xbb, 0x11},
		Commitments: []common.Hash{{0xcc, 0x11}},
	}
	input2 = keccakTypes.InputData{
		Input:       []byte{0xbb, 0x22},
		Commitments: []common.Hash{{0xcc, 0x22}},
	}
	input3 = keccakTypes.InputData{
		Input:       []byte{0xbb, 0x33},
		Commitments: []common.Hash{{0xcc, 0x33}},
	}
	input4 = keccakTypes.InputData{
		Input:       []byte{0xbb, 0x44},
		Commitments: []common.Hash{{0xcc, 0x44}},
		Finalize:    true,
	}
)

func TestFetchLeaves_NoBlocks(t *testing.T) {
	fetcher, oracle, _ := setupFetcherTest(t)
	oracle.leafBlocks = []uint64{}
	leaves, err := fetcher.FetchInputs(context.Background(), blockHash, oracle, ident)
	require.NoError(t, err)
	require.Empty(t, leaves)
}

func TestFetchLeaves_ErrorOnUnavailableInputBlocks(t *testing.T) {
	fetcher, oracle, _ := setupFetcherTest(t)
	mockErr := fmt.Errorf("oops")
	oracle.inputDataBlocksError = mockErr

	leaves, err := fetcher.FetchInputs(context.Background(), blockHash, oracle, ident)
	require.ErrorContains(t, err, "failed to retrieve leaf block nums")
	require.Empty(t, leaves)
}

func TestFetchLeaves_ErrorOnUnavailableL1Block(t *testing.T) {
	blockNum := uint64(7)
	fetcher, oracle, _ := setupFetcherTest(t)
	oracle.leafBlocks = []uint64{blockNum}

	// No txs means stubL1Source will return an error when we try to fetch the block
	leaves, err := fetcher.FetchInputs(context.Background(), blockHash, oracle, ident)
	require.ErrorContains(t, err, fmt.Sprintf("failed getting tx for block %v", blockNum))
	require.Empty(t, leaves)
}

func TestFetchLeaves_SingleTxSingleLog(t *testing.T) {
	cases := []struct {
		name       string
		txSender   *ecdsa.PrivateKey
		txModifier TxModifier
	}{
		{"from EOA claimant address", claimantKey, ValidTx},
		{"from contract call", otherKey, WithToAddr(otherAddr)},
		{"from contract creation", otherKey, WithoutToAddr()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fetcher, oracle, l1Source := setupFetcherTest(t)
			blockNum := uint64(7)
			oracle.leafBlocks = []uint64{blockNum}

			proposal := oracle.createProposal(input1)
			tx := l1Source.createTx(blockNum, tc.txSender, tc.txModifier)
			l1Source.createLog(tx, proposal)

			inputs, err := fetcher.FetchInputs(context.Background(), blockHash, oracle, ident)
			require.NoError(t, err)
			require.Equal(t, []keccakTypes.InputData{input1}, inputs)
		})
	}
}

func TestFetchLeaves_SingleTxMultipleLogs(t *testing.T) {
	fetcher, oracle, l1Source := setupFetcherTest(t)
	blockNum := uint64(7)
	oracle.leafBlocks = []uint64{blockNum}

	proposal1 := oracle.createProposal(input1)
	proposal2 := oracle.createProposal(input2)
	tx := l1Source.createTx(blockNum, otherKey, WithToAddr(otherAddr))
	l1Source.createLog(tx, proposal1)
	l1Source.createLog(tx, proposal2)

	inputs, err := fetcher.FetchInputs(context.Background(), blockHash, oracle, ident)
	require.NoError(t, err)
	require.Equal(t, []keccakTypes.InputData{input1, input2}, inputs)
}

func TestFetchLeaves_MultipleBlocksAndLeaves(t *testing.T) {
	fetcher, oracle, l1Source := setupFetcherTest(t)
	block1 := uint64(7)
	block2 := uint64(15)
	oracle.leafBlocks = []uint64{block1, block2}

	proposal1 := oracle.createProposal(input1)
	proposal2 := oracle.createProposal(input2)
	proposal3 := oracle.createProposal(input3)
	proposal4 := oracle.createProposal(input4)
	block1Tx := l1Source.createTx(block1, claimantKey, ValidTx)
	block2TxA := l1Source.createTx(block2, claimantKey, ValidTx)
	l1Source.createTx(block2, claimantKey, ValidTx) // Add tx with no logs
	block2TxB := l1Source.createTx(block2, otherKey, WithoutToAddr())
	l1Source.createLog(block1Tx, proposal1)
	l1Source.createLog(block2TxA, proposal2)
	l1Source.createLog(block2TxB, proposal3)
	l1Source.createLog(block2TxB, proposal4)

	inputs, err := fetcher.FetchInputs(context.Background(), blockHash, oracle, ident)
	require.NoError(t, err)
	require.Equal(t, []keccakTypes.InputData{input1, input2, input3, input4}, inputs)
}

func TestFetchLeaves_SkipLogFromWrongContract(t *testing.T) {
	fetcher, oracle, l1Source := setupFetcherTest(t)
	blockNum := uint64(7)
	oracle.leafBlocks = []uint64{blockNum}

	// Emit log from an irrelevant contract address
	proposal1 := oracle.createProposal(input2)
	tx1 := l1Source.createTx(blockNum, claimantKey, ValidTx)
	log1 := l1Source.createLog(tx1, proposal1)
	log1.Address = otherAddr
	// Valid tx
	proposal2 := oracle.createProposal(input1)
	tx2 := l1Source.createTx(blockNum, claimantKey, ValidTx)
	l1Source.createLog(tx2, proposal2)

	inputs, err := fetcher.FetchInputs(context.Background(), blockHash, oracle, ident)
	require.NoError(t, err)
	require.Equal(t, []keccakTypes.InputData{input1}, inputs)
}

func TestFetchLeaves_SkipProposalWithWrongUUID(t *testing.T) {
	fetcher, oracle, l1Source := setupFetcherTest(t)
	blockNum := uint64(7)
	oracle.leafBlocks = []uint64{blockNum}

	// Valid tx but with a different UUID
	proposal1 := oracle.createProposal(input2)
	proposal1.uuid = big.NewInt(874927294)
	tx1 := l1Source.createTx(blockNum, claimantKey, ValidTx)
	l1Source.createLog(tx1, proposal1)
	// Valid tx
	proposal2 := oracle.createProposal(input1)
	tx2 := l1Source.createTx(blockNum, claimantKey, ValidTx)
	l1Source.createLog(tx2, proposal2)

	inputs, err := fetcher.FetchInputs(context.Background(), blockHash, oracle, ident)
	require.NoError(t, err)
	require.Equal(t, []keccakTypes.InputData{input1}, inputs)
}

func TestFetchLeaves_SkipProposalWithWrongClaimant(t *testing.T) {
	fetcher, oracle, l1Source := setupFetcherTest(t)
	blockNum := uint64(7)
	oracle.leafBlocks = []uint64{blockNum}

	// Valid tx but with a different claimant
	proposal1 := oracle.createProposal(input2)
	proposal1.claimantAddr = otherAddr
	tx1 := l1Source.createTx(blockNum, claimantKey, ValidTx)
	l1Source.createLog(tx1, proposal1)
	// Valid tx
	proposal2 := oracle.createProposal(input1)
	tx2 := l1Source.createTx(blockNum, claimantKey, ValidTx)
	l1Source.createLog(tx2, proposal2)

	inputs, err := fetcher.FetchInputs(context.Background(), blockHash, oracle, ident)
	require.NoError(t, err)
	require.Equal(t, []keccakTypes.InputData{input1}, inputs)
}

func TestFetchLeaves_SkipInvalidProposal(t *testing.T) {
	fetcher, oracle, l1Source := setupFetcherTest(t)
	blockNum := uint64(7)
	oracle.leafBlocks = []uint64{blockNum}

	// Set up proposal decoding to fail
	proposal1 := oracle.createProposal(input2)
	proposal1.valid = false
	tx1 := l1Source.createTx(blockNum, claimantKey, ValidTx)
	l1Source.createLog(tx1, proposal1)
	// Valid tx
	proposal2 := oracle.createProposal(input1)
	tx2 := l1Source.createTx(blockNum, claimantKey, ValidTx)
	l1Source.createLog(tx2, proposal2)

	inputs, err := fetcher.FetchInputs(context.Background(), blockHash, oracle, ident)
	require.NoError(t, err)
	require.Equal(t, []keccakTypes.InputData{input1}, inputs)
}

func TestFetchLeaves_SkipProposalWithInsufficientData(t *testing.T) {
	fetcher, oracle, l1Source := setupFetcherTest(t)
	blockNum := uint64(7)
	oracle.leafBlocks = []uint64{blockNum}

	// Log contains insufficient data
	// It should hold a 20 byte address followed by the proposal payload
	proposal1 := oracle.createProposal(input2)
	tx1 := l1Source.createTx(blockNum, claimantKey, ValidTx)
	log1 := l1Source.createLog(tx1, proposal1)
	log1.Data = proposal1.claimantAddr[:19]
	// Valid tx
	proposal2 := oracle.createProposal(input1)
	tx2 := l1Source.createTx(blockNum, claimantKey, ValidTx)
	l1Source.createLog(tx2, proposal2)

	inputs, err := fetcher.FetchInputs(context.Background(), blockHash, oracle, ident)
	require.NoError(t, err)
	require.Equal(t, []keccakTypes.InputData{input1}, inputs)
}

func TestFetchLeaves_SkipProposalMissingCallData(t *testing.T) {
	fetcher, oracle, l1Source := setupFetcherTest(t)
	blockNum := uint64(7)
	oracle.leafBlocks = []uint64{blockNum}

	// Truncate call data from log so that is only contains an address
	proposal1 := oracle.createProposal(input2)
	tx1 := l1Source.createTx(blockNum, claimantKey, ValidTx)
	log1 := l1Source.createLog(tx1, proposal1)
	log1.Data = log1.Data[0:20]
	// Valid tx
	proposal2 := oracle.createProposal(input1)
	tx2 := l1Source.createTx(blockNum, claimantKey, ValidTx)
	l1Source.createLog(tx2, proposal2)

	inputs, err := fetcher.FetchInputs(context.Background(), blockHash, oracle, ident)
	require.NoError(t, err)
	require.Equal(t, []keccakTypes.InputData{input1}, inputs)
}

func TestFetchLeaves_SkipTxWithReceiptStatusFail(t *testing.T) {
	fetcher, oracle, l1Source := setupFetcherTest(t)
	blockNum := uint64(7)
	oracle.leafBlocks = []uint64{blockNum}

	// Valid proposal, but tx reverted
	proposal1 := oracle.createProposal(input2)
	tx1 := l1Source.createTx(blockNum, claimantKey, ValidTx)
	l1Source.createLog(tx1, proposal1)
	l1Source.rcptStatus[tx1.Hash()] = types.ReceiptStatusFailed
	// Valid tx
	proposal2 := oracle.createProposal(input1)
	tx2 := l1Source.createTx(blockNum, claimantKey, ValidTx)
	l1Source.createLog(tx2, proposal2)

	inputs, err := fetcher.FetchInputs(context.Background(), blockHash, oracle, ident)
	require.NoError(t, err)
	require.Equal(t, []keccakTypes.InputData{input1}, inputs)
}

func TestFetchLeaves_ErrorsOnMissingReceipt(t *testing.T) {
	fetcher, oracle, l1Source := setupFetcherTest(t)
	blockNum := uint64(7)
	oracle.leafBlocks = []uint64{blockNum}

	// Valid tx
	proposal1 := oracle.createProposal(input1)
	tx1 := l1Source.createTx(blockNum, claimantKey, ValidTx)
	l1Source.createLog(tx1, proposal1)
	// Valid proposal, but tx receipt is missing
	proposal2 := oracle.createProposal(input2)
	tx2 := l1Source.createTx(blockNum, claimantKey, ValidTx)
	l1Source.createLog(tx2, proposal2)
	l1Source.rcptStatus[tx2.Hash()] = MissingReceiptStatus

	input, err := fetcher.FetchInputs(context.Background(), blockHash, oracle, ident)
	require.ErrorContains(t, err, fmt.Sprintf("failed to retrieve receipt for tx %v", tx2.Hash()))
	require.Nil(t, input)
}

func TestFetchLeaves_ErrorsWhenNoValidLeavesInBlock(t *testing.T) {
	fetcher, oracle, l1Source := setupFetcherTest(t)
	blockNum := uint64(7)
	oracle.leafBlocks = []uint64{blockNum}

	// Irrelevant tx - reverted
	proposal1 := oracle.createProposal(input2)
	tx1 := l1Source.createTx(blockNum, claimantKey, ValidTx)
	l1Source.createLog(tx1, proposal1)
	l1Source.rcptStatus[tx1.Hash()] = types.ReceiptStatusFailed
	// Irrelevant tx - no logs are emitted
	l1Source.createTx(blockNum, claimantKey, ValidTx)

	inputs, err := fetcher.FetchInputs(context.Background(), blockHash, oracle, ident)
	require.ErrorIs(t, err, ErrNoLeavesFound)
	require.Nil(t, inputs)
}

func setupFetcherTest(t *testing.T) (*InputFetcher, *stubOracle, *stubL1Source) {
	oracle := &stubOracle{
		proposals: make(map[byte]*proposalConfig),
	}
	l1Source := &stubL1Source{
		txs:        make(map[uint64]types.Transactions),
		rcptStatus: make(map[common.Hash]uint64),
		logs:       make(map[common.Hash][]*types.Log),
	}
	fetcher := NewPreimageFetcher(testlog.Logger(t, log.LevelTrace), l1Source)
	return fetcher, oracle, l1Source
}

type proposalConfig struct {
	id           byte
	claimantAddr common.Address
	inputData    keccakTypes.InputData
	uuid         *big.Int
	valid        bool
}

type stubOracle struct {
	leafBlocks     []uint64
	nextProposalId byte
	proposals      map[byte]*proposalConfig
	// Add a field to allow for mocking of errors
	inputDataBlocksError error
}

func (o *stubOracle) Addr() common.Address {
	return oracleAddr
}

func (o *stubOracle) GetInputDataBlocks(_ context.Context, _ rpcblock.Block, _ keccakTypes.LargePreimageIdent) ([]uint64, error) {
	if o.inputDataBlocksError != nil {
		return nil, o.inputDataBlocksError
	}
	return o.leafBlocks, nil
}

func (o *stubOracle) DecodeInputData(data []byte) (*big.Int, keccakTypes.InputData, error) {
	if len(data) == 0 {
		return nil, keccakTypes.InputData{}, contracts.ErrInvalidAddLeavesCall
	}
	proposalId := data[0]
	proposal, ok := o.proposals[proposalId]
	if !ok || !proposal.valid {
		return nil, keccakTypes.InputData{}, contracts.ErrInvalidAddLeavesCall
	}

	return proposal.uuid, proposal.inputData, nil
}

type TxModifier func(tx *types.DynamicFeeTx)

var ValidTx TxModifier = func(_ *types.DynamicFeeTx) {
	// no-op
}

func WithToAddr(addr common.Address) TxModifier {
	return func(tx *types.DynamicFeeTx) {
		tx.To = &addr
	}
}

func WithoutToAddr() TxModifier {
	return func(tx *types.DynamicFeeTx) {
		tx.To = nil
	}
}

func (o *stubOracle) createProposal(input keccakTypes.InputData) *proposalConfig {
	id := o.nextProposalId
	o.nextProposalId++

	proposal := &proposalConfig{
		id:           id,
		claimantAddr: ident.Claimant,
		inputData:    input,
		uuid:         ident.UUID,
		valid:        true,
	}
	o.proposals[id] = proposal

	return proposal
}

type stubL1Source struct {
	nextTxId uint64
	// Map block number to tx
	txs map[uint64]types.Transactions
	// Map txHash to receipt
	rcptStatus map[common.Hash]uint64
	// Map txHash to logs
	logs map[common.Hash][]*types.Log
}

func (s *stubL1Source) ChainID(_ context.Context) (*big.Int, error) {
	return chainID, nil
}

func (s *stubL1Source) BlockByNumber(_ context.Context, number *big.Int) (*types.Block, error) {
	txs, ok := s.txs[number.Uint64()]
	if !ok {
		return nil, errors.New("not found")
	}
	return (&types.Block{}).WithBody(types.Body{Transactions: txs}), nil
}

func (s *stubL1Source) TransactionReceipt(_ context.Context, txHash common.Hash) (*types.Receipt, error) {
	rcptStatus, ok := s.rcptStatus[txHash]
	if !ok {
		rcptStatus = types.ReceiptStatusSuccessful
	} else if rcptStatus == MissingReceiptStatus {
		return nil, errors.New("not found")
	}

	logs := s.logs[txHash]
	return &types.Receipt{Status: rcptStatus, Logs: logs}, nil
}

func (s *stubL1Source) createTx(blockNum uint64, key *ecdsa.PrivateKey, txMod TxModifier) *types.Transaction {
	txId := s.nextTxId
	s.nextTxId++

	inner := &types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     txId,
		To:        &oracleAddr,
		Value:     big.NewInt(0),
		GasTipCap: big.NewInt(1),
		GasFeeCap: big.NewInt(2),
		Gas:       3,
		Data:      []byte{},
	}
	txMod(inner)
	tx := types.MustSignNewTx(key, types.LatestSignerForChainID(inner.ChainID), inner)

	// Track tx internally
	txSet := s.txs[blockNum]
	txSet = append(txSet, tx)
	s.txs[blockNum] = txSet

	return tx
}

func (s *stubL1Source) createLog(tx *types.Transaction, proposal *proposalConfig) *types.Log {
	// Concat the claimant address and the proposal id
	// These will be split back into address and id in fetcher.extractRelevantLeavesFromTx
	data := append(proposal.claimantAddr[:], proposal.id)

	txLog := &types.Log{
		Address: oracleAddr,
		Data:    data,
		Topics:  []common.Hash{},

		// ignored (zeroed):
		BlockNumber: 0,
		TxHash:      common.Hash{},
		TxIndex:     0,
		BlockHash:   common.Hash{},
		Index:       0,
		Removed:     false,
	}

	// Track tx log
	logSet := s.logs[tx.Hash()]
	logSet = append(logSet, txLog)
	s.logs[tx.Hash()] = logSet

	return txLog
}
