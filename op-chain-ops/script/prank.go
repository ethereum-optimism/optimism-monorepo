package script

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script/addresses"

	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
)

// Prank represents an active prank task for the next sub-call.
// This is embedded into a call-frame, to then influence the sub-call through a caller-override.
type Prank struct {
	// Sender overrides msg.sender
	Sender *common.Address
	// Origin overrides tx.origin (set to actual origin if not part of the prank)
	Origin *common.Address
	// PrevOrigin is the tx.origin to restore after the prank
	PrevOrigin common.Address
	// Repeat is true if the prank persists after returning from a sub-call
	Repeat bool
	// A Prank may be a broadcast also.
	Broadcast bool
}

// prankRef implements the vm.ContractRef interface, to mock a caller.
type prankRef struct {
	prank common.Address
	ref   vm.ContractRef
}

var _ vm.ContractRef = (*prankRef)(nil)

func (p *prankRef) Address() common.Address {
	return p.prank
}

// Value returns the value send into this contract context.
// The delegate call tracer implicitly relies on this being implemented on ContractRef
func (p *prankRef) Value() *uint256.Int {
	return p.ref.(interface{ Value() *uint256.Int }).Value()
}

func (h *Host) handleCaller(caller vm.ContractRef) vm.ContractRef {
	// apply prank, if top call-frame had set up a prank
	if len(h.callStack) > 0 {
		parentCallFrame := h.callStack[len(h.callStack)-1]
		if parentCallFrame.Prank != nil && caller.Address() != addresses.VMAddr { // pranks do not apply to the cheatcode precompile
			if parentCallFrame.Prank.Broadcast && parentCallFrame.LastOp == vm.CREATE2 && h.useCreate2Deployer {
				return &prankRef{
					prank: DeterministicDeployerAddress,
					ref:   caller,
				}
			}

			if parentCallFrame.Prank.Sender != nil {
				return &prankRef{
					prank: *parentCallFrame.Prank.Sender,
					ref:   caller,
				}
			}
			if parentCallFrame.Prank.Origin != nil {
				h.env.TxContext.Origin = *parentCallFrame.Prank.Origin
			}
		}
	}
	return caller
}

// Prank applies a prank to the current call-frame.
// Any sub-call will apply the prank to their frame context.
func (h *Host) Prank(msgSender *common.Address, txOrigin *common.Address, repeat bool, broadcast bool) error {
	if len(h.callStack) == 0 {
		h.log.Warn("no call stack")
		return nil // cannot prank while not in a call.
	}
	cf := h.callStack[len(h.callStack)-1]
	if cf.Prank != nil {
		if cf.Prank.Broadcast && !broadcast {
			return errors.New("you have an active broadcast; broadcasting and pranks are not compatible")
		}
		if !cf.Prank.Broadcast && broadcast {
			return errors.New("you have an active prank; broadcasting and pranks are not compatible")
		}
	}
	if broadcast {
		h.log.Debug("starting broadcast", "sender", msgSender, "repeat", repeat)
	} else {
		h.log.Debug("starting prank", "sender", msgSender, "repeat", repeat)
	}
	cf.Prank = &Prank{
		Sender:     msgSender,
		Origin:     txOrigin,
		PrevOrigin: h.env.TxContext.Origin,
		Repeat:     repeat,
		Broadcast:  broadcast,
	}
	return nil
}

// StopPrank disables the current prank. Any sub-call will not be pranked.
func (h *Host) StopPrank(broadcast bool) error {
	if len(h.callStack) == 0 {
		return nil
	}
	cf := h.callStack[len(h.callStack)-1]
	if cf.Prank == nil {
		if broadcast {
			return errors.New("no broadcast in progress to stop")
		}
		return nil
	}
	if cf.Prank.Broadcast && !broadcast {
		// stopPrank on active broadcast is silent and no-op
		return nil
	}
	if !cf.Prank.Broadcast && broadcast {
		return errors.New("no broadcast in progress to stop")
	}
	if broadcast {
		h.log.Debug("stopping broadcast")
	} else {
		h.log.Debug("stopping prank")
	}
	cf.Prank = nil
	return nil
}

// CallerMode returns the type of the top-most callframe,
// i.e. if we are in regular operation, a prank, or a broadcast (special kind of prank).
func (h *Host) CallerMode() CallerMode {
	if len(h.callStack) == 0 {
		return CallerModeNone
	}
	cf := h.callStack[len(h.callStack)-1]
	if cf.Prank != nil {
		if cf.Prank.Broadcast {
			if cf.Prank.Repeat {
				return CallerModeRecurrentBroadcast
			}
			return CallerModeBroadcast
		}
		if cf.Prank.Repeat {
			return CallerModeRecurrentPrank
		}
		return CallerModePrank
	}
	return CallerModeNone
}

// CallerMode matches the CallerMode forge cheatcode enum.
type CallerMode uint8

func (cm CallerMode) Big() *big.Int {
	return big.NewInt(int64(cm))
}

const (
	CallerModeNone CallerMode = iota
	CallerModeBroadcast
	CallerModeRecurrentBroadcast
	CallerModePrank
	CallerModeRecurrentPrank
)

type BroadcastType string

const (
	BroadcastCall   BroadcastType = "call"
	BroadcastCreate BroadcastType = "create"
	// BroadcastCreate2 is to be broadcast via the Create2Deployer,
	// and not really documented much anywhere.
	BroadcastCreate2 BroadcastType = "create2"
)

func (bt BroadcastType) String() string {
	return string(bt)
}

func (bt BroadcastType) MarshalText() ([]byte, error) {
	return []byte(bt.String()), nil
}

func (bt *BroadcastType) UnmarshalText(data []byte) error {
	v := BroadcastType(data)
	switch v {
	case BroadcastCall, BroadcastCreate, BroadcastCreate2:
		*bt = v
		return nil
	default:
		return fmt.Errorf("unrecognized broadcast type bytes: %x", data)
	}
}

// Broadcast captures a transaction that was selected to be broadcast
// via vm.broadcast(). Actually submitting the transaction is left up
// to other tools.
type Broadcast struct {
	From    common.Address `json:"from"`
	To      common.Address `json:"to"`    // set to expected contract address, if this is a deployment
	Input   hexutil.Bytes  `json:"input"` // set to contract-creation code, if this is a deployment
	Value   *hexutil.U256  `json:"value"`
	Salt    common.Hash    `json:"salt"` // set if this is a Create2 broadcast
	GasUsed uint64         `json:"gasUsed"`
	Type    BroadcastType  `json:"type"`
	Nonce   uint64         `json:"nonce"` // pre-state nonce of From, before any increment (always 0 if create2)
}

// ID returns a hash that can be used to identify the broadcast.
// This is used instead of the transaction hash since broadcasting
// tools can change gas limits and other fields which would change
// the resulting transaction hash.
func (b Broadcast) ID() common.Hash {
	h := sha256.New()
	_, _ = h.Write(b.From[:])
	_, _ = h.Write(b.To[:])
	_, _ = h.Write(b.Input)
	_, _ = h.Write(((*uint256.Int)(b.Value)).Bytes())
	_, _ = h.Write(b.Salt[:])
	nonce := make([]byte, 8)
	binary.BigEndian.PutUint64(nonce, b.Nonce)
	_, _ = h.Write(nonce)
	sum := h.Sum(nil)
	return common.BytesToHash(sum)
}

// NewBroadcast creates a Broadcast from a parent callframe, and the completed child callframe.
// This method is preferred to manually creating the struct since it correctly handles
// data that must be copied prior to being returned to prevent accidental mutation.
func NewBroadcast(parent, current *CallFrame) Broadcast {
	ctx := current.Ctx

	value := ctx.CallValue()
	if value == nil {
		value = uint256.NewInt(0)
	}

	// Code is tracked separate from calldata input,
	// even though they are the same thing for a regular contract creation
	input := ctx.CallInput()
	if ctx.Contract.IsDeployment {
		input = ctx.Contract.Code
	}

	bcast := Broadcast{
		From: ctx.Caller(),
		To:   ctx.Address(),
		// Need to clone the input below since memory is reused in the VM
		Input:   bytes.Clone(input),
		Value:   (*hexutil.U256)(value.Clone()),
		GasUsed: current.GasUsed,
	}

	switch parent.LastOp {
	case vm.CREATE:
		bcast.Type = BroadcastCreate
		// Nonce bump was already applied, but we need the pre-state
		bcast.Nonce = current.CallerNonce - 1
		expectedAddr := crypto.CreateAddress(bcast.From, bcast.Nonce)
		if expectedAddr != bcast.To {
			panic(fmt.Errorf("script bug: create broadcast has "+
				"unexpected address: %s, expected %s. Sender: %s, Nonce: %d",
				bcast.To, expectedAddr, bcast.From, bcast.Nonce))
		}
	case vm.CREATE2:
		bcast.Salt = parent.LastCreate2Salt
		initHash := crypto.Keccak256Hash(bcast.Input)
		expectedAddr := crypto.CreateAddress2(bcast.From, bcast.Salt, initHash[:])
		// Sanity-check the create2 salt is correct by checking the address computation.
		if expectedAddr != bcast.To {
			panic(fmt.Errorf("script bug: create2 broadcast has "+
				"unexpected address: %s, expected %s. Sender: %s, Salt: %s, Inithash: %s",
				bcast.To, expectedAddr, bcast.From, bcast.Salt, initHash))
		}
		bcast.Type = BroadcastCreate2
		bcast.Nonce = 0 // always 0. The nonce should not matter for create2.
	case vm.CALL:
		bcast.Type = BroadcastCall
		// Nonce bump was already applied, but we need the pre-state
		bcast.Nonce = current.CallerNonce - 1
	default:
		panic(fmt.Errorf("unexpected broadcast operation %s", parent.LastOp))
	}

	return bcast
}
