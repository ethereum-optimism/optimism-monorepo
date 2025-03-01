package txplan

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestPlannedTx_Defaults(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	ptx := NewPlannedTx(WithPrivateKey(key), WithValue(big.NewInt(123)))
	t.Log("tx", ptx.Signed.String())
	ptx.AgainstBlock.Set(&types.Header{BaseFee: big.NewInt(7e9)})

	expectedAddr := crypto.PubkeyToAddress(key.PublicKey)
	signer := types.LatestSignerForChainID(big.NewInt(1))
	{
		tx, err := ptx.Signed.Eval(context.Background())
		require.NoError(t, err)

		sender, err := signer.Sender(tx)
		require.NoError(t, err)
		require.Equal(t, expectedAddr, sender)

		require.Equal(t, big.NewInt(123), tx.Value())
	}

	// Get a new signed tx
	ptx.Value.Set(big.NewInt(42))
	{
		tx, err := ptx.Signed.Eval(context.Background())
		require.NoError(t, err)

		sender, err := signer.Sender(tx)
		require.NoError(t, err)
		require.Equal(t, expectedAddr, sender)

		require.Equal(t, big.NewInt(42), tx.Value())
	}
}
