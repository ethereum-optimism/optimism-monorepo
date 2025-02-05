package fault

import (
	"context"
	"testing"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	faultTypes "github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-challenger/game/registry"
	"github.com/ethereum-optimism/optimism/op-challenger/metrics"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/test"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/snapshots"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

func TestRegisterOracle_MissingGameImpl(t *testing.T) {
	gameFactoryAddr := common.Address{0xaa}
	rpc := test.NewAbiBasedRpc(t, gameFactoryAddr, snapshots.LoadDisputeGameFactoryABI())
	m := metrics.NoopMetrics
	caller := batching.NewMultiCaller(rpc, batching.DefaultBatchSize)
	gameFactory := contracts.NewDisputeGameFactoryContract(m, gameFactoryAddr, caller)

	logger, logs := testlog.CaptureLogger(t, log.LvlInfo)
	oracles := registry.NewOracleRegistry()
	gameType := faultTypes.CannonGameType

	rpc.SetResponse(gameFactoryAddr, "gameImpls", rpcblock.Latest, []interface{}{gameType}, []interface{}{common.Address{}})

	err := registerOracle(context.Background(), logger, m, oracles, gameFactory, caller, gameType)
	require.NoError(t, err)
	require.NotNil(t, logs.FindLog(
		testlog.NewMessageFilter("No game implementation set for game type"),
		testlog.NewAttributesFilter("gameType", gameType.String())))
}

func TestRegisterOracle_AddsOracle(t *testing.T) {
	gameFactoryAddr := common.Address{0xaa}
	gameImplAddr := common.Address{0xbb}
	vmAddr := common.Address{0xcc}
	oracleAddr := common.Address{0xdd}
	rpc := test.NewAbiBasedRpc(t, gameFactoryAddr, snapshots.LoadDisputeGameFactoryABI())
	rpc.AddContract(gameImplAddr, snapshots.LoadFaultDisputeGameABI())
	rpc.AddContract(vmAddr, snapshots.LoadMIPSABI())
	rpc.AddContract(oracleAddr, snapshots.LoadPreimageOracleABI())
	m := metrics.NoopMetrics
	caller := batching.NewMultiCaller(rpc, batching.DefaultBatchSize)
	gameFactory := contracts.NewDisputeGameFactoryContract(m, gameFactoryAddr, caller)

	logger := testlog.Logger(t, log.LvlInfo)
	oracles := registry.NewOracleRegistry()
	gameType := faultTypes.CannonGameType

	// Use the latest v1 of these contracts. Doesn't have to be an exact match for the version.
	rpc.SetResponse(gameImplAddr, "version", rpcblock.Latest, []interface{}{}, []interface{}{"1.100.0"})
	rpc.SetResponse(oracleAddr, "version", rpcblock.Latest, []interface{}{}, []interface{}{"1.100.0"})

	rpc.SetResponse(gameFactoryAddr, "gameImpls", rpcblock.Latest, []interface{}{gameType}, []interface{}{gameImplAddr})
	rpc.SetResponse(gameImplAddr, "vm", rpcblock.Latest, []interface{}{}, []interface{}{vmAddr})
	rpc.SetResponse(vmAddr, "oracle", rpcblock.Latest, []interface{}{}, []interface{}{oracleAddr})

	err := registerOracle(context.Background(), logger, m, oracles, gameFactory, caller, gameType)
	require.NoError(t, err)
	registered := oracles.Oracles()
	require.Len(t, registered, 1)
	require.Equal(t, oracleAddr, registered[0].Addr())
}
