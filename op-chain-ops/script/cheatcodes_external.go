package script

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"golang.org/x/exp/maps"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
)

// Ffi implements https://book.getfoundry.sh/cheatcodes/ffi
func (c *CheatCodesPrecompile) Ffi(args []string) ([]byte, error) {
	return nil, vm.ErrExecutionReverted
}

// Prompt implements https://book.getfoundry.sh/cheatcodes/prompt
func (c *CheatCodesPrecompile) Prompt() error {
	return vm.ErrExecutionReverted
}

// ProjectRoot implements https://book.getfoundry.sh/cheatcodes/project-root
func (c *CheatCodesPrecompile) ProjectRoot() string {
	return ""
}

func (c *CheatCodesPrecompile) getArtifact(input string) (*foundry.Artifact, error) {
	// fetching by relative file path, or using a contract version, is not supported
	parts := strings.SplitN(input, ":", 2)
	name := parts[0] + ".sol"
	contract := parts[0]
	if len(parts) == 2 {
		name = parts[0]
		contract = parts[1]
	}
	return c.h.af.ReadArtifact(name, contract)
}

// GetCode implements https://book.getfoundry.sh/cheatcodes/get-code
func (c *CheatCodesPrecompile) GetCode(input string) ([]byte, error) {
	artifact, err := c.getArtifact(input)
	if err != nil {
		return nil, err
	}
	return bytes.Clone(artifact.Bytecode.Object), nil
}

// GetDeployedCode implements https://book.getfoundry.sh/cheatcodes/get-deployed-code
func (c *CheatCodesPrecompile) GetDeployedCode(input string) ([]byte, error) {
	artifact, err := c.getArtifact(input)
	if err != nil {
		return nil, err
	}
	return bytes.Clone(artifact.DeployedBytecode.Object), nil
}

// Sleep implements https://book.getfoundry.sh/cheatcodes/sleep
func (c *CheatCodesPrecompile) Sleep(ms *big.Int) error {
	if !ms.IsUint64() {
		return vm.ErrExecutionReverted
	}
	time.Sleep(time.Duration(ms.Uint64()) * time.Millisecond)
	return nil
}

// UnixTime implements https://book.getfoundry.sh/cheatcodes/unix-time
func (c *CheatCodesPrecompile) UnixTime() (ms *big.Int) {
	return big.NewInt(time.Now().UnixMilli())
}

// SetEnv implements https://book.getfoundry.sh/cheatcodes/set-env
func (c *CheatCodesPrecompile) SetEnv(key string, value string) error {
	if key == "" {
		return errors.New("env key must not be empty")
	}
	if strings.ContainsRune(key, '=') {
		return errors.New("env key must not contain = sign")
	}
	if strings.ContainsRune(key, 0) {
		return errors.New("env key must not contain NUL")
	}
	if strings.ContainsRune(value, 0) {
		return errors.New("env value must not contain NUL")
	}
	c.h.SetEnvVar(key, value)
	return nil
}

// helper function for EnvOr, implements https://book.getfoundry.sh/cheatcodes/env-or
func envOrSingular[E any](key string,
	getFn func(key string) (string, bool),
	fn func(v string) (E, error),
	defaultValue E,
) (out E, err error) {
	envValue, ok := getFn(key)
	if !ok {
		return defaultValue, nil
	}
	v, err := fn(envValue)
	if err != nil {
		return out, fmt.Errorf("failed to parse env var %q: %w", key, err)
	}
	return v, nil
}

// helper function for EnvOr, implements https://book.getfoundry.sh/cheatcodes/env-or
func envOrList[E any](key string,
	getFn func(key string) (string, bool),
	delimiter string, fn func(v string) (E, error),
	defaultValues []E,
) ([]E, error) {
	envValue, ok := getFn(key)
	if !ok {
		return defaultValues, nil
	}
	parts := strings.Split(envValue, delimiter)
	out := make([]E, len(parts))
	for i, p := range parts {
		v, err := fn(p)
		if err != nil {
			return nil, fmt.Errorf("failed to parse entry %d of envVar %q: %w", i, key, err)
		}
		out[i] = v
	}
	return out, nil
}

func (c *CheatCodesPrecompile) EnvOr_4777f3cf(key string, defaultValue bool) (bool, error) {
	return envOrSingular(key, c.h.GetEnvVar, c.ParseBool, defaultValue)
}
func (c *CheatCodesPrecompile) EnvOr_5e97348f(key string, defaultValue *big.Int) (*big.Int, error) {
	return envOrSingular(key, c.h.GetEnvVar, c.ParseUint, defaultValue)
}
func (c *CheatCodesPrecompile) EnvOr_bbcb713e(key string, defaultValue *ABIInt256) (*ABIInt256, error) {
	return envOrSingular(key, c.h.GetEnvVar, c.ParseInt, defaultValue)
}
func (c *CheatCodesPrecompile) EnvOr_561fe540(key string, defaultValue common.Address) (common.Address, error) {
	return envOrSingular(key, c.h.GetEnvVar, c.ParseAddress, defaultValue)
}
func (c *CheatCodesPrecompile) EnvOr_b4a85892(key string, defaultValue [32]byte) ([32]byte, error) {
	return envOrSingular(key, c.h.GetEnvVar, c.ParseBytes32, defaultValue)
}
func (c *CheatCodesPrecompile) EnvOr_d145736c(key string, defaultValue string) (string, error) {
	return envOrSingular(key, c.h.GetEnvVar, func(v string) (string, error) {
		return v, nil
	}, defaultValue)
}
func (c *CheatCodesPrecompile) EnvOr_b3e47705(key string, defaultValue []byte) ([]byte, error) {
	return envOrSingular(key, c.h.GetEnvVar, c.ParseBytes, defaultValue)
}

func (c *CheatCodesPrecompile) EnvOr_eb85e83b(key string, delimiter string, defaultValue []bool) ([]bool, error) {
	return envOrList(key, c.h.GetEnvVar, delimiter, c.ParseBool, defaultValue)
}
func (c *CheatCodesPrecompile) EnvOr_74318528(key string, delimiter string, defaultValue []*big.Int) ([]*big.Int, error) {
	return envOrList(key, c.h.GetEnvVar, delimiter, c.ParseUint, defaultValue)
}
func (c *CheatCodesPrecompile) EnvOr_4700d74b(key string, delimiter string, defaultValue []*ABIInt256) ([]*ABIInt256, error) {
	return envOrList(key, c.h.GetEnvVar, delimiter, c.ParseInt, defaultValue)
}
func (c *CheatCodesPrecompile) EnvOr_c74e9deb(key string, delimiter string, defaultValue []common.Address) ([]common.Address, error) {
	return envOrList(key, c.h.GetEnvVar, delimiter, c.ParseAddress, defaultValue)
}
func (c *CheatCodesPrecompile) EnvOr_2281f367(key string, delimiter string, defaultValue [][32]byte) ([][32]byte, error) {
	return envOrList(key, c.h.GetEnvVar, delimiter, c.ParseBytes32, defaultValue)
}
func (c *CheatCodesPrecompile) EnvOr_859216bc(key string, delimiter string, defaultValue []string) ([]string, error) {
	return envOrList(key, c.h.GetEnvVar, delimiter, func(v string) (string, error) {
		return v, nil
	}, defaultValue)
}
func (c *CheatCodesPrecompile) EnvOr_64bc3e64(key string, delimiter string, defaultValue [][]byte) ([][]byte, error) {
	return envOrList(key, c.h.GetEnvVar, delimiter, c.ParseBytes, defaultValue)
}

func envSingular[E any](key string,
	getFn func(key string) (string, bool),
	fn func(v string) (E, error),
) (out E, err error) {
	envValue, ok := getFn(key)
	if !ok {
		return out, fmt.Errorf("environment variable %q not found", key)
	}
	v, err := fn(envValue)
	if err != nil {
		return out, fmt.Errorf("failed to parse env var %q: %w", key, err)
	}
	return v, nil
}

func envList[E any](key string,
	getFn func(key string) (string, bool),
	delimiter string, fn func(v string) (E, error),
) ([]E, error) {
	envValue, ok := getFn(key)
	if !ok {
		return nil, fmt.Errorf("environment variable %q not found", key)
	}
	parts := strings.Split(envValue, delimiter)
	out := make([]E, len(parts))
	for i, p := range parts {
		v, err := fn(p)
		if err != nil {
			return nil, fmt.Errorf("failed to parse entry %d of envVar %q: %w", i, key, err)
		}
		out[i] = v
	}
	return out, nil
}

// EnvBool implements https://book.getfoundry.sh/cheatcodes/env-bool
func (c *CheatCodesPrecompile) EnvBool_7ed1ec7d(key string) (bool, error) {
	return envSingular[bool](key, c.h.GetEnvVar, c.ParseBool)
}

func (c *CheatCodesPrecompile) EnvBool_aaaddeaf(key string, delimiter string) ([]bool, error) {
	return envList[bool](key, c.h.GetEnvVar, delimiter, c.ParseBool)
}

// EnvUint implements https://book.getfoundry.sh/cheatcodes/env-uint
func (c *CheatCodesPrecompile) EnvUint_c1978d1f(key string) (*big.Int, error) {
	return envSingular[*big.Int](key, c.h.GetEnvVar, c.ParseUint)
}

func (c *CheatCodesPrecompile) EnvUint_f3dec099(key string, delimiter string) ([]*big.Int, error) {
	return envList[*big.Int](key, c.h.GetEnvVar, delimiter, c.ParseUint)
}

// EnvInt implements https://book.getfoundry.sh/cheatcodes/env-int
func (c *CheatCodesPrecompile) EnvInt_892a0c61(key string) (*ABIInt256, error) {
	return envSingular[*ABIInt256](key, c.h.GetEnvVar, c.ParseInt)
}

func (c *CheatCodesPrecompile) EnvInt_42181150(key string, delimiter string) ([]*ABIInt256, error) {
	return envList[*ABIInt256](key, c.h.GetEnvVar, delimiter, c.ParseInt)
}

// EnvAddress implements https://book.getfoundry.sh/cheatcodes/env-address
func (c *CheatCodesPrecompile) EnvAddress_350d56bf(key string) (common.Address, error) {
	return envSingular[common.Address](key, c.h.GetEnvVar, c.ParseAddress)
}

func (c *CheatCodesPrecompile) EnvAddress_ad31b9fa(key string, delimiter string) ([]common.Address, error) {
	return envList[common.Address](key, c.h.GetEnvVar, delimiter, c.ParseAddress)
}

// EnvBytes32 implements https://book.getfoundry.sh/cheatcodes/env-bytes32
func (c *CheatCodesPrecompile) EnvBytes32_97949042(key string) ([32]byte, error) {
	return envSingular[[32]byte](key, c.h.GetEnvVar, c.ParseBytes32)
}

func (c *CheatCodesPrecompile) EnvBytes32_5af231c1(key string, delimiter string) ([][32]byte, error) {
	return envList[[32]byte](key, c.h.GetEnvVar, delimiter, c.ParseBytes32)
}

// EnvString implements https://book.getfoundry.sh/cheatcodes/env-string
func (c *CheatCodesPrecompile) EnvString_f877cb19(key string) (string, error) {
	return envSingular[string](key, c.h.GetEnvVar, func(v string) (string, error) {
		return v, nil
	})
}

func (c *CheatCodesPrecompile) EnvString_14b02bc9(key string, delimiter string) ([]string, error) {
	return envList[string](key, c.h.GetEnvVar, delimiter, func(v string) (string, error) {
		return v, nil
	})
}

// EnvBytes implements https://book.getfoundry.sh/cheatcodes/env-bytes
func (c *CheatCodesPrecompile) EnvBytes_4d7baf06(key string) ([]byte, error) {
	return envSingular[[]byte](key, c.h.GetEnvVar, c.ParseBytes)
}

func (c *CheatCodesPrecompile) EnvBytes_ddc2651b(key string, delimiter string) ([][]byte, error) {
	return envList[[]byte](key, c.h.GetEnvVar, delimiter, c.ParseBytes)
}

// KeyExists implements https://book.getfoundry.sh/cheatcodes/key-exists
func (c *CheatCodesPrecompile) KeyExists(jsonData string, key string) (bool, error) {
	return c.KeyExistsJson(jsonData, key)

}

// KeyExistsJson implements https://book.getfoundry.sh/cheatcodes/key-exists-json
func (c *CheatCodesPrecompile) KeyExistsJson(data string, key string) (bool, error) {
	var x map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &x); err != nil {
		return false, err
	}
	_, ok := x[key]
	return ok, nil
}

// KeyExistsToml implements https://book.getfoundry.sh/cheatcodes/key-exists-toml
func (c *CheatCodesPrecompile) KeyExistsToml(data string, key string) (bool, error) {
	var x map[string]any
	if err := toml.Unmarshal([]byte(data), &x); err != nil {
		return false, err
	}
	_, ok := x[key]
	return ok, nil
}

// ParseJSON implements https://book.getfoundry.sh/cheatcodes/parse-json
func (c *CheatCodesPrecompile) ParseJson_85940ef1(data string, key string) {
	panic("parseJson(string,string) is not supported") // inferring an ABI type dynamically from sorted JSON content, this is mad.
}

func (c *CheatCodesPrecompile) ParseJson_6a82600a(data string) {
	panic("parseJson(string) is not supported")
}

// ParseToml implements https://book.getfoundry.sh/cheatcodes/parse-toml
func (c *CheatCodesPrecompile) ParseToml_37736e08(data string, key string) {
	panic("parseToml(string,string) is not supported")
}

func (c *CheatCodesPrecompile) ParseToml_592151f0(data string) {
	panic("parseToml(string) is not supported")
}

// See https://github.com/foundry-rs/foundry/issues/8672
// Forge uses dots and `[%d]` mixed together for JSON paths.
// It's like jq, but does not match foundry docs, and hard to parse.
func takePath(query string) (trailing string, stringKey string, index uint64, err error) {
	if query == "" {
		return "", "", 0, errors.New("empty keys are not supported")
	}
	dotIndex := strings.Index(query, ".")
	openBracketIndex := strings.Index(query, "[")
	if dotIndex < 0 || (openBracketIndex >= 0 && openBracketIndex < dotIndex) {
		if openBracketIndex < 0 {
			return "", query, 0, nil
		} else if openBracketIndex == 0 {
			closingBracketIndex := strings.Index(query, "]")
			if closingBracketIndex <= openBracketIndex {
				return "", "", 0, fmt.Errorf("invalid query: %q", query)
			}
			index, err := strconv.ParseUint(query[1:closingBracketIndex], 10, 64)
			if err != nil {
				return "", "", 0, fmt.Errorf("invalid index in query: %w", err)
			}
			return query[closingBracketIndex+1:], "", index, nil
		} else {
			return query[openBracketIndex:], query[:openBracketIndex], 0, nil
		}
	} else {
		return query[dotIndex+1:], query[:dotIndex], 0, nil
	}
}

func lookupKeys(v any, query string) ([]string, error) {
	if query == "$" || query == "" {
		switch x := v.(type) {
		case map[string]any:
			return maps.Keys(x), nil
		default:
			return nil, fmt.Errorf("JSON value (Type %T) is not an object", x)
		}
	}
	trailing, stringKey, index, err := takePath(query)
	if err != nil {
		return nil, err
	}
	switch x := v.(type) {
	case []any:
		if stringKey != "" {
			return nil, fmt.Errorf("expected array index, but got string key in path: %q", stringKey)
		}
		if index >= uint64(len(x)) {
			return nil, fmt.Errorf("index %d larger than length %d", index, len(x))
		}
		return lookupKeys(x[index], trailing)
	case map[string]any:
		if stringKey == "" {
			return nil, fmt.Errorf("expected string key, but got index in path: %q", index)
		}
		if stringKey == "$" {
			if trailing != "" {
				return nil, errors.New("cannot continue query after $ sign")
			}
			return maps.Keys(x), nil
		}
		data, ok := x[stringKey]
		if !ok {
			return nil, fmt.Errorf("unknown key %q", stringKey)
		}
		return lookupKeys(data, trailing)
	default:
		return nil, fmt.Errorf("cannot read keys of value of type %T", x)
	}
}

// ParseJsonKeys implements https://book.getfoundry.sh/cheatcodes/parse-json-keys
func (c *CheatCodesPrecompile) ParseJsonKeys(data string, key string) ([]string, error) {
	var x map[string]any
	if err := json.Unmarshal([]byte(data), &x); err != nil {
		return nil, err
	}
	if key != "$" {
		if !strings.HasPrefix(key, ".") {
			return nil, fmt.Errorf("key %q is invalid. A key must be \"$\" or start with \".\"", key)
		}
		key = strings.TrimPrefix(key, ".")
	}
	return lookupKeys(x, key)
}

// ParseTomlKeys implements https://book.getfoundry.sh/cheatcodes/parse-toml-keys
func (c *CheatCodesPrecompile) ParseTomlKeys(data string, key string) ([]string, error) {
	var x map[string]any
	if err := toml.Unmarshal([]byte(data), &x); err != nil {
		return nil, err
	}
	return lookupKeys(x, key)
}

// SerializeJson implements https://book.getfoundry.sh/cheatcodes/serialize-json
func (c *CheatCodesPrecompile) SerializeJson(objectKey string, value string) (string, error) {
	var x json.RawMessage
	if err := json.Unmarshal([]byte(value), &x); err != nil {
		return "", fmt.Errorf("invalid JSON value: %w", err)
	}
	c.h.serializerStates[objectKey] = x
	return string(x), nil
}

func (c *CheatCodesPrecompile) serializeJsonValue(objectKey string, valueKey string, value any) (string, error) {
	st, ok := c.h.serializerStates[objectKey]
	if !ok {
		st = json.RawMessage("{}")
	}
	var x map[string]json.RawMessage
	if err := json.Unmarshal(st, &x); err != nil {
		return "", fmt.Errorf("failed to decode existing JSON serializer state of %q: %w", objectKey, err)
	}
	v, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("failed to encode value: %w", err)
	}
	x[valueKey] = v
	out, err := json.Marshal(x)
	if err != nil {
		return "", fmt.Errorf("failed to encode updated JSON serializer state of %q: %w", objectKey, err)
	}
	c.h.serializerStates[objectKey] = out
	return string(out), nil
}

func (c *CheatCodesPrecompile) SerializeBool_ac22e971(objectKey string, valueKey string, value bool) (string, error) {
	return c.serializeJsonValue(objectKey, valueKey, value)
}

func (c *CheatCodesPrecompile) SerializeUint_129e9002(objectKey string, valueKey string, value *big.Int) (string, error) {
	return c.serializeJsonValue(objectKey, valueKey, value)
}

func (c *CheatCodesPrecompile) SerializeInt_3f33db60(objectKey string, valueKey string, value *ABIInt256) (string, error) {
	return c.serializeJsonValue(objectKey, valueKey, value)
}

func (c *CheatCodesPrecompile) SerializeAddress_972c6062(objectKey string, valueKey string, value common.Address) (string, error) {
	return c.serializeJsonValue(objectKey, valueKey, value)
}

func (c *CheatCodesPrecompile) SerializeBytes32_2d812b44(objectKey string, valueKey string, value common.Hash) (string, error) {
	return c.serializeJsonValue(objectKey, valueKey, value)
}

func (c *CheatCodesPrecompile) SerializeString_88da6d35(objectKey string, valueKey string, value string) (string, error) {
	return c.serializeJsonValue(objectKey, valueKey, value)
}

func (c *CheatCodesPrecompile) SerializeBytes_f21d52c7(objectKey string, valueKey string, value hexutil.Bytes) (string, error) {
	return c.serializeJsonValue(objectKey, valueKey, value)
}

func (c *CheatCodesPrecompile) SerializeBool_92925aa1(objectKey string, valueKey string, values []bool) (string, error) {
	return c.serializeJsonValue(objectKey, valueKey, values)
}

func (c *CheatCodesPrecompile) SerializeUint_fee9a469(objectKey string, valueKey string, values []*big.Int) (string, error) {
	return c.serializeJsonValue(objectKey, valueKey, values)
}

func (c *CheatCodesPrecompile) SerializeInt_7676e127(objectKey string, valueKey string, values []*ABIInt256) (string, error) {
	return c.serializeJsonValue(objectKey, valueKey, values)
}

func (c *CheatCodesPrecompile) SerializeAddress_1e356e1a(objectKey string, valueKey string, values []common.Address) (string, error) {
	return c.serializeJsonValue(objectKey, valueKey, values)
}

func (c *CheatCodesPrecompile) SerializeBytes32_201e43e2(objectKey string, valueKey string, values []common.Hash) (string, error) {
	return c.serializeJsonValue(objectKey, valueKey, values)
}

func (c *CheatCodesPrecompile) SerializeString_561cd6f3(objectKey string, valueKey string, values []string) (string, error) {
	return c.serializeJsonValue(objectKey, valueKey, values)
}

func (c *CheatCodesPrecompile) SerializeBytes_9884b232(objectKey string, valueKey string, values []hexutil.Bytes) (string, error) {
	return c.serializeJsonValue(objectKey, valueKey, values)
}

// WriteJson implements https://book.getfoundry.sh/cheatcodes/write-json
func (c *CheatCodesPrecompile) WriteJson_e23cd19f(data string, path string) error {
	return vm.ErrExecutionReverted
}

func (c *CheatCodesPrecompile) WriteJson_35d6ad46(data string, path string, valueKey string) error {
	return vm.ErrExecutionReverted
}

// WriteToml implements https://book.getfoundry.sh/cheatcodes/write-toml
func (c *CheatCodesPrecompile) WriteToml_c0865ba7(data string, path string) error {
	return vm.ErrExecutionReverted
}

func (c *CheatCodesPrecompile) WriteToml_51ac6a33(data string, path string, valueKey string) error {
	return vm.ErrExecutionReverted
}
