package contracts

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"slices"
	"testing"
	"time"

	contractMetrics "github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts/metrics"
	faultTypes "github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-challenger/game/types"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	batchingTest "github.com/ethereum-optimism/optimism/op-service/sources/batching/test"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/snapshots"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/require"
)

//go:embed abis/FaultDisputeGame-1.2.0.json
var faultDisputeGameAbi120 []byte

var (
	fdgAddr    = common.HexToAddress("0x24112842371dFC380576ebb09Ae16Cb6B6caD7CB")
	vmAddr     = common.HexToAddress("0x33332842371dFC380576ebb09Ae16Cb6B6c3333")
	oracleAddr = common.HexToAddress("0x44442842371dFC380576ebb09Ae16Cb6B6ca4444")
)

type contractVersion struct {
	version string
	loadAbi func() *abi.ABI
}

func (c contractVersion) Is(versions ...string) bool {
	return slices.Contains(versions, c.version)
}

const (
	vers080    = "0.8.0"
	vers0180   = "0.18.0"
	vers111    = "1.1.1"
	vers120    = "1.2.0"
	vers131    = "1.3.1"
	versLatest = "1.4.0"
)

var versions = []contractVersion{
	{
		version: vers080,
		loadAbi: func() *abi.ABI {
			return mustParseAbi(faultDisputeGameAbi020)
		},
	},
	{
		version: vers0180,
		loadAbi: func() *abi.ABI {
			return mustParseAbi(faultDisputeGameAbi0180)
		},
	},
	{
		version: vers111,
		loadAbi: func() *abi.ABI {
			return mustParseAbi(faultDisputeGameAbi111)
		},
	},
	{
		version: vers120,
		loadAbi: func() *abi.ABI {
			return mustParseAbi(faultDisputeGameAbi120)
		},
	},
	{
		version: vers131,
		loadAbi: func() *abi.ABI {
			return mustParseAbi(faultDisputeGameAbi131)
		},
	},
	{
		version: versLatest,
		loadAbi: snapshots.LoadFaultDisputeGameABI,
	},
}

func TestSimpleGetters(t *testing.T) {
	tests := []struct {
		methodAlias string
		method      string
		args        []interface{}
		result      interface{}
		expected    interface{} // Defaults to expecting the same as result
		call        func(game FaultDisputeGameContract) (any, error)
		applies     func(version contractVersion) bool
	}{
		{
			methodAlias: "status",
			method:      methodStatus,
			result:      types.GameStatusChallengerWon,
			call: func(game FaultDisputeGameContract) (any, error) {
				return game.GetStatus(context.Background())
			},
		},
		{
			methodAlias: "maxClockDuration",
			method:      methodMaxClockDuration,
			result:      uint64(5566),
			expected:    5566 * time.Second,
			call: func(game FaultDisputeGameContract) (any, error) {
				return game.GetMaxClockDuration(context.Background())
			},
			applies: func(version contractVersion) bool {
				return version.version != vers080
			},
		},
		{
			methodAlias: "gameDuration",
			method:      methodGameDuration,
			result:      uint64(5566) * 2,
			expected:    5566 * time.Second,
			call: func(game FaultDisputeGameContract) (any, error) {
				return game.GetMaxClockDuration(context.Background())
			},
			applies: func(version contractVersion) bool {
				return version.version == vers080
			},
		},
		{
			methodAlias: "maxGameDepth",
			method:      methodMaxGameDepth,
			result:      big.NewInt(128),
			expected:    faultTypes.Depth(128),
			call: func(game FaultDisputeGameContract) (any, error) {
				return game.GetMaxGameDepth(context.Background())
			},
		},
		{
			methodAlias: "absolutePrestate",
			method:      methodAbsolutePrestate,
			result:      common.Hash{0xab},
			call: func(game FaultDisputeGameContract) (any, error) {
				return game.GetAbsolutePrestateHash(context.Background())
			},
		},
		{
			methodAlias: "claimCount",
			method:      methodClaimCount,
			result:      big.NewInt(9876),
			expected:    uint64(9876),
			call: func(game FaultDisputeGameContract) (any, error) {
				return game.GetClaimCount(context.Background())
			},
		},
		{
			methodAlias: "l1Head",
			method:      methodL1Head,
			result:      common.Hash{0xdd, 0xbb},
			call: func(game FaultDisputeGameContract) (any, error) {
				return game.GetL1Head(context.Background())
			},
		},
		{
			methodAlias: "resolve",
			method:      methodResolve,
			result:      types.GameStatusInProgress,
			call: func(game FaultDisputeGameContract) (any, error) {
				return game.CallResolve(context.Background())
			},
		},
		{
			methodAlias: "resolvedAt",
			method:      methodResolvedAt,
			result:      uint64(240402),
			expected:    time.Unix(240402, 0),
			call: func(game FaultDisputeGameContract) (any, error) {
				return game.GetResolvedAt(context.Background(), rpcblock.Latest)
			},
		},
	}
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			for _, test := range tests {
				test := test
				t.Run(test.methodAlias, func(t *testing.T) {
					if test.applies != nil && !test.applies(version) {
						t.Skip("Skipping for this version")
					}
					stubRpc, game := setupFaultDisputeGameTest(t, version)
					stubRpc.SetResponse(fdgAddr, test.method, rpcblock.Latest, nil, []interface{}{test.result})
					status, err := test.call(game)
					require.NoError(t, err)
					expected := test.expected
					if expected == nil {
						expected = test.result
					}
					require.Equal(t, expected, status)
				})
			}
		})
	}
}

func TestBondDistributionMode(t *testing.T) {
	unsupportedVersions := []string{vers080, vers0180, vers111, vers120, vers131}
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			supported := !slices.Contains(unsupportedVersions, version.version)
			stubRpc, game := setupFaultDisputeGameTest(t, version)
			if supported {
				stubRpc.SetResponse(fdgAddr, methodBondDistributionMode, rpcblock.Latest, nil, []interface{}{faultTypes.RefundDistributionMode})
			}
			status, err := game.GetBondDistributionMode(context.Background(), rpcblock.Latest)
			require.NoError(t, err)
			if supported {
				require.Equal(t, faultTypes.RefundDistributionMode, status)
			} else {
				require.Equal(t, faultTypes.LegacyDistributionMode, status)
			}
		})
	}
}

func TestClock_EncodingDecoding(t *testing.T) {
	t.Run("DurationAndTimestamp", func(t *testing.T) {
		by := common.FromHex("00000000000000050000000000000002")
		encoded := new(big.Int).SetBytes(by)
		clock := decodeClock(encoded)
		require.Equal(t, 5*time.Second, clock.Duration)
		require.Equal(t, time.Unix(2, 0), clock.Timestamp)
		require.Equal(t, encoded, packClock(clock))
	})

	t.Run("ZeroDuration", func(t *testing.T) {
		by := common.FromHex("00000000000000000000000000000002")
		encoded := new(big.Int).SetBytes(by)
		clock := decodeClock(encoded)
		require.Equal(t, 0*time.Second, clock.Duration)
		require.Equal(t, time.Unix(2, 0), clock.Timestamp)
		require.Equal(t, encoded, packClock(clock))
	})

	t.Run("ZeroTimestamp", func(t *testing.T) {
		by := common.FromHex("00000000000000050000000000000000")
		encoded := new(big.Int).SetBytes(by)
		clock := decodeClock(encoded)
		require.Equal(t, 5*time.Second, clock.Duration)
		require.Equal(t, time.Unix(0, 0), clock.Timestamp)
		require.Equal(t, encoded, packClock(clock))
	})

	t.Run("ZeroClock", func(t *testing.T) {
		by := common.FromHex("00000000000000000000000000000000")
		encoded := new(big.Int).SetBytes(by)
		clock := decodeClock(encoded)
		require.Equal(t, 0*time.Second, clock.Duration)
		require.Equal(t, time.Unix(0, 0), clock.Timestamp)
		require.Equal(t, encoded.Uint64(), packClock(clock).Uint64())
	})
}

func TestGetOracleAddr(t *testing.T) {
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			stubRpc, game := setupFaultDisputeGameTest(t, version)
			stubRpc.SetResponse(fdgAddr, methodVM, rpcblock.Latest, nil, []interface{}{vmAddr})
			stubRpc.SetResponse(vmAddr, methodOracle, rpcblock.Latest, nil, []interface{}{oracleAddr})

			actual, err := game.GetOracle(context.Background())
			require.NoError(t, err)
			require.Equal(t, oracleAddr, actual.Addr())
		})
	}
}

func TestGetClaim(t *testing.T) {
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			stubRpc, game := setupFaultDisputeGameTest(t, version)
			idx := big.NewInt(2)
			parentIndex := uint32(1)
			counteredBy := common.Address{0x01}
			claimant := common.Address{0x02}
			bond := big.NewInt(5)
			value := common.Hash{0xab}
			position := big.NewInt(2)
			clock := big.NewInt(1234)
			stubRpc.SetResponse(fdgAddr, methodClaim, rpcblock.Latest, []interface{}{idx}, []interface{}{parentIndex, counteredBy, claimant, bond, value, position, clock})
			status, err := game.GetClaim(context.Background(), idx.Uint64())
			require.NoError(t, err)
			require.Equal(t, faultTypes.Claim{
				ClaimData: faultTypes.ClaimData{
					Value:    value,
					Position: faultTypes.NewPositionFromGIndex(position),
					Bond:     bond,
				},
				CounteredBy:         counteredBy,
				Claimant:            claimant,
				Clock:               decodeClock(big.NewInt(1234)),
				ContractIndex:       int(idx.Uint64()),
				ParentContractIndex: 1,
			}, status)
		})
	}
}

func TestGetAllClaims(t *testing.T) {
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			stubRpc, game := setupFaultDisputeGameTest(t, version)
			claim0 := faultTypes.Claim{
				ClaimData: faultTypes.ClaimData{
					Value:    common.Hash{0xaa},
					Position: faultTypes.NewPositionFromGIndex(big.NewInt(1)),
					Bond:     big.NewInt(5),
				},
				CounteredBy:         common.Address{0x01},
				Claimant:            common.Address{0x02},
				Clock:               decodeClock(big.NewInt(1234)),
				ContractIndex:       0,
				ParentContractIndex: math.MaxUint32,
			}
			claim1 := faultTypes.Claim{
				ClaimData: faultTypes.ClaimData{
					Value:    common.Hash{0xab},
					Position: faultTypes.NewPositionFromGIndex(big.NewInt(2)),
					Bond:     big.NewInt(5),
				},
				CounteredBy:         common.Address{0x02},
				Claimant:            common.Address{0x01},
				Clock:               decodeClock(big.NewInt(4455)),
				ContractIndex:       1,
				ParentContractIndex: 0,
			}
			claim2 := faultTypes.Claim{
				ClaimData: faultTypes.ClaimData{
					Value:    common.Hash{0xbb},
					Position: faultTypes.NewPositionFromGIndex(big.NewInt(6)),
					Bond:     big.NewInt(5),
				},
				Claimant:            common.Address{0x02},
				Clock:               decodeClock(big.NewInt(7777)),
				ContractIndex:       2,
				ParentContractIndex: 1,
			}
			expectedClaims := []faultTypes.Claim{claim0, claim1, claim2}
			block := rpcblock.ByNumber(42)
			stubRpc.SetResponse(fdgAddr, methodClaimCount, block, nil, []interface{}{big.NewInt(int64(len(expectedClaims)))})
			for _, claim := range expectedClaims {
				expectGetClaim(stubRpc, block, claim)
			}
			claims, err := game.GetAllClaims(context.Background(), block)
			require.NoError(t, err)
			require.Equal(t, expectedClaims, claims)
		})
	}
}

func TestGetBalance(t *testing.T) {
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			wethAddr := common.Address{0x11, 0x55, 0x66}
			balance := big.NewInt(9995877)
			delaySeconds := big.NewInt(429829)
			delay := time.Duration(delaySeconds.Int64()) * time.Second
			block := rpcblock.ByNumber(424)
			stubRpc, game := setupFaultDisputeGameTest(t, version)
			stubRpc.SetResponse(fdgAddr, methodWETH, block, nil, []interface{}{wethAddr})
			stubRpc.AddContract(wethAddr, snapshots.LoadDelayedWETHABI())
			stubRpc.SetResponse(wethAddr, methodDelay, block, nil, []interface{}{delaySeconds})
			stubRpc.AddExpectedCall(batchingTest.NewGetBalanceCall(wethAddr, block, balance))

			actualBalance, actualDelay, actualAddr, err := game.GetBalanceAndDelay(context.Background(), block)
			require.NoError(t, err)
			require.Equal(t, wethAddr, actualAddr)
			require.Equal(t, delay, actualDelay)
			require.Truef(t, balance.Cmp(actualBalance) == 0, "Expected balance %v but was %v", balance, actualBalance)
		})
	}
}

func TestCallResolveClaim(t *testing.T) {
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			stubRpc, game := setupFaultDisputeGameTest(t, version)
			if version.version == vers080 {
				stubRpc.SetResponse(fdgAddr, methodResolveClaim, rpcblock.Latest, []interface{}{big.NewInt(123)}, nil)
			} else {
				stubRpc.SetResponse(fdgAddr, methodResolveClaim, rpcblock.Latest, []interface{}{big.NewInt(123), maxChildChecks}, nil)
			}
			err := game.CallResolveClaim(context.Background(), 123)
			require.NoError(t, err)
		})
	}
}

func TestResolveClaimTxTest(t *testing.T) {
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			stubRpc, game := setupFaultDisputeGameTest(t, version)
			if version.version == vers080 {
				stubRpc.SetResponse(fdgAddr, methodResolveClaim, rpcblock.Latest, []interface{}{big.NewInt(123)}, nil)
			} else {
				stubRpc.SetResponse(fdgAddr, methodResolveClaim, rpcblock.Latest, []interface{}{big.NewInt(123), maxChildChecks}, nil)
			}
			tx, err := game.ResolveClaimTx(123)
			require.NoError(t, err)
			stubRpc.VerifyTxCandidate(tx)
		})
	}
}

func TestResolveTx(t *testing.T) {
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			stubRpc, game := setupFaultDisputeGameTest(t, version)
			stubRpc.SetResponse(fdgAddr, methodResolve, rpcblock.Latest, nil, nil)
			tx, err := game.ResolveTx()
			require.NoError(t, err)
			stubRpc.VerifyTxCandidate(tx)
		})
	}
}

func TestAttackTx(t *testing.T) {
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			stubRpc, game := setupFaultDisputeGameTest(t, version)
			bond := big.NewInt(1044)
			value := common.Hash{0xaa}
			parent := faultTypes.Claim{ClaimData: faultTypes.ClaimData{Value: common.Hash{0xbb}}, ContractIndex: 111}
			stubRpc.SetResponse(fdgAddr, methodRequiredBond, rpcblock.Latest, []interface{}{parent.Position.Attack().ToGIndex()}, []interface{}{bond})
			if version.Is(vers080, vers0180, vers111) {
				stubRpc.SetResponse(fdgAddr, methodAttack, rpcblock.Latest, []interface{}{big.NewInt(111), value}, nil)
			} else {
				stubRpc.SetResponse(fdgAddr, methodAttack, rpcblock.Latest, []interface{}{parent.Value, big.NewInt(111), value}, nil)
			}
			tx, err := game.AttackTx(context.Background(), parent, value)
			require.NoError(t, err)
			stubRpc.VerifyTxCandidate(tx)
			require.Equal(t, bond, tx.Value)
		})
	}
}

func TestDefendTx(t *testing.T) {
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			stubRpc, game := setupFaultDisputeGameTest(t, version)
			bond := big.NewInt(1044)
			value := common.Hash{0xaa}
			parent := faultTypes.Claim{ClaimData: faultTypes.ClaimData{Value: common.Hash{0xbb}}, ContractIndex: 111}
			stubRpc.SetResponse(fdgAddr, methodRequiredBond, rpcblock.Latest, []interface{}{parent.Position.Defend().ToGIndex()}, []interface{}{bond})
			if version.Is(vers080, vers0180, vers111) {
				stubRpc.SetResponse(fdgAddr, methodDefend, rpcblock.Latest, []interface{}{big.NewInt(111), value}, nil)
			} else {
				stubRpc.SetResponse(fdgAddr, methodDefend, rpcblock.Latest, []interface{}{parent.Value, big.NewInt(111), value}, nil)
			}
			tx, err := game.DefendTx(context.Background(), parent, value)
			require.NoError(t, err)
			stubRpc.VerifyTxCandidate(tx)
			require.Equal(t, bond, tx.Value)
		})
	}
}

func TestStepTx(t *testing.T) {
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			stubRpc, game := setupFaultDisputeGameTest(t, version)
			stateData := []byte{1, 2, 3}
			proofData := []byte{4, 5, 6, 7, 8, 9}
			stubRpc.SetResponse(fdgAddr, methodStep, rpcblock.Latest, []interface{}{big.NewInt(111), true, stateData, proofData}, nil)
			tx, err := game.StepTx(111, true, stateData, proofData)
			require.NoError(t, err)
			stubRpc.VerifyTxCandidate(tx)
		})
	}
}

func expectGetClaim(stubRpc *batchingTest.AbiBasedRpc, block rpcblock.Block, claim faultTypes.Claim) {
	stubRpc.SetResponse(
		fdgAddr,
		methodClaim,
		block,
		[]interface{}{big.NewInt(int64(claim.ContractIndex))},
		[]interface{}{
			uint32(claim.ParentContractIndex),
			claim.CounteredBy,
			claim.Claimant,
			claim.Bond,
			claim.Value,
			claim.Position.ToGIndex(),
			packClock(claim.Clock),
		})
}

func TestGetBlockRange(t *testing.T) {
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			stubRpc, contract := setupFaultDisputeGameTest(t, version)
			expectedStart := uint64(65)
			expectedEnd := uint64(102)
			stubRpc.SetResponse(fdgAddr, methodStartingBlockNumber, rpcblock.Latest, nil, []interface{}{new(big.Int).SetUint64(expectedStart)})
			stubRpc.SetResponse(fdgAddr, methodL2BlockNumber, rpcblock.Latest, nil, []interface{}{new(big.Int).SetUint64(expectedEnd)})
			start, end, err := contract.GetBlockRange(context.Background())
			require.NoError(t, err)
			require.Equal(t, expectedStart, start)
			require.Equal(t, expectedEnd, end)
		})
	}
}

func TestGetSplitDepth(t *testing.T) {
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			stubRpc, contract := setupFaultDisputeGameTest(t, version)
			expectedSplitDepth := faultTypes.Depth(15)
			stubRpc.SetResponse(fdgAddr, methodSplitDepth, rpcblock.Latest, nil, []interface{}{new(big.Int).SetUint64(uint64(expectedSplitDepth))})
			splitDepth, err := contract.GetSplitDepth(context.Background())
			require.NoError(t, err)
			require.Equal(t, expectedSplitDepth, splitDepth)
		})
	}
}

func TestGetGameMetadata(t *testing.T) {
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			stubRpc, contract := setupFaultDisputeGameTest(t, version)
			expectedL1Head := common.Hash{0x0a, 0x0b}
			expectedL2BlockNumber := uint64(123)
			expectedMaxClockDuration := uint64(456)
			expectedRootClaim := common.Hash{0x01, 0x02}
			expectedStatus := types.GameStatusChallengerWon
			expectedL2BlockNumberChallenged := true
			expectedL2BlockNumberChallenger := common.Address{0xee}
			block := rpcblock.ByNumber(889)
			stubRpc.SetResponse(fdgAddr, methodL1Head, block, nil, []interface{}{expectedL1Head})
			stubRpc.SetResponse(fdgAddr, methodL2BlockNumber, block, nil, []interface{}{new(big.Int).SetUint64(expectedL2BlockNumber)})
			stubRpc.SetResponse(fdgAddr, methodRootClaim, block, nil, []interface{}{expectedRootClaim})
			stubRpc.SetResponse(fdgAddr, methodStatus, block, nil, []interface{}{expectedStatus})
			if version.version == vers080 {
				expectedL2BlockNumberChallenged = false
				expectedL2BlockNumberChallenger = common.Address{}
				stubRpc.SetResponse(fdgAddr, methodGameDuration, block, nil, []interface{}{expectedMaxClockDuration * 2})
			} else if version.version == vers0180 {
				expectedL2BlockNumberChallenged = false
				expectedL2BlockNumberChallenger = common.Address{}
				stubRpc.SetResponse(fdgAddr, methodMaxClockDuration, block, nil, []interface{}{expectedMaxClockDuration})
			} else {
				stubRpc.SetResponse(fdgAddr, methodMaxClockDuration, block, nil, []interface{}{expectedMaxClockDuration})
				stubRpc.SetResponse(fdgAddr, methodL2BlockNumberChallenged, block, nil, []interface{}{expectedL2BlockNumberChallenged})
				stubRpc.SetResponse(fdgAddr, methodL2BlockNumberChallenger, block, nil, []interface{}{expectedL2BlockNumberChallenger})
			}
			actual, err := contract.GetGameMetadata(context.Background(), block)
			expected := GameMetadata{
				L1Head:                  expectedL1Head,
				L2BlockNum:              expectedL2BlockNumber,
				RootClaim:               expectedRootClaim,
				Status:                  expectedStatus,
				MaxClockDuration:        expectedMaxClockDuration,
				L2BlockNumberChallenged: expectedL2BlockNumberChallenged,
				L2BlockNumberChallenger: expectedL2BlockNumberChallenger,
			}
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		})
	}
}

func TestGetStartingRootHash(t *testing.T) {
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			stubRpc, contract := setupFaultDisputeGameTest(t, version)
			expectedOutputRoot := common.HexToHash("0x1234")
			stubRpc.SetResponse(fdgAddr, methodStartingRootHash, rpcblock.Latest, nil, []interface{}{expectedOutputRoot})
			startingOutputRoot, err := contract.GetStartingRootHash(context.Background())
			require.NoError(t, err)
			require.Equal(t, expectedOutputRoot, startingOutputRoot)
		})
	}
}

func TestFaultDisputeGame_UpdateOracleTx(t *testing.T) {
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			t.Run("Local", func(t *testing.T) {
				stubRpc, game := setupFaultDisputeGameTest(t, version)
				data := faultTypes.NewPreimageOracleData(common.Hash{0x01, 0xbc}.Bytes(), []byte{1, 2, 3, 4, 5, 6, 7}, 16)
				claimIdx := uint64(6)
				stubRpc.SetResponse(fdgAddr, methodAddLocalData, rpcblock.Latest, []interface{}{
					data.GetIdent(),
					new(big.Int).SetUint64(claimIdx),
					new(big.Int).SetUint64(uint64(data.OracleOffset)),
				}, nil)
				tx, err := game.UpdateOracleTx(context.Background(), claimIdx, data)
				require.NoError(t, err)
				stubRpc.VerifyTxCandidate(tx)
			})

			t.Run("Global", func(t *testing.T) {
				stubRpc, game := setupFaultDisputeGameTest(t, version)
				data := faultTypes.NewPreimageOracleData(common.Hash{0x02, 0xbc}.Bytes(), []byte{1, 2, 3, 4, 5, 6, 7, 9, 10, 11, 12, 13, 14, 15}, 16)
				claimIdx := uint64(6)
				stubRpc.SetResponse(fdgAddr, methodVM, rpcblock.Latest, nil, []interface{}{vmAddr})
				stubRpc.SetResponse(vmAddr, methodOracle, rpcblock.Latest, nil, []interface{}{oracleAddr})
				stubRpc.SetResponse(oracleAddr, methodLoadKeccak256PreimagePart, rpcblock.Latest, []interface{}{
					new(big.Int).SetUint64(uint64(data.OracleOffset)),
					data.GetPreimageWithoutSize(),
				}, nil)
				tx, err := game.UpdateOracleTx(context.Background(), claimIdx, data)
				require.NoError(t, err)
				stubRpc.VerifyTxCandidate(tx)
			})
		})
	}
}

func TestFaultDisputeGame_GetCredit(t *testing.T) {
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			stubRpc, game := setupFaultDisputeGameTest(t, version)
			addr := common.Address{0x01}
			expectedCredit := big.NewInt(4284)
			expectedStatus := types.GameStatusChallengerWon
			stubRpc.SetResponse(fdgAddr, methodCredit, rpcblock.Latest, []interface{}{addr}, []interface{}{expectedCredit})
			stubRpc.SetResponse(fdgAddr, methodStatus, rpcblock.Latest, nil, []interface{}{expectedStatus})

			actualCredit, actualStatus, err := game.GetCredit(context.Background(), addr)
			require.NoError(t, err)
			require.Equal(t, expectedCredit, actualCredit)
			require.Equal(t, expectedStatus, actualStatus)
		})
	}
}

func TestFaultDisputeGame_GetCredits(t *testing.T) {
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			stubRpc, game := setupFaultDisputeGameTest(t, version)

			block := rpcblock.ByNumber(482)

			addrs := []common.Address{{0x01}, {0x02}, {0x03}}
			expected := []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(0)}

			for i, addr := range addrs {
				stubRpc.SetResponse(fdgAddr, methodCredit, block, []interface{}{addr}, []interface{}{expected[i]})
			}

			actual, err := game.GetCredits(context.Background(), block, addrs...)
			require.NoError(t, err)
			require.Equal(t, len(expected), len(actual))
			for i := range expected {
				require.Zerof(t, expected[i].Cmp(actual[i]), "expected: %v actual: %v", expected[i], actual[i])
			}
		})
	}
}

func TestFaultDisputeGame_ClaimCreditTx(t *testing.T) {
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			t.Run("Success", func(t *testing.T) {
				stubRpc, game := setupFaultDisputeGameTest(t, version)
				addr := common.Address{0xaa}

				stubRpc.SetResponse(fdgAddr, methodClaimCredit, rpcblock.Latest, []interface{}{addr}, nil)
				tx, err := game.ClaimCreditTx(context.Background(), addr)
				require.NoError(t, err)
				stubRpc.VerifyTxCandidate(tx)
			})

			t.Run("SimulationFails", func(t *testing.T) {
				stubRpc, game := setupFaultDisputeGameTest(t, version)
				addr := common.Address{0xaa}

				stubRpc.SetError(fdgAddr, methodClaimCredit, rpcblock.Latest, []interface{}{addr}, errors.New("still locked"))
				tx, err := game.ClaimCreditTx(context.Background(), addr)
				require.ErrorIs(t, err, ErrSimulationFailed)
				require.Equal(t, txmgr.TxCandidate{}, tx)
			})
		})
	}
}

func TestFaultDisputeGame_IsResolved(t *testing.T) {
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			stubRpc, game := setupFaultDisputeGameTest(t, version)

			block := rpcblock.ByNumber(482)

			claims := []faultTypes.Claim{
				{ContractIndex: 1},
				{ContractIndex: 5},
				{ContractIndex: 13},
			}
			claimIdxs := []*big.Int{big.NewInt(1), big.NewInt(5), big.NewInt(13)}
			expected := []bool{false, true, true}

			if version.version == vers080 {
				claimCount := 14
				stubRpc.SetResponse(fdgAddr, methodClaimCount, block, nil, []interface{}{big.NewInt(int64(claimCount))})
				for idx := 0; idx < claimCount; idx++ {
					bond := big.NewInt(42)
					if idx == 5 || idx == 13 { // The two claims expected to be resolved
						bond = resolvedBondAmount
					}
					expectGetClaim(stubRpc, block, faultTypes.Claim{
						ContractIndex: idx,
						ClaimData: faultTypes.ClaimData{
							Bond: bond,
						},
					})
				}
			} else {
				for i, idx := range claimIdxs {
					stubRpc.SetResponse(fdgAddr, methodResolvedSubgames, block, []interface{}{idx}, []interface{}{expected[i]})
				}
			}

			actual, err := game.IsResolved(context.Background(), block, claims...)
			require.NoError(t, err)
			require.Equal(t, len(expected), len(actual))
			for i := range expected {
				require.Equal(t, expected[i], actual[i])
			}
		})
	}
}

func TestFaultDisputeGameContractLatest_IsL2BlockNumberChallenged(t *testing.T) {
	for _, version := range versions {
		version := version
		for _, expected := range []bool{true, false} {
			expected := expected
			t.Run(fmt.Sprintf("%v-%v", version.version, expected), func(t *testing.T) {
				block := rpcblock.ByHash(common.Hash{0x43})
				stubRpc, game := setupFaultDisputeGameTest(t, version)
				supportsL2BlockNumChallenge := version.version != vers080 && version.version != vers0180
				if supportsL2BlockNumChallenge {
					stubRpc.SetResponse(fdgAddr, methodL2BlockNumberChallenged, block, nil, []interface{}{expected})
				} else if expected {
					t.Skip("Can't have challenged L2 block number on this contract version")
				}
				challenged, err := game.IsL2BlockNumberChallenged(context.Background(), block)
				require.NoError(t, err)
				require.Equal(t, expected, challenged)
			})
		}
	}
}

func TestFaultDisputeGameContractLatest_ChallengeL2BlockNumberTx(t *testing.T) {
	for _, version := range versions {
		version := version
		t.Run(version.version, func(t *testing.T) {
			rng := rand.New(rand.NewSource(0))
			stubRpc, game := setupFaultDisputeGameTest(t, version)
			challenge := &faultTypes.InvalidL2BlockNumberChallenge{
				Output: &eth.OutputResponse{
					Version:               eth.Bytes32{},
					OutputRoot:            eth.Bytes32{0xaa},
					BlockRef:              eth.L2BlockRef{Hash: common.Hash{0xbb}},
					WithdrawalStorageRoot: common.Hash{0xcc},
					StateRoot:             common.Hash{0xdd},
				},
				Header: testutils.RandomHeader(rng),
			}
			supportsL2BlockNumChallenge := version.version != vers080 && version.version != vers0180
			if supportsL2BlockNumChallenge {
				headerRlp, err := rlp.EncodeToBytes(challenge.Header)
				require.NoError(t, err)
				stubRpc.SetResponse(fdgAddr, methodChallengeRootL2Block, rpcblock.Latest, []interface{}{
					outputRootProof{
						Version:                  challenge.Output.Version,
						StateRoot:                challenge.Output.StateRoot,
						MessagePasserStorageRoot: challenge.Output.WithdrawalStorageRoot,
						LatestBlockhash:          challenge.Output.BlockRef.Hash,
					},
					headerRlp,
				}, nil)
			}
			tx, err := game.ChallengeL2BlockNumberTx(challenge)
			if supportsL2BlockNumChallenge {
				require.NoError(t, err)
				stubRpc.VerifyTxCandidate(tx)
			} else {
				require.ErrorIs(t, err, ErrChallengeL2BlockNotSupported)
				require.Equal(t, txmgr.TxCandidate{}, tx)
			}
		})
	}
}

func setupFaultDisputeGameTest(t *testing.T, version contractVersion) (*batchingTest.AbiBasedRpc, FaultDisputeGameContract) {
	fdgAbi := version.loadAbi()

	vmAbi := snapshots.LoadMIPSABI()
	oracleAbi := snapshots.LoadPreimageOracleABI()

	stubRpc := batchingTest.NewAbiBasedRpc(t, fdgAddr, fdgAbi)
	stubRpc.AddContract(vmAddr, vmAbi)
	stubRpc.AddContract(oracleAddr, oracleAbi)
	caller := batching.NewMultiCaller(stubRpc, batching.DefaultBatchSize)

	stubRpc.SetResponse(fdgAddr, methodVersion, rpcblock.Latest, nil, []interface{}{version.version})
	stubRpc.SetResponse(oracleAddr, methodVersion, rpcblock.Latest, nil, []interface{}{oracleLatest})
	game, err := NewFaultDisputeGameContract(context.Background(), contractMetrics.NoopContractMetrics, fdgAddr, caller)
	require.NoError(t, err)
	return stubRpc, game
}
