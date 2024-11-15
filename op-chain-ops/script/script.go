package script

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script/addresses"
	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/ethereum/go-ethereum/triedb/hashdb"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-chain-ops/script/forking"
	"github.com/ethereum-optimism/optimism/op-chain-ops/srcmap"
)

// jumpHistory is the amount of successful jumps to track for debugging.
const jumpHistory = 5

// CallFrame encodes the scope context of the current call
type CallFrame struct {
	Depth int

	LastOp vm.OpCode
	LastPC uint64

	// To reconstruct a create2 later, e.g. on broadcast
	LastCreate2Salt [32]byte

	// Reverts often happen in generated code.
	// We want to fallback to logging the source-map position of
	// the non-generated code, i.e. the origin of the last successful jump.
	// And beyond that, a short history of the latest jumps is useful for debugging.
	// This is a list of program-counters at the time of the jump (i.e. before raching JUMPDEST).
	LastJumps []uint64

	Ctx *vm.ScopeContext

	// Prank overrides the msg.sender, and optionally the origin.
	// Forge script does not support nested pranks on the same call-depth.
	// Pranks can also be broadcasting.
	Prank *Prank

	// GasUsed keeps track of the amount of gas used by this call frame.
	// This is useful for broadcasts, which sometimes cannot correctly
	// estimate gas when sending transactions in parallel.
	GasUsed uint64

	// CallerNonce keeps track of the nonce of the caller who entered the callframe
	// (nonce of pranked caller, if pranked).
	CallerNonce uint64
}

// Host is an EVM executor that runs Forge scripts.
type Host struct {
	log      log.Logger
	af       *foundry.ArtifactsFS
	chainCfg *params.ChainConfig
	env      *vm.EVM

	state     *forking.ForkableState
	baseState *state.StateDB

	// only known contracts may utilize cheatcodes and logging
	allowedCheatcodes map[common.Address]struct{}

	cheatcodes *Precompile[*CheatCodesPrecompile]
	console    *Precompile[*ConsolePrecompile]

	precompiles map[common.Address]vm.PrecompiledContract

	callStack []*CallFrame

	// serializerStates are in-progress JSON payloads by name,
	// for the serializeX family of cheat codes, see:
	// https://book.getfoundry.sh/cheatcodes/serialize-json
	serializerStates map[string]json.RawMessage

	envVars map[string]string
	labels  map[common.Address]string

	// srcFS enables src-map loading;
	// this is a bit more expensive, but provides useful debug information.
	// src-maps are disabled if this is nil.
	srcFS   *foundry.SourceMapFS
	srcMaps map[common.Address]*srcmap.SourceMap

	onLabel []func(name string, addr common.Address)

	hooks *Hooks

	// isolateBroadcasts will flush the journal changes,
	// and prepare the ephemeral tx context again,
	// to make gas accounting of a broadcast sub-call more accurate.
	isolateBroadcasts bool

	// useCreate2Deployer uses the Create2Deployer for broadcasted
	// create2 calls.
	useCreate2Deployer bool
}

type HostOption func(h *Host)

type BroadcastHook func(broadcast Broadcast)

type Hooks struct {
	OnBroadcast BroadcastHook
	OnFork      ForkHook
}

func WithBroadcastHook(hook BroadcastHook) HostOption {
	return func(h *Host) {
		h.hooks.OnBroadcast = hook
	}
}

func WithForkHook(hook ForkHook) HostOption {
	return func(h *Host) {
		h.hooks.OnFork = hook
	}
}

// WithIsolatedBroadcasts makes each broadcast clean the context,
// by flushing the dirty storage changes, and preparing the ephemeral state again.
// This then produces more accurate gas estimation for broadcast calls.
// This is not compatible with state-snapshots: upon cleaning,
// it is assumed that the state has to never revert back, similar to the state-dump guarantees.
func WithIsolatedBroadcasts() HostOption {
	return func(h *Host) {
		h.isolateBroadcasts = true
	}
}

// WithCreate2Deployer proxies each CREATE2 call through the CREATE2 deployer
// contract located at 0x4e59b44847b379578588920cA78FbF26c0B4956C. This is the Arachnid
// Create2Deployer contract Forge uses. See https://github.com/Arachnid/deterministic-deployment-proxy
// for the implementation.
func WithCreate2Deployer() HostOption {
	return func(h *Host) {
		h.useCreate2Deployer = true
	}
}

// NewHost creates a Host that can load contracts from the given Artifacts FS,
// and with an EVM initialized to the given executionContext.
// Optionally src-map loading may be enabled, by providing a non-nil srcFS to read sources from.
func NewHost(
	logger log.Logger,
	fs *foundry.ArtifactsFS,
	srcFS *foundry.SourceMapFS,
	executionContext Context,
	options ...HostOption,
) *Host {
	h := &Host{
		log:              logger,
		af:               fs,
		serializerStates: make(map[string]json.RawMessage),
		envVars:          make(map[string]string),
		labels:           make(map[common.Address]string),
		precompiles:      make(map[common.Address]vm.PrecompiledContract),
		srcFS:            srcFS,
		srcMaps:          make(map[common.Address]*srcmap.SourceMap),
		hooks: &Hooks{
			OnBroadcast: func(broadcast Broadcast) {},
			OnFork: func(opts *ForkConfig) (forking.ForkSource, error) {
				return nil, errors.New("no forking configured")
			},
		},
		allowedCheatcodes: make(map[common.Address]struct{}),
	}

	for _, opt := range options {
		opt(h)
	}

	// Init a default chain config, with all the mainnet L1 forks activated
	h.chainCfg = &params.ChainConfig{
		ChainID: executionContext.ChainID,
		// Ethereum forks in proof-of-work era.
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		MuirGlacierBlock:    big.NewInt(0),
		BerlinBlock:         big.NewInt(0),
		LondonBlock:         big.NewInt(0),
		ArrowGlacierBlock:   big.NewInt(0),
		GrayGlacierBlock:    big.NewInt(0),
		MergeNetsplitBlock:  big.NewInt(0),
		// Ethereum forks in proof-of-stake era.
		TerminalTotalDifficulty:       big.NewInt(1),
		TerminalTotalDifficultyPassed: true,
		ShanghaiTime:                  new(uint64),
		CancunTime:                    new(uint64),
		PragueTime:                    nil,
		VerkleTime:                    nil,
		// OP-Stack forks are disabled, since we use this for L1.
		BedrockBlock: nil,
		RegolithTime: nil,
		CanyonTime:   nil,
		EcotoneTime:  nil,
		FjordTime:    nil,
		GraniteTime:  nil,
		HoloceneTime: nil,
		InteropTime:  nil,
		Optimism:     nil,
	}

	// Create an in-memory database, to host our temporary script state changes
	rawDB := rawdb.NewMemoryDatabase()
	stateDB := state.NewDatabase(triedb.NewDatabase(rawDB, &triedb.Config{
		Preimages: true, // To be able to iterate the state we need the Preimages
		IsVerkle:  false,
		HashDB:    hashdb.Defaults,
		PathDB:    nil,
	}), nil)
	var err error
	h.baseState, err = state.New(types.EmptyRootHash, stateDB)
	if err != nil {
		panic(fmt.Errorf("failed to create memory state db: %w", err))
	}
	h.state = forking.NewForkableState(h.baseState)

	// Initialize a block-context for the EVM to access environment variables.
	// The block context (after embedding inside of the EVM environment) may be mutated later.
	blockContext := vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash: func(n uint64) (out common.Hash) {
			binary.BigEndian.PutUint64(out[:8], n)
			return crypto.Keccak256Hash(out[:])
		},
		L1CostFunc:  nil,
		Coinbase:    executionContext.FeeRecipient,
		GasLimit:    executionContext.GasLimit,
		BlockNumber: new(big.Int).SetUint64(executionContext.BlockNum),
		Time:        executionContext.Timestamp,
		Difficulty:  nil, // not used anymore post-merge
		BaseFee:     big.NewInt(0),
		BlobBaseFee: big.NewInt(0),
		Random:      &executionContext.PrevRandao,
	}

	// Initialize a transaction-context for the EVM to access environment variables.
	// The transaction context (after embedding inside of the EVM environment) may be mutated later.
	txContext := vm.TxContext{
		Origin:       executionContext.Origin,
		GasPrice:     big.NewInt(0),
		BlobHashes:   executionContext.BlobHashes,
		BlobFeeCap:   big.NewInt(0),
		AccessEvents: state.NewAccessEvents(h.baseState.PointCache()),
	}

	// Hook up the Host to capture the EVM environment changes
	trHooks := &tracing.Hooks{
		OnEnter:         h.onEnter,
		OnExit:          h.onExit,
		OnOpcode:        h.onOpcode,
		OnFault:         h.onFault,
		OnStorageChange: h.onStorageChange,
		OnLog:           h.onLog,
	}

	// Configure the EVM without basefee (because scripting), our trace hooks, and runtime precompile overrides.
	vmCfg := vm.Config{
		NoBaseFee:           true,
		Tracer:              trHooks,
		PrecompileOverrides: h.getPrecompile,
		CallerOverride:      h.handleCaller,
	}

	h.env = vm.NewEVM(blockContext, txContext, h.state, h.chainCfg, vmCfg)

	return h
}

// AllowCheatcodes allows the given address to utilize the cheatcodes and logging precompiles
func (h *Host) AllowCheatcodes(addr common.Address) {
	h.log.Debug("Allowing cheatcodes", "address", addr, "label", h.labels[addr])
	h.allowedCheatcodes[addr] = struct{}{}
}

// AllowedCheatcodes returns whether the given address is allowed to use cheatcodes
func (h *Host) AllowedCheatcodes(addr common.Address) bool {
	_, ok := h.allowedCheatcodes[addr]
	return ok
}

// EnableCheats enables the Forge/HVM cheat-codes precompile and the Hardhat-style console2 precompile.
func (h *Host) EnableCheats() error {
	vmPrecompile, err := NewPrecompile[*CheatCodesPrecompile](&CheatCodesPrecompile{h: h})
	if err != nil {
		return fmt.Errorf("failed to init VM cheatcodes precompile: %w", err)
	}
	h.cheatcodes = vmPrecompile
	// Solidity does EXTCODESIZE checks on functions without return-data.
	// We need to insert some placeholder code to prevent it from aborting calls.
	// Emulates Forge script: https://github.com/foundry-rs/foundry/blob/224fe9cbf76084c176dabf7d3b2edab5df1ab818/crates/evm/evm/src/executors/mod.rs#L108
	h.state.SetCode(addresses.VMAddr, []byte{0x00})
	h.precompiles[addresses.VMAddr] = h.cheatcodes

	consolePrecompile, err := NewPrecompile[*ConsolePrecompile](&ConsolePrecompile{
		logger: h.log,
		sender: h.MsgSender,
	})
	if err != nil {
		return fmt.Errorf("failed to init console precompile: %w", err)
	}
	h.console = consolePrecompile
	h.precompiles[addresses.ConsoleAddr] = h.console
	// The Console precompile does not need bytecode,
	// calls all go through a console lib, which avoids the EXTCODESIZE.
	return nil
}

// prelude is a helper function to prepare the Host for a new call/create on the EVM environment.
func (h *Host) prelude(from common.Address, to *common.Address) {
	rules := h.chainCfg.Rules(h.env.Context.BlockNumber, true, h.env.Context.Time)
	activePrecompiles := vm.ActivePrecompiles(rules)
	h.env.StateDB.Prepare(rules, from, h.env.Context.Coinbase, to, activePrecompiles, nil)
}

// Call calls a contract in the EVM. The state changes persist.
func (h *Host) Call(from common.Address, to common.Address, input []byte, gas uint64, value *uint256.Int) (returnData []byte, leftOverGas uint64, err error) {
	h.prelude(from, &to)
	return h.env.Call(vm.AccountRef(from), to, input, gas, value)
}

// LoadContract loads the bytecode of a contract, and deploys it with regular CREATE.
func (h *Host) LoadContract(artifactName, contractName string) (common.Address, error) {
	artifact, err := h.af.ReadArtifact(artifactName, contractName)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to load %s / %s: %w", artifactName, contractName, err)
	}
	addr, err := h.Create(h.TxOrigin(), artifact.Bytecode.Object)
	if err != nil {
		return common.Address{}, err
	}
	h.RememberArtifact(addr, artifact, contractName)
	return addr, nil
}

// RememberArtifact registers an address as originating from a particular artifact.
// This register a source-map, if the Host is configured with a source-map FS.
func (h *Host) RememberArtifact(addr common.Address, artifact *foundry.Artifact, contract string) {
	if h.srcFS == nil {
		return
	}
	code := h.state.GetCode(addr)
	if !bytes.Equal(code, artifact.DeployedBytecode.Object) {
		h.log.Warn("src map warning: state bytecode does not match artifact deployed bytecode", "addr", addr)
	}

	srcMap, err := h.srcFS.SourceMap(artifact, contract)
	if err != nil {
		h.log.Warn("failed to load srcmap", "addr", addr, "err", err)
		return
	}
	h.srcMaps[addr] = srcMap
}

// Create a contract with unlimited gas, and 0 ETH value.
// This create function helps deploy contracts quickly for scripting etc.
func (h *Host) Create(from common.Address, initCode []byte) (common.Address, error) {
	h.prelude(from, nil)
	ret, addr, _, err := h.env.Create(vm.AccountRef(from),
		initCode, DefaultFoundryGasLimit, uint256.NewInt(0))
	if err != nil {
		retStr := fmt.Sprintf("%x", ret)
		if len(retStr) > 20 {
			retStr = retStr[:20] + "..."
		}
		return common.Address{}, fmt.Errorf("failed to create contract, return: %s, err: %w", retStr, err)
	}
	return addr, nil
}

// Wipe an account: removing the code, and setting address and balance to 0. This makes the account "empty".
// Note that storage is not removed.
func (h *Host) Wipe(addr common.Address) {
	if h.state.GetCodeSize(addr) > 0 {
		h.state.SetCode(addr, nil)
	}
	h.state.SetNonce(addr, 0)
	h.state.SetBalance(addr, uint256.NewInt(0), tracing.BalanceChangeUnspecified)
}

// SetNonce sets an account's nonce in state.
func (h *Host) SetNonce(addr common.Address, nonce uint64) {
	h.state.SetNonce(addr, nonce)
}

// GetNonce returs an account's nonce from state.
func (h *Host) GetNonce(addr common.Address) uint64 {
	return h.state.GetNonce(addr)
}

// ImportState imports a set of foundry.ForgeAllocs into the
// host's state database. It does not erase existing state
// when importing.
func (h *Host) ImportState(allocs *foundry.ForgeAllocs) {
	for addr, alloc := range allocs.Accounts {
		h.ImportAccount(addr, alloc)
	}
}

func (h *Host) ImportAccount(addr common.Address, account types.Account) {
	var balance *uint256.Int
	if account.Balance == nil {
		balance = uint256.NewInt(0)
	} else {
		balance = uint256.MustFromBig(account.Balance)
	}
	h.state.SetBalance(addr, balance, tracing.BalanceChangeUnspecified)
	h.state.SetNonce(addr, account.Nonce)
	h.state.SetCode(addr, account.Code)
	for key, value := range account.Storage {
		h.state.SetState(addr, key, value)
	}
}

// getPrecompile overrides any accounts during runtime, to insert special precompiles, if activated.
func (h *Host) getPrecompile(rules params.Rules, original vm.PrecompiledContract, addr common.Address) vm.PrecompiledContract {
	if p, ok := h.precompiles[addr]; ok {
		return &AccessControlledPrecompile{h: h, inner: p}
	}
	return original
}

// SetPrecompile inserts a precompile at the given address.
// If the precompile is nil, it removes the precompile override from that address, and wipes the account.
func (h *Host) SetPrecompile(addr common.Address, precompile vm.PrecompiledContract) {
	if precompile == nil {
		h.log.Debug("removing precompile", "addr", addr)
		delete(h.precompiles, addr)
		h.Wipe(addr)
		return
	}
	h.log.Debug("adding precompile", "addr", addr)
	h.precompiles[addr] = precompile
	// insert non-empty placeholder bytecode, so EXTCODESIZE checks pass
	h.state.SetCode(addr, []byte{0})
}

// HasPrecompileOverride inspects if there exists an active precompile-override at the given address.
func (h *Host) HasPrecompileOverride(addr common.Address) bool {
	_, ok := h.precompiles[addr]
	return ok
}

// GetCode returns the code of an account from the state.
func (h *Host) GetCode(addr common.Address) []byte {
	return h.state.GetCode(addr)
}

// onEnter is a trace-hook, which we use to apply changes to the state-DB, to simulate isolated broadcast calls,
// for better gas estimation of the exact broadcast call execution.
func (h *Host) onEnter(depth int, typ byte, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	if len(h.callStack) == 0 {
		return
	}
	parentCallFrame := h.callStack[len(h.callStack)-1]
	if parentCallFrame.Prank == nil {
		return
	}
	// sanity check our callframe is set up correctly
	if parentCallFrame.LastOp != vm.OpCode(typ) {
		panic(fmt.Errorf("parent call-frame has invalid last Op: %d", typ))
	}
	if !parentCallFrame.Prank.Broadcast {
		return
	}
	if to == addresses.VMAddr || to == addresses.ConsoleAddr { // no broadcasts to the cheatcode or console address
		return
	}

	// Bump nonce value, such that a broadcast Call or CREATE2 appears like a tx
	if parentCallFrame.LastOp == vm.CALL || parentCallFrame.LastOp == vm.CREATE2 {
		sender := parentCallFrame.Ctx.Address()
		if parentCallFrame.Prank.Sender != nil {
			sender = *parentCallFrame.Prank.Sender
		}
		h.state.SetNonce(sender, h.state.GetNonce(sender)+1)
	}

	if h.isolateBroadcasts {
		var dest *common.Address
		switch parentCallFrame.LastOp {
		case vm.CREATE, vm.CREATE2:
			dest = nil // no destination address to warm up
		case vm.CALL:
			dest = &to
		default:
			return
		}
		h.state.Finalise(true)
		// the prank msg.sender, if any, has already been applied to 'from' before onEnter
		h.prelude(from, dest)
	}
}

// onExit is a trace-hook, which we use to maintain an accurate view of functions, and log any revert warnings.
func (h *Host) onExit(depth int, output []byte, gasUsed uint64, err error, reverted bool) {
	// Note: onExit runs also when going deeper, exiting the context into a nested context.
	addr := h.SelfAddress()
	if reverted {
		h.LogCallStack()
		if msg, revertInspectErr := abi.UnpackRevert(output); revertInspectErr == nil {
			h.log.Warn("Revert", "addr", addr, "err", err, "revertMsg", msg, "depth", depth)
		} else {
			h.log.Warn("Revert", "addr", addr, "err", err, "revertData", hexutil.Bytes(output), "depth", depth)
		}
	}

	h.callStack[len(h.callStack)-1].GasUsed += gasUsed
	h.unwindCallstack(depth)
}

// onFault is a trace-hook, catches things more generic than regular EVM reverts.
func (h *Host) onFault(pc uint64, op byte, gas, cost uint64, scope tracing.OpContext, depth int, err error) {
	h.log.Warn("Fault", "addr", scope.Address(), "err", err, "depth", depth)
}

// unwindCallstack is a helper to remove call-stack entries.
func (h *Host) unwindCallstack(depth int) {
	// pop the callstack until the depth matches
	for len(h.callStack) > 0 && h.callStack[len(h.callStack)-1].Depth > depth {
		// unset the prank, if the parent call-frame had set up a prank that does not repeat
		if len(h.callStack) > 1 {
			parentCallFrame := h.callStack[len(h.callStack)-2]
			if parentCallFrame.Prank != nil {
				if parentCallFrame.Prank.Broadcast {
					if parentCallFrame.LastOp == vm.DELEGATECALL {
						h.log.Warn("Cannot broadcast a delegate-call. Ignoring broadcast hook.")
					} else if parentCallFrame.LastOp == vm.STATICCALL {
						h.log.Trace("Broadcast is active, ignoring static-call.")
					} else {
						currentCallFrame := h.callStack[len(h.callStack)-1]
						bcast := NewBroadcast(parentCallFrame, currentCallFrame)
						h.log.Debug(
							"calling broadcast hook",
							"from", bcast.From,
							"to", bcast.To,
							"input", bcast.Input,
							"value", bcast.Value,
							"type", bcast.Type,
						)
						h.hooks.OnBroadcast(bcast)
					}
				}

				// While going back to the parent, restore the tx.origin.
				// It will later be re-applied on sub-calls if the prank persists (if Repeat == true).
				if parentCallFrame.Prank.Origin != nil {
					h.env.TxContext.Origin = parentCallFrame.Prank.PrevOrigin
				}
				if !parentCallFrame.Prank.Repeat {
					parentCallFrame.Prank = nil
				}
			}
		}
		// Now pop the call-frame
		h.callStack[len(h.callStack)-1] = nil // don't hold on to the underlying call-frame resources
		h.callStack = h.callStack[:len(h.callStack)-1]
	}
}

// onOpcode is a trace-hook, used to maintain a view of the call-stack, and do any per op-code overrides.
func (h *Host) onOpcode(pc uint64, op byte, gas, cost uint64, scope tracing.OpContext, rData []byte, depth int, err error) {
	h.unwindCallstack(depth)
	scopeCtx := scope.(*vm.ScopeContext)
	if scopeCtx.Contract.IsDeployment {
		// If we are not yet allowed access to cheatcodes, but if the caller is,
		// and if this is a contract-creation, then we are automatically granted cheatcode access.
		if !h.AllowedCheatcodes(scopeCtx.Address()) && h.AllowedCheatcodes(scopeCtx.Caller()) {
			h.AllowCheatcodes(scopeCtx.Address())
		}
	}
	// Check if we are entering a new depth, add it to the call-stack if so.
	// We do this here, instead of onEnter, to capture an initialized scope.
	if len(h.callStack) == 0 || h.callStack[len(h.callStack)-1].Depth < depth {
		h.callStack = append(h.callStack, &CallFrame{
			Depth:       depth,
			LastOp:      vm.OpCode(op),
			LastPC:      pc,
			Ctx:         scopeCtx,
			CallerNonce: h.GetNonce(scopeCtx.Caller()),
		})
	}
	// Sanity check that top of the call-stack matches the scope context now
	if len(h.callStack) == 0 || h.callStack[len(h.callStack)-1].Ctx != scopeCtx {
		panic("scope context changed without call-frame pop/push")
	}
	cf := h.callStack[len(h.callStack)-1]
	if vm.OpCode(op) == vm.JUMPDEST { // remember the last PC before successful jump
		cf.LastJumps = append(cf.LastJumps, cf.LastPC)
		if len(cf.LastJumps) > jumpHistory {
			copy(cf.LastJumps[:], cf.LastJumps[len(cf.LastJumps)-jumpHistory:])
			cf.LastJumps = cf.LastJumps[:jumpHistory]
		}
	}
	cf.LastOp = vm.OpCode(op)
	cf.LastPC = pc
	if cf.LastOp == vm.CREATE2 {
		cf.LastCreate2Salt = scopeCtx.Stack.Back(3).Bytes32()
	}
}

// onStorageChange is a trace-hook to capture state changes
func (h *Host) onStorageChange(addr common.Address, slot common.Hash, prev, new common.Hash) {
	h.log.Debug("storage change", "addr", addr, "slot", slot, "prev_value", prev, "new_value", new)
	// future storage recording
}

// onLog is a trace-hook to capture log events
func (h *Host) onLog(ev *types.Log) {
	logger := h.log
	for i, topic := range ev.Topics {
		logger = logger.With(fmt.Sprintf("topic%d", i), topic)
	}
	logger.Debug("log event", "data", hexutil.Bytes(ev.Data))
	// future log recording
}

// CurrentCall returns the top of the callstack. Or zeroed if there was no call frame yet.
// If zeroed, the call-frame has a nil scope context.
func (h *Host) CurrentCall() *CallFrame {
	if len(h.callStack) == 0 {
		return &CallFrame{}
	}
	return h.callStack[len(h.callStack)-1]
}

// MsgSender returns the msg.sender of the current active EVM call-frame,
// or a zero address if there is no active call-frame.
func (h *Host) MsgSender() common.Address {
	cf := h.CurrentCall()
	if cf.Ctx == nil {
		return common.Address{}
	}
	return cf.Ctx.Caller()
}

// SelfAddress returns the current executing address of the current active EVM call-frame,
// or a zero address if there is no active call-frame.
func (h *Host) SelfAddress() common.Address {
	cf := h.CurrentCall()
	if cf.Ctx == nil {
		return common.Address{}
	}
	return cf.Ctx.Address()
}

func (h *Host) GetEnvVar(key string) (value string, ok bool) {
	value, ok = h.envVars[key]
	return
}

func (h *Host) SetEnvVar(key string, value string) {
	h.envVars[key] = value
}

// StateDump turns the current EVM state into a foundry-allocs dump
// (wrapping a geth Account allocs type). This is used to export the state.
// Note that upon dumping, the state-DB is committed and flushed.
// This affects any remaining self-destructs, as all accounts are flushed to persistent state.
// After flushing the EVM state also cannot revert to a previous snapshot state:
// the state should not be dumped within contract-execution that needs to revert.
func (h *Host) StateDump() (*foundry.ForgeAllocs, error) {
	if id, ok := h.state.ActiveFork(); ok {
		return nil, fmt.Errorf("cannot state-dump while fork %s is active", id)
	}
	baseState := h.baseState
	// We have to commit the existing state to the trie,
	// for all the state-changes to be captured by the trie iterator.
	root, err := baseState.Commit(h.env.Context.BlockNumber.Uint64(), true)
	if err != nil {
		return nil, fmt.Errorf("failed to commit state: %w", err)
	}
	// We need a state object around the state DB
	st, err := state.New(root, baseState.Database())
	if err != nil {
		return nil, fmt.Errorf("failed to create state object for state-dumping: %w", err)
	}
	// After Commit we cannot reuse the old State, so we update the host to use the new one
	h.baseState = st
	h.state.SubstituteBaseState(st)

	// We use the new state object for state-dumping & future state-access, wrapped around
	// the just committed trie that has all changes in it.
	// I.e. the trie is committed and ready to provide all data,
	// and the state is new and iterable, prepared specifically for FromState(state).
	var allocs foundry.ForgeAllocs
	allocs.FromState(st)

	// Sanity check we have no lingering scripts.
	for i := uint64(0); i <= allocs.Accounts[addresses.ScriptDeployer].Nonce; i++ {
		scriptAddr := crypto.CreateAddress(addresses.ScriptDeployer, i)
		h.log.Info("removing script from state-dump", "addr", scriptAddr, "label", h.labels[scriptAddr])
		delete(allocs.Accounts, scriptAddr)
	}

	// Clean out empty storage slots in the dump - this is necessary for compatibility
	// with the superchain registry.
	for _, account := range allocs.Accounts {
		toDelete := make([]common.Hash, 0)

		for slot, value := range account.Storage {
			if value == (common.Hash{}) {
				toDelete = append(toDelete, slot)
			}
		}

		for _, slot := range toDelete {
			delete(account.Storage, slot)
		}
	}

	// Remove the script deployer from the output
	delete(allocs.Accounts, addresses.ScriptDeployer)
	delete(allocs.Accounts, addresses.ForgeDeployer)

	// The cheatcodes VM has a placeholder bytecode,
	// because solidity checks if the code exists prior to regular EVM-calls to it.
	delete(allocs.Accounts, addresses.VMAddr)

	// Precompile overrides come with temporary state account placeholders. Ignore those.
	for addr := range h.precompiles {
		delete(allocs.Accounts, addr)
	}

	return &allocs, nil
}

func (h *Host) SetTxOrigin(addr common.Address) {
	h.env.TxContext.Origin = addr
}

func (h *Host) TxOrigin() common.Address {
	return h.env.TxContext.Origin
}

// ScriptBackendFn is a convenience method for scripts to attach to the Host.
// It return a function pre-configured with the given destination-address,
// to call the destination script.
func (h *Host) ScriptBackendFn(to common.Address) CallBackendFn {
	return func(data []byte) ([]byte, error) {
		ret, _, err := h.Call(h.env.TxContext.Origin, to, data, DefaultFoundryGasLimit, uint256.NewInt(0))
		return ret, err
	}
}

// EnforceMaxCodeSize configures the EVM to enforce (if true), or not enforce (if false),
// the maximum contract bytecode size.
func (h *Host) EnforceMaxCodeSize(v bool) {
	h.env.Config.NoMaxCodeSize = !v
}

// LogCallStack is a convenience method for debugging,
// to log details of each call-frame (from bottom to top) to the logger.
func (h *Host) LogCallStack() {
	for _, cf := range h.callStack {
		callsite := ""
		srcMap, ok := h.srcMaps[cf.Ctx.Address()]
		if !ok && cf.Ctx.Contract.CodeAddr != nil { // if delegate-call, we might know the implementation code.
			srcMap, ok = h.srcMaps[*cf.Ctx.Contract.CodeAddr]
		}
		if ok {
			callsite = srcMap.FormattedInfo(cf.LastPC)
			if callsite == "unknown:0:0" && len(cf.LastJumps) > 0 {
				callsite = srcMap.FormattedInfo(cf.LastJumps[len(cf.LastJumps)-1])
			}
		}
		input := cf.Ctx.CallInput()
		byte4 := ""
		if len(input) >= 4 {
			byte4 = fmt.Sprintf("0x%x", input[:4])
		}
		h.log.Debug("callframe input", "depth", cf.Depth, "input", hexutil.Bytes(input), "pc", cf.LastPC, "op", cf.LastOp)
		h.log.Warn("callframe", "depth", cf.Depth, "byte4", byte4,
			"addr", cf.Ctx.Address(), "callsite", callsite, "label", h.labels[cf.Ctx.Address()])
		if srcMap != nil {
			for _, jmpPC := range cf.LastJumps {
				h.log.Debug("recent jump", "depth", cf.Depth, "callsite", srcMap.FormattedInfo(jmpPC), "pc", jmpPC)
			}
		}
	}
}

// Label an address with a name, like the foundry vm.label cheatcode.
func (h *Host) Label(addr common.Address, label string) {
	h.log.Debug("labeling", "addr", addr, "label", label)
	h.labels[addr] = label

	for _, fn := range h.onLabel {
		fn(label, addr)
	}
}

// NewScriptAddress creates a new address for the ScriptDeployer account, and bumps the nonce.
func (h *Host) NewScriptAddress() common.Address {
	deployer := addresses.ScriptDeployer
	deployNonce := h.state.GetNonce(deployer)
	// compute address of script contract to be deployed
	addr := crypto.CreateAddress(deployer, deployNonce)
	h.state.SetNonce(deployer, deployNonce+1)
	return addr
}

func (h *Host) ChainID() *big.Int {
	return new(big.Int).Set(h.chainCfg.ChainID)
}

func (h *Host) Artifacts() *foundry.ArtifactsFS {
	return h.af
}

// RememberOnLabel links the contract source-code of srcFile upon a given label
func (h *Host) RememberOnLabel(label, srcFile, contract string) error {
	artifact, err := h.af.ReadArtifact(srcFile, contract)
	if err != nil {
		return fmt.Errorf("failed to read artifact %s (contract %s) for label %q", srcFile, contract, label)
	}
	h.onLabel = append(h.onLabel, func(v string, addr common.Address) {
		if label == v {
			h.RememberArtifact(addr, artifact, contract)
		}
	})
	return nil
}

func (h *Host) CreateSelectFork(opts ...ForkOption) (*big.Int, error) {
	src, err := h.onFork(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to setup fork source: %w", err)
	}
	id, err := h.state.CreateSelectFork(src)
	if err != nil {
		return nil, fmt.Errorf("failed to create-select fork: %w", err)
	}
	return id.U256().ToBig(), nil
}
