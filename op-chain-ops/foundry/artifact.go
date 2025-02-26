package foundry

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum-optimism/optimism/op-chain-ops/solc"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// Artifact represents a foundry compilation artifact.
// JSON marshaling logic is implemented to maintain the ability
// to roundtrip serialize an artifact
type Artifact struct {
	ABI              abi.ABI
	abi              json.RawMessage
	StorageLayout    solc.StorageLayout
	DeployedBytecode DeployedBytecode
	Bytecode         Bytecode
	Metadata         Metadata
}

func (a *Artifact) UnmarshalJSON(data []byte) error {
	artifact := artifactMarshaling{}
	if err := json.Unmarshal(data, &artifact); err != nil {
		return err
	}
	parsed, err := abi.JSON(strings.NewReader(string(artifact.ABI)))
	if err != nil {
		return err
	}
	a.ABI = parsed
	a.abi = artifact.ABI
	a.StorageLayout = artifact.StorageLayout
	a.DeployedBytecode = artifact.DeployedBytecode
	a.Bytecode = artifact.Bytecode
	a.Metadata = artifact.Metadata
	return nil
}

func (a Artifact) MarshalJSON() ([]byte, error) {
	artifact := artifactMarshaling{
		ABI:              a.abi,
		StorageLayout:    a.StorageLayout,
		DeployedBytecode: a.DeployedBytecode,
		Bytecode:         a.Bytecode,
		Metadata:         a.Metadata,
	}
	return json.Marshal(artifact)
}

// artifactMarshaling is a helper struct for marshaling and unmarshalling
// foundry artifacts.
type artifactMarshaling struct {
	ABI              json.RawMessage    `json:"abi"`
	Source           string             `json:"source"`
	StorageLayout    solc.StorageLayout `json:"storageLayout"`
	DeployedBytecode DeployedBytecode   `json:"deployedBytecode"`
	Bytecode         Bytecode           `json:"bytecode"`
	Metadata         Metadata           `json:"metadata"`
}

// Metadata is the subset of metadata in a foundry contract artifact that we use in OP-Stack tooling.
type Metadata struct {
	Compiler struct {
		Version string `json:"version"`
	} `json:"compiler"`

	Language string `json:"language"`

	Output json.RawMessage `json:"output"`

	Settings struct {
		// Remappings of the contract imports
		Remappings []string `json:"remappings"`
		// Optimizer settings affect the compiler output, but can be arbitrary.
		// We load them opaquely, to include it in the hash of what we run.
		Optimizer json.RawMessage `json:"optimizer"`
		// Metadata is loaded opaquely, similar to the Optimizer, to include in hashing.
		// E.g. the bytecode-hash contract suffix as setting is enabled/disabled in here.
		Metadata json.RawMessage `json:"metadata"`
		// Map of full contract path to compiled contract name.
		CompilationTarget map[string]string `json:"compilationTarget"`
		// EVM version affects output, and hence included.
		EVMVersion string `json:"evmVersion"`
		// Libraries data
		Libraries json.RawMessage `json:"libraries"`
	} `json:"settings"`

	Sources map[string]ContractSource `json:"sources"`

	Version int `json:"version"`
}

// ContractSource represents a JSON value in the "sources" map of a contract metadata dump.
// This uniquely identifies the source code of the contract.
type ContractSource struct {
	Keccak256 common.Hash `json:"keccak256"`
	URLs      []string    `json:"urls"`
	Content   string      `json:"content"`
	License   string      `json:"license"`
}

var ErrLinkingUnsupported = errors.New("cannot load bytecode with linking placeholders")

// LinkableBytecode is not purely hex, it returns an ErrLinkingUnsupported error when
// input contains __$aaaaaaa$__ style linking placeholders.
// See https://docs.soliditylang.org/en/latest/using-the-compiler.html#library-linking
// In practice this is only used by test contracts to link in large test libraries.
type LinkableBytecode []byte

func (lb *LinkableBytecode) UnmarshalJSON(data []byte) error {
	if bytes.Contains(data, []byte("__$")) {
		return ErrLinkingUnsupported
	}
	return (*hexutil.Bytes)(lb).UnmarshalJSON(data)
}

func (lb LinkableBytecode) MarshalText() ([]byte, error) {
	return (hexutil.Bytes)(lb).MarshalText()
}

// DeployedBytecode represents the deployed bytecode section of the solc compiler output.
type DeployedBytecode struct {
	SourceMap           string           `json:"sourceMap"`
	Object              LinkableBytecode `json:"object"`
	LinkReferences      json.RawMessage  `json:"linkReferences"`
	ImmutableReferences json.RawMessage  `json:"immutableReferences,omitempty"`
}

// Bytecode represents the bytecode section of the solc compiler output.
type Bytecode struct {
	SourceMap string `json:"sourceMap"`
	// not purely hex, can contain __$aaaaaaa$__ style linking placeholders
	Object              LinkableBytecode `json:"object"`
	LinkReferences      json.RawMessage  `json:"linkReferences"`
	ImmutableReferences json.RawMessage  `json:"immutableReferences,omitempty"`
}

// ReadArtifact will read an artifact from disk given a path.
func ReadArtifact(path string) (*Artifact, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("artifact at %s not found: %w", path, err)
	}
	artifact := Artifact{}
	if err := json.Unmarshal(file, &artifact); err != nil {
		return nil, err
	}
	return &artifact, nil
}

// SearchRemappings applies the configured remappings to a given source path,
// or returns the source path unchanged if no remapping is found. It assumes that
// each remapping is of the form "alias/=actualPath".
func (a Artifact) SearchRemappings(sourcePath string) string {
	for _, mapping := range a.Metadata.Settings.Remappings {
		parts := strings.Split(mapping, "/=")
		if len(parts) != 2 {
			continue
		}
		alias := parts[0]
		if !strings.HasSuffix(alias, "/") {
			alias += "/"
		}
		actualPath := parts[1]
		if !strings.HasSuffix(actualPath, "/") {
			actualPath += "/"
		}
		if strings.HasPrefix(sourcePath, actualPath) {
			return alias + sourcePath[len(actualPath):]
		}
	}
	return sourcePath
}
