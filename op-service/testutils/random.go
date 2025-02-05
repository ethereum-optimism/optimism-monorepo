package testutils

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"math/rand"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie"
)

func RandomBool(rng *rand.Rand) bool {
	if b := rng.Intn(2); b == 0 {
		return false
	}
	return true
}

func RandomHash(rng *rand.Rand) (out common.Hash) {
	rng.Read(out[:])
	return
}

func RandomAddress(rng *rand.Rand) (out common.Address) {
	rng.Read(out[:])
	return
}

func RandomETH(rng *rand.Rand, max int64) *big.Int {
	x := big.NewInt(rng.Int63n(max))
	x = new(big.Int).Mul(x, big.NewInt(1e18))
	return x
}

func RandomKey() *ecdsa.PrivateKey {
	key, err := crypto.GenerateKey()
	if err != nil {
		panic("couldn't generate key: " + err.Error())
	}
	return key
}

func RandomData(rng *rand.Rand, size int) []byte {
	out := make([]byte, size)
	rng.Read(out)
	return out
}

func RandomBlockID(rng *rand.Rand) eth.BlockID {
	return eth.BlockID{
		Hash:   RandomHash(rng),
		Number: rng.Uint64() & ((1 << 50) - 1), // be json friendly
	}
}

func RandomBlockRef(rng *rand.Rand) eth.L1BlockRef {
	return eth.L1BlockRef{
		Hash:       RandomHash(rng),
		Number:     rng.Uint64(),
		ParentHash: RandomHash(rng),
		Time:       rng.Uint64(),
	}
}

func NextRandomRef(rng *rand.Rand, parent eth.L1BlockRef) eth.L1BlockRef {
	return eth.L1BlockRef{
		Hash:       RandomHash(rng),
		Number:     parent.Number + 1,
		ParentHash: parent.Hash,
		Time:       parent.Time + uint64(rng.Intn(100)),
	}
}

func RandomL2BlockRef(rng *rand.Rand) eth.L2BlockRef {
	return eth.L2BlockRef{
		Hash:           RandomHash(rng),
		Number:         rng.Uint64(),
		ParentHash:     RandomHash(rng),
		Time:           rng.Uint64(),
		L1Origin:       RandomBlockID(rng),
		SequenceNumber: rng.Uint64(),
	}
}

func NextRandomL2Ref(rng *rand.Rand, l2BlockTime uint64, parent eth.L2BlockRef, origin eth.BlockID) eth.L2BlockRef {
	seq := parent.SequenceNumber + 1
	if parent.L1Origin != origin {
		seq = 0
	}
	return eth.L2BlockRef{
		Hash:           RandomHash(rng),
		Number:         parent.Number + 1,
		ParentHash:     parent.Hash,
		Time:           parent.Time + l2BlockTime,
		L1Origin:       eth.BlockID{},
		SequenceNumber: seq,
	}
}

// InsecureRandomKey returns a random private key from a limited set of keys.
// Output is deterministic when the supplied rng generates the same random sequence.
func InsecureRandomKey(rng *rand.Rand) *ecdsa.PrivateKey {
	idx := rng.Intn(len(randomEcdsaKeys))
	key, err := crypto.ToECDSA(common.FromHex(randomEcdsaKeys[idx]))
	if err != nil {
		// Should never happen because the list of keys is hard coded and known to be valid.
		panic(fmt.Errorf("invalid pre-generated ecdsa key at index %v: %w", idx, err))
	}
	return key
}

func RandomLog(rng *rand.Rand) *types.Log {
	topics := make([]common.Hash, rng.Intn(3))
	for i := 0; i < len(topics); i++ {
		topics[i] = RandomHash(rng)
	}
	return &types.Log{
		Address:     RandomAddress(rng),
		Topics:      topics,
		Data:        RandomData(rng, rng.Intn(1000)),
		BlockNumber: 0,
		TxHash:      common.Hash{},
		TxIndex:     0,
		BlockHash:   common.Hash{},
		Index:       0,
		Removed:     false,
	}
}

func RandomTo(rng *rand.Rand) *common.Address {
	if rng.Intn(2) == 0 {
		return nil
	}
	to := RandomAddress(rng)
	return &to
}

func RandomTx(rng *rand.Rand, baseFee *big.Int, signer types.Signer) *types.Transaction {
	txTypeList := []int{types.LegacyTxType, types.AccessListTxType, types.DynamicFeeTxType}
	txType := txTypeList[rng.Intn(len(txTypeList))]
	var tx *types.Transaction
	switch txType {
	case types.LegacyTxType:
		tx = RandomLegacyTx(rng, signer)
	case types.AccessListTxType:
		tx = RandomAccessListTx(rng, signer)
	case types.DynamicFeeTxType:
		tx = RandomDynamicFeeTxWithBaseFee(rng, baseFee, signer)
	default:
		panic("invalid tx type")
	}
	return tx
}

func RandomLegacyTxNotProtected(rng *rand.Rand) *types.Transaction {
	return RandomLegacyTx(rng, types.HomesteadSigner{})
}

func RandomLegacyTx(rng *rand.Rand, signer types.Signer) *types.Transaction {
	key := InsecureRandomKey(rng)
	txData := &types.LegacyTx{
		Nonce:    rng.Uint64(),
		GasPrice: new(big.Int).SetUint64(rng.Uint64()),
		Gas:      params.TxGas + uint64(rng.Int63n(2_000_000)),
		To:       RandomTo(rng),
		Value:    RandomETH(rng, 10),
		Data:     RandomData(rng, rng.Intn(RandomDataSize)),
	}
	tx, err := types.SignNewTx(key, signer, txData)
	if err != nil {
		panic(err)
	}
	return tx
}

func RandomAccessListTx(rng *rand.Rand, signer types.Signer) *types.Transaction {
	key := InsecureRandomKey(rng)
	txData := &types.AccessListTx{
		ChainID:    signer.ChainID(),
		Nonce:      rng.Uint64(),
		GasPrice:   new(big.Int).SetUint64(rng.Uint64()),
		Gas:        params.TxGas + uint64(rng.Int63n(2_000_000)),
		To:         RandomTo(rng),
		Value:      RandomETH(rng, 10),
		Data:       RandomData(rng, rng.Intn(RandomDataSize)),
		AccessList: nil,
	}
	tx, err := types.SignNewTx(key, signer, txData)
	if err != nil {
		panic(err)
	}
	return tx
}

func RandomDynamicFeeTxWithBaseFee(rng *rand.Rand, baseFee *big.Int, signer types.Signer) *types.Transaction {
	key := InsecureRandomKey(rng)
	tip := big.NewInt(rng.Int63n(10 * params.GWei))
	txData := &types.DynamicFeeTx{
		ChainID:    signer.ChainID(),
		Nonce:      rng.Uint64(),
		GasTipCap:  tip,
		GasFeeCap:  new(big.Int).Add(baseFee, tip),
		Gas:        params.TxGas + uint64(rng.Int63n(2_000_000)),
		To:         RandomTo(rng),
		Value:      RandomETH(rng, 10),
		Data:       RandomData(rng, rng.Intn(RandomDataSize)),
		AccessList: nil,
	}
	tx, err := types.SignNewTx(key, signer, txData)
	if err != nil {
		panic(err)
	}
	return tx
}

var RandomDataSize = 1000

func RandomDynamicFeeTx(rng *rand.Rand, signer types.Signer) *types.Transaction {
	baseFee := new(big.Int).SetUint64(rng.Uint64())
	return RandomDynamicFeeTxWithBaseFee(rng, baseFee, signer)
}

func RandomReceipt(rng *rand.Rand, signer types.Signer, tx *types.Transaction, txIndex uint64, cumulativeGasUsed uint64) *types.Receipt {
	gasUsed := params.TxGas + uint64(rng.Int63n(int64(tx.Gas()-params.TxGas+1)))
	logs := make([]*types.Log, rng.Intn(10))
	for i := range logs {
		logs[i] = RandomLog(rng)
	}
	var contractAddr common.Address
	if tx.To() == nil {
		sender, err := signer.Sender(tx)
		if err != nil {
			panic(err)
		}
		contractAddr = crypto.CreateAddress(sender, tx.Nonce())
	}
	return &types.Receipt{
		Type:              tx.Type(),
		Status:            uint64(rng.Intn(2)),
		CumulativeGasUsed: cumulativeGasUsed + gasUsed,
		Bloom:             types.Bloom{},
		Logs:              logs,
		TxHash:            tx.Hash(),
		ContractAddress:   contractAddr,
		GasUsed:           gasUsed,
		TransactionIndex:  uint(txIndex),
	}
}

func RandomHeaderWithTime(rng *rand.Rand, t uint64) *types.Header {
	return &types.Header{
		ParentHash:  RandomHash(rng),
		UncleHash:   types.EmptyUncleHash,
		Coinbase:    RandomAddress(rng),
		Root:        RandomHash(rng),
		TxHash:      types.EmptyRootHash,
		ReceiptHash: types.EmptyRootHash,
		Bloom:       types.Bloom{},
		Difficulty:  big.NewInt(0),
		Number:      big.NewInt(1 + rng.Int63n(100_000_000)),
		GasLimit:    0,
		GasUsed:     0,
		Time:        t,
		Extra:       RandomData(rng, rng.Intn(33)),
		MixDigest:   common.Hash{},
		Nonce:       types.BlockNonce{},
		BaseFee:     big.NewInt(rng.Int63n(300_000_000_000)),
	}
}

func RandomHeader(rng *rand.Rand) *types.Header {
	t := uint64(rng.Int63n(2_000_000_000))
	return RandomHeaderWithTime(rng, t)
}

func RandomBlock(rng *rand.Rand, txCount uint64) (*types.Block, []*types.Receipt) {
	return RandomBlockPrependTxs(rng, int(txCount))
}

func RandomBlockPrependTxsWithTime(rng *rand.Rand, txCount int, t uint64, ptxs ...*types.Transaction) (*types.Block, []*types.Receipt) {
	header := RandomHeaderWithTime(rng, t)
	chainID := big.NewInt(rng.Int63n(1000))
	signer := types.NewLondonSigner(chainID)
	txs := make([]*types.Transaction, 0, txCount+len(ptxs))
	txs = append(txs, ptxs...)
	for i := 0; i < txCount; i++ {
		txs = append(txs, RandomTx(rng, header.BaseFee, signer))
	}
	receipts := make([]*types.Receipt, 0, len(txs))
	cumulativeGasUsed := uint64(0)
	for i, tx := range txs {
		r := RandomReceipt(rng, signer, tx, uint64(i), cumulativeGasUsed)
		cumulativeGasUsed += r.GasUsed
		receipts = append(receipts, r)
	}
	header.GasUsed = cumulativeGasUsed
	header.GasLimit = cumulativeGasUsed + uint64(rng.Int63n(int64(cumulativeGasUsed)))
	body := types.Body{
		Transactions: txs,
	}
	block := types.NewBlock(header, &body, receipts, trie.NewStackTrie(nil), types.DefaultBlockConfig)
	logIndex := uint(0)
	for i, r := range receipts {
		r.BlockHash = block.Hash()
		r.BlockNumber = block.Number()
		for _, l := range r.Logs {
			l.BlockHash = block.Hash()
			l.BlockNumber = block.NumberU64()
			l.TxIndex = uint(i)
			l.TxHash = txs[i].Hash()
			l.Index = logIndex
			logIndex += 1
		}
	}
	return block, receipts
}

// RandomBlockPrependTxs returns a random block with txCount randomly generated
// transactions and additionally the transactions ptxs prepended. So the total
// number of transactions is len(ptxs) + txCount.
func RandomBlockPrependTxs(rng *rand.Rand, txCount int, ptxs ...*types.Transaction) (*types.Block, []*types.Receipt) {
	t := uint64(rng.Int63n(2_000_000_000))
	return RandomBlockPrependTxsWithTime(rng, txCount, t, ptxs...)
}

func RandomOutputResponse(rng *rand.Rand) *eth.OutputResponse {
	return &eth.OutputResponse{
		Version:               eth.Bytes32(RandomHash(rng)),
		OutputRoot:            eth.Bytes32(RandomHash(rng)),
		BlockRef:              RandomL2BlockRef(rng),
		WithdrawalStorageRoot: RandomHash(rng),
		StateRoot:             RandomHash(rng),
		Status: &eth.SyncStatus{
			CurrentL1:          RandomBlockRef(rng),
			CurrentL1Finalized: RandomBlockRef(rng),
			HeadL1:             RandomBlockRef(rng),
			SafeL1:             RandomBlockRef(rng),
			FinalizedL1:        RandomBlockRef(rng),
			UnsafeL2:           RandomL2BlockRef(rng),
			SafeL2:             RandomL2BlockRef(rng),
			FinalizedL2:        RandomL2BlockRef(rng),
			PendingSafeL2:      RandomL2BlockRef(rng),
		},
	}
}

func RandomOutputV0(rng *rand.Rand) *eth.OutputV0 {
	return &eth.OutputV0{
		StateRoot:                eth.Bytes32(RandomHash(rng)),
		MessagePasserStorageRoot: eth.Bytes32(RandomHash(rng)),
		BlockHash:                RandomHash(rng),
	}
}
