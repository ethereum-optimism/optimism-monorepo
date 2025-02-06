package script

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const tomlTest = `
foo = "0x0d4CE7B6a91A35c31D7D62b327D19617c8da6F23"

[foomap]
[foomap."bar.bump"]
baz = "0xff4ce7b6a91a35c31d7d62b327d19617c8da6f23"
`

func TestSplitJSONPathKeys(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected []string
	}{
		{
			"simple",
			".foo.bar",
			[]string{"foo", "bar"},
		},
		{
			"bracket keys",
			".foo[\"hey\"].bar",
			[]string{"foo", "hey", "bar"},
		},
		{
			"bracket keys with dots",
			".foo[\"hey.there\"].bar",
			[]string{"foo", "hey.there", "bar"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SplitJSONPathKeys(tt.path)
			require.NoError(t, err)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestParseTomlAddress(t *testing.T) {
	c := &CheatCodesPrecompile{}

	addr, err := c.ParseTomlAddress_65e7c844(tomlTest, "foo")
	require.NoError(t, err)
	require.Equal(t, common.HexToAddress("0x0d4ce7b6a91a35c31d7d62b327d19617c8da6f23"), addr)

	addr, err = c.ParseTomlAddress_65e7c844(tomlTest, "foomap[\"bar.bump\"].baz")
	require.NoError(t, err)
	require.Equal(t, common.HexToAddress("0xff4ce7b6a91a35c31d7d62b327d19617c8da6f23"), addr)
}

func TestComputeCreate2Address(t *testing.T) {
	c := &CheatCodesPrecompile{}
	var salt [32]byte
	salt[31] = 'S'
	var codeHash [32]byte
	codeHash[31] = 'C'
	addr, err := c.ComputeCreate2Address_890c283b(salt, codeHash)
	require.NoError(t, err)
	require.EqualValues(t, common.HexToAddress("0x2f29AF1b5a7083bf98C4A89976c2f17FF980735f"), addr)
}

type createCase struct {
	Deployer common.Address
	Nonce    *big.Int
	Expected common.Address
}

func TestComputeCreateAddress(t *testing.T) {
	c := &CheatCodesPrecompile{}
	sender := common.Address(crypto.Keccak256([]byte("example sender"))[12:])
	for _, testCase := range []createCase{
		{
			Deployer: sender,
			Nonce:    big.NewInt(0),
			Expected: crypto.CreateAddress(sender, 0),
		},
		{
			Deployer: sender,
			Nonce:    big.NewInt(123),
			Expected: crypto.CreateAddress(sender, 123),
		},
		{
			Deployer: sender,
			Nonce:    new(uint256.Int).Not(uint256.NewInt(0)).ToBig(), // max value
			Expected: common.Address{},                                // expecting an error
		},
	} {
		t.Run(fmt.Sprintf("create-%s-%s", testCase.Nonce, testCase.Deployer), func(t *testing.T) {
			addr, err := c.ComputeCreateAddress_74637a7a(testCase.Deployer, testCase.Nonce)
			if testCase.Expected == (common.Address{}) {
				require.ErrorContains(t, err, "too large")
			} else {
				require.NoError(t, err)
				require.Equal(t, testCase.Expected, addr)
			}
		})
	}

}
