package contracts

import (
	"context"
	"math"
	"math/big"
	"testing"

	faultTypes "github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-challenger/game/types"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	batchingTest "github.com/ethereum-optimism/optimism/op-service/sources/batching/test"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

var (
	fdgAddr    = common.HexToAddress("0x24112842371dFC380576ebb09Ae16Cb6B6caD7CB")
	vmAddr     = common.HexToAddress("0x33332842371dFC380576ebb09Ae16Cb6B6c3333")
	oracleAddr = common.HexToAddress("0x44442842371dFC380576ebb09Ae16Cb6B6ca4444")
)

type disputeGameSetupFunc func(t *testing.T) (*batchingTest.AbiBasedRpc, *disputeGameContract)

func runCommonDisputeGameTests(t *testing.T, setup disputeGameSetupFunc) {
	tests := []struct {
		name   string
		method func(t *testing.T, setup disputeGameSetupFunc)
	}{
		{"SimpleGetters", runSimpleGettersTest},
		{"GetClaim", runGetClaimTest},
		{"GetAllClaims", runGetAllClaimsTest},
		{"CallResolveClaim", runCallResolveClaimTest},
		{"ResolveClaimTx", runResolveClaimTxTest},
		{"ResolveTx", runResolveTxTest},
		{"AttackTx", runAttackTxTest},
		{"DefendTx", runDefendTxTest},
		{"StepTx", runStepTxTest},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			test.method(t, setup)
		})
	}
}

func runSimpleGettersTest(t *testing.T, setup disputeGameSetupFunc) {
	tests := []struct {
		methodAlias string
		method      string
		args        []interface{}
		result      interface{}
		expected    interface{} // Defaults to expecting the same as result
		call        func(game *disputeGameContract) (any, error)
	}{
		{
			methodAlias: "status",
			method:      methodStatus,
			result:      types.GameStatusChallengerWon,
			call: func(game *disputeGameContract) (any, error) {
				return game.GetStatus(context.Background())
			},
		},
		{
			methodAlias: "gameDuration",
			method:      methodGameDuration,
			result:      uint64(5566),
			call: func(game *disputeGameContract) (any, error) {
				return game.GetGameDuration(context.Background())
			},
		},
		{
			methodAlias: "maxGameDepth",
			method:      methodMaxGameDepth,
			result:      big.NewInt(128),
			expected:    uint64(128),
			call: func(game *disputeGameContract) (any, error) {
				return game.GetMaxGameDepth(context.Background())
			},
		},
		{
			methodAlias: "absolutePrestate",
			method:      methodAbsolutePrestate,
			result:      common.Hash{0xab},
			call: func(game *disputeGameContract) (any, error) {
				return game.GetAbsolutePrestateHash(context.Background())
			},
		},
		{
			methodAlias: "claimCount",
			method:      methodClaimCount,
			result:      big.NewInt(9876),
			expected:    uint64(9876),
			call: func(game *disputeGameContract) (any, error) {
				return game.GetClaimCount(context.Background())
			},
		},
		{
			methodAlias: "l1Head",
			method:      methodL1Head,
			result:      common.Hash{0xdd, 0xbb},
			call: func(game *disputeGameContract) (any, error) {
				return game.GetL1Head(context.Background())
			},
		},
		{
			methodAlias: "resolve",
			method:      methodResolve,
			result:      types.GameStatusInProgress,
			call: func(game *disputeGameContract) (any, error) {
				return game.CallResolve(context.Background())
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.methodAlias, func(t *testing.T) {
			stubRpc, game := setup(t)
			stubRpc.SetResponse(fdgAddr, test.method, batching.BlockLatest, nil, []interface{}{test.result})
			status, err := test.call(game)
			require.NoError(t, err)
			expected := test.expected
			if expected == nil {
				expected = test.result
			}
			require.Equal(t, expected, status)
		})
	}
}

func runGetClaimTest(t *testing.T, setup disputeGameSetupFunc) {
	stubRpc, game := setup(t)
	idx := big.NewInt(2)
	parentIndex := uint32(1)
	counteredBy := common.Address{0x01}
	claimant := common.Address{0x02}
	bond := big.NewInt(5)
	value := common.Hash{0xab}
	position := big.NewInt(2)
	clock := big.NewInt(1234)
	stubRpc.SetResponse(fdgAddr, methodClaim, batching.BlockLatest, []interface{}{idx}, []interface{}{parentIndex, counteredBy, claimant, bond, value, position, clock})
	status, err := game.GetClaim(context.Background(), idx.Uint64())
	require.NoError(t, err)
	require.Equal(t, faultTypes.Claim{
		ClaimData: faultTypes.ClaimData{
			Value:    value,
			Bond:     bond,
			Position: faultTypes.NewPositionFromGIndex(position),
		},
		CounteredBy:         counteredBy,
		Claimant:            claimant,
		Clock:               1234,
		ContractIndex:       int(idx.Uint64()),
		ParentContractIndex: 1,
	}, status)
}

func runGetAllClaimsTest(t *testing.T, setup disputeGameSetupFunc) {
	stubRpc, game := setup(t)
	claim0 := faultTypes.Claim{
		ClaimData: faultTypes.ClaimData{
			Value:    common.Hash{0xaa},
			Bond:     big.NewInt(5),
			Position: faultTypes.NewPositionFromGIndex(big.NewInt(1)),
		},
		CounteredBy:         common.Address{0x01},
		Claimant:            common.Address{0x02},
		Clock:               1234,
		ContractIndex:       0,
		ParentContractIndex: math.MaxUint32,
	}
	claim1 := faultTypes.Claim{
		ClaimData: faultTypes.ClaimData{
			Value:    common.Hash{0xab},
			Bond:     big.NewInt(5),
			Position: faultTypes.NewPositionFromGIndex(big.NewInt(2)),
		},
		CounteredBy:         common.Address{0x02},
		Claimant:            common.Address{0x01},
		Clock:               4455,
		ContractIndex:       1,
		ParentContractIndex: 0,
	}
	claim2 := faultTypes.Claim{
		ClaimData: faultTypes.ClaimData{
			Value:    common.Hash{0xbb},
			Bond:     big.NewInt(5),
			Position: faultTypes.NewPositionFromGIndex(big.NewInt(6)),
		},
		CounteredBy:         common.Address{},
		Claimant:            common.Address{0x02},
		Clock:               7777,
		ContractIndex:       2,
		ParentContractIndex: 1,
	}
	expectedClaims := []faultTypes.Claim{claim0, claim1, claim2}
	stubRpc.SetResponse(fdgAddr, methodClaimCount, batching.BlockLatest, nil, []interface{}{big.NewInt(int64(len(expectedClaims)))})
	for _, claim := range expectedClaims {
		expectGetClaim(stubRpc, claim)
	}
	claims, err := game.GetAllClaims(context.Background())
	require.NoError(t, err)
	require.Equal(t, expectedClaims, claims)
}

func runCallResolveClaimTest(t *testing.T, setup disputeGameSetupFunc) {
	stubRpc, game := setup(t)
	stubRpc.SetResponse(fdgAddr, methodResolveClaim, batching.BlockLatest, []interface{}{big.NewInt(123)}, nil)
	err := game.CallResolveClaim(context.Background(), 123)
	require.NoError(t, err)
}

func runResolveClaimTxTest(t *testing.T, setup disputeGameSetupFunc) {
	stubRpc, game := setup(t)
	stubRpc.SetResponse(fdgAddr, methodResolveClaim, batching.BlockLatest, []interface{}{big.NewInt(123)}, nil)
	tx, err := game.ResolveClaimTx(123)
	require.NoError(t, err)
	stubRpc.VerifyTxCandidate(tx)
}

func runResolveTxTest(t *testing.T, setup disputeGameSetupFunc) {
	stubRpc, game := setup(t)
	stubRpc.SetResponse(fdgAddr, methodResolve, batching.BlockLatest, nil, nil)
	tx, err := game.ResolveTx()
	require.NoError(t, err)
	stubRpc.VerifyTxCandidate(tx)
}

func runAttackTxTest(t *testing.T, setup disputeGameSetupFunc) {
	stubRpc, game := setup(t)
	value := common.Hash{0xaa}
	stubRpc.SetResponse(fdgAddr, methodAttack, batching.BlockLatest, []interface{}{big.NewInt(111), value}, nil)
	tx, err := game.AttackTx(111, value)
	require.NoError(t, err)
	stubRpc.VerifyTxCandidate(tx)
}

func runDefendTxTest(t *testing.T, setup disputeGameSetupFunc) {
	stubRpc, game := setup(t)
	value := common.Hash{0xaa}
	stubRpc.SetResponse(fdgAddr, methodDefend, batching.BlockLatest, []interface{}{big.NewInt(111), value}, nil)
	tx, err := game.DefendTx(111, value)
	require.NoError(t, err)
	stubRpc.VerifyTxCandidate(tx)
}

func runStepTxTest(t *testing.T, setup disputeGameSetupFunc) {
	stubRpc, game := setup(t)
	stateData := []byte{1, 2, 3}
	proofData := []byte{4, 5, 6, 7, 8, 9}
	stubRpc.SetResponse(fdgAddr, methodStep, batching.BlockLatest, []interface{}{big.NewInt(111), true, stateData, proofData}, nil)
	tx, err := game.StepTx(111, true, stateData, proofData)
	require.NoError(t, err)
	stubRpc.VerifyTxCandidate(tx)
}

func expectGetClaim(stubRpc *batchingTest.AbiBasedRpc, claim faultTypes.Claim) {
	stubRpc.SetResponse(
		fdgAddr,
		methodClaim,
		batching.BlockLatest,
		[]interface{}{big.NewInt(int64(claim.ContractIndex))},
		[]interface{}{
			uint32(claim.ParentContractIndex),
			claim.CounteredBy,
			claim.Claimant,
			claim.Bond,
			claim.Value,
			claim.Position.ToGIndex(),
			big.NewInt(int64(claim.Clock)),
		})
}
