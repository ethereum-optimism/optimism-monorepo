package runcfg

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"

	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/endpoint"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

func TestRecommendedProtocolVersionChange(t *testing.T) {
	op_e2e.InitParallel(t)

	cfg := e2esys.DefaultSystemConfig(t)
	require.NotEqual(t, common.Address{}, cfg.L1Deployments.ProtocolVersions, "need ProtocolVersions contract deployment")
	// to speed up the test, make it reload the config more often, and do not impose a long conf depth
	cfg.Nodes["verifier"].RuntimeConfigReloadInterval = time.Second * 5
	cfg.Nodes["verifier"].Driver.VerifierConfDepth = 1

	sys, err := cfg.Start(t)
	require.Nil(t, err, "Error starting up system")

	runtimeConfig := sys.RollupNodes["verifier"].RuntimeConfig()

	// Change the superchain-config via L1
	l1 := sys.NodeClient("l1")

	_, build, major, minor, patch, preRelease := params.OPStackSupport.Parse()
	newRecommendedProtocolVersion := params.ProtocolVersionV0{Build: build, Major: major + 1, Minor: minor, Patch: patch, PreRelease: preRelease}.Encode()
	require.NotEqual(t, runtimeConfig.RecommendedProtocolVersion(), newRecommendedProtocolVersion, "changing to a different protocol version")

	protVersions, err := bindings.NewProtocolVersions(cfg.L1Deployments.ProtocolVersionsProxy, l1)
	require.NoError(t, err)

	// ProtocolVersions contract is owned by the deployer in devnet
	opts, err := bind.NewKeyedTransactorWithChainID(cfg.Secrets.Deployer, cfg.L1ChainIDBig())
	require.NoError(t, err)

	// Change recommended protocol version
	tx, err := protVersions.SetRecommended(opts, new(big.Int).SetBytes(newRecommendedProtocolVersion[:]))
	require.NoError(t, err)

	// wait for the change to confirm
	_, err = wait.ForReceiptOK(context.Background(), l1, tx.Hash())
	require.NoError(t, err)

	// wait for the recommended protocol version to change
	_, err = retry.Do(context.Background(), 10, retry.Fixed(time.Second*10), func() (struct{}, error) {
		v := sys.RollupNodes["verifier"].RuntimeConfig().RecommendedProtocolVersion()
		if v == newRecommendedProtocolVersion {
			return struct{}{}, nil
		}
		return struct{}{}, fmt.Errorf("no change yet, seeing %s but looking for %s", v, newRecommendedProtocolVersion)
	})
	require.NoError(t, err)
}

func TestRequiredProtocolVersionChangeAndHalt(t *testing.T) {
	op_e2e.InitParallel(t)

	cfg := e2esys.DefaultSystemConfig(t)
	// to speed up the test, make it reload the config more often, and do not impose a long conf depth
	cfg.Nodes["verifier"].RuntimeConfigReloadInterval = time.Second * 5
	cfg.Nodes["verifier"].Driver.VerifierConfDepth = 1
	// configure halt in verifier op-node
	cfg.Nodes["verifier"].RollupHalt = "major"
	// configure halt in verifier op-geth node
	cfg.GethOptions["verifier"] = append(cfg.GethOptions["verifier"], []geth.GethOption{
		func(ethCfg *ethconfig.Config, nodeCfg *node.Config) error {
			ethCfg.RollupHaltOnIncompatibleProtocolVersion = "major"
			return nil
		},
	}...)

	sys, err := cfg.Start(t)
	require.Nil(t, err, "Error starting up system")

	runtimeConfig := sys.RollupNodes["verifier"].RuntimeConfig()

	// Change the superchain-config via L1
	l1 := sys.NodeClient("l1")

	_, build, major, minor, patch, preRelease := params.OPStackSupport.Parse()
	newRequiredProtocolVersion := params.ProtocolVersionV0{Build: build, Major: major + 1, Minor: minor, Patch: patch, PreRelease: preRelease}.Encode()
	require.NotEqual(t, runtimeConfig.RequiredProtocolVersion(), newRequiredProtocolVersion, "changing to a different protocol version")

	protVersions, err := bindings.NewProtocolVersions(cfg.L1Deployments.ProtocolVersionsProxy, l1)
	require.NoError(t, err)

	// ProtocolVersions contract is owned by same key as SystemConfig in devnet
	opts, err := bind.NewKeyedTransactorWithChainID(cfg.Secrets.Deployer, cfg.L1ChainIDBig())
	require.NoError(t, err)

	// Change required protocol version
	tx, err := protVersions.SetRequired(opts, new(big.Int).SetBytes(newRequiredProtocolVersion[:]))
	require.NoError(t, err)

	// wait for the change to confirm
	_, err = wait.ForReceiptOK(context.Background(), l1, tx.Hash())
	require.NoError(t, err)

	// wait for the required protocol version to take effect by halting the verifier that opted in, and halting the op-geth node that opted in.
	_, err = retry.Do(context.Background(), 10, retry.Fixed(time.Second*10), func() (struct{}, error) {
		if !sys.RollupNodes["verifier"].Stopped() {
			return struct{}{}, errors.New("verifier rollup node is not closed yet")
		}
		return struct{}{}, nil
	})
	require.NoError(t, err)
	t.Log("verified that op-node closed!")
	// Checking if the engine is down is not trivial in op-e2e.
	// In op-geth we have halting tests covering the Engine API, in op-e2e we instead check if the API stops.
	_, err = retry.Do(context.Background(), 10, retry.Fixed(time.Second*10), func() (struct{}, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		available := client.IsURLAvailable(ctx, sys.NodeEndpoint("verifier").(endpoint.HttpRPC).HttpRPC())
		if !available && ctx.Err() == nil { // waiting for client to stop responding to RPC requests (slow dials with timeout don't count)
			return struct{}{}, nil
		}
		return struct{}{}, errors.New("verifier EL node is not closed yet")
	})
	require.NoError(t, err)
	t.Log("verified that op-geth closed!")
}
