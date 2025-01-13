package broadcaster

import (
	"context"
	"testing"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestCalldataBroadcaster(t *testing.T) {
	bcast := new(CalldataBroadcaster)

	bcasts := []script.Broadcast{
		{
			Type:  script.BroadcastCall,
			To:    common.Address{'1'},
			Input: []byte{'D', '1'},
			Value: (*hexutil.U256)(new(uint256.Int).SetUint64(123)),
		},
		{
			Type:  script.BroadcastCreate,
			Input: []byte{'D', '2'},
		},
	}
	for _, b := range bcasts {
		bcast.Hook(b)
	}

	res, err := bcast.Broadcast(context.Background())
	require.NoError(t, err)
	require.Nil(t, res)

	dump, err := bcast.Dump()
	require.NoError(t, err)

	expValues := make([]CalldataDump, len(bcasts))
	for i, b := range bcasts {
		var to *common.Address
		if b.To != (common.Address{}) {
			to = &b.To
		}

		var value *hexutil.Big
		if b.Value != nil {
			value = (*hexutil.Big)((*uint256.Int)(b.Value).ToBig())
		}

		expValues[i] = CalldataDump{
			To:    to,
			Value: value,
			Data:  b.Input,
		}
	}
	require.EqualValues(t, expValues, dump)
}
