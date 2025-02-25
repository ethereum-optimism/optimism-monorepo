package derive

import (
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-chain-ops/interopgen"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
)

func TestJovianSourcesMatchSpec(t *testing.T) {
	for _, test := range []struct {
		source       UpgradeDepositSource
		expectedHash string
	}{
		{
			source:       deployJovianL1BlockSource,
			expectedHash: "0xbb1a656f65401240fac3db12e7a79ebb954b11e62f7626eb11691539b798d3bf",
		},
	} {
		require.Equal(t, common.HexToHash(test.expectedHash), test.source.SourceHash())
	}
}

func TestJovianNetworkTransactions(t *testing.T) {
	upgradeTxns, err := JovianNetworkUpgradeTransactions()
	require.NoError(t, err)
	require.Len(t, upgradeTxns, 1)

	deployL1BlockSender, deployL1Block := toDepositTxn(t, upgradeTxns[0])
	require.Equal(t, deployL1BlockSender, common.HexToAddress("0x4210000000000000000000000000000000000005"))
	require.Equal(t, deployJovianL1BlockSource.SourceHash(), deployL1Block.SourceHash())
	require.Nil(t, deployL1Block.To())
	require.Equal(t, uint64(375_000), deployL1Block.Gas())
	require.Equal(t, jovianL1BlockDeploymentBytecode, deployL1Block.Data())

	l1 := interopgen.CreateL1(log.Root(), nil, nil, &interopgen.L1Config{
		ChainID: big.NewInt(1337),
	})
	address, err := l1.Create(deployL1BlockSender, jovianL1BlockDeploymentBytecode)
	require.NoError(t, err)
	require.Equal(t, address, common.HexToAddress("0x4fa2Be8cd41504037F1838BcE3bCC93bC68Ff537"))
	codeHash := crypto.Keccak256Hash(l1.GetCode(address))
	require.Equal(t, codeHash, common.HexToHash("0xea1f176e3bcab831c781395fca0974d470ea540e602c230b471814fb43883e74"))
}
