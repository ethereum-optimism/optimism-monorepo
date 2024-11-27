package coverage

import (
	"fmt"
	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-chain-ops/srcmap"
	"github.com/ethereum/go-ethereum/core/tracing"
	"log"
	"os"
)

type CoverageTracer struct {
	SourceMapFS      *foundry.SourceMapFS
	ExecutedSources  map[string]map[int]bool
	SourceMaps       map[string]*srcmap.SourceMap
	Artifacts        []*foundry.Artifact
	ContractMappings map[string]string
}

func NewCoverageTracer(artifacts []*foundry.Artifact) (*CoverageTracer, error) {
	tracer := &CoverageTracer{
		SourceMapFS:      foundry.NewSourceMapFS(os.DirFS("../../packages/contracts-bedrock")),
		ExecutedSources:  make(map[string]map[int]bool),
		SourceMaps:       make(map[string]*srcmap.SourceMap),
		Artifacts:        artifacts,
		ContractMappings: make(map[string]string),
	}

	// Load source maps during initialization
	for _, artifact := range artifacts {
		for _, name := range artifact.Metadata.Settings.CompilationTarget {
			srcMap, err := tracer.SourceMapFS.SourceMap(artifact, name)
			if err != nil {
				log.Printf("Failed to load SourceMap for contract %s: %v", name, err)
				continue
			}

			tracer.SourceMaps[name] = srcMap
			log.Printf("Loaded SourceMap for contract %s", name)
		}
	}

	return tracer, nil
}

func (s *CoverageTracer) OnOpCode(pc uint64, opcode byte, gas, cost uint64, scope tracing.OpContext, rData []byte, depth int, err error) {
	contractAddr := scope.Address().String()

	// Dynamically map contract addresses to names if not already mapped
	if _, exists := s.ContractMappings[contractAddr]; !exists {
		// Attempt to match the deployed bytecode with a contract name
		for name, srcMap := range s.SourceMaps {
			if srcMap != nil {
				// Match based on unique properties (extend logic if needed)
				s.ContractMappings[contractAddr] = name
				log.Printf("Mapped contract address %s to name %s", contractAddr, name)
				break
			}
		}
	}

	contractName, ok := s.ContractMappings[contractAddr]
	if !ok {
		log.Printf("No contract mapping found for address: %s", contractAddr)
		return
	}

	srcMap, ok := s.SourceMaps[contractName]
	if !ok {
		log.Printf("No SourceMap found for contract: %s", contractName)
		return
	}

	source, line, _, err := srcMap.Info(pc)
	if err != nil {
		log.Printf("Error mapping PC to source for contract %s: %v", contractName, err)
		return
	}

	if source == "generated" || source == "unknown" || line == 0 {
		return
	}

	if _, exists := s.ExecutedSources[source]; !exists {
		s.ExecutedSources[source] = make(map[int]bool)
	}
	s.ExecutedSources[source][int(line)] = true
}

func (s *CoverageTracer) GenerateLCOV(outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create LCOV file: %w", err)
	}
	defer file.Close()

	for filePath, lines := range s.ExecutedSources {
		fmt.Fprintf(file, "SF:%s\n", filePath)
		for line := range lines {
			fmt.Fprintf(file, "DA:%d,1\n", line)
		}
		fmt.Fprintln(file, "end_of_record")
	}

	log.Printf("LCOV report generated at %s\n", outputPath)
	return nil
}

func (s *CoverageTracer) Hooks() *tracing.Hooks {
	return &tracing.Hooks{
		OnOpcode: s.OnOpCode,
	}
}
