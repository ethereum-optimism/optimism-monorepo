package utils

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type LocalGameInputs struct {
	L1Head         common.Hash
	L2Head         common.Hash
	L2OutputRoot   common.Hash
	AgreedPreState []byte
	L2Claim        common.Hash
	L2BlockNumber  *big.Int
}

type L2HeaderSource interface {
	HeaderByNumber(context.Context, *big.Int) (*ethtypes.Header, error)
}

type L1HeadSource interface {
	GetL1Head(ctx context.Context) (common.Hash, error)
}

type GameInputsSource interface {
	L1HeadSource
	GetProposals(ctx context.Context) (agreed Proposal, disputed Proposal, err error)
}

type Proposal struct {
	L2BlockNumber *big.Int
	OutputRoot    common.Hash
}

func FetchLocalInputs(ctx context.Context, caller GameInputsSource, l2Client L2HeaderSource) (LocalGameInputs, error) {
	agreedOutput, claimedOutput, err := caller.GetProposals(ctx)
	if err != nil {
		return LocalGameInputs{}, fmt.Errorf("fetch proposals: %w", err)
	}
	l1Head, err := caller.GetL1Head(ctx)
	if err != nil {
		return LocalGameInputs{}, fmt.Errorf("fetch L1 head: %w", err)
	}

	return FetchLocalInputsFromProposals(ctx, l1Head, l2Client, agreedOutput, claimedOutput)
}

func FetchLocalInputsFromProposals(ctx context.Context, l1Head common.Hash, l2Client L2HeaderSource, agreedOutput Proposal, claimedOutput Proposal) (LocalGameInputs, error) {
	agreedHeader, err := l2Client.HeaderByNumber(ctx, agreedOutput.L2BlockNumber)
	if err != nil {
		return LocalGameInputs{}, fmt.Errorf("fetch L2 block header %v: %w", agreedOutput.L2BlockNumber, err)
	}
	l2Head := agreedHeader.Hash()

	return LocalGameInputs{
		L1Head:        l1Head,
		L2Head:        l2Head,
		L2OutputRoot:  agreedOutput.OutputRoot,
		L2Claim:       claimedOutput.OutputRoot,
		L2BlockNumber: claimedOutput.L2BlockNumber,
	}, nil
}
