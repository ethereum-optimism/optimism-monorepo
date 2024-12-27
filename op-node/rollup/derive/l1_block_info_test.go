package derive

import (
	crand "crypto/rand"
	"math/big"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

var (
	MockDepositContractAddr               = common.HexToAddress("0xdeadbeefdeadbeefdeadbeefdeadbeef00000000")
	_                       eth.BlockInfo = (*testutils.MockBlockInfo)(nil)
)

type infoTest struct {
	name    string
	mkInfo  func(rng *rand.Rand) *testutils.MockBlockInfo
	mkL1Cfg func(rng *rand.Rand, l1Info eth.BlockInfo) eth.SystemConfig
	seqNr   func(rng *rand.Rand) uint64
}

func randomL1Cfg(rng *rand.Rand, l1Info eth.BlockInfo) eth.SystemConfig {
	return eth.SystemConfig{
		BatcherAddr: testutils.RandomAddress(rng),
		Overhead:    [32]byte{},
		Scalar:      [32]byte{},
		GasLimit:    1234567,
	}
}

func TestParseL1InfoDepositTxData(t *testing.T) {
	randomSeqNr := func(rng *rand.Rand) uint64 {
		return rng.Uint64()
	}
	// Go 1.18 will have native fuzzing for us to use, until then, we cover just the below cases
	cases := []infoTest{
		{"random", testutils.MakeBlockInfo(nil), randomL1Cfg, randomSeqNr},
		{"zero basefee", testutils.MakeBlockInfo(func(l *testutils.MockBlockInfo) {
			l.InfoBaseFee = new(big.Int)
		}), randomL1Cfg, randomSeqNr},
		{"zero time", testutils.MakeBlockInfo(func(l *testutils.MockBlockInfo) {
			l.InfoTime = 0
		}), randomL1Cfg, randomSeqNr},
		{"zero num", testutils.MakeBlockInfo(func(l *testutils.MockBlockInfo) {
			l.InfoNum = 0
		}), randomL1Cfg, randomSeqNr},
		{"zero seq", testutils.MakeBlockInfo(nil), randomL1Cfg, func(rng *rand.Rand) uint64 {
			return 0
		}},
		{"all zero", func(rng *rand.Rand) *testutils.MockBlockInfo {
			return &testutils.MockBlockInfo{InfoBaseFee: new(big.Int)}
		}, randomL1Cfg, func(rng *rand.Rand) uint64 {
			return 0
		}},
	}
	assertInfo := func(rollupCfg *rollup.Config, info *testutils.MockBlockInfo, depTx *types.DepositTx, l2BlockTime, seqNr uint64, l1Cfg eth.SystemConfig) {
		res, err := L1BlockInfoFromBytes(rollupCfg, l2BlockTime, depTx.Data)
		require.NoError(t, err, "expected valid deposit info")
		assert.Equal(t, res.Number, info.NumberU64())
		assert.Equal(t, res.Time, info.Time())
		assert.True(t, res.BaseFee.Sign() >= 0)
		assert.Equal(t, res.BaseFee.Bytes(), info.BaseFee().Bytes())
		assert.Equal(t, res.BlockHash, info.Hash())
		assert.Equal(t, res.SequenceNumber, seqNr)
		assert.Equal(t, res.BatcherAddr, l1Cfg.BatcherAddr)
		assert.Equal(t, res.L1FeeOverhead, l1Cfg.Overhead)
		assert.Equal(t, res.L1FeeScalar, l1Cfg.Scalar)
	}
	var rollupCfg rollup.Config
	for i, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			rng := rand.New(rand.NewSource(int64(1234 + i)))
			info := testCase.mkInfo(rng)
			l1Cfg := testCase.mkL1Cfg(rng, info)
			seqNr := testCase.seqNr(rng)
			depTx, err := L1InfoDeposit(&rollupCfg, l1Cfg, seqNr, info, 0)
			require.NoError(t, err)
			assertInfo(&rollupCfg, info, depTx, 0, seqNr, l1Cfg)
		})
	}
	t.Run("no data", func(t *testing.T) {
		_, err := L1BlockInfoFromBytes(&rollupCfg, 0, nil)
		assert.Error(t, err)
	})
	t.Run("not enough data", func(t *testing.T) {
		_, err := L1BlockInfoFromBytes(&rollupCfg, 0, []byte{1, 2, 3, 4})
		assert.Error(t, err)
	})
	t.Run("too much data", func(t *testing.T) {
		_, err := L1BlockInfoFromBytes(&rollupCfg, 0, make([]byte, 4+32+32+32+32+32+1))
		assert.Error(t, err)
	})
	t.Run("invalid selector", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1234))
		info := testutils.MakeBlockInfo(nil)(rng)
		depTx, err := L1InfoDeposit(&rollupCfg, randomL1Cfg(rng, info), randomSeqNr(rng), info, 0)
		require.NoError(t, err)
		_, err = crand.Read(depTx.Data[0:4])
		require.NoError(t, err)
		_, err = L1BlockInfoFromBytes(&rollupCfg, info.Time(), depTx.Data)
		require.ErrorContains(t, err, "function signature")
	})
	t.Run("regolith", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1234))
		info := testutils.MakeBlockInfo(nil)(rng)
		rollupCfg := rollup.Config{}
		rollupCfg.ActivateAtGenesis(rollup.Regolith)
		l1Cfg := randomL1Cfg(rng, info)
		seqNr := randomSeqNr(rng)
		depTx, err := L1InfoDeposit(&rollupCfg, l1Cfg, seqNr, info, 0)
		require.NoError(t, err)
		require.False(t, depTx.IsSystemTransaction)
		require.Equal(t, depTx.Gas, uint64(RegolithSystemTxGas))
		assertInfo(&rollupCfg, info, depTx, 0, seqNr, l1Cfg)
	})
	t.Run("ecotone", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1234))
		info := testutils.MakeBlockInfo(nil)(rng)
		rollupCfg := rollup.Config{BlockTime: 2, Genesis: rollup.Genesis{L2Time: 1000}}
		rollupCfg.ActivateAtGenesis(rollup.Ecotone)
		// run 1 block after ecotone transition
		timestamp := rollupCfg.Genesis.L2Time + rollupCfg.BlockTime
		l1Cfg := randomL1Cfg(rng, info)
		seqNr := randomSeqNr(rng)
		depTx, err := L1InfoDeposit(&rollupCfg, l1Cfg, seqNr, info, timestamp)
		require.NoError(t, err)
		require.False(t, depTx.IsSystemTransaction)
		require.Equal(t, depTx.Gas, uint64(RegolithSystemTxGas))
		require.Equal(t, L1InfoEcotoneLen, len(depTx.Data))
		assertInfo(&rollupCfg, info, depTx, timestamp, seqNr, l1Cfg)
	})
	t.Run("activation-block ecotone", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1234))
		info := testutils.MakeBlockInfo(nil)(rng)
		rollupCfg := rollup.Config{BlockTime: 2, Genesis: rollup.Genesis{L2Time: 1000}}
		rollupCfg.ActivateAtGenesis(rollup.Delta)
		ecotoneTime := rollupCfg.Genesis.L2Time + rollupCfg.BlockTime // activate ecotone just after genesis
		rollupCfg.EcotoneTime = &ecotoneTime
		l1Cfg := randomL1Cfg(rng, info)
		seqNr := randomSeqNr(rng)
		depTx, err := L1InfoDeposit(&rollupCfg, l1Cfg, seqNr, info, ecotoneTime)
		require.NoError(t, err)
		require.False(t, depTx.IsSystemTransaction)
		require.Equal(t, depTx.Gas, uint64(RegolithSystemTxGas))
		require.Equal(t, L1InfoBedrockLen, len(depTx.Data))
		assertInfo(&rollupCfg, info, depTx, ecotoneTime, seqNr, l1Cfg)
	})
	t.Run("genesis-block ecotone", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1234))
		info := testutils.MakeBlockInfo(nil)(rng)
		rollupCfg := rollup.Config{BlockTime: 2, Genesis: rollup.Genesis{L2Time: 1000}}
		rollupCfg.ActivateAtGenesis(rollup.Ecotone)
		l1Cfg := randomL1Cfg(rng, info)
		seqNr := randomSeqNr(rng)
		depTx, err := L1InfoDeposit(&rollupCfg, l1Cfg, seqNr, info, rollupCfg.Genesis.L2Time)
		require.NoError(t, err)
		require.False(t, depTx.IsSystemTransaction)
		require.Equal(t, depTx.Gas, uint64(RegolithSystemTxGas))
		require.Equal(t, L1InfoEcotoneLen, len(depTx.Data))
		assertInfo(&rollupCfg, info, depTx, rollupCfg.Genesis.L2Time, seqNr, l1Cfg)
	})
	t.Run("isthmus", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1234))
		info := testutils.MakeBlockInfo(nil)(rng)
		rollupCfg := rollup.Config{BlockTime: 2, Genesis: rollup.Genesis{L2Time: 1000}}
		rollupCfg.ActivateAtGenesis(rollup.Isthmus)
		// run 1 block after isthmus transition
		timestamp := rollupCfg.Genesis.L2Time + rollupCfg.BlockTime
		l1Cfg := randomL1Cfg(rng, info)
		seqNr := randomSeqNr(rng)
		depTx, err := L1InfoDeposit(&rollupCfg, l1Cfg, seqNr, info, timestamp)
		require.NoError(t, err)
		require.False(t, depTx.IsSystemTransaction)
		require.Equal(t, depTx.Gas, uint64(RegolithSystemTxGas))
		require.Equal(t, L1InfoIsthmusLen, len(depTx.Data), "the length is same in isthmus")
		require.Equal(t, L1InfoFuncIsthmusBytes4, depTx.Data[:4], "upgrade is active, need isthmus signature")
		assertInfo(&rollupCfg, info, depTx, timestamp, seqNr, l1Cfg)
	})
	t.Run("activation-block isthmus", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1234))
		info := testutils.MakeBlockInfo(nil)(rng)
		rollupCfg := rollup.Config{BlockTime: 2, Genesis: rollup.Genesis{L2Time: 1000}}
		rollupCfg.ActivateAtGenesis(rollup.Holocene)
		isthmusTime := rollupCfg.Genesis.L2Time + rollupCfg.BlockTime // activate isthmus just after genesis
		rollupCfg.IsthmusTime = &isthmusTime
		l1Cfg := randomL1Cfg(rng, info)
		seqNr := randomSeqNr(rng)
		depTx, err := L1InfoDeposit(&rollupCfg, l1Cfg, seqNr, info, isthmusTime)
		require.NoError(t, err)
		require.False(t, depTx.IsSystemTransaction)
		require.Equal(t, depTx.Gas, uint64(RegolithSystemTxGas))
		// Isthmus activates, but ecotone L1 info is still used at this upgrade block
		require.Equal(t, L1InfoEcotoneLen, len(depTx.Data))
		require.Equal(t, L1InfoFuncEcotoneBytes4, depTx.Data[:4])
		assertInfo(&rollupCfg, info, depTx, isthmusTime, seqNr, l1Cfg)
	})
	t.Run("genesis-block isthmus", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1234))
		info := testutils.MakeBlockInfo(nil)(rng)
		rollupCfg := rollup.Config{BlockTime: 2, Genesis: rollup.Genesis{L2Time: 1000}}
		rollupCfg.ActivateAtGenesis(rollup.Isthmus)
		l1Cfg := randomL1Cfg(rng, info)
		seqNr := randomSeqNr(rng)
		depTx, err := L1InfoDeposit(&rollupCfg, l1Cfg, seqNr, info, rollupCfg.Genesis.L2Time)
		require.NoError(t, err)
		require.False(t, depTx.IsSystemTransaction)
		require.Equal(t, depTx.Gas, uint64(RegolithSystemTxGas))
		require.Equal(t, L1InfoIsthmusLen, len(depTx.Data))
		assertInfo(&rollupCfg, info, depTx, rollupCfg.Genesis.L2Time, seqNr, l1Cfg)
	})
	t.Run("interop", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1234))
		info := testutils.MakeBlockInfo(nil)(rng)
		rollupCfg := rollup.Config{BlockTime: 2, Genesis: rollup.Genesis{L2Time: 1000}}
		rollupCfg.ActivateAtGenesis(rollup.Interop)
		// run 1 block after interop transition
		timestamp := rollupCfg.Genesis.L2Time + rollupCfg.BlockTime
		l1Cfg := randomL1Cfg(rng, info)
		seqNr := randomSeqNr(rng)
		depTx, err := L1InfoDeposit(&rollupCfg, l1Cfg, seqNr, info, timestamp)
		require.NoError(t, err)
		require.False(t, depTx.IsSystemTransaction)
		require.Equal(t, depTx.Gas, uint64(RegolithSystemTxGas))
		require.Equal(t, L1InfoIsthmusLen, len(depTx.Data), "the length is same in interop")
		require.Equal(t, L1InfoFuncInteropBytes4, depTx.Data[:4], "upgrade is active, need interop signature")
		assertInfo(&rollupCfg, info, depTx, timestamp, seqNr, l1Cfg)
	})
	t.Run("activation-block interop", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1234))
		info := testutils.MakeBlockInfo(nil)(rng)
		rollupCfg := rollup.Config{BlockTime: 2, Genesis: rollup.Genesis{L2Time: 1000}}
		rollupCfg.ActivateAtGenesis(rollup.Fjord)
		interopTime := rollupCfg.Genesis.L2Time + rollupCfg.BlockTime // activate interop just after genesis
		rollupCfg.InteropTime = &interopTime
		l1Cfg := randomL1Cfg(rng, info)
		seqNr := randomSeqNr(rng)
		depTx, err := L1InfoDeposit(&rollupCfg, l1Cfg, seqNr, info, interopTime)
		require.NoError(t, err)
		require.False(t, depTx.IsSystemTransaction)
		require.Equal(t, depTx.Gas, uint64(RegolithSystemTxGas))
		// Interop activates, but ecotone L1 info is still used at this upgrade block
		require.Equal(t, L1InfoEcotoneLen, len(depTx.Data))
		require.Equal(t, L1InfoFuncEcotoneBytes4, depTx.Data[:4])
		assertInfo(&rollupCfg, info, depTx, interopTime, seqNr, l1Cfg)
	})
	t.Run("genesis-block interop", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1234))
		info := testutils.MakeBlockInfo(nil)(rng)
		rollupCfg := rollup.Config{BlockTime: 2, Genesis: rollup.Genesis{L2Time: 1000}}
		rollupCfg.ActivateAtGenesis(rollup.Interop)
		l1Cfg := randomL1Cfg(rng, info)
		seqNr := randomSeqNr(rng)
		depTx, err := L1InfoDeposit(&rollupCfg, l1Cfg, seqNr, info, rollupCfg.Genesis.L2Time)
		require.NoError(t, err)
		require.False(t, depTx.IsSystemTransaction)
		require.Equal(t, depTx.Gas, uint64(RegolithSystemTxGas))
		require.Equal(t, L1InfoIsthmusLen, len(depTx.Data))
		assertInfo(&rollupCfg, info, depTx, rollupCfg.Genesis.L2Time, seqNr, l1Cfg)
	})
}

func TestDepositsCompleteBytes(t *testing.T) {
	randomSeqNr := func(rng *rand.Rand) uint64 {
		return rng.Uint64()
	}
	t.Run("valid return bytes", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1234))
		info := testutils.MakeBlockInfo(nil)(rng)
		depTxByes, err := DepositsCompleteBytes(randomSeqNr(rng), info)
		require.NoError(t, err)
		var depTx types.Transaction
		require.NoError(t, depTx.UnmarshalBinary(depTxByes))
		require.Equal(t, uint8(types.DepositTxType), depTx.Type())
		require.Equal(t, depTx.Data(), DepositsCompleteBytes4)
		require.Equal(t, DepositsCompleteLen, len(depTx.Data()))
		require.Equal(t, DepositsCompleteGas, depTx.Gas())
		require.False(t, depTx.IsSystemTx())
		require.Equal(t, depTx.Value(), big.NewInt(0))
		signer := types.LatestSignerForChainID(depTx.ChainId())
		sender, err := signer.Sender(&depTx)
		require.NoError(t, err)
		require.Equal(t, L1InfoDepositerAddress, sender)
	})
	t.Run("valid return Transaction", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1234))
		info := testutils.MakeBlockInfo(nil)(rng)
		depTx, err := DepositsCompleteDeposit(randomSeqNr(rng), info)
		require.NoError(t, err)
		require.Equal(t, depTx.Data, DepositsCompleteBytes4)
		require.Equal(t, DepositsCompleteLen, len(depTx.Data))
		require.Equal(t, DepositsCompleteGas, depTx.Gas)
		require.False(t, depTx.IsSystemTransaction)
		require.Equal(t, depTx.Value, big.NewInt(0))
		require.Equal(t, L1InfoDepositerAddress, depTx.From)
	})
}
