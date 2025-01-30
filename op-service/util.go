package op_service

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/ethereum-optimism/optimism/op-service/ctxinterrupt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/urfave/cli/v2"
)

// PrefixEnvVar adds a prefix to the environment variable,
// and returns the env-var wrapped in a slice for usage with urfave CLI v2.
func PrefixEnvVar(prefix, suffix string) []string {
	return []string{prefix + "_" + suffix}
}

// ValidateEnvVars logs all env vars that are found where the env var is
// prefixed with the supplied prefix (like OP_BATCHER) but there is no
// actual env var with that name.
// It helps validate that the supplied env vars are in fact valid.
func ValidateEnvVars(prefix string, flags []cli.Flag, log log.Logger) {
	for _, envVar := range validateEnvVars(prefix, os.Environ(), cliFlagsToEnvVars(flags)) {
		log.Warn("Unknown env var", "prefix", prefix, "env_var", envVar)
	}
}

func FlagNameToEnvVarName(f string, prefix string) string {
	f = strings.ReplaceAll(strings.ReplaceAll(strings.ToUpper(f), ".", "_"), "-", "_")
	return fmt.Sprintf("%s_%s", prefix, f)
}

func cliFlagsToEnvVars(flags []cli.Flag) map[string]struct{} {
	definedEnvVars := make(map[string]struct{})
	for _, flag := range flags {
		envVars := reflect.ValueOf(flag).Elem().FieldByName("EnvVars")
		for i := 0; i < envVars.Len(); i++ {
			envVarField := envVars.Index(i)
			definedEnvVars[envVarField.String()] = struct{}{}
		}
	}
	return definedEnvVars
}

// validateEnvVars returns a list of the unknown environment variables that match the prefix.
func validateEnvVars(prefix string, providedEnvVars []string, definedEnvVars map[string]struct{}) []string {
	var out []string
	for _, envVar := range providedEnvVars {
		parts := strings.Split(envVar, "=")
		if len(parts) == 0 {
			continue
		}
		key := parts[0]
		if strings.HasPrefix(key, prefix) {
			if _, ok := definedEnvVars[key]; !ok {
				out = append(out, envVar)
			}
		}
	}
	return out
}

// WarnOnDeprecatedFlags iterates through the provided deprecatedFlags and logs a warning for each that is set.
func WarnOnDeprecatedFlags(ctx *cli.Context, deprecatedFlags []cli.Flag, log log.Logger) {
	for _, flag := range deprecatedFlags {
		if ctx.IsSet(flag.Names()[0]) {
			log.Warn("Found a deprecated flag which will be removed in a future version", "flag_name", flag.Names()[0])
		}
	}
}

// ParseAddress parses an ETH address from a hex string. This method will fail if
// the address is not a valid hexadecimal address.
func ParseAddress(address string) (common.Address, error) {
	if common.IsHexAddress(address) {
		return common.HexToAddress(address), nil
	}
	return common.Address{}, fmt.Errorf("invalid address: %v", address)
}

// CloseAction runs the function in the background, until it finishes or until it is closed by the
// user with an interrupt.
func CloseAction(ctx context.Context, fn func(ctx context.Context) error) error {
	ctx, stop := ctxinterrupt.WithSignalWaiter(ctx)
	defer stop()
	finished := make(chan error, 1)
	go func() {
		finished <- fn(ctx)
	}()

	select {
	case <-ctx.Done():
		// Stop catching interrupts.
		stop()
		select {
		case err := <-finished:
			return err
		case <-time.After(time.Second * 10):
			return errors.New("command action is unresponsive for more than 10 seconds... shutting down")
		}
	case err := <-finished:
		return err
	}
}

// FindMonorepoRoot will recursively search upwards for a go.mod file.
// This depends on the structure of the monorepo having a go.mod file at the root.
func FindMonorepoRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		modulePath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(modulePath); err == nil {
			return dir, nil
		}
		parentDir := filepath.Dir(dir)
		// Check if we reached the filesystem root
		if parentDir == dir {
			break
		}
		dir = parentDir
	}
	return "", errors.New("monorepo root not found")
}

// Parse256BitChainID parses a 256-bit chain ID from a string. Chain IDs
// can be defined as either an integer or a hex string. If the string
// starts with "0x", it is treated as a hex string, otherwise it is
// treated as an integer string.
func Parse256BitChainID(in string) (common.Hash, error) {
	var chainIDBig *big.Int
	if strings.HasPrefix(in, "0x") {
		in = strings.TrimPrefix(in, "0x")
		var ok bool
		chainIDBig, ok = new(big.Int).SetString(in, 16)
		if !ok {
			return common.Hash{}, fmt.Errorf("failed to parse chain ID %s", in)
		}
	} else {
		inUint, err := strconv.ParseUint(in, 10, 64)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to parse chain ID %s: %w", in, err)
		}

		chainIDBig = new(big.Int).SetUint64(inUint)
	}

	if chainIDBig.BitLen() > 256 {
		return common.Hash{}, fmt.Errorf("chain ID %s is too large", in)
	}

	return common.BigToHash(chainIDBig), nil
}

func U64UtilPtr(in uint64) *hexutil.Uint64 {
	util := hexutil.Uint64(in)
	return &util
}
