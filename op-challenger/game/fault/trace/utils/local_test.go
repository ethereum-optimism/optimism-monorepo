package utils

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestFetchLocalInputs(t *testing.T) {
	ctx := context.Background()
	contract := &mockGameInputsSource{
		l1Head: common.Hash{0xcc},
		starting: Proposal{
			L2BlockNumber: big.NewInt(2222),
			OutputRoot:    common.Hash{0xdd},
		},
		disputed: Proposal{
			L2BlockNumber: big.NewInt(3333),
			OutputRoot:    common.Hash{0xee},
		},
	}
	l2Client := &mockL2DataSource{
		chainID: big.NewInt(88422),
		header: ethtypes.Header{
			Number: contract.starting.L2BlockNumber,
		},
	}

	inputs, err := FetchLocalInputs(ctx, contract, l2Client)
	require.NoError(t, err)

	require.Equal(t, contract.l1Head, inputs.L1Head)
	require.Equal(t, l2Client.header.Hash(), inputs.L2Head)
	require.EqualValues(t, contract.starting.OutputRoot, inputs.L2OutputRoot)
	require.EqualValues(t, contract.disputed.OutputRoot, inputs.L2Claim)
	require.Equal(t, contract.disputed.L2BlockNumber, inputs.L2BlockNumber)
}

func TestFetchLocalInputsFromProposals(t *testing.T) {
	ctx := context.Background()
	agreed := Proposal{
		L2BlockNumber: big.NewInt(2222),
		OutputRoot:    common.Hash{0xdd},
	}
	claimed := Proposal{
		L2BlockNumber: big.NewInt(3333),
		OutputRoot:    common.Hash{0xee},
	}
	l1Head := common.Hash{0xcc}
	l2Client := &mockL2DataSource{
		chainID: big.NewInt(88422),
		header: ethtypes.Header{
			Number: agreed.L2BlockNumber,
		},
	}

	inputs, err := FetchLocalInputsFromProposals(ctx, l1Head, l2Client, agreed, claimed)
	require.NoError(t, err)

	require.Equal(t, l1Head, inputs.L1Head)
	require.Equal(t, l2Client.header.Hash(), inputs.L2Head)
	require.EqualValues(t, agreed.OutputRoot, inputs.L2OutputRoot)
	require.EqualValues(t, claimed.OutputRoot, inputs.L2Claim)
	require.Equal(t, claimed.L2BlockNumber, inputs.L2BlockNumber)
}

type mockGameInputsSource struct {
	l1Head   common.Hash
	starting Proposal
	disputed Proposal
}

func (s *mockGameInputsSource) GetL1Head(_ context.Context) (common.Hash, error) {
	return s.l1Head, nil
}

func (s *mockGameInputsSource) GetProposals(_ context.Context) (Proposal, Proposal, error) {
	return s.starting, s.disputed, nil
}

type mockL2DataSource struct {
	chainID *big.Int
	header  ethtypes.Header
}

func (s *mockL2DataSource) ChainID(_ context.Context) (*big.Int, error) {
	return s.chainID, nil
}

func (s *mockL2DataSource) HeaderByNumber(_ context.Context, num *big.Int) (*ethtypes.Header, error) {
	if s.header.Number.Cmp(num) == 0 {
		return &s.header, nil
	}
	return nil, ethereum.NotFound
}
