package main

import (
	"strings"
	"testing"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-challenger/runner"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestParseRunArg(t *testing.T) {
	tests := []struct {
		arg      string
		expected runner.RunConfig
		err      error
	}{
		{arg: "unknown/test1/0x1234", err: ErrUnknownTraceType},
		{arg: "cannon", expected: runner.RunConfig{TraceType: types.TraceTypeCannon, Name: types.TraceTypeCannon.String()}},
		{arg: "asterisc", expected: runner.RunConfig{TraceType: types.TraceTypeAsterisc, Name: types.TraceTypeAsterisc.String()}},
		{arg: "cannon/test1", expected: runner.RunConfig{TraceType: types.TraceTypeCannon, Name: "test1"}},
		{arg: "cannon/test1/0x1234", expected: runner.RunConfig{TraceType: types.TraceTypeCannon, Name: "test1", Prestate: common.HexToHash("0x1234")}},
		{arg: "cannon/test1/invalid", err: ErrInvalidPrestateHash},
	}
	for _, test := range tests {
		test := test
		// Slash characters in test names confuse some things that parse the output as it looks like a subtest
		t.Run(strings.ReplaceAll(test.arg, "/", "_"), func(t *testing.T) {
			actual, err := parseRunArg(test.arg)
			require.ErrorIs(t, err, test.err)
			require.Equal(t, test.expected, actual)
		})
	}
}
