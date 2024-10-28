package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/ethereum-optimism/optimism/op-program/verify"
	"github.com/ethereum/go-ethereum/common"
)

func main() {
	var l1RpcUrl string
	var l1RpcKind string
	var l1BeaconUrl string
	var l2RpcUrl string
	var dataDir string
	var l1HashStr string
	var l2Start uint64
	var l2End uint64
	flag.StringVar(&l1RpcUrl, "l1", "", "L1 RPC URL to use")
	flag.StringVar(&l1BeaconUrl, "l1.beacon", "", "L1 Beacon URL to use")
	flag.StringVar(&l1RpcKind, "l1-rpckind", "", "L1 RPC kind")
	flag.StringVar(&l2RpcUrl, "l2", "", "L2 RPC URL to use")
	flag.StringVar(&dataDir, "datadir", "",
		"Directory to use for storing pre-images. If not set a temporary directory will be used.")
	flag.StringVar(&l1HashStr, "l1.head", "", "Hash of L1 block to use")
	flag.Uint64Var(&l2Start, "l2.start", 0, "Block number of agreed L2 block")
	flag.Uint64Var(&l2End, "l2.end", 0, "Block number of claimed L2 block")
	flag.Parse()

	if l1RpcUrl == "" {
		_, _ = fmt.Fprintln(os.Stderr, "Must specify --l1 RPC URL")
		os.Exit(2)
	}
	if l1BeaconUrl == "" {
		_, _ = fmt.Fprintln(os.Stderr, "Must specify --l1.beacon URL")
		os.Exit(2)
	}
	if l2RpcUrl == "" {
		_, _ = fmt.Fprintln(os.Stderr, "Must specify --l2 RPC URL")
		os.Exit(2)
	}

	// Apply the custom configs by running op-program
	runner, err := verify.NewRunner(l1RpcUrl, l1RpcKind, l1BeaconUrl, l2RpcUrl, dataDir, "901", 901, false)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to create runner: %v\n", err.Error())
		os.Exit(1)
	}

	if l1HashStr == "" && l2Start == 0 && l2End == 0 {
		err = runner.RunToFinalized(context.Background())
	} else {
		l1Hash := common.HexToHash(l1HashStr)
		if l1Hash == (common.Hash{}) {
			_, _ = fmt.Fprintf(os.Stderr, "Invalid --l1.head: %v\n", l1HashStr)
			os.Exit(2)
		}
		err = runner.RunBetweenBlocks(context.Background(), l1Hash, l2Start, l2End)
	}
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed: %v\n", err.Error())
		os.Exit(1)
	}
}
