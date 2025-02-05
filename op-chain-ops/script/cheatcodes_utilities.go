package script

import (
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	hdwallet "github.com/ethereum-optimism/go-ethereum-hdwallet"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
)

// Addr implements https://book.getfoundry.sh/cheatcodes/addr
func (c *CheatCodesPrecompile) Addr(privateKey *big.Int) (common.Address, error) {
	priv, err := crypto.ToECDSA(leftPad32(privateKey.Bytes()))
	if err != nil {
		return common.Address{}, err
	}
	return crypto.PubkeyToAddress(priv.PublicKey), nil
}

// Sign implements https://book.getfoundry.sh/cheatcodes/sign
func (c *CheatCodesPrecompile) Sign() error {
	return vm.ErrExecutionReverted
}

// Skip implements https://book.getfoundry.sh/cheatcodes/skip
func (c *CheatCodesPrecompile) Skip() error {
	return vm.ErrExecutionReverted
}

// Label implements https://book.getfoundry.sh/cheatcodes/label
func (c *CheatCodesPrecompile) Label(addr common.Address, label string) {
	c.h.Label(addr, label)
}

// GetLabel implements https://book.getfoundry.sh/cheatcodes/get-label
func (c *CheatCodesPrecompile) GetLabel(addr common.Address) string {
	label, ok := c.h.labels[addr]
	if !ok {
		return "unlabeled:" + addr.String()
	}
	return label
}

// DeriveKey_6229498b implements https://book.getfoundry.sh/cheatcodes/derive-key
func (c *CheatCodesPrecompile) DeriveKey_6229498b(mnemonic string, index uint32) (*big.Int, error) {
	return c.DeriveKey_6bcb2c1b(mnemonic, "m/44'/60'/0'/0/", index)
}

// DeriveKey_6bcb2c1b implements https://book.getfoundry.sh/cheatcodes/derive-key
func (c *CheatCodesPrecompile) DeriveKey_6bcb2c1b(mnemonic string, path string, index uint32) (*big.Int, error) {
	w, err := hdwallet.NewFromMnemonic(mnemonic)
	if err != nil {
		return nil, fmt.Errorf("invalid mnemonic: %w", err)
	}
	account := accounts.Account{URL: accounts.URL{Path: path + strconv.FormatInt(int64(index), 10)}}
	priv, err := w.PrivateKey(account)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key of path %s: %w", account.URL.Path, err)
	}
	return common.Hash(crypto.FromECDSA(priv)).Big(), nil
}

// ParseBytes implements https://book.getfoundry.sh/cheatcodes/parse-bytes
func (c *CheatCodesPrecompile) ParseBytes(stringifiedValue string) ([]byte, error) {
	return hexutil.Decode(stringifiedValue)
}

// ParseAddress implements https://book.getfoundry.sh/cheatcodes/parse-address
func (c *CheatCodesPrecompile) ParseAddress(stringifiedValue string) (common.Address, error) {
	var out common.Address
	err := out.UnmarshalText([]byte(stringifiedValue))
	return out, err
}

// ParseUint implements https://book.getfoundry.sh/cheatcodes/parse-uint
func (c *CheatCodesPrecompile) ParseUint(stringifiedValue string) (*big.Int, error) {
	out := new(big.Int)
	err := out.UnmarshalText([]byte(stringifiedValue))
	if err != nil {
		return big.NewInt(0), err
	}
	if out.BitLen() > 256 {
		return big.NewInt(0), fmt.Errorf("value %d is not a uint256, got %d bits", out, out.BitLen())
	}
	if out.Sign() < 0 {
		return big.NewInt(0), fmt.Errorf("value %d is not a uint256, it has a negative sign", out)
	}
	return out, nil
}

var (
	topBit    = math.BigPow(2, 255)
	maxInt256 = new(big.Int).Sub(topBit, big.NewInt(1))
	minInt256 = new(big.Int).Neg(topBit)
)

// ParseInt implements https://book.getfoundry.sh/cheatcodes/parse-int
func (c *CheatCodesPrecompile) ParseInt(stringifiedValue string) (*ABIInt256, error) {
	out := new(big.Int)
	err := out.UnmarshalText([]byte(stringifiedValue))
	if err != nil {
		return (*ABIInt256)(big.NewInt(0)), err
	}
	if out.Cmp(minInt256) < 0 || out.Cmp(maxInt256) > 0 {
		return (*ABIInt256)(big.NewInt(0)), fmt.Errorf("input %q out of int256 bounds", stringifiedValue)
	}
	return (*ABIInt256)(out), nil
}

// ParseBytes32 implements https://book.getfoundry.sh/cheatcodes/parse-bytes32
func (c *CheatCodesPrecompile) ParseBytes32(stringifiedValue string) ([32]byte, error) {
	var out common.Hash
	err := out.UnmarshalText([]byte(stringifiedValue))
	return out, err
}

// ParseBool implements https://book.getfoundry.sh/cheatcodes/parse-bool
func (c *CheatCodesPrecompile) ParseBool(stringifiedValue string) (bool, error) {
	switch strings.ToLower(stringifiedValue) {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("failed parsing %q as type `bool`", stringifiedValue)
	}
}

// RememberKey implements https://book.getfoundry.sh/cheatcodes/remember-key
func (c *CheatCodesPrecompile) RememberKey(privateKey *big.Int) (common.Address, error) {
	// We don't store the key, but we can return the address of it, to not break compat
	return c.Addr(privateKey)
}

// ToString_56ca623e implements https://book.getfoundry.sh/cheatcodes/to-string
func (c *CheatCodesPrecompile) ToString_56ca623e(v common.Address) string {
	return v.String()
}

// ToString_71dce7da implements https://book.getfoundry.sh/cheatcodes/to-string
func (c *CheatCodesPrecompile) ToString_71dce7da(v bool) string {
	if v {
		return "true"
	} else {
		return "false"
	}
}

// ToString_6900a3ae implements https://book.getfoundry.sh/cheatcodes/to-string
func (c *CheatCodesPrecompile) ToString_6900a3ae(v *big.Int) string {
	return v.String()
}

// ToString_a322c40e implements https://book.getfoundry.sh/cheatcodes/to-string
func (c *CheatCodesPrecompile) ToString_a322c40e(v *ABIInt256) string {
	return (*big.Int)(v).String()
}

// ToString_b11a19e8 implements https://book.getfoundry.sh/cheatcodes/to-string
func (c *CheatCodesPrecompile) ToString_b11a19e8(v [32]byte) string {
	return common.Hash(v).String()
}

// ToString_71aad10d implements https://book.getfoundry.sh/cheatcodes/to-string
func (c *CheatCodesPrecompile) ToString_71aad10d(v []byte) string {
	return hexutil.Bytes(v).String()
}

// Breakpoint_f0259e92 implements https://book.getfoundry.sh/cheatcodes/breakpoint
func (c *CheatCodesPrecompile) Breakpoint_f0259e92(name string) {
	c.h.log.Debug("breakpoint hit", "name", name)
}

// Breakpoint_f7d39a8d implements https://book.getfoundry.sh/cheatcodes/breakpoint
func (c *CheatCodesPrecompile) Breakpoint_f7d39a8d(name string, v bool) {
	if v {
		c.h.log.Debug("breakpoint set", "name", name)
	} else {
		c.h.log.Debug("breakpoint unset", "name", name)
	}
}

// ParseTomlAddress_65e7c844 implements https://book.getfoundry.sh/cheatcodes/parse-toml. This
// method is not well optimized or implemented. It's optimized for quickly delivering OPCM. We
// can come back and clean it up more later.
func (c *CheatCodesPrecompile) ParseTomlAddress_65e7c844(tomlStr string, key string) (common.Address, error) {
	var data map[string]any
	if err := toml.Unmarshal([]byte(tomlStr), &data); err != nil {
		return common.Address{}, fmt.Errorf("failed to parse TOML: %w", err)
	}

	keys, err := SplitJSONPathKeys(key)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to split keys: %w", err)
	}

	loc := data
	for i, k := range keys {
		value, ok := loc[k]
		if !ok {
			return common.Address{}, fmt.Errorf("key %q not found in TOML", k)
		}

		if i == len(keys)-1 {
			addrStr, ok := value.(string)
			if !ok {
				return common.Address{}, fmt.Errorf("key %q is not a string", key)
			}
			if !common.IsHexAddress(addrStr) {
				return common.Address{}, fmt.Errorf("key %q is not a valid address", key)
			}
			return common.HexToAddress(addrStr), nil
		}

		next, ok := value.(map[string]any)
		if !ok {
			return common.Address{}, fmt.Errorf("key %q is not a nested map", key)
		}
		loc = next
	}

	panic("should never get here")
}

func (c *CheatCodesPrecompile) ComputeCreate2Address_890c283b(salt, codeHash [32]byte) (common.Address, error) {
	data := make([]byte, 1+20+32+32)
	data[0] = 0xff
	copy(data[1:], DeterministicDeployerAddress.Bytes())
	copy(data[1+20:], salt[:])
	copy(data[1+20+32:], codeHash[:])
	finalHash := crypto.Keccak256(data)
	// Take the last 20 bytes of the hash to get the address
	return common.BytesToAddress(finalHash[12:]), nil
}

// unsupported
//func (c *CheatCodesPrecompile) CreateWallet() {}

// SplitJSONPathKeys splits a JSON path into keys. It supports bracket notation. There is a much
// better way to implement this, but I'm keeping this simple for now.
func SplitJSONPathKeys(path string) ([]string, error) {
	var out []string
	bracketSplit := regexp.MustCompile(`[\[\]]`).Split(path, -1)
	for _, split := range bracketSplit {
		if len(split) == 0 {
			continue
		}

		split = strings.ReplaceAll(split, "\"", "")
		split = strings.ReplaceAll(split, " ", "")

		if !strings.HasPrefix(split, ".") {
			out = append(out, split)
			continue
		}

		keys := strings.Split(split, ".")
		for _, key := range keys {
			if len(key) == 0 {
				continue
			}
			out = append(out, key)
		}
	}

	return out, nil
}
