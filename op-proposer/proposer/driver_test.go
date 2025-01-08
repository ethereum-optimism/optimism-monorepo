package proposer

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-proposer/bindings"
	"github.com/ethereum-optimism/optimism/op-proposer/metrics"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	txmgrmocks "github.com/ethereum-optimism/optimism/op-service/txmgr/mocks"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockL2OOContract struct {
	mock.Mock
}

func (m *MockL2OOContract) Version(opts *bind.CallOpts) (string, error) {
	args := m.Called(opts)
	return args.String(0), args.Error(1)
}

func (m *MockL2OOContract) NextBlockNumber(opts *bind.CallOpts) (*big.Int, error) {
	args := m.Called(opts)
	return args.Get(0).(*big.Int), args.Error(1)
}

type StubDGFContract struct {
	hasProposedCount int
}

func (m *StubDGFContract) HasProposedSince(_ context.Context, _ common.Address, _ time.Time, _ uint32) (bool, time.Time, common.Hash, error) {
	m.hasProposedCount++
	return false, time.Unix(1000, 0), common.Hash{0xdd}, nil
}

func (m *StubDGFContract) ProposalTx(_ context.Context, _ uint32, _ common.Hash, _ uint64) (txmgr.TxCandidate, error) {
	panic("not implemented")
}

func (m *StubDGFContract) Version(_ context.Context) (string, error) {
	panic("not implemented")
}

type mockRollupEndpointProvider struct {
	rollupClient    *testutils.MockRollupClient
	rollupClientErr error
}

func newEndpointProvider() *mockRollupEndpointProvider {
	return &mockRollupEndpointProvider{
		rollupClient: new(testutils.MockRollupClient),
	}
}

func (p *mockRollupEndpointProvider) RollupClient(context.Context) (dial.RollupClientInterface, error) {
	return p.rollupClient, p.rollupClientErr
}

func (p *mockRollupEndpointProvider) Close() {}

func setup(t *testing.T, testName string) (*L2OutputSubmitter, *mockRollupEndpointProvider, *MockL2OOContract, *StubDGFContract, *txmgrmocks.TxManager, *testlog.CapturingHandler) {
	ep := newEndpointProvider()

	l2OutputOracleAddr := common.HexToAddress("0x3F8A862E63E759a77DA22d384027D21BF096bA9E")

	proposerConfig := ProposerConfig{
		PollInterval:       time.Microsecond,
		ProposalInterval:   time.Microsecond,
		L2OutputOracleAddr: &l2OutputOracleAddr,
	}

	txmgr := txmgrmocks.NewTxManager(t)

	lgr, logs := testlog.CaptureLogger(t, log.LevelDebug)
	setup := DriverSetup{
		Log:            lgr,
		Metr:           metrics.NoopMetrics,
		Cfg:            proposerConfig,
		Txmgr:          txmgr,
		RollupProvider: ep,
	}

	parsed, err := bindings.L2OutputOracleMetaData.GetAbi()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	l2OutputSubmitter := L2OutputSubmitter{
		DriverSetup: setup,
		done:        make(chan struct{}),
		l2ooABI:     parsed,
		ctx:         ctx,
		cancel:      cancel,
	}
	var mockDGFContract *StubDGFContract
	var mockL2OOContract *MockL2OOContract
	if testName == "DGF" {
		mockDGFContract = new(StubDGFContract)
		l2OutputSubmitter.dgfContract = mockDGFContract
	} else {
		mockL2OOContract = new(MockL2OOContract)
		l2OutputSubmitter.l2ooContract = mockL2OOContract
	}

	txmgr.On("BlockNumber", mock.Anything).Return(uint64(100), nil).Once()
	txmgr.On("Send", mock.Anything, mock.Anything).
		Return(&types.Receipt{Status: uint64(1), TxHash: common.Hash{}}, nil).
		Once().
		Run(func(_ mock.Arguments) {
			// let loops return after first Send call
			t.Log("Closing proposer.")
			close(l2OutputSubmitter.done)
		})

	return &l2OutputSubmitter, ep, mockL2OOContract, mockDGFContract, txmgr, logs
}

func TestL2OutputSubmitter_OutputRetry(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "L2OO"},
		{name: "DGF"},
	}

	proposerAddr := common.Address{0xab}
	const numFails = 3
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps, ep, l2ooContract, dgfContract, txmgr, logs := setup(t, tt.name)

			ep.rollupClient.On("SyncStatus").Return(&eth.SyncStatus{FinalizedL2: eth.L2BlockRef{Number: 42}}, nil).Times(numFails + 1)
			ep.rollupClient.ExpectOutputAtBlock(42, nil, fmt.Errorf("TEST: failed to fetch output")).Times(numFails)
			ep.rollupClient.ExpectOutputAtBlock(
				42,
				&eth.OutputResponse{
					Version:  supportedL2OutputVersion,
					BlockRef: eth.L2BlockRef{Number: 42},
					Status: &eth.SyncStatus{
						CurrentL1:   eth.L1BlockRef{Hash: common.Hash{}},
						FinalizedL2: eth.L2BlockRef{Number: 42},
					},
				},
				nil,
			)

			txmgr.On("From").Return(proposerAddr).Times(numFails + 1)

			if tt.name == "L2OO" {
				l2ooContract.On("NextBlockNumber", mock.AnythingOfType("*bind.CallOpts")).Return(big.NewInt(42), nil).Times(numFails + 1)
			}
			ps.wg.Add(1)
			ps.loop()

			ep.rollupClient.AssertExpectations(t)
			if tt.name == "L2OO" {
				l2ooContract.AssertExpectations(t)
			} else {
				require.Equal(t, numFails+1, dgfContract.hasProposedCount)
			}

			require.Len(t, logs.FindLogs(testlog.NewMessageContainsFilter("Error getting output")), numFails)
			require.NotNil(t, logs.FindLog(testlog.NewMessageFilter("Proposer tx successfully published")))
			require.NotNil(t, logs.FindLog(testlog.NewMessageFilter("loop returning")))
		})
	}
}
