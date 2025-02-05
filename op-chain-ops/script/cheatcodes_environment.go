package script

import (
	"bytes"
	"math/big"

	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
)

// Warp implements https://book.getfoundry.sh/cheatcodes/warp
func (c *CheatCodesPrecompile) Warp(timestamp *big.Int) {
	c.h.env.Context.Time = timestamp.Uint64()
}

// Roll implements https://book.getfoundry.sh/cheatcodes/roll
func (c *CheatCodesPrecompile) Roll(num *big.Int) {
	c.h.env.Context.BlockNumber = num
}

// Fee implements https://book.getfoundry.sh/cheatcodes/fee
func (c *CheatCodesPrecompile) Fee(fee *big.Int) {
	c.h.env.Context.BaseFee = fee
}

// GetBlockTimestamp implements https://book.getfoundry.sh/cheatcodes/get-block-timestamp
func (c *CheatCodesPrecompile) GetBlockTimestamp() *big.Int {
	return new(big.Int).SetUint64(c.h.env.Context.Time)
}

// GetBlockNumber implements https://book.getfoundry.sh/cheatcodes/get-block-number
func (c *CheatCodesPrecompile) GetBlockNumber() *big.Int {
	return c.h.env.Context.BlockNumber
}

// Difficulty implements https://book.getfoundry.sh/cheatcodes/difficulty
func (c *CheatCodesPrecompile) Difficulty(_ *big.Int) error {
	return vm.ErrExecutionReverted // only post-merge is supported
}

// Prevrandao implements https://book.getfoundry.sh/cheatcodes/prevrandao
func (c *CheatCodesPrecompile) Prevrandao(v [32]byte) {
	c.h.env.Context.Random = (*common.Hash)(&v)
}

// ChainId implements https://book.getfoundry.sh/cheatcodes/chain-id
func (c *CheatCodesPrecompile) ChainId(id *big.Int) {
	c.h.env.ChainConfig().ChainID = id
	c.h.chainCfg.ChainID = id
	// c.h.env.rules.ChainID is unused, but should maybe also be modified
}

// Store implements https://book.getfoundry.sh/cheatcodes/store
func (c *CheatCodesPrecompile) Store(account common.Address, slot [32]byte, value [32]byte) {
	c.h.state.SetState(account, slot, value)
}

// Load implements https://book.getfoundry.sh/cheatcodes/load
func (c *CheatCodesPrecompile) Load(account common.Address, slot [32]byte) [32]byte {
	return c.h.state.GetState(account, slot)
}

// Etch implements https://book.getfoundry.sh/cheatcodes/etch
func (c *CheatCodesPrecompile) Etch(who common.Address, code []byte) {
	c.h.state.SetCode(who, bytes.Clone(code)) // important to clone; geth EVM will reuse the calldata memory.
	if len(code) > 0 {
		// if we're not just zeroing out the account: allow it to access cheatcodes
		c.h.AllowCheatcodes(who)
	}
}

// Deal implements https://book.getfoundry.sh/cheatcodes/deal
func (c *CheatCodesPrecompile) Deal(who common.Address, newBalance *big.Int) {
	c.h.state.SetBalance(who, uint256.MustFromBig(newBalance), tracing.BalanceChangeUnspecified)
}

// Prank_ca669fa7 implements https://book.getfoundry.sh/cheatcodes/prank
func (c *CheatCodesPrecompile) Prank_ca669fa7(sender common.Address) error {
	return c.h.Prank(&sender, nil, false, false)
}

// Prank_47e50cce implements https://book.getfoundry.sh/cheatcodes/prank
func (c *CheatCodesPrecompile) Prank_47e50cce(sender common.Address, origin common.Address) error {
	return c.h.Prank(&sender, &origin, false, false)
}

// StartPrank_06447d56 implements https://book.getfoundry.sh/cheatcodes/start-prank
func (c *CheatCodesPrecompile) StartPrank_06447d56(sender common.Address) error {
	return c.h.Prank(&sender, nil, true, false)
}

// StartPrank_45b56078 implements https://book.getfoundry.sh/cheatcodes/start-prank
func (c *CheatCodesPrecompile) StartPrank_45b56078(sender common.Address, origin common.Address) error {
	return c.h.Prank(&sender, &origin, true, false)
}

// StopPrank implements https://book.getfoundry.sh/cheatcodes/stop-prank
func (c *CheatCodesPrecompile) StopPrank() error {
	return c.h.StopPrank(false)
}

// ReadCallers implements https://book.getfoundry.sh/cheatcodes/read-callers
func (c *CheatCodesPrecompile) ReadCallers() (callerMode *big.Int, msgSender common.Address, txOrigin common.Address) {
	return c.h.CallerMode().Big(), c.h.MsgSender(), c.h.env.TxContext.Origin
}

// Record implements https://book.getfoundry.sh/cheatcodes/record
func (c *CheatCodesPrecompile) Record() error {
	panic("vm.record not supported")
}

// Accesses implements https://book.getfoundry.sh/cheatcodes/accesses
func (c *CheatCodesPrecompile) Accesses() (reads [][32]byte, writes [][32]byte, err error) {
	panic("vm.accesses not supported")
}

// RecordLogs implements https://book.getfoundry.sh/cheatcodes/record-logs
func (c *CheatCodesPrecompile) RecordLogs() error {
	panic("vm.recordLogs not supported")
}

type Log struct {
	Topics  [][32]byte
	Data    []byte
	Emitter common.Address
}

// GetRecordedLogs implements https://book.getfoundry.sh/cheatcodes/get-recorded-logs
//func (c *CheatCodesPrecompile) GetRecordedLogs() []Log {
//	return nil // TODO
//}

// SetNonce implements https://book.getfoundry.sh/cheatcodes/set-nonce
func (c *CheatCodesPrecompile) SetNonce(account common.Address, nonce uint64) {
	c.h.state.SetNonce(account, nonce)
}

// GetNonce implements https://book.getfoundry.sh/cheatcodes/get-nonce
func (c *CheatCodesPrecompile) GetNonce(addr common.Address) uint64 {
	return c.h.state.GetNonce(addr)
}

func (c *CheatCodesPrecompile) ResetNonce(addr common.Address) {
	// Undocumented cheatcode of forge, but used a lot.
	// Resets nonce to 0 if EOA, or 1 if contract.
	// In scripts often set code to empty first when using it, it then becomes 0.
	if c.h.state.GetCodeHash(addr) == types.EmptyCodeHash {
		c.h.state.SetNonce(addr, 0)
	} else {
		c.h.state.SetNonce(addr, 1)
	}
}

// MockCall_b96213e4 implements https://book.getfoundry.sh/cheatcodes/mock-call
func (c *CheatCodesPrecompile) MockCall_b96213e4(where common.Address, data []byte, retdata []byte) error {
	panic("mockCall not supported")
}

// MockCall_81409b91 implements https://book.getfoundry.sh/cheatcodes/mock-call
func (c *CheatCodesPrecompile) MockCall_81409b91(where common.Address, value *big.Int, data []byte, retdata []byte) error {
	panic("vm.mockCall not supported")
}

// MockCallRevert_dbaad147 implements https://book.getfoundry.sh/cheatcodes/mock-call-revert
func (c *CheatCodesPrecompile) MockCallRevert_dbaad147(where common.Address, data []byte, retdata []byte) error {
	panic("vm.mockCall not supported")
}

// MockCallRevert_d23cd037 implements https://book.getfoundry.sh/cheatcodes/mock-call-revert
func (c *CheatCodesPrecompile) MockCallRevert_d23cd037(where common.Address, value *big.Int, data []byte, retdata []byte) error {
	panic("vm.mockCall not supported")
}

// ClearMockedCalls implements https://book.getfoundry.sh/cheatcodes/clear-mocked-calls
func (c *CheatCodesPrecompile) ClearMockedCalls() error {
	panic("vm.clearMockedCalls not supported")
}

// Coinbase implements https://book.getfoundry.sh/cheatcodes/coinbase
func (c *CheatCodesPrecompile) Coinbase(addr common.Address) {
	c.h.env.Context.Coinbase = addr
}

// Broadcast_afc98040 implements https://book.getfoundry.sh/cheatcodes/broadcast
func (c *CheatCodesPrecompile) Broadcast_afc98040() error {
	return c.h.Prank(nil, nil, false, true)
}

// Broadcast_e6962cdb implements https://book.getfoundry.sh/cheatcodes/broadcast
func (c *CheatCodesPrecompile) Broadcast_e6962cdb(who common.Address) error {
	return c.h.Prank(&who, nil, false, true)
}

// StartBroadcast_7fb5297f implements https://book.getfoundry.sh/cheatcodes/start-broadcast
func (c *CheatCodesPrecompile) StartBroadcast_7fb5297f() error {
	return c.h.Prank(nil, nil, true, true)
}

// StartBroadcast_7fec2a8d implements https://book.getfoundry.sh/cheatcodes/start-broadcast
func (c *CheatCodesPrecompile) StartBroadcast_7fec2a8d(who common.Address) error {
	return c.h.Prank(&who, nil, true, true)
}

// StopBroadcast implements https://book.getfoundry.sh/cheatcodes/stop-broadcast
func (c *CheatCodesPrecompile) StopBroadcast() error {
	return c.h.StopPrank(true)
}

// PauseGasMetering implements https://book.getfoundry.sh/cheatcodes/pause-gas-metering
func (c *CheatCodesPrecompile) PauseGasMetering() error {
	panic("vm.pauseGasMetering not supported")
}

// ResumeGasMetering implements https://book.getfoundry.sh/cheatcodes/resume-gas-metering
func (c *CheatCodesPrecompile) ResumeGasMetering() error {
	panic("vm.resumeGasMetering not supported")
}

// TxGasPrice implements https://book.getfoundry.sh/cheatcodes/tx-gas-price
func (c *CheatCodesPrecompile) TxGasPrice(newGasPrice *big.Int) {
	c.h.env.TxContext.GasPrice = newGasPrice
}

// StartStateDiffRecording implements https://book.getfoundry.sh/cheatcodes/start-state-diff-recording
func (c *CheatCodesPrecompile) StartStateDiffRecording() error {
	panic("vm.startStateDiffRecording not supported")
}

// StopAndReturnStateDiff implements https://book.getfoundry.sh/cheatcodes/stop-and-return-state-diff
func (c *CheatCodesPrecompile) StopAndReturnStateDiff() error {
	panic("vm.stopAndReturnStateDiff not supported")
}
