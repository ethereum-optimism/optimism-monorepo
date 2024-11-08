package testutils

import (
	"context"
	"math/big"

	"github.com/stretchr/testify/mock"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type MockEthClient struct {
	mock.Mock
}

func (m *MockEthClient) Client() *rpc.Client {
	out := m.Mock.Called()
	return out.Get(0).(*rpc.Client)
}

func (m *MockEthClient) InfoByHash(ctx context.Context, hash common.Hash) (eth.BlockInfo, error) {
	out := m.Mock.Called(hash)
	return *out.Get(0).(*eth.BlockInfo), out.Error(1)
}

func (m *MockEthClient) ExpectInfoByHash(hash common.Hash, info eth.BlockInfo, err error) {
	m.Mock.On("InfoByHash", hash).Once().Return(&info, err)
}

func (m *MockEthClient) InfoByNumber(ctx context.Context, number uint64) (eth.BlockInfo, error) {
	out := m.Mock.Called(number)
	return *out.Get(0).(*eth.BlockInfo), out.Error(1)
}

func (m *MockEthClient) ExpectInfoByNumber(number uint64, info eth.BlockInfo, err error) {
	m.Mock.On("InfoByNumber", number).Once().Return(&info, err)
}

func (m *MockEthClient) InfoByLabel(ctx context.Context, label eth.BlockLabel) (eth.BlockInfo, error) {
	out := m.Mock.Called(label)
	return *out.Get(0).(*eth.BlockInfo), out.Error(1)
}

func (m *MockEthClient) ExpectInfoByLabel(label eth.BlockLabel, info eth.BlockInfo, err error) {
	m.Mock.On("InfoByLabel", label).Once().Return(&info, err)
}

func (m *MockEthClient) InfoAndTxsByHash(ctx context.Context, hash common.Hash) (eth.BlockInfo, types.Transactions, error) {
	out := m.Mock.Called(hash)
	return out.Get(0).(eth.BlockInfo), out.Get(1).(types.Transactions), out.Error(2)
}

func (m *MockEthClient) ExpectInfoAndTxsByHash(hash common.Hash, info eth.BlockInfo, transactions types.Transactions, err error) {
	m.Mock.On("InfoAndTxsByHash", hash).Once().Return(info, transactions, err)
}

func (m *MockEthClient) InfoAndTxsByNumber(ctx context.Context, number uint64) (eth.BlockInfo, types.Transactions, error) {
	out := m.Mock.Called(number)
	return out.Get(0).(eth.BlockInfo), out.Get(1).(types.Transactions), out.Error(2)
}

func (m *MockEthClient) ExpectInfoAndTxsByNumber(number uint64, info eth.BlockInfo, transactions types.Transactions, err error) {
	m.Mock.On("InfoAndTxsByNumber", number).Once().Return(info, transactions, err)
}

func (m *MockEthClient) InfoAndTxsByLabel(ctx context.Context, label eth.BlockLabel) (eth.BlockInfo, types.Transactions, error) {
	out := m.Mock.Called(label)
	return out.Get(0).(eth.BlockInfo), out.Get(1).(types.Transactions), out.Error(2)
}

func (m *MockEthClient) ExpectInfoAndTxsByLabel(label eth.BlockLabel, info eth.BlockInfo, transactions types.Transactions, err error) {
	m.Mock.On("InfoAndTxsByLabel", label).Once().Return(info, transactions, err)
}

func (m *MockEthClient) PayloadByHash(ctx context.Context, hash common.Hash) (*eth.ExecutionPayloadEnvelope, error) {
	out := m.Mock.Called(hash)
	return out.Get(0).(*eth.ExecutionPayloadEnvelope), out.Error(1)
}

func (m *MockEthClient) ExpectPayloadByHash(hash common.Hash, payload *eth.ExecutionPayloadEnvelope, err error) {
	m.Mock.On("PayloadByHash", hash).Once().Return(payload, err)
}

func (m *MockEthClient) PayloadByNumber(ctx context.Context, n uint64) (*eth.ExecutionPayloadEnvelope, error) {
	out := m.Mock.MethodCalled("PayloadByNumber", n)
	return out[0].(*eth.ExecutionPayloadEnvelope), *out[1].(*error)
}

func (m *MockEthClient) ExpectPayloadByNumber(n uint64, payload *eth.ExecutionPayloadEnvelope, err error) {
	m.Mock.On("PayloadByNumber", n).Once().Return(payload, &err)
}

func (m *MockEthClient) PayloadByLabel(ctx context.Context, label eth.BlockLabel) (*eth.ExecutionPayloadEnvelope, error) {
	out := m.Mock.Called(label)
	return out.Get(0).(*eth.ExecutionPayloadEnvelope), out.Error(1)
}

func (m *MockEthClient) ExpectPayloadByLabel(label eth.BlockLabel, payload *eth.ExecutionPayloadEnvelope, err error) {
	m.Mock.On("PayloadByLabel", label).Once().Return(payload, err)
}

func (m *MockEthClient) FetchReceipts(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Receipts, error) {
	out := m.Mock.Called(blockHash)
	return *out.Get(0).(*eth.BlockInfo), out.Get(1).(types.Receipts), out.Error(2)
}

func (m *MockEthClient) ExpectFetchReceipts(hash common.Hash, info eth.BlockInfo, receipts types.Receipts, err error) {
	m.Mock.On("FetchReceipts", hash).Once().Return(&info, receipts, err)
}

func (m *MockEthClient) GetProof(ctx context.Context, address common.Address, storage []common.Hash, blockTag string) (*eth.AccountResult, error) {
	out := m.Mock.Called(address, storage, blockTag)
	return out.Get(0).(*eth.AccountResult), out.Error(1)
}

func (m *MockEthClient) ExpectGetProof(address common.Address, storage []common.Hash, blockTag string, result *eth.AccountResult, err error) {
	m.Mock.On("GetProof", address, storage, blockTag).Once().Return(result, err)
}

func (m *MockEthClient) GetStorageAt(ctx context.Context, address common.Address, storageSlot common.Hash, blockTag string) (common.Hash, error) {
	out := m.Mock.Called(address, storageSlot, blockTag)
	return out.Get(0).(common.Hash), out.Error(1)
}

func (m *MockEthClient) ExpectGetStorageAt(ctx context.Context, address common.Address, storageSlot common.Hash, blockTag string, result common.Hash, err error) {
	m.Mock.On("GetStorageAt", address, storageSlot, blockTag).Once().Return(result, err)
}

func (m *MockEthClient) ReadStorageAt(ctx context.Context, address common.Address, storageSlot common.Hash, blockHash common.Hash) (common.Hash, error) {
	out := m.Mock.Called(address, storageSlot, blockHash)
	return out.Get(0).(common.Hash), out.Error(1)
}

func (m *MockEthClient) ExpectReadStorageAt(ctx context.Context, address common.Address, storageSlot common.Hash, blockHash common.Hash, result common.Hash, err error) {
	m.Mock.On("ReadStorageAt", address, storageSlot, blockHash).Once().Return(result, err)
}

func (m *MockEthClient) BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error) {
	out := m.Mock.Called(number)
	return out.Get(0).(*types.Block), out.Error(1)
}

func (m *MockEthClient) ExpectBlockByNumber(number *big.Int, block *types.Block, err error) {
	m.Mock.On("BlockByNumber", number).Once().Return(block, err)
}

func (m *MockEthClient) ExpectClose() {
	m.Mock.On("Close").Once()
}

func (m *MockEthClient) MaybeClose() {
	m.Mock.On("Close").Maybe()
}

func (m *MockEthClient) Close() {
	m.Mock.Called()
}
