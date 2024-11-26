package coverage

import (
	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"log"
	"testing"
)

func TestEverything(t *testing.T) {
	artifactFS := foundry.OpenArtifactsDir(".../../packages/contracts-bedrock/forge-artifacts")

	artifact1, err := artifactFS.ReadArtifact("SimpleStorage.sol", "SimpleStorage")
	if err != nil {
		log.Fatalf("Failed to load artifact: %v", err)
	}

	artifacts := []*foundry.Artifact{artifact1}

	tracer := NewCoverageTracer(artifacts)

	if err := tracer.LoadSourceMaps(); err != nil {
		log.Fatalf("Failed to load SourceMaps: %v", err)
	}
	
	if err := tracer.GenerateLCOV("coverage.lcov"); err != nil {
		log.Fatalf("Failed to generate LCOV: %v", err)
	}
}
