package disputegame

import (
	"crypto/ecdsa"
	"testing"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/super"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

type SuperGameHelper struct {
	SplitGameHelper
}

func NewSuperGameHelper(t *testing.T, require *require.Assertions, client *ethclient.Client, opts *bind.TransactOpts, privKey *ecdsa.PrivateKey,
	game contracts.FaultDisputeGameContract, factoryAddr common.Address, addr common.Address, correctOutputProvider *super.SuperTraceProvider, system DisputeSystem) *SuperGameHelper {
	return &SuperGameHelper{
		SplitGameHelper: SplitGameHelper{
			T:                     t,
			Require:               require,
			Client:                client,
			Opts:                  opts,
			PrivKey:               privKey,
			Game:                  game,
			FactoryAddr:           factoryAddr,
			Addr:                  addr,
			CorrectOutputProvider: correctOutputProvider,
			System:                system,
			DescribePosition: func(pos types.Position, splitDepth types.Depth) string {
				return ""
			},
		},
	}
}
