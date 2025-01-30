package cliutil

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

type textUnmarshalerThing struct {
	text string
}

func (t *textUnmarshalerThing) UnmarshalText(text []byte) error {
	t.text = string(text)
	return nil
}

func TestPopulateStruct(t *testing.T) {
	type testStruct struct {
		Str             string                `cli:"str"`
		Bool            bool                  `cli:"bool"`
		Int             int                   `cli:"int"`
		Int64           int64                 `cli:"int64"`
		Uint64          uint64                `cli:"uint64"`
		Address         common.Address        `cli:"address"`
		TextUnmarshaler *textUnmarshalerThing `cli:"text-unmarshaler"`
		NotTagged       string
	}

	tests := []struct {
		name   string
		args   []string
		exp    testStruct
		expErr string
	}{
		{
			name: "all flags",
			args: []string{
				"--str=test",
				"--bool",
				"--int=1",
				"--int64=2",
				"--uint64=3",
				fmt.Sprintf("--address=%s", common.HexToAddress("0x42")),
				"--text-unmarshaler=hello",
			},
			exp: testStruct{
				Str:     "test",
				Bool:    true,
				Int:     1,
				Int64:   2,
				Uint64:  3,
				Address: common.HexToAddress("0x42"),
				TextUnmarshaler: &textUnmarshalerThing{
					text: "hello",
				},
			},
		},
		{
			name: "no flags",
			args: []string{},
			exp:  testStruct{},
		},
		{
			name: "invalid address flag",
			args: []string{
				"--address=not-an-address",
			},
			expErr: "invalid address",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &cli.App{
				Name: "test",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name: "str",
					},
					&cli.BoolFlag{
						Name: "bool",
					},
					&cli.IntFlag{
						Name: "int",
					},
					&cli.Int64Flag{
						Name: "int64",
					},
					&cli.Uint64Flag{
						Name: "uint64",
					},
					&cli.StringFlag{
						Name: "address",
					},
					&cli.StringFlag{
						Name: "text-unmarshaler",
					},
				},
				Action: func(cliCtx *cli.Context) error {
					ts := testStruct{}

					if tt.expErr == "" {
						require.NoError(t, PopulateStruct(&ts, cliCtx))
						require.EqualValues(t, tt.exp, ts)
					} else {
						require.ErrorContains(t, PopulateStruct(&ts, cliCtx), tt.expErr)
					}
					return nil
				},
			}

			require.NoError(t, app.Run(append([]string{"program-goes-here"}, tt.args...)))
		})
	}
}
