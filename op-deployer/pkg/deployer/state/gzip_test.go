package state

import (
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestGzipData_Marshaling(t *testing.T) {
	type ts struct {
		Field *GzipData[foundry.ForgeAllocs]
	}

	tests := []struct {
		name string
		in   ts
		out  string
	}{
		{
			name: "empty",
			in:   ts{},
			out:  "null",
		},
		{
			name: "contains some data",
			in: ts{
				Field: &GzipData[foundry.ForgeAllocs]{
					Data: &foundry.ForgeAllocs{
						Accounts: map[common.Address]types.Account{
							common.HexToAddress("0x1"): {
								Balance: big.NewInt(1),
							},
						},
					},
				},
			},
			out: `"H4sIAAAAAAAA/6pWMqgwIA4YKllVKyUl5iTmJacqWSkZVBgq1dYCAgAA//9hulF0QAAAAA=="`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.in)
			require.NoError(t, err)
			require.Equal(t, fmt.Sprintf(`{"Field":%s}`, tt.out), string(data))
			var unmarshalled ts
			err = json.Unmarshal(data, &unmarshalled)
			require.NoError(t, err)
			require.EqualValues(t, tt.in, unmarshalled)
		})
	}
}
