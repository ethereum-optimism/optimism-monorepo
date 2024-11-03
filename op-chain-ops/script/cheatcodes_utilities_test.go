package script

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
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
