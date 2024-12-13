package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

func SetStartBlockLiveStrategy(ctx context.Context, env *Env, st *state.State, chainID common.Hash) error {
	lgr := env.Logger.New("stage", "set-start-block", "strategy", "live")
	lgr.Info("setting start block", "id", chainID.Hex())

	thisChainState, err := st.Chain(chainID)
	if err != nil {
		return fmt.Errorf("failed to get chain state: %w", err)
	}

	startHeader, err := env.L1Client.HeaderByNumber(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get start block: %w", err)
	}
	thisChainState.StartBlock = startHeader

	return nil
}

func SetStartBlockGenesisStrategy(env *Env, st *state.State, chainID common.Hash) error {
	lgr := env.Logger.New("stage", "set-start-block", "strategy", "genesis")
	lgr.Info("setting start block", "id", chainID.Hex())

	thisChainState, err := st.Chain(chainID)
	if err != nil {
		return fmt.Errorf("failed to get chain state: %w", err)
	}

	deployConfig := &genesis.DeployConfig{
		DevL1DeployConfig: genesis.DevL1DeployConfig{
			L1BlockTime:             12,
			L1GenesisBlockTimestamp: hexutil.Uint64(time.Now().Unix()),
		},
		L2InitializationConfig: genesis.L2InitializationConfig{
			L2CoreDeployConfig: genesis.L2CoreDeployConfig{
				L1ChainID: 900,
			},
			DevDeployConfig: genesis.DevDeployConfig{
				FundDevAccounts: true,
			},
		},
	}

	devGenesis, err := genesis.BuildL1DeveloperGenesis(deployConfig, st.L1StateDump.Data, &genesis.L1Deployments{})
	if err != nil {
		return fmt.Errorf("failed to build L1 developer genesis: %w", err)
	}
	thisChainState.StartBlock = devGenesis.ToBlock().Header()

	return nil
}
