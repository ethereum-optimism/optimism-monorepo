package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	nat "github.com/ethereum-optimism/nat/pkg"
	"github.com/ethereum-optimism/nat/validators/tests"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/kr/pretty"
	"github.com/urfave/cli/v3"
)

var (
	flagRPC                string = "rpc"
	flagSenderSecretKey    string = "sender-secret-key"
	flagReceiverPublicKeys string = "receiver-public-keys"
	flagDebug              string = "debug"
	flagOutputFormat       string = "output-format"
)

var app = &cli.Command{
	Name:  "test",
	Usage: "run acceptance tests",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  flagRPC,
			Value: "http://localhost:8545",
			Usage: "Network RPC URL",
		},
		&cli.BoolFlag{
			Name:  flagDebug,
			Value: false,
			Usage: "Enable debug mode",
		},
		&cli.StringFlag{
			Name:  flagSenderSecretKey,
			Value: "",
			Usage: "Sender secret key",
		},
		&cli.StringSliceFlag{
			Name:  flagReceiverPublicKeys,
			Value: []string{"0x9B383f8e4Cd5d3DD5F9006B6A508960A1e730375"}, // OP-Sepolia Whale
			Usage: "Receiver public keys",
		},
		&cli.StringFlag{
			Name:  flagOutputFormat,
			Value: "table",
			Usage: "Output mode: table",
		},
	},
	Action: func(_ context.Context, c *cli.Command) error {
		// Parse flags
		fRPC := c.String(flagRPC)
		fOutputFormat := c.String(flagOutputFormat)
		fSenderSecretKey := c.String(flagSenderSecretKey)
		fReceiverPublicKeys := c.StringSlice(flagReceiverPublicKeys)
		fDebug := c.Bool(flagDebug)
		if !strings.Contains(fRPC, "http") {
			return cli.Exit("RPC URL is malformed", 1)
		}

		// Config
		cfg := nat.NewConfig(fRPC, fSenderSecretKey, fReceiverPublicKeys, validators)
		if fDebug {
			pretty.Printf("Config:\n%# v\n", cfg)
		}
		// Run tests
		pretty.Println("Running tests...")
		tester := nat.New(cfg)
		ok, err := tester.Run()

		// Print results
		if fOutputFormat == "table" {
			printTable(ok)
		}

		if err != nil {
			return cli.Exit(fmt.Sprintf("error running tests: %v", err), 1)
		}
		if !ok {
			return cli.Exit("tests failed", 1)
		}
		return nil
	},
	Suggest: true,
}

func main() {
	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

var validators = []nat.Validator{
	tests.TxFuzz,
}

func printTable(ok bool) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Test", "Result"})
	t.AppendRows([]table.Row{
		{"tx-fuzz", ok},
	})
	t.AppendSeparator()
	t.AppendRow([]interface{}{"SUMMARY", ok})
	t.Render()
}
