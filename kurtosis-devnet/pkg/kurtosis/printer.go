package kurtosis

import (
	"fmt"
	"strings"
)

func (env *KurtosisEnvironment) Print() {
	fmt.Println("=== Optimism Development Network ===")
	fmt.Println()

	// Print L1 chain info
	if env.L1 != nil {
		fmt.Println("=== L1 Chain (Ethereum) ===")
		printChain(env.L1)
		fmt.Println()
	}

	// Print L2 chains info
	if len(env.L2) > 0 {
		fmt.Println("=== L2 Chains ===")
		for _, chain := range env.L2 {
			fmt.Printf("--- Chain: %s (ID: %s) ---\n", chain.Name, chain.ID)
			printChain(chain)
			fmt.Println()
		}
	}
}

func printChain(chain *Chain) {
	// Print chain services
	if len(chain.Services) > 0 {
		fmt.Println("Services:")
		for name, service := range chain.Services {
			fmt.Printf("  %s:\n", name)
			printEndpoints(service.Endpoints)
		}
	}

	// Print chain nodes
	if len(chain.Nodes) > 0 {
		fmt.Println("Nodes:")
		for i, node := range chain.Nodes {
			fmt.Printf("  Node %d:\n", i+1)
			for name, service := range node.Services {
				fmt.Printf("    %s:\n", name)
				printEndpoints(service.Endpoints)
			}
		}
	}

	// Print contract addresses if available
	if len(chain.Addresses) > 0 {
		fmt.Println("Contract Addresses:")
		for name, addr := range chain.Addresses {
			fmt.Printf("  %s: %s\n", name, addr)
		}
	}

	// Print wallets if available
	if len(chain.Wallets) > 0 {
		fmt.Println("Wallets:")
		for name, wallet := range chain.Wallets {
			fmt.Printf("  %s:\n", name)
			fmt.Printf("    Address: %s\n", wallet.Address)
			if wallet.PrivateKey != "" {
				fmt.Printf("    Private Key: %s\n", wallet.PrivateKey)
			}
		}
	}
}

func printEndpoints(endpoints EndpointMap) {
	for name, info := range endpoints {
		host := info.Host
		if host == "" {
			host = "localhost"
		}
		protocol := name
		if strings.Contains(name, "http") || strings.Contains(name, "rpc") {
			protocol = "http"
			fmt.Printf("      %s: %s://%s:%d\n", name, protocol, host, info.Port)
		} else if strings.Contains(name, "ws") {
			protocol = "ws"
			fmt.Printf("      %s: %s://%s:%d\n", name, protocol, host, info.Port)
		}
	}
}
