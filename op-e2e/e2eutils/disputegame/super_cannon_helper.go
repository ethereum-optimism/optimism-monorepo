package disputegame

import (
	"crypto/ecdsa"
	"testing"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/super"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/challenger"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

type SuperCannonGameHelper struct {
	SuperGameHelper
	CannonHelper
}

func NewSuperCannonGameHelper(t *testing.T, client *ethclient.Client, opts *bind.TransactOpts, key *ecdsa.PrivateKey, game contracts.FaultDisputeGameContract, factoryAddr common.Address, gameAddr common.Address, provider *super.SuperTraceProvider, system DisputeSystem) *SuperCannonGameHelper {
	superGameHelper := NewSuperGameHelper(t, require.New(t), client, opts, key, game, factoryAddr, gameAddr, provider, system)
	defaultChallengerOptions := func() []challenger.Option {
		return []challenger.Option{
			challenger.WithCannon(t, system),
			challenger.WithFactoryAddress(factoryAddr),
			challenger.WithGameAddress(gameAddr),
		}
	}
	return &SuperCannonGameHelper{
		SuperGameHelper: *superGameHelper,
		CannonHelper:    *NewCannonHelper(&superGameHelper.SplitGameHelper, defaultChallengerOptions),
	}
}
