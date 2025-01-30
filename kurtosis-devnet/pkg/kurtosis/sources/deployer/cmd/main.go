package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/deployer"
)

func main() {
	// Parse command line flags
	enclave := flag.String("enclave", "", "Name of the Kurtosis enclave")
	flag.Parse()

	if *enclave == "" {
		fmt.Fprintln(os.Stderr, "Error: --enclave flag is required")
		flag.Usage()
		os.Exit(1)
	}

	// Get deployer data
	d := deployer.NewDeployer(*enclave)
	ctx := context.Background()
	data, err := d.ExtractData(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing deployer data: %v\n", err)
		os.Exit(1)
	}

	// Encode as JSON and write to stdout
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}
