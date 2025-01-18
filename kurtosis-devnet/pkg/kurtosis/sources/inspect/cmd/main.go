// Package main reproduces a lightweight version of the "kurtosis enclave inspect" command
// It can be used to sanity check the results, as writing tests against a fake
// enclave is not practical right now.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/inspect"
)

func main() {
	ctx := context.Background()

	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s <enclave-id>\n", os.Args[0])
		os.Exit(1)
	}

	enclaveID := flag.Arg(0)
	inspector := inspect.NewInspector(enclaveID)

	data, err := inspector.ExtractData(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error inspecting enclave: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("File Artifacts:")
	for _, artifact := range data.FileArtifacts {
		fmt.Printf("  %s\n", artifact)
	}

	fmt.Println("\nServices:")
	for svc, ports := range data.UserServices {
		fmt.Printf("  %s:\n", svc)
		for portName, portInfo := range ports {
			host := portInfo.Host
			if host == "" {
				host = "localhost"
			}
			fmt.Printf("    %s: %s:%d\n", portName, host, portInfo.Port)
		}
	}
}
