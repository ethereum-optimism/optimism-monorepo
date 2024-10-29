package interop

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/op-chain-ops/devkeys"
	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-chain-ops/interopgen"
	op_service "github.com/ethereum-optimism/optimism/op-service"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum/go-ethereum/crypto"
)

var EnvPrefix = "OP_INTEROP"

var (
	l1ChainIDFlag = &cli.Uint64Flag{
		Name:    "l1.chainid",
		Value:   900100,
		EnvVars: op_service.PrefixEnvVar(EnvPrefix, "L1_CHAINID"),
	}
	l2ChainIDsFlag = &cli.Uint64SliceFlag{
		Name:    "l2.chainids",
		Value:   cli.NewUint64Slice(900200, 900201),
		EnvVars: op_service.PrefixEnvVar(EnvPrefix, "L2_CHAINIDS"),
	}
	timestampFlag = &cli.Uint64Flag{
		Name:    "timestamp",
		Value:   0,
		EnvVars: op_service.PrefixEnvVar(EnvPrefix, "TIMESTAMP"),
		Usage:   "Will use current timestamp, plus 5 seconds, if not set",
	}
	artifactsDirFlag = &cli.StringFlag{
		Name:    "artifacts-dir",
		Value:   "packages/contracts-bedrock/forge-artifacts",
		EnvVars: op_service.PrefixEnvVar(EnvPrefix, "ARTIFACTS_DIR"),
	}
	foundryDirFlag = &cli.StringFlag{
		Name:    "foundry-dir",
		Value:   "packages/contracts-bedrock",
		EnvVars: op_service.PrefixEnvVar(EnvPrefix, "FOUNDRY_DIR"),
		Usage:   "Optional, for source-map info during genesis generation",
	}
	outDirFlag = &cli.StringFlag{
		Name:    "out-dir",
		Value:   ".interop-devnet",
		EnvVars: op_service.PrefixEnvVar(EnvPrefix, "OUT_DIR"),
	}
	// used in both dev-setup and devkey commands
	mnemonicFlag = &cli.StringFlag{
		Name:    "mnemonic",
		Value:   devkeys.TestMnemonic,
		EnvVars: op_service.PrefixEnvVar(EnvPrefix, "MNEMONIC"),
	}
	// for devkey command
	devkeyDomainFlag = &cli.StringFlag{
		Name:    "domain",
		Value:   "chain-operator",
		EnvVars: op_service.PrefixEnvVar(EnvPrefix, "DEVKEY_DOMAIN"),
	}
	devkeyChainIdFlag = &cli.Uint64Flag{
		Name:    "chainid",
		Value:   0,
		EnvVars: op_service.PrefixEnvVar(EnvPrefix, "DEVKEY_CHAINID"),
	}
	devkeyNameFlag = &cli.StringFlag{
		Name:    "name",
		EnvVars: op_service.PrefixEnvVar(EnvPrefix, "DEVKEY_NAME"),
	}
)

var InteropDevSetup = &cli.Command{
	Name:  "dev-setup",
	Usage: "Generate devnet genesis configs with one L1 and multiple L2s",
	Flags: cliapp.ProtectFlags(append([]cli.Flag{
		l1ChainIDFlag,
		l2ChainIDsFlag,
		timestampFlag,
		mnemonicFlag,
		artifactsDirFlag,
		foundryDirFlag,
		outDirFlag,
	}, oplog.CLIFlags(EnvPrefix)...)),
	Action: func(cliCtx *cli.Context) error {
		logCfg := oplog.ReadCLIConfig(cliCtx)
		logger := oplog.NewLogger(cliCtx.App.Writer, logCfg)

		recipe := &interopgen.InteropDevRecipe{
			L1ChainID:        cliCtx.Uint64(l1ChainIDFlag.Name),
			L2ChainIDs:       cliCtx.Uint64Slice(l2ChainIDsFlag.Name),
			GenesisTimestamp: cliCtx.Uint64(timestampFlag.Name),
		}
		if recipe.GenesisTimestamp == 0 {
			recipe.GenesisTimestamp = uint64(time.Now().Unix() + 5)
		}
		mnemonic := strings.TrimSpace(cliCtx.String(mnemonicFlag.Name))
		if mnemonic == devkeys.TestMnemonic {
			logger.Warn("Using default test mnemonic!")
		}
		keys, err := devkeys.NewMnemonicDevKeys(mnemonic)
		if err != nil {
			return fmt.Errorf("failed to setup dev keys from mnemonic: %w", err)
		}
		worldCfg, err := recipe.Build(keys)
		if err != nil {
			return fmt.Errorf("failed to build deploy configs from interop recipe: %w", err)
		}
		if err := worldCfg.Check(logger); err != nil {
			return fmt.Errorf("invalid deploy configs: %w", err)
		}
		artifactsDir := cliCtx.String(artifactsDirFlag.Name)
		af := foundry.OpenArtifactsDir(artifactsDir)
		var srcFs *foundry.SourceMapFS
		if cliCtx.IsSet(foundryDirFlag.Name) {
			srcDir := cliCtx.String(foundryDirFlag.Name)
			srcFs = foundry.NewSourceMapFS(os.DirFS(srcDir))
		}
		worldDeployment, worldOutput, err := interopgen.Deploy(logger, af, srcFs, worldCfg)
		if err != nil {
			return fmt.Errorf("failed to deploy interop dev setup: %w", err)
		}
		outDir := cliCtx.String(outDirFlag.Name)
		// Write deployments
		{
			deploymentsDir := filepath.Join(outDir, "deployments")
			l1Dir := filepath.Join(deploymentsDir, "l1")
			if err := writeJson(filepath.Join(l1Dir, "common.json"), worldDeployment.L1); err != nil {
				return fmt.Errorf("failed to write L1 deployment data: %w", err)
			}
			if err := writeJson(filepath.Join(l1Dir, "superchain.json"), worldDeployment.Superchain); err != nil {
				return fmt.Errorf("failed to write Superchain deployment data: %w", err)
			}
			l2sDir := filepath.Join(deploymentsDir, "l2")
			for id, dep := range worldDeployment.L2s {
				l2Dir := filepath.Join(l2sDir, id)
				if err := writeJson(filepath.Join(l2Dir, "addresses.json"), dep); err != nil {
					return fmt.Errorf("failed to write L2 %s deployment data: %w", id, err)
				}
			}
		}
		// write genesis
		{
			genesisDir := filepath.Join(outDir, "genesis")
			l1Dir := filepath.Join(genesisDir, "l1")
			if err := writeJson(filepath.Join(l1Dir, "genesis.json"), worldOutput.L1.Genesis); err != nil {
				return fmt.Errorf("failed to write L1 genesis data: %w", err)
			}
			l2sDir := filepath.Join(genesisDir, "l2")
			for id, dep := range worldOutput.L2s {
				l2Dir := filepath.Join(l2sDir, id)
				if err := writeJson(filepath.Join(l2Dir, "genesis.json"), dep.Genesis); err != nil {
					return fmt.Errorf("failed to write L2 %s genesis config: %w", id, err)
				}
				if err := writeJson(filepath.Join(l2Dir, "rollup.json"), dep.RollupCfg); err != nil {
					return fmt.Errorf("failed to write L2 %s rollup config: %w", id, err)
				}
			}
		}
		return nil
	},
}

func writeJson(path string, content any) error {
	return jsonutil.WriteJSON[any](content, ioutil.ToBasicFile(path, 0o755))
}

var DevKeySecretCmd = &cli.Command{
	Name:  "secret",
	Usage: "Retrieve devkey secret, by specifying domain, chain ID, name.",
	Flags: cliapp.ProtectFlags([]cli.Flag{
		mnemonicFlag,
		devkeyDomainFlag,
		devkeyChainIdFlag,
		devkeyNameFlag,
	}),
	Action: func(context *cli.Context) error {
		mnemonic := context.String(mnemonicFlag.Name)
		domain := context.String(devkeyDomainFlag.Name)
		chainID := context.Uint64(devkeyChainIdFlag.Name)
		chainIDBig := new(big.Int).SetUint64(chainID)
		name := context.String(devkeyNameFlag.Name)
		k, err := parseKey(domain, chainIDBig, name)
		if err != nil {
			return err
		}
		mnemonicKeys, err := devkeys.NewMnemonicDevKeys(mnemonic)
		if err != nil {
			return err
		}
		secret, err := mnemonicKeys.Secret(k)
		if err != nil {
			return err
		}
		secretBin := crypto.FromECDSA(secret)
		_, err = fmt.Fprintf(context.App.Writer, "%x", secretBin)
		if err != nil {
			return fmt.Errorf("failed to output secret key: %w", err)
		}
		return nil
	},
}

var DevKeyAddressCmd = &cli.Command{
	Name:  "address",
	Usage: "Retrieve devkey address, by specifying domain, chain ID, name.",
	Flags: cliapp.ProtectFlags([]cli.Flag{
		mnemonicFlag,
		devkeyDomainFlag,
		devkeyChainIdFlag,
		devkeyNameFlag,
	}),
	Action: func(context *cli.Context) error {
		mnemonic := context.String(mnemonicFlag.Name)
		domain := context.String(devkeyDomainFlag.Name)
		chainID := context.Uint64(devkeyChainIdFlag.Name)
		chainIDBig := new(big.Int).SetUint64(chainID)
		name := context.String(devkeyNameFlag.Name)
		k, err := parseKey(domain, chainIDBig, name)
		if err != nil {
			return err
		}
		mnemonicKeys, err := devkeys.NewMnemonicDevKeys(mnemonic)
		if err != nil {
			return err
		}
		addr, err := mnemonicKeys.Address(k)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(context.App.Writer, "%s", addr)
		if err != nil {
			return fmt.Errorf("failed to output address: %w", err)
		}
		return nil
	},
}

var DevKeyCmd = &cli.Command{
	Name:  "devkey",
	Usage: "Retrieve devkey secret or address",
	Subcommands: cli.Commands{
		DevKeySecretCmd,
		DevKeyAddressCmd,
	},
}

func parseKey(domain string, chainID *big.Int, name string) (devkeys.Key, error) {
	switch domain {
	case "user":
		index, err := strconv.ParseUint(name, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse user index: %w", err)
		}
		return devkeys.ChainUserKey{
			ChainID: chainID,
			Index:   index,
		}, nil
	case "chain-operator":
		var role devkeys.ChainOperatorRole
		if err := role.UnmarshalText([]byte(name)); err != nil {
			return nil, fmt.Errorf("failed to parse chain operator role: %w", err)
		}
		return devkeys.ChainOperatorKey{
			ChainID: chainID,
			Role:    role,
		}, nil
	case "superchain-operator":
		var role devkeys.SuperchainOperatorRole
		if err := role.UnmarshalText([]byte(name)); err != nil {
			return nil, fmt.Errorf("failed to parse chain operator role: %w", err)
		}
		return devkeys.SuperchainOperatorKey{
			ChainID: chainID,
			Role:    role,
		}, nil
	default:
		return nil, fmt.Errorf("unknown devkey domain %q", domain)
	}
}

var InteropCmd = &cli.Command{
	Name:  "interop",
	Usage: "Experimental tools for OP-Stack interop networks.",
	Subcommands: cli.Commands{
		InteropDevSetup,
		DevKeyCmd,
	},
}
