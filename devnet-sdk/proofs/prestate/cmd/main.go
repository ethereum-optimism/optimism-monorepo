package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ethereum-optimism/optimism/devnet-sdk/proofs/prestate"
)

type chainConfig struct {
	id            string
	rollupConfig  string
	genesisConfig string
}

func parseChainFlag(s string) (*chainConfig, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 3 {
		return nil, fmt.Errorf("chain flag must contain exactly 1 id and 2 files separated by comma")
	}
	return &chainConfig{
		id:            strings.TrimSpace(parts[0]),
		rollupConfig:  strings.TrimSpace(parts[1]),
		genesisConfig: strings.TrimSpace(parts[2]),
	}, nil
}

func main() {
	var (
		clientURL = flag.String("url", "http://localhost:8080", "URL of the prestate builder service")
		interop   = flag.Bool("interop", false, "Generate interop dependency set")
		chains    = make(chainConfigList, 0)
	)

	flag.Var(&chains, "chain", "Chain configuration files in format: rollup-config.json,genesis-config.json (can be specified multiple times)")
	flag.Parse()

	client := prestate.NewPrestateBuilderClient(*clientURL)
	ctx := context.Background()

	// Build options list
	opts := make([]prestate.PrestateBuilderOption, 0)

	if *interop {
		opts = append(opts, prestate.WithGeneratedInteropDepSet())
	}

	// Add chain configs
	for i, chain := range chains {
		rollupFile, err := os.Open(chain.rollupConfig)
		if err != nil {
			log.Fatalf("Failed to open rollup config file for chain %d: %v", i, err)
		}
		defer rollupFile.Close()

		genesisFile, err := os.Open(chain.genesisConfig)
		if err != nil {
			log.Fatalf("Failed to open genesis config file for chain %d: %v", i, err)
		}
		defer genesisFile.Close()

		opts = append(opts, prestate.WithChainConfig(
			chain.id,
			rollupFile,
			genesisFile,
		))
	}

	// Build prestate
	manifest, err := client.BuildPrestate(ctx, opts...)
	if err != nil {
		log.Fatalf("Failed to build prestate: %v", err)
	}

	// Print manifest
	for id, hash := range manifest {
		fmt.Printf("%s: %s\n", id, hash)
	}
}

// chainConfigList implements flag.Value interface for repeated chain flags
type chainConfigList []*chainConfig

func (c *chainConfigList) String() string {
	return fmt.Sprintf("%v", *c)
}

func (c *chainConfigList) Set(value string) error {
	config, err := parseChainFlag(value)
	if err != nil {
		return err
	}
	*c = append(*c, config)
	return nil
}
