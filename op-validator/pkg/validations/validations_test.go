package validations

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/testutils/mockrpc"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		version   string
		validator func(rpcClient *rpc.Client) Validator
	}{
		{
			version: "v180",
			validator: func(rpcClient *rpc.Client) Validator {
				return NewV180Validator(rpcClient)
			},
		},
		{
			version: "v200",
			validator: func(rpcClient *rpc.Client) Validator {
				return NewV200Validator(rpcClient)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			mockRPC := mockrpc.NewMockRPC(t, testlog.Logger(t, slog.LevelInfo), mockrpc.WithExpectationsFile(t, fmt.Sprintf("testdata/validations-%s.json", tt.version)))
			rpcClient, err := rpc.Dial(mockRPC.Endpoint())
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			errCodes, err := tt.validator(rpcClient).Validate(ctx, BaseValidatorInput{
				ProxyAdminAddress:   common.HexToAddress("0x189aBAAaa82DfC015A588A7dbaD6F13b1D3485Bc"),
				SystemConfigAddress: common.HexToAddress("0x034edD2A225f7f429A63E0f1D2084B9E0A93b538"),
				AbsolutePrestate:    common.HexToHash("0x038512e02c4c3f7bdaec27d00edf55b7155e0905301e1a88083e4e0a6764d54c"),
				L2ChainID:           big.NewInt(11155420),
			})
			require.NoError(t, err)
			require.Equal(t, []string{"PDDG-40", "PDDG-DWETH-30", "PDDG-ANCHORP-40", "PLDG-40", "PLDG-DWETH-30", "PLDG-ANCHORP-40"}, errCodes)
			mockRPC.AssertExpectations(t)
		})
	}
}
