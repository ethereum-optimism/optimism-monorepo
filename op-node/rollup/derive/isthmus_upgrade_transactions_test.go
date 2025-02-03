package derive

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
)

func TestIsthmusSourcesMatchSpec(t *testing.T) {
	for _, test := range []struct {
		source       UpgradeDepositSource
		expectedHash string
	}{
		{
			source:       blockHashDeployerSource,
			expectedHash: "0xbfb734dae514c5974ddf803e54c1bc43d5cdb4a48ae27e1d9b875a5a150b553a",
		},
	} {
		require.Equal(t, common.HexToHash(test.expectedHash), test.source.SourceHash())
	}
}

func TestIsthmusNetworkTransactions(t *testing.T) {
	upgradeTxns, err := IsthmusNetworkUpgradeTransactions()
	require.NoError(t, err)
	require.Len(t, upgradeTxns, 1)

	deployBlockHashesSender, deployBlockHashesContract := toDepositTxn(t, upgradeTxns[0])
	require.Equal(t, deployBlockHashesSender, common.HexToAddress("0xE9f0662359Bb2c8111840eFFD73B9AFA77CbDE10"))
	require.Equal(t, blockHashDeployerSource.SourceHash(), deployBlockHashesContract.SourceHash())
	require.Nil(t, deployBlockHashesContract.To())
	require.Equal(t, uint64(250_000), deployBlockHashesContract.Gas())
	require.Equal(t, blockHashDeploymentBytecode, deployBlockHashesContract.Data())
}
