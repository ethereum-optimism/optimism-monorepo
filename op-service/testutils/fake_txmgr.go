package testutils

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

// FakeTxMgr is a fake txmgr.TxManager for testing the op-batcher.
type FakeTxMgr struct {
	log               log.Logger
	FromAddr          common.Address
	Closed            bool
	Nonce             uint64
	errorEveryNthSend uint // 0 means never error, 1 means every send errors, etc.
	sendCount         uint
}

var _ txmgr.TxManager = (*FakeTxMgr)(nil)

func NewFakeTxMgr(log log.Logger, from common.Address) *FakeTxMgr {
	return &FakeTxMgr{
		log:      log,
		FromAddr: from,
	}
}

func (f *FakeTxMgr) ErrorEveryNthSend(n uint) {
	f.errorEveryNthSend = n
}

func (f *FakeTxMgr) Send(ctx context.Context, candidate txmgr.TxCandidate) (*types.Receipt, error) {
	// We currently only use the FakeTxMgr to test the op-batcher, which only uses SendAsync.
	// Send makes it harder to track failures and nonce management (prob need to add mutex, etc).
	// We can implement this if/when its needed.
	panic("FakeTxMgr does not implement Send")
}
func (f *FakeTxMgr) SendAsync(ctx context.Context, candidate txmgr.TxCandidate, ch chan txmgr.SendResponse) {
	f.log.Debug("SendingAsync tx", "nonce", f.Nonce)
	f.sendCount++
	var sendResponse txmgr.SendResponse
	if f.errorEveryNthSend != 0 && f.sendCount%f.errorEveryNthSend == 0 {
		sendResponse.Err = errors.New("errorEveryNthSend")
	} else {
		sendResponse.Receipt = &types.Receipt{
			BlockHash:   common.Hash{},
			BlockNumber: big.NewInt(0),
		}
		sendResponse.Nonce = f.Nonce
		f.Nonce++
	}
	ch <- sendResponse
}
func (f *FakeTxMgr) From() common.Address {
	return f.FromAddr
}
func (f *FakeTxMgr) BlockNumber(ctx context.Context) (uint64, error) {
	return 0, nil
}
func (f *FakeTxMgr) API() rpc.API {
	return rpc.API{}
}
func (f *FakeTxMgr) Close() {
	f.Closed = true
}
func (f *FakeTxMgr) IsClosed() bool {
	return f.Closed
}
func (f *FakeTxMgr) SuggestGasPriceCaps(ctx context.Context) (tipCap *big.Int, baseFee *big.Int, blobBaseFee *big.Int, err error) {
	return nil, nil, nil, nil
}
