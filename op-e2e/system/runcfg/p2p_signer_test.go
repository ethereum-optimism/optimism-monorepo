package runcfg

import (
	"context"
	"fmt"
	"testing"
	"time"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"

	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestRuntimeConfigReload(t *testing.T) {
	op_e2e.InitParallel(t)

	cfg := e2esys.DefaultSystemConfig(t)
	// to speed up the test, make it reload the config more often, and do not impose a long conf depth
	cfg.Nodes["verifier"].RuntimeConfigReloadInterval = time.Second * 5
	cfg.Nodes["verifier"].Driver.VerifierConfDepth = 1

	sys, err := cfg.Start(t)
	require.Nil(t, err, "Error starting up system")

	initialRuntimeConfig := sys.RollupNodes["verifier"].RuntimeConfig()

	// close the EL node, since we want to block derivation, to solely rely on the reloading mechanism for updates.
	sys.EthInstances["verifier"].Close()

	l1 := sys.NodeClient("l1")

	// Change the system-config via L1
	sysCfgContract, err := bindings.NewSystemConfig(cfg.L1Deployments.SystemConfigProxy, l1)
	require.NoError(t, err)
	newUnsafeBlocksSigner := common.Address{0x12, 0x23, 0x45}
	require.NotEqual(t, initialRuntimeConfig.P2PSequencerAddress(), newUnsafeBlocksSigner, "changing to a different address")
	opts, err := bind.NewKeyedTransactorWithChainID(cfg.Secrets.Deployer, cfg.L1ChainIDBig())
	require.Nil(t, err)
	// the unsafe signer address is part of the runtime config
	tx, err := sysCfgContract.SetUnsafeBlockSigner(opts, newUnsafeBlocksSigner)
	require.NoError(t, err)

	// wait for the change to confirm
	_, err = wait.ForReceiptOK(context.Background(), l1, tx.Hash())
	require.NoError(t, err)

	// wait for the address to change
	_, err = retry.Do(context.Background(), 10, retry.Fixed(time.Second*10), func() (struct{}, error) {
		v := sys.RollupNodes["verifier"].RuntimeConfig().P2PSequencerAddress()
		if v == newUnsafeBlocksSigner {
			return struct{}{}, nil
		}
		return struct{}{}, fmt.Errorf("no change yet, seeing %s but looking for %s", v, newUnsafeBlocksSigner)
	})
	require.NoError(t, err)
}
