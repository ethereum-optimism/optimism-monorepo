package genesis

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWithdrawalNetworkValid checks that valid withdrawal networks are detected.
func TestWithdrawalNetworkValid(t *testing.T) {
	localWithdrawalNetwork := WithdrawalNetwork("local")
	require.True(t, localWithdrawalNetwork.Valid())

	remoteWithdrawalNetwork := WithdrawalNetwork("remote")
	require.True(t, remoteWithdrawalNetwork.Valid())

	invalidWithdrawalNetwork := WithdrawalNetwork("invalid")
	require.False(t, invalidWithdrawalNetwork.Valid())
}

// TestWithdrawalNetworkToUint8 checks that withdrawal networks are converted to uint8 correctly.
func TestWithdrawalNetworkToUint8(t *testing.T) {
	localWithdrawalNetwork := WithdrawalNetwork("local")
	require.EqualValues(t, 1, localWithdrawalNetwork.ToUint8())

	remoteWithdrawalNetwork := WithdrawalNetwork("remote")
	require.EqualValues(t, 0, remoteWithdrawalNetwork.ToUint8())

	invalidWithdrawalNetwork := WithdrawalNetwork("invalid")
	require.EqualValues(t, 1, invalidWithdrawalNetwork.ToUint8())
}

// TestWithdrawalNetworkFromUint8 checks that uint8s are converted to withdrawal networks correctly.
func TestWithdrawalNetworkFromUint8(t *testing.T) {
	require.EqualValues(t, "local", FromUint8(1))
	require.EqualValues(t, "remote", FromUint8(0))
	// invalid uint8s are converted to their uint8 string representation
	// this will be caught by the Valid() check
	require.EqualValues(t, "2", FromUint8(2))
}

// TestWithdrawalNetworkUnmarshalJSON checks that withdrawal networks are unmarshalled correctly.
func TestWithdrawalNetworkUnmarshalJSON(t *testing.T) {
	t.Run("LocalInt", func(t *testing.T) {
		var w WithdrawalNetwork
		require.NoError(t, json.Unmarshal([]byte(`1`), &w))
		require.EqualValues(t, "local", w)
	})

	t.Run("RemoteInt", func(t *testing.T) {
		var w WithdrawalNetwork
		require.NoError(t, json.Unmarshal([]byte(`0`), &w))
		require.EqualValues(t, "remote", w)
	})

	t.Run("InvalidInt", func(t *testing.T) {
		var w WithdrawalNetwork
		require.Error(t, json.Unmarshal([]byte(`2`), &w))
	})

	t.Run("LocalString", func(t *testing.T) {
		var w WithdrawalNetwork
		require.NoError(t, json.Unmarshal([]byte(`"local"`), &w))
		require.EqualValues(t, "local", w)
	})

	t.Run("RemoteString", func(t *testing.T) {
		var w WithdrawalNetwork
		require.NoError(t, json.Unmarshal([]byte(`"remote"`), &w))
		require.EqualValues(t, "remote", w)
	})

	t.Run("InvalidString", func(t *testing.T) {
		var w WithdrawalNetwork
		require.Error(t, json.Unmarshal([]byte(`"invalid"`), &w))
	})
}

// TestWithdrawalNetworkInlineJSON tests unmarshalling of withdrawal networks in inline JSON.
func TestWithdrawalNetworkInlineJSON(t *testing.T) {
	type tempNetworks struct {
		BaseFeeVaultWithdrawalNetwork      WithdrawalNetwork `json:"baseFeeVaultWithdrawalNetwork"`
		L1FeeVaultWithdrawalNetwork        WithdrawalNetwork `json:"l1FeeVaultWithdrawalNetwork"`
		SequencerFeeVaultWithdrawalNetwork WithdrawalNetwork `json:"sequencerFeeVaultWithdrawalNetwork"`
	}

	jsonString := `{
		"baseFeeVaultWithdrawalNetwork": "remote",
		"l1FeeVaultWithdrawalNetwork": "local",
		"sequencerFeeVaultWithdrawalNetwork": "local"
	}`

	intJsonString := `{
		"baseFeeVaultWithdrawalNetwork": 0,
		"l1FeeVaultWithdrawalNetwork": 1,
		"sequencerFeeVaultWithdrawalNetwork": 1
	}`

	t.Run("StringMarshaling", func(t *testing.T) {
		decoded := new(tempNetworks)
		require.NoError(t, json.Unmarshal([]byte(jsonString), decoded))

		require.Equal(t, WithdrawalNetwork("remote"), decoded.BaseFeeVaultWithdrawalNetwork)
		require.Equal(t, WithdrawalNetwork("local"), decoded.L1FeeVaultWithdrawalNetwork)
		require.Equal(t, WithdrawalNetwork("local"), decoded.SequencerFeeVaultWithdrawalNetwork)

		encoded, err := json.Marshal(decoded)
		require.NoError(t, err)
		require.JSONEq(t, intJsonString, string(encoded))
	})

	t.Run("IntMarshaling", func(t *testing.T) {

		decoded := new(tempNetworks)
		require.NoError(t, json.Unmarshal([]byte(intJsonString), decoded))

		require.Equal(t, WithdrawalNetwork("remote"), decoded.BaseFeeVaultWithdrawalNetwork)
		require.Equal(t, WithdrawalNetwork("local"), decoded.L1FeeVaultWithdrawalNetwork)
		require.Equal(t, WithdrawalNetwork("local"), decoded.SequencerFeeVaultWithdrawalNetwork)

		encoded, err := json.Marshal(decoded)
		require.NoError(t, err)
		require.JSONEq(t, intJsonString, string(encoded))
	})
}

func TestWithdrawalNetworkMarshalJSON(t *testing.T) {
	type test struct {
		Network WithdrawalNetwork
	}

	tests := []struct {
		network WithdrawalNetwork
		exp     string
	}{
		{WithdrawalNetwork("local"), `{"Network":1}`},
		{WithdrawalNetwork("remote"), `{"Network":0}`},
	}
	for _, tt := range tests {
		t.Run(string(tt.network), func(t *testing.T) {
			data, err := json.Marshal(test{tt.network})
			require.NoError(t, err)
			require.JSONEq(t, tt.exp, string(data))
		})
	}
}
