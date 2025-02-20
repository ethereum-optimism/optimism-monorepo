package attributes

import (
	"math/rand" // nosemgrep
	"testing"

	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

var defaultOpConfig = &params.OptimismConfig{
	EIP1559Elasticity:        6,
	EIP1559Denominator:       50,
	EIP1559DenominatorCanyon: ptr(uint64(250)),
}

func ptr[T any](t T) *T {
	return &t
}

type matchArgs struct {
	envelope   *eth.ExecutionPayloadEnvelope
	attrs      *eth.PayloadAttributes
	parentHash common.Hash
}

func holoceneArgs() matchArgs {
	var (
		validParentHash       = common.HexToHash("0x123")
		validTimestamp        = eth.Uint64Quantity(50)
		validParentBeaconRoot = common.HexToHash("0x456")
		validPrevRandao       = eth.Bytes32(common.HexToHash("0x789"))
		validGasLimit         = eth.Uint64Quantity(1000)
		validFeeRecipient     = predeploys.SequencerFeeVaultAddr
		validTx               = testutils.RandomLegacyTxNotProtected(rand.New(rand.NewSource(42)))
		validTxData, _        = validTx.MarshalBinary()

		validHoloceneExtraData = eth.BytesMax32(eip1559.EncodeHoloceneExtraData(
			*defaultOpConfig.EIP1559DenominatorCanyon, defaultOpConfig.EIP1559Elasticity))
		validHoloceneEIP1559Params = new(eth.Bytes8)
	)

	return matchArgs{
		envelope: &eth.ExecutionPayloadEnvelope{
			ParentBeaconBlockRoot: &validParentBeaconRoot,
			ExecutionPayload: &eth.ExecutionPayload{
				ParentHash:   validParentHash,
				Timestamp:    validTimestamp,
				PrevRandao:   validPrevRandao,
				GasLimit:     validGasLimit,
				Transactions: []eth.Data{validTxData},
				Withdrawals:  &types.Withdrawals{},
				FeeRecipient: validFeeRecipient,
				ExtraData:    validHoloceneExtraData,
			},
		},
		attrs: &eth.PayloadAttributes{
			Timestamp:             validTimestamp,
			PrevRandao:            validPrevRandao,
			GasLimit:              &validGasLimit,
			ParentBeaconBlockRoot: &validParentBeaconRoot,
			Transactions:          []eth.Data{validTxData},
			Withdrawals:           &types.Withdrawals{},
			SuggestedFeeRecipient: validFeeRecipient,
			EIP1559Params:         validHoloceneEIP1559Params,
		},
		parentHash: validParentHash,
	}
}

func ecotoneArgs() matchArgs {
	args := holoceneArgs()
	args.attrs.EIP1559Params = nil
	args.envelope.ExecutionPayload.ExtraData = nil
	return args
}

func canyonArgs() matchArgs {
	args := ecotoneArgs()
	args.attrs.ParentBeaconBlockRoot = nil
	args.envelope.ParentBeaconBlockRoot = nil
	return args
}

func bedrockArgs() matchArgs {
	args := canyonArgs()
	args.attrs.Withdrawals = nil
	args.envelope.ExecutionPayload.Withdrawals = nil
	return args
}

func ecotoneNoParentBeaconBlockRoot() matchArgs {
	args := ecotoneArgs()
	args.envelope.ParentBeaconBlockRoot = nil
	return args
}

func ecotoneUnexpectedParentBeaconBlockRoot() matchArgs {
	args := ecotoneArgs()
	args.attrs.ParentBeaconBlockRoot = nil
	return args
}

func ecotoneMismatchParentBeaconBlockRoot() matchArgs {
	args := ecotoneArgs()
	h := common.HexToHash("0xabc")
	args.attrs.ParentBeaconBlockRoot = &h
	return args
}

func ecotoneMismatchParentBeaconBlockRootPtr() matchArgs {
	args := ecotoneArgs()
	cpy := *args.attrs.ParentBeaconBlockRoot
	args.attrs.ParentBeaconBlockRoot = &cpy
	return args
}

func ecotoneNilParentBeaconBlockRoots() matchArgs {
	args := ecotoneArgs()
	args.attrs.ParentBeaconBlockRoot = nil
	args.envelope.ParentBeaconBlockRoot = nil
	return args
}

func mismatchedParentHashArgs() matchArgs {
	args := ecotoneArgs()
	args.parentHash = common.HexToHash("0xabc")
	return args
}

func createMismatchedPrevRandao() matchArgs {
	args := ecotoneArgs()
	args.attrs.PrevRandao = eth.Bytes32(common.HexToHash("0xabc"))
	return args
}

func createMismatchedGasLimit() matchArgs {
	args := ecotoneArgs()
	val := eth.Uint64Quantity(2000)
	args.attrs.GasLimit = &val
	return args
}

func createNilGasLimit() matchArgs {
	args := ecotoneArgs()
	args.attrs.GasLimit = nil
	return args
}

func createMismatchedTimestamp() matchArgs {
	args := ecotoneArgs()
	args.attrs.Timestamp++
	return args
}

func createMismatchedTransactions() matchArgs {
	args := ecotoneArgs()
	args.attrs.Transactions = append(args.attrs.Transactions, args.attrs.Transactions[0])
	return args
}

func createMismatchedFeeRecipient() matchArgs {
	args := ecotoneArgs()
	args.attrs.SuggestedFeeRecipient = common.Address{0xde, 0xad}
	return args
}

func createMismatchedEIP1559Params() matchArgs {
	args := holoceneArgs()
	args.attrs.EIP1559Params[0]++ // so denominator is != 0
	return args
}

func TestAttributesMatch(t *testing.T) {
	// default valid timestamp is 50
	pastTime := uint64(0)
	futureTime := uint64(100)

	rollupCfgPreCanyon := &rollup.Config{CanyonTime: &futureTime, ChainOpConfig: defaultOpConfig}
	rollupCfgPreIsthmus := &rollup.Config{CanyonTime: &pastTime, IsthmusTime: &futureTime, ChainOpConfig: defaultOpConfig}

	tests := []struct {
		args      matchArgs
		rollupCfg *rollup.Config
		err       string
		desc      string
	}{
		{
			args:      bedrockArgs(),
			rollupCfg: rollupCfgPreCanyon,
			desc:      "validBedrockArgs",
		},
		{
			args:      bedrockArgs(),
			rollupCfg: rollupCfgPreIsthmus,
			err:       ErrCanyonMustHaveWithdrawals.Error() + ": block",
			desc:      "bedrockArgsPostCanyon",
		},
		{
			args:      canyonArgs(),
			rollupCfg: rollupCfgPreIsthmus,
			desc:      "validCanyonArgs",
		},
		{
			args:      ecotoneArgs(),
			rollupCfg: rollupCfgPreIsthmus,
			desc:      "validEcotoneArgs",
		},
		{
			args:      holoceneArgs(),
			rollupCfg: rollupCfgPreIsthmus,
			desc:      "validholoceneArgs",
		},
		{
			args:      mismatchedParentHashArgs(),
			rollupCfg: rollupCfgPreIsthmus,
			err:       "parent hash field does not match",
			desc:      "mismatchedParentHashArgs",
		},
		{
			args:      createMismatchedTimestamp(),
			rollupCfg: rollupCfgPreIsthmus,
			err:       "timestamp field does not match",
			desc:      "createMismatchedTimestamp",
		},
		{
			args:      createMismatchedPrevRandao(),
			rollupCfg: rollupCfgPreIsthmus,
			err:       "random field does not match",
			desc:      "createMismatchedPrevRandao",
		},
		{
			args:      createMismatchedTransactions(),
			rollupCfg: rollupCfgPreIsthmus,
			err:       "transaction count does not match",
			desc:      "createMismatchedTransactions",
		},
		{
			args:      ecotoneNoParentBeaconBlockRoot(),
			rollupCfg: rollupCfgPreIsthmus,
			err:       "expected non-nil parent beacon block root",
			desc:      "ecotoneNoParentBeaconBlockRoot",
		},
		{
			args:      ecotoneUnexpectedParentBeaconBlockRoot(),
			rollupCfg: rollupCfgPreIsthmus,
			err:       "expected nil parent beacon block root but got non-nil",
			desc:      "ecotoneUnexpectedParentBeaconBlockRoot",
		},
		{
			args:      ecotoneMismatchParentBeaconBlockRoot(),
			rollupCfg: rollupCfgPreIsthmus,
			err:       "parent beacon block root does not match",
			desc:      "ecotoneMismatchParentBeaconBlockRoot",
		},
		{
			args:      ecotoneMismatchParentBeaconBlockRootPtr(),
			rollupCfg: rollupCfgPreIsthmus,
			desc:      "ecotoneMismatchParentBeaconBlockRootPtr",
		},
		{
			args:      ecotoneNilParentBeaconBlockRoots(),
			rollupCfg: rollupCfgPreIsthmus,
			desc:      "ecotoneNilParentBeaconBlockRoots",
		},
		{
			args:      createMismatchedGasLimit(),
			rollupCfg: rollupCfgPreIsthmus,
			err:       "gas limit does not match",
			desc:      "createMismatchedGasLimit",
		},
		{
			args:      createNilGasLimit(),
			rollupCfg: rollupCfgPreIsthmus,
			err:       "expected gaslimit in attributes to not be nil",
			desc:      "createNilGasLimit",
		},
		{
			args:      createMismatchedFeeRecipient(),
			rollupCfg: rollupCfgPreIsthmus,
			err:       "fee recipient data does not match",
			desc:      "createMismatchedFeeRecipient",
		},
		{
			args:      createMismatchedEIP1559Params(),
			rollupCfg: rollupCfgPreIsthmus,
			err:       "eip1559 parameters do not match",
			desc:      "createMismatchedEIP1559Params",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := AttributesMatchBlock(test.rollupCfg,
				test.args.attrs, test.args.parentHash, test.args.envelope,
				testlog.Logger(t, log.LevelInfo),
			)
			if test.err == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.err)
			}
		})
	}
}

func TestWithdrawalsMatch(t *testing.T) {
	canyonTimeInFuture := uint64(100)
	canyonTimeInPast := uint64(0)
	isthmusTimeInPast := uint64(150)
	isthmusTimeInFuture := uint64(250)

	emptyWithdrawals := make(types.Withdrawals, 0)

	rollupCfgPreCanyonChecks := &rollup.Config{CanyonTime: &canyonTimeInFuture, ChainOpConfig: defaultOpConfig}
	rollupCfgPreIsthmusChecks := &rollup.Config{CanyonTime: &canyonTimeInPast, IsthmusTime: &isthmusTimeInFuture, ChainOpConfig: defaultOpConfig}
	rollupCfgPostIsthmusChecks := &rollup.Config{CanyonTime: &canyonTimeInPast, IsthmusTime: &isthmusTimeInPast, ChainOpConfig: defaultOpConfig}

	tests := []struct {
		cfg   *rollup.Config
		attrs *eth.PayloadAttributes
		block *eth.ExecutionPayload
		err   error
		desc  string
	}{
		{
			cfg:   rollupCfgPreCanyonChecks,
			attrs: nil,
			block: nil,
			err:   ErrNilBlockOrAttributes,
			desc:  "nil attributes/block",
		},
		{
			cfg:   rollupCfgPreCanyonChecks,
			attrs: &eth.PayloadAttributes{Withdrawals: nil},
			block: &eth.ExecutionPayload{Timestamp: 0},
			desc:  "pre-canyon: nil attr withdrawals",
		},
		{
			cfg: rollupCfgPreCanyonChecks,
			attrs: &eth.PayloadAttributes{
				Withdrawals: &types.Withdrawals{
					&types.Withdrawal{
						Index: 1,
					},
				},
			},
			block: &eth.ExecutionPayload{Timestamp: 0},
			err:   ErrBedrockMustHaveEmptyWithdrawals,
			desc:  "pre-canyon: non-nil withdrawals",
		},
		{
			cfg:   rollupCfgPostIsthmusChecks,
			attrs: &eth.PayloadAttributes{},
			block: &eth.ExecutionPayload{
				Timestamp: 200,
				Withdrawals: &types.Withdrawals{
					&types.Withdrawal{
						Index: 1,
					},
				},
			},
			err:  ErrCanyonMustHaveWithdrawals,
			desc: "post-isthmus: non-empty block withdrawals list",
		},
		{
			cfg: rollupCfgPostIsthmusChecks,
			attrs: &eth.PayloadAttributes{
				Withdrawals: &emptyWithdrawals,
			},
			block: &eth.ExecutionPayload{
				Timestamp:       200,
				WithdrawalsRoot: nil,
				Withdrawals:     &emptyWithdrawals,
			},
			err:  ErrIsthmusMustHaveWithdrawalsRoot,
			desc: "post-isthmus: nil block withdrawalsRoot",
		},
		{
			cfg: rollupCfgPostIsthmusChecks,
			attrs: &eth.PayloadAttributes{
				Withdrawals: &types.Withdrawals{
					&types.Withdrawal{
						Index: 1,
					},
				},
			},
			block: &eth.ExecutionPayload{
				Timestamp:       200,
				WithdrawalsRoot: &common.Hash{},
				Withdrawals:     &emptyWithdrawals,
			},
			err:  ErrCanyonMustHaveWithdrawals,
			desc: "post-isthmus: non-empty attr withdrawals list",
		},
		{
			cfg: rollupCfgPostIsthmusChecks,
			attrs: &eth.PayloadAttributes{
				Withdrawals: &emptyWithdrawals,
			},
			block: &eth.ExecutionPayload{
				Timestamp:       200,
				WithdrawalsRoot: &common.Hash{},
				Withdrawals:     &emptyWithdrawals,
			},
			desc: "post-isthmus: non-empty block withdrawalsRoot and empty block/attr withdrawals list",
		},
		{
			cfg:   rollupCfgPreIsthmusChecks,
			attrs: &eth.PayloadAttributes{},
			block: &eth.ExecutionPayload{
				Timestamp: 200,
				Withdrawals: &types.Withdrawals{
					&types.Withdrawal{
						Index: 1,
					},
				},
			},
			err:  ErrCanyonMustHaveWithdrawals,
			desc: "pre-isthmus: non-empty block withdrawals list",
		},
		{
			cfg: rollupCfgPreIsthmusChecks,
			attrs: &eth.PayloadAttributes{
				Withdrawals: &emptyWithdrawals,
			},
			block: &eth.ExecutionPayload{
				Timestamp:       200,
				Withdrawals:     &types.Withdrawals{},
				WithdrawalsRoot: &common.Hash{},
			},
			err:  ErrCanyonWithdrawalsRoot,
			desc: "pre-isthmus: non-empty block withdrawalsRoot",
		},
		{
			cfg: rollupCfgPreIsthmusChecks,
			attrs: &eth.PayloadAttributes{
				Withdrawals: &types.Withdrawals{
					&types.Withdrawal{
						Index: 1,
					},
				},
			},
			block: &eth.ExecutionPayload{
				Timestamp:       200,
				Withdrawals:     &types.Withdrawals{},
				WithdrawalsRoot: nil,
			},
			err:  ErrCanyonMustHaveWithdrawals,
			desc: "pre-isthmus: non-empty attr withdrawals list",
		},
		{
			cfg: rollupCfgPreIsthmusChecks,
			attrs: &eth.PayloadAttributes{
				Withdrawals: &emptyWithdrawals,
			},
			block: &eth.ExecutionPayload{
				Timestamp:       200,
				WithdrawalsRoot: nil,
				Withdrawals:     &emptyWithdrawals,
			},
			desc: "pre-isthmus: nil block withdrawalsRoot and empty block/attr withdrawals list",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := checkWithdrawals(test.cfg, test.attrs, test.block)

			if test.err != nil {
				require.ErrorIs(t, err, test.err, "test: %s", test.desc)
			} else {
				require.NoError(t, err, "test: %s", test.desc)
			}
		})
	}
}

func TestCheckEIP1559ParamsMatch(t *testing.T) {
	params := eth.Bytes8{1, 2, 3, 4, 5, 6, 7, 8}
	paramsAlt := eth.Bytes8{1, 2, 3, 4, 5, 6, 7, 9}
	paramsInvalid := eth.Bytes8{0, 0, 0, 0, 5, 6, 7, 8}
	defaultExtraData := eth.BytesMax32(eip1559.EncodeHoloceneExtraData(
		*defaultOpConfig.EIP1559DenominatorCanyon, defaultOpConfig.EIP1559Elasticity))

	for _, test := range []struct {
		desc           string
		attrParams     *eth.Bytes8
		blockExtraData eth.BytesMax32
		err            string
	}{
		{
			desc: "match-empty",
		},
		{
			desc:           "match-zero-attrs",
			attrParams:     new(eth.Bytes8),
			blockExtraData: defaultExtraData,
		},
		{
			desc:           "match-non-zero",
			attrParams:     &params,
			blockExtraData: append(eth.BytesMax32{0}, params[:]...),
		},
		{
			desc:           "err-both-zero",
			attrParams:     new(eth.Bytes8),
			blockExtraData: make(eth.BytesMax32, 9),
			err:            "eip1559 parameters do not match, attributes: 250, 6 (translated from 0,0), block: 0, 0",
		},
		{
			desc:           "err-invalid-params",
			attrParams:     &paramsInvalid,
			blockExtraData: append(eth.BytesMax32{0}, paramsInvalid[:]...),
			err:            "invalid attributes EIP1559 parameters: holocene params cannot have a 0 denominator unless elasticity is also 0",
		},
		{
			desc:           "err-invalid-extra",
			attrParams:     &params,
			blockExtraData: append(eth.BytesMax32{42}, params[:]...),
			err:            "invalid block extraData: holocene extraData should have 0 version byte, got 42",
		},
		{
			desc:           "err-no-match",
			attrParams:     &paramsAlt,
			blockExtraData: append(eth.BytesMax32{0}, params[:]...),
			err:            "eip1559 parameters do not match",
		},
		{
			desc:           "err-non-nil-extra",
			blockExtraData: make(eth.BytesMax32, 9),
			err:            "nil EIP1559Params in attributes but non-nil extraData in block",
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			err := checkEIP1559ParamsMatch(defaultOpConfig, test.attrParams, test.blockExtraData)
			if test.err == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.err)
			}
		})
	}
}

func TestGetMissingTxnHashes(t *testing.T) {
	depositTxs := make([]*types.Transaction, 5)

	for i := 0; i < len(depositTxs); i++ {
		rng := rand.New(rand.NewSource(1234 + int64(i)))
		safeDeposit := testutils.GenerateDeposit(testutils.RandomHash(rng), rng)
		depositTxs[i] = types.NewTx(safeDeposit)
	}

	tests := []struct {
		safeTransactions            []hexutil.Bytes
		unsafeTransactions          []hexutil.Bytes
		expectedSafeMissingHashes   []common.Hash
		expectedUnsafeMissingHashes []common.Hash
		expectErr                   bool
	}{
		{
			safeTransactions:            []hexutil.Bytes{},
			unsafeTransactions:          []hexutil.Bytes{depositTxToBytes(t, depositTxs[0])},
			expectedSafeMissingHashes:   []common.Hash{depositTxs[0].Hash()},
			expectedUnsafeMissingHashes: []common.Hash{},
			expectErr:                   false,
		},
		{
			safeTransactions:            []hexutil.Bytes{depositTxToBytes(t, depositTxs[0])},
			unsafeTransactions:          []hexutil.Bytes{},
			expectedSafeMissingHashes:   []common.Hash{},
			expectedUnsafeMissingHashes: []common.Hash{depositTxs[0].Hash()},
			expectErr:                   false,
		},
		{
			safeTransactions: []hexutil.Bytes{
				depositTxToBytes(t, depositTxs[0]),
			},
			unsafeTransactions: []hexutil.Bytes{
				depositTxToBytes(t, depositTxs[0]),
				depositTxToBytes(t, depositTxs[1]),
				depositTxToBytes(t, depositTxs[2]),
			},
			expectedSafeMissingHashes: []common.Hash{
				depositTxs[1].Hash(),
				depositTxs[2].Hash(),
			},
			expectedUnsafeMissingHashes: []common.Hash{},
			expectErr:                   false,
		},
		{
			safeTransactions: []hexutil.Bytes{
				depositTxToBytes(t, depositTxs[0]),
				depositTxToBytes(t, depositTxs[1]),
				depositTxToBytes(t, depositTxs[2]),
			},
			unsafeTransactions: []hexutil.Bytes{
				depositTxToBytes(t, depositTxs[0]),
			},
			expectedSafeMissingHashes: []common.Hash{},
			expectedUnsafeMissingHashes: []common.Hash{
				depositTxs[1].Hash(),
				depositTxs[2].Hash(),
			},
			expectErr: false,
		},
		{
			safeTransactions: []hexutil.Bytes{
				depositTxToBytes(t, depositTxs[0]),
				depositTxToBytes(t, depositTxs[1]),
				depositTxToBytes(t, depositTxs[2]),
			},
			unsafeTransactions: []hexutil.Bytes{
				depositTxToBytes(t, depositTxs[2]),
				depositTxToBytes(t, depositTxs[3]),
				depositTxToBytes(t, depositTxs[4]),
			},
			expectedSafeMissingHashes: []common.Hash{
				depositTxs[3].Hash(),
				depositTxs[4].Hash(),
			},
			expectedUnsafeMissingHashes: []common.Hash{
				depositTxs[0].Hash(),
				depositTxs[1].Hash(),
			},
			expectErr: false,
		},
		{
			safeTransactions:            []hexutil.Bytes{{1, 2, 3}},
			unsafeTransactions:          []hexutil.Bytes{},
			expectedSafeMissingHashes:   []common.Hash{},
			expectedUnsafeMissingHashes: []common.Hash{},
			expectErr:                   true,
		},
		{
			safeTransactions:            []hexutil.Bytes{},
			unsafeTransactions:          []hexutil.Bytes{{1, 2, 3}},
			expectedSafeMissingHashes:   []common.Hash{},
			expectedUnsafeMissingHashes: []common.Hash{},
			expectErr:                   true,
		},
	}

	for _, test := range tests {
		missingSafeHashes, missingUnsafeHashes, err := getMissingTxnHashes(
			testlog.Logger(t, log.LevelError),
			test.safeTransactions,
			test.unsafeTransactions,
		)

		if test.expectErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.ElementsMatch(t, test.expectedSafeMissingHashes, missingSafeHashes)
			require.ElementsMatch(t, test.expectedUnsafeMissingHashes, missingUnsafeHashes)
		}
	}
}

func depositTxToBytes(t *testing.T, tx *types.Transaction) hexutil.Bytes {
	txBytes, err := tx.MarshalBinary()
	require.NoError(t, err)

	return txBytes
}
