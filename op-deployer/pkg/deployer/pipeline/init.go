package pipeline

import (
	"context"
	"crypto/rand"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"

	"github.com/ethereum/go-ethereum/common"
)

func IsSupportedStateVersion(version int) bool {
	return version == 1
}

func InitLiveStrategy(ctx context.Context, env *Env, intent *state.Intent, st *state.State) error {
	lgr := env.Logger.New("stage", "init", "strategy", "live")
	lgr.Info("initializing pipeline")

	if err := initCommonChecks(st); err != nil {
		return err
	}

	if intent.L1ContractsLocator.IsTag() {
		superCfg, err := standard.SuperchainFor(intent.L1ChainID)
		if err != nil {
			return fmt.Errorf("error getting superchain config: %w", err)
		}

		proxyAdmin, err := standard.ManagerOwnerAddrFor(intent.L1ChainID)
		if err != nil {
			return fmt.Errorf("error getting superchain proxy admin address: %w", err)
		}

		// Have to do this weird pointer thing below because the Superchain Registry defines its
		// own Address type.
		st.SuperchainDeployment = &state.SuperchainDeployment{
			ProxyAdminAddress:            proxyAdmin,
			ProtocolVersionsProxyAddress: common.Address(*superCfg.Config.ProtocolVersionsAddr),
			SuperchainConfigProxyAddress: common.Address(*superCfg.Config.SuperchainConfigAddr),
		}

		opcmAddress, err := standard.ManagerImplementationAddrFor(intent.L1ChainID)
		if err != nil {
			return fmt.Errorf("error getting OPCM proxy address: %w", err)
		}
		st.ImplementationsDeployment = &state.ImplementationsDeployment{
			OpcmAddress: opcmAddress,
		}
	}

	l1ChainID, err := env.L1Client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get L1 chain ID: %w", err)
	}

	if l1ChainID.Cmp(intent.L1ChainIDBig()) != 0 {
		return fmt.Errorf("l1 chain ID mismatch: got %d, expected %d", l1ChainID, intent.L1ChainID)
	}

	deployerCode, err := env.L1Client.CodeAt(ctx, script.DeterministicDeployerAddress, nil)
	if err != nil {
		return fmt.Errorf("failed to get deployer code: %w", err)
	}
	if len(deployerCode) == 0 {
		return fmt.Errorf("deterministic deployer is not deployed on this chain - please deploy it first")
	}

	// If the state has never been applied, we don't need to perform
	// any additional checks.
	if st.AppliedIntent == nil {
		return nil
	}

	// If the state has been applied, we need to check if any immutable
	// fields have changed.
	if st.AppliedIntent.L1ChainID != intent.L1ChainID {
		return immutableErr("L1ChainID", st.AppliedIntent.L1ChainID, intent.L1ChainID)
	}

	if st.AppliedIntent.FundDevAccounts != intent.FundDevAccounts {
		return immutableErr("fundDevAccounts", st.AppliedIntent.FundDevAccounts, intent.FundDevAccounts)
	}

	// TODO: validate individual

	return nil
}

func initCommonChecks(st *state.State) error {
	// Ensure the state version is supported.
	if !IsSupportedStateVersion(st.Version) {
		return fmt.Errorf("unsupported state version: %d", st.Version)
	}

	if st.Create2Salt == (common.Hash{}) {
		_, err := rand.Read(st.Create2Salt[:])
		if err != nil {
			return fmt.Errorf("failed to generate CREATE2 salt: %w", err)
		}
	}
	return nil
}

func InitGenesisStrategy(env *Env, intent *state.Intent, st *state.State) error {
	lgr := env.Logger.New("stage", "init", "strategy", "genesis")
	lgr.Info("initializing pipeline")

	if err := initCommonChecks(st); err != nil {
		return err
	}

	if intent.SuperchainRoles == nil {
		return fmt.Errorf("superchain roles must be set for genesis strategy")
	}

	// Mostly a stub for now.

	return nil
}

func immutableErr(field string, was, is any) error {
	return fmt.Errorf("%s is immutable: was %v, is %v", field, was, is)
}
