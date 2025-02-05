package script

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script/addresses"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script/forking"
	"github.com/stretchr/testify/mock"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

//go:generate ./testdata/generate.sh

// MockRPCClient implements RPCClient interface for testing
type MockRPCClient struct {
	mock.Mock
}

func (m *MockRPCClient) CallContext(ctx context.Context, result any, method string, args ...any) error {
	return m.Called(ctx, result, method, args).Error(0)
}

func TestScript(t *testing.T) {
	logger, captLog := testlog.CaptureLogger(t, log.LevelInfo)
	af := foundry.OpenArtifactsDir("./testdata/test-artifacts")

	scriptContext := DefaultContext
	h := NewHost(logger, af, nil, scriptContext)
	require.NoError(t, h.EnableCheats())

	addr, err := h.LoadContract("ScriptExample.s.sol", "ScriptExample")
	require.NoError(t, err)
	h.AllowCheatcodes(addr)
	t.Logf("allowing %s to access cheatcodes", addr)

	h.SetEnvVar("EXAMPLE_BOOL", "true")
	input := bytes4("run()")
	returnData, _, err := h.Call(scriptContext.Sender, addr, input[:], DefaultFoundryGasLimit, uint256.NewInt(0))
	require.NoError(t, err, "call failed: %x", string(returnData))
	require.NotNil(t, captLog.FindLog(testlog.NewMessageFilter("sender nonce 1")))

	require.NoError(t, h.cheatcodes.Precompile.DumpState("noop"))
	// and a second time, to see if we can revisit the host state.
	require.NoError(t, h.cheatcodes.Precompile.DumpState("noop"))
}

func mustEncodeStringCalldata(t *testing.T, method, input string) []byte {
	packer, err := abi.JSON(strings.NewReader(fmt.Sprintf(`[{"type":"function","name":"%s","inputs":[{"type":"string","name":"input"}]}]`, method)))
	require.NoError(t, err)

	data, err := packer.Pack(method, input)
	require.NoError(t, err)
	return data
}

func TestScriptBroadcast(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	af := foundry.OpenArtifactsDir("./testdata/test-artifacts")

	fooBar, err := af.ReadArtifact("ScriptExample.s.sol", "FooBar")
	require.NoError(t, err)

	expectedInitCode := bytes.Clone(fooBar.Bytecode.Object)
	// Add the contract init argument we use in the script
	expectedInitCode = append(expectedInitCode, leftPad32(big.NewInt(1234).Bytes())...)
	salt := uint256.NewInt(42).Bytes32()

	senderAddr := common.HexToAddress("0x0000000000000000000000000000000000Badc0d")
	scriptAddr := common.HexToAddress("0x5b73c5498c1e3b4dba84de0f1833c4a029d90519")
	coffeeAddr := common.HexToAddress("0x0000000000000000000000000000000000C0FFEE")
	cafeAddr := common.HexToAddress("0xcafe")
	expBroadcasts := []Broadcast{
		{
			From:    scriptAddr,
			To:      scriptAddr,
			Input:   mustEncodeStringCalldata(t, "call1", "single_call1"),
			Value:   (*hexutil.U256)(uint256.NewInt(0)),
			GasUsed: 23421,
			Type:    BroadcastCall,
			Nonce:   1, // first action by script (script already has a nonce of 1)
		},
		{
			From:    coffeeAddr,
			To:      scriptAddr,
			Input:   mustEncodeStringCalldata(t, "call1", "startstop_call1"),
			Value:   (*hexutil.U256)(uint256.NewInt(0)),
			GasUsed: 1521,
			Type:    BroadcastCall,
			Nonce:   0, // first action by 0xc0ffee
		},
		{
			From:    coffeeAddr,
			To:      scriptAddr,
			Input:   mustEncodeStringCalldata(t, "call2", "startstop_call2"),
			Value:   (*hexutil.U256)(uint256.NewInt(0)),
			GasUsed: 1565,
			Type:    BroadcastCall,
			Nonce:   1, // second action of 0xc0ffee
		},
		{
			From:    common.HexToAddress("0x1234"),
			To:      scriptAddr,
			Input:   mustEncodeStringCalldata(t, "nested1", "nested"),
			Value:   (*hexutil.U256)(uint256.NewInt(0)),
			GasUsed: 2763,
			Type:    BroadcastCall,
			Nonce:   0, // first action of 0x1234
		},
		{
			From:    common.HexToAddress("0x123456"),
			To:      crypto.CreateAddress(common.HexToAddress("0x123456"), 0),
			Input:   expectedInitCode,
			Value:   (*hexutil.U256)(uint256.NewInt(0)),
			GasUsed: 39112,
			Type:    BroadcastCreate,
			Nonce:   0, // first action of 0x123456
		},
		{
			From:    DeterministicDeployerAddress,
			To:      crypto.CreateAddress2(DeterministicDeployerAddress, salt, crypto.Keccak256(expectedInitCode)),
			Input:   expectedInitCode,
			Value:   (*hexutil.U256)(uint256.NewInt(0)),
			Type:    BroadcastCreate2,
			GasUsed: 39112,
			Salt:    salt,
			Nonce:   0, // first action of 0xcafe
		},
		{
			From:    scriptAddr,
			To:      crypto.CreateAddress(scriptAddr, 2),
			Input:   expectedInitCode,
			Value:   (*hexutil.U256)(uint256.NewInt(0)),
			GasUsed: 39112,
			Type:    BroadcastCreate,
			Nonce:   2, // second action, on top of starting at 1.
		},
	}

	var broadcasts []Broadcast
	hook := func(broadcast Broadcast) {
		broadcasts = append(broadcasts, broadcast)
	}
	h := NewHost(logger, af, nil, DefaultContext, WithBroadcastHook(hook), WithCreate2Deployer())
	require.NoError(t, h.EnableCheats())

	addr, err := h.LoadContract("ScriptExample.s.sol", "ScriptExample")
	require.NoError(t, err)
	h.AllowCheatcodes(addr)

	input := bytes4("runBroadcast()")
	returnData, _, err := h.Call(senderAddr, addr, input[:], DefaultFoundryGasLimit, uint256.NewInt(0))
	require.NoError(t, err, "call failed: %x", string(returnData))

	expected, err := json.MarshalIndent(expBroadcasts, "  ", "  ")
	require.NoError(t, err)
	got, err := json.MarshalIndent(broadcasts, "  ", "  ")
	require.NoError(t, err)
	require.Equal(t, string(expected), string(got))

	// Assert that the nonces for accounts participating in the
	// broadcast increase. The scriptAddr check is set to 3 to
	// account for the initial deployment of the contract and
	// two additional calls.
	require.EqualValues(t, 0, h.GetNonce(senderAddr))
	require.EqualValues(t, 3, h.GetNonce(scriptAddr))
	require.EqualValues(t, 2, h.GetNonce(coffeeAddr))
	// This is one because we still need to bump the nonce of the
	// address that will perform the send to the Create2Deployer.
	require.EqualValues(t, 1, h.GetNonce(cafeAddr))
}

func TestScriptStateDump(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	af := foundry.OpenArtifactsDir("./testdata/test-artifacts")

	h := NewHost(logger, af, nil, DefaultContext)
	require.NoError(t, h.EnableCheats())

	addr, err := h.LoadContract("ScriptExample.s.sol", "ScriptExample")
	require.NoError(t, err)
	h.AllowCheatcodes(addr)

	counterStorageSlot := common.Hash{}

	dump, err := h.StateDump()
	require.NoError(t, err, "dump 1")
	require.Contains(t, dump.Accounts, addr, "has contract")
	require.NotContains(t, dump.Accounts[addr].Storage, counterStorageSlot, "not counted yet")

	dat := mustEncodeStringCalldata(t, "call1", "call A")
	returnData, _, err := h.Call(addresses.DefaultSenderAddr, addr, dat, DefaultFoundryGasLimit, uint256.NewInt(0))
	require.NoError(t, err, "call A failed: %x", string(returnData))

	dump, err = h.StateDump()
	require.NoError(t, err, "dump 2")
	require.Contains(t, dump.Accounts, addr, "has contract")
	require.Equal(t, dump.Accounts[addr].Storage[counterStorageSlot], common.Hash{31: 1}, "counted to 1")

	dat = mustEncodeStringCalldata(t, "call1", "call B")
	returnData, _, err = h.Call(addresses.DefaultSenderAddr, addr, dat, DefaultFoundryGasLimit, uint256.NewInt(0))
	require.NoError(t, err, "call B failed: %x", string(returnData))

	dump, err = h.StateDump()
	require.NoError(t, err, "dump 3")
	require.Contains(t, dump.Accounts, addr, "has contract")
	require.Equal(t, dump.Accounts[addr].Storage[counterStorageSlot], common.Hash{31: 2}, "counted to 2")
}

type forkConfig struct {
	blockNum     uint64
	stateRoot    common.Hash
	blockHash    common.Hash
	nonce        uint64
	storageValue *big.Int
	code         []byte
	balance      uint64
}

func TestForkingScript(t *testing.T) {
	logger := testlog.Logger(t, log.LevelInfo)
	af := foundry.OpenArtifactsDir("./testdata/test-artifacts")

	forkedContract, err := af.ReadArtifact("ScriptExample.s.sol", "ForkedContract")
	require.NoError(t, err)
	code := forkedContract.DeployedBytecode.Object

	fork1Config := forkConfig{
		blockNum:     12345,
		stateRoot:    common.HexToHash("0x1111"),
		blockHash:    common.HexToHash("0x2222"),
		nonce:        12345,
		storageValue: big.NewInt(1),
		code:         code,
		balance:      1,
	}

	fork2Config := forkConfig{
		blockNum:     23456,
		stateRoot:    common.HexToHash("0x3333"),
		blockHash:    common.HexToHash("0x4444"),
		nonce:        23456,
		storageValue: big.NewInt(2),
		code:         code,
		balance:      2,
	}

	// Map of URL/alias to RPC client
	rpcClients := map[string]*MockRPCClient{
		"fork1": setupMockRPC(fork1Config),
		"fork2": setupMockRPC(fork2Config),
	}
	forkHook := func(opts *ForkConfig) (forking.ForkSource, error) {
		client, ok := rpcClients[opts.URLOrAlias]
		if !ok {
			return nil, fmt.Errorf("unknown fork URL/alias: %s", opts.URLOrAlias)
		}
		return forking.RPCSourceByNumber(opts.URLOrAlias, client, *opts.BlockNumber)
	}

	scriptContext := DefaultContext
	h := NewHost(logger, af, nil, scriptContext, WithForkHook(forkHook))
	require.NoError(t, h.EnableCheats())

	addr, err := h.LoadContract("ScriptExample.s.sol", "ForkTester")
	require.NoError(t, err)
	h.AllowCheatcodes(addr)
	// Make this script excluded so it doesn't call the fork RPC.
	h.state.MakeExcluded(addr)
	t.Logf("allowing %s to access cheatcodes", addr)

	input := bytes4("run()")
	returnData, _, err := h.Call(scriptContext.Sender, addr, input[:], DefaultFoundryGasLimit, uint256.NewInt(0))
	require.NoError(t, err, "call failed: %x", string(returnData))

	for _, client := range rpcClients {
		client.AssertExpectations(t)
	}
}

// setupMockRPC creates a mock RPC client with the specified fork configuration
func setupMockRPC(config forkConfig) *MockRPCClient {
	mockRPC := new(MockRPCClient)
	testAddr := common.HexToAddress("0x1234")

	forkArgs := []any{testAddr, config.blockHash}

	// Mock block header
	mockRPC.On("CallContext", mock.Anything, mock.AnythingOfType("**forking.Header"),
		"eth_getBlockByNumber", []any{hexutil.Uint64(config.blockNum), false}).
		Run(func(args mock.Arguments) {
			result := args.Get(1).(**forking.Header)
			*result = &forking.Header{
				StateRoot: config.stateRoot,
				BlockHash: config.blockHash,
			}
		}).Return(nil).Once()

	mockRPC.On("CallContext", mock.Anything, mock.AnythingOfType("*hexutil.Uint64"),
		"eth_getTransactionCount", forkArgs).
		Run(func(args mock.Arguments) {
			result := args.Get(1).(*hexutil.Uint64)
			*result = hexutil.Uint64(config.nonce)
		}).Return(nil)

	// Mock balance
	mockRPC.On("CallContext", mock.Anything, mock.AnythingOfType("*hexutil.U256"),
		"eth_getBalance", forkArgs).
		Run(func(args mock.Arguments) {
			result := args.Get(1).(*hexutil.U256)
			*result = hexutil.U256(*uint256.NewInt(config.balance))
		}).Return(nil)

	// Mock contract code
	mockRPC.On("CallContext", mock.Anything, mock.AnythingOfType("*hexutil.Bytes"),
		"eth_getCode", forkArgs).
		Run(func(args mock.Arguments) {
			result := args.Get(1).(*hexutil.Bytes)
			*result = config.code
		}).Return(nil)

	// Mock storage value
	mockRPC.On("CallContext", mock.Anything, mock.AnythingOfType("*common.Hash"),
		"eth_getStorageAt", []any{testAddr, common.Hash{}, config.blockHash}).
		Run(func(args mock.Arguments) {
			result := args.Get(1).(*common.Hash)
			*result = common.BigToHash(config.storageValue)
		}).Return(nil)

	return mockRPC
}
