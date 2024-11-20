package script

import (
	"fmt"
	"log/slog"
	"math/big"
	"math/rand" // nosemgrep
	"testing"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script/addresses"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

func TestConsole(t *testing.T) {
	logger, captLog := testlog.CaptureLogger(t, log.LevelDebug)

	rng := rand.New(rand.NewSource(123))
	sender := testutils.RandomAddress(rng)
	alice := testutils.RandomAddress(rng)
	bob := testutils.RandomAddress(rng)
	t.Logf("sender: %s", sender)
	t.Logf("alice: %s", alice)
	t.Logf("bob: %s", bob)
	c := &ConsolePrecompile{
		logger: logger,
		sender: func() common.Address { return sender },
	}
	p, err := NewPrecompile[*ConsolePrecompile](c)
	require.NoError(t, err)

	// test Log_daf0d4aa
	input := make([]byte, 0, 4+32+32)
	input = append(input, hexutil.MustDecode("0xdaf0d4aa")...)
	input = append(input, leftPad32(alice[:])...)
	input = append(input, leftPad32(bob[:])...)
	t.Logf("input: %x", input)

	_, err = p.Run(input)
	require.NoError(t, err)

	for i, l := range *captLog.Logs {
		t.Logf("log %d", i)
		l.Attrs(func(attr slog.Attr) bool {
			t.Logf("attr: k: %s, v: %s", attr.Key, attr.Value.String())
			return true
		})
	}

	require.NotNil(t, captLog.FindLog(testlog.NewMessageFilter(fmt.Sprintf("%s %s", alice, bob))))
	require.NotNil(t, captLog.FindLog(testlog.NewAttributesFilter("sender", sender.String())))
}

func TestFormatter(t *testing.T) {
	got := consoleFormat("hello %d world %x example %3e",
		big.NewInt(3), big.NewInt(0xc0ffee), big.NewInt(42), big.NewInt(123))
	require.Equal(t, "hello 3 world 0xc0ffee example 0.042 123", got)
	require.Equal(t, "4.2", consoleFormat("%8e", big.NewInt(420000000)))
	require.Equal(t, "foo true bar false", consoleFormat("foo %s bar %s", true, false))
	require.Equal(t, "foo 1 bar 0", consoleFormat("foo %d bar %d", true, false))
	require.Equal(t, "sender: "+addresses.DefaultSenderAddr.String(),
		consoleFormat("sender: %s", addresses.DefaultSenderAddr))
	require.Equal(t, "long 0.000000000000000042 number", consoleFormat("long %18e number", big.NewInt(42)))
	require.Equal(t, "long 4200.000000000000000003 number", consoleFormat("long %18e number",
		new(big.Int).Add(new(big.Int).Mul(
			big.NewInt(42),
			new(big.Int).Exp(big.NewInt(10), big.NewInt(20), nil),
		), big.NewInt(3))))
	require.Equal(t, "1.23456e5", consoleFormat("%e", big.NewInt(123456)))
	require.Equal(t, "-1.23456e5", consoleFormat("%e", (*ABIInt256)(big.NewInt(-123456))))
}
