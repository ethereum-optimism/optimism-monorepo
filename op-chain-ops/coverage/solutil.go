package coverage

import (
	"encoding/hex"
	"fmt"
	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-chain-ops/srcmap"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/crypto"
	"log"
	"os"
	"regexp"
)

type CoverageTracer struct {
	SourceMapFS        *foundry.SourceMapFS
	ExecutedSources    map[string]map[int]bool
	ExecutedFunctions  map[string]map[string]bool
	FunctionSignatures map[string]map[string]string
	SourceMaps         map[string]*srcmap.SourceMap
	Artifacts          []*foundry.Artifact
	ContractMappings   map[string]string
}

func NewCoverageTracer(artifacts []*foundry.Artifact) (*CoverageTracer, error) {
	tracer := &CoverageTracer{
		SourceMapFS:        foundry.NewSourceMapFS(os.DirFS("../../packages/contracts-bedrock")),
		ExecutedSources:    make(map[string]map[int]bool),
		ExecutedFunctions:  make(map[string]map[string]bool),
		FunctionSignatures: make(map[string]map[string]string),
		SourceMaps:         make(map[string]*srcmap.SourceMap),
		Artifacts:          artifacts,
		ContractMappings:   make(map[string]string),
	}

	for _, artifact := range artifacts {
		for _, name := range artifact.Metadata.Settings.CompilationTarget {
			srcMap, err := tracer.SourceMapFS.SourceMap(artifact, name)
			if err != nil {
				log.Printf("Failed to load SourceMap for contract %s: %v", name, err)
				continue
			}

			tracer.SourceMaps[name] = srcMap
			log.Printf("Loaded SourceMap for contract %s", name)

			for pc := 0; pc < len(artifact.DeployedBytecode.Object); pc++ {
				source, line, _, err := srcMap.Info(uint64(pc))
				if source == "unknown" || line == 0 {
					continue
				}

				if err != nil {
					log.Printf("Error mapping PC to source for contract %s: %v", name, err)
					break
				}

				if _, exists := tracer.ExecutedSources[source]; !exists {
					tracer.ExecutedSources[source] = make(map[int]bool)
				}
				tracer.ExecutedSources[source][int(line)] = false
			}
			contractSignatures := make(map[string]string)
			executedFunctions := make(map[string]bool)

			for _, method := range artifact.ABI.Methods {
				selector := fmt.Sprintf("%x", crypto.Keccak256([]byte(method.Sig))[:4]) // Calculate selector
				contractSignatures[selector] = method.Name
				executedFunctions[method.Name] = false
			}
			tracer.FunctionSignatures[name] = contractSignatures
			tracer.ExecutedFunctions[name] = executedFunctions
		}
	}

	return tracer, nil
}

func (s *CoverageTracer) OnOpCode(pc uint64, opcode byte, gas, cost uint64, scope tracing.OpContext, rData []byte, depth int, err error) {
	contractAddr := scope.Address().String()

	if _, exists := s.ContractMappings[contractAddr]; !exists {
		for name, srcMap := range s.SourceMaps {
			if srcMap != nil {
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
	if depth == 1 { // Check for function entry
		code := scope.CallInput()
		if len(code) >= 4 {
			selector := code[:4] // Function selector

			if functionName, exists := s.FunctionSignatures[contractName][hex.EncodeToString(selector)]; exists {
				if _, exists := s.ExecutedFunctions[contractName]; !exists {
					s.ExecutedFunctions[contractName] = make(map[string]bool)
				}
				s.ExecutedFunctions[contractName][functionName] = true
			}

		}
	}
}

func (s *CoverageTracer) GenerateLCOV(outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create LCOV file: %w", err)
	}
	defer file.Close()

	for filePath, lines := range s.ExecutedSources {
		re := regexp.MustCompile(`([^/]+)\.[^/]+$`)
		match := re.FindStringSubmatch(filePath)

		fmt.Fprintf(file, "SF:%s\n", filePath)

		if functions, ok := s.ExecutedFunctions[match[1]]; ok {
			for function, executed := range functions {
				fmt.Fprintf(file, "FN:0,%s\n", function)
				executionStatus := 0
				if executed {
					executionStatus = 1
				}
				fmt.Fprintf(file, "FNDA:%d,%s\n", executionStatus, function)
			}
		}

		for line, executed := range lines {
			executionStatus := 0
			if executed {
				executionStatus = 1
			}
			fmt.Fprintf(file, "DA:%d,%ds\n", line, executionStatus)
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
