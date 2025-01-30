package solc

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

type AbiType struct {
	Parsed abi.ABI
	Raw    interface{}
}

func (a *AbiType) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &a.Raw); err != nil {
		return err
	}
	return json.Unmarshal(data, &a.Parsed)
}

type CompilerInput struct {
	Language string                       `json:"language"`
	Sources  map[string]map[string]string `json:"sources"`
	Settings CompilerSettings             `json:"settings"`
}

type CompilerSettings struct {
	Optimizer       OptimizerSettings              `json:"optimizer"`
	Metadata        CompilerInputMetadata          `json:"metadata"`
	OutputSelection map[string]map[string][]string `json:"outputSelection"`
	EvmVersion      string                         `json:"evmVersion,omitempty"`
	Libraries       map[string]map[string]string   `json:"libraries,omitempty"`
}

type OptimizerSettings struct {
	Enabled bool `json:"enabled"`
	Runs    uint `json:"runs"`
}

type CompilerInputMetadata struct {
	UseLiteralContent bool `json:"useLiteralContent"`
}

type CompilerOutput struct {
	Contracts map[string]CompilerOutputContracts `json:"contracts"`
	Sources   CompilerOutputSources              `json:"sources"`
}

type CompilerOutputContracts map[string]CompilerOutputContract

// CompilerOutputContract represents the solc compiler output for a contract.
// Ignoring some fields such as devdoc and userdoc.
type CompilerOutputContract struct {
	Abi           AbiType           `json:"abi"`
	Evm           CompilerOutputEvm `json:"evm"`
	Metadata      string            `json:"metadata"`
	StorageLayout StorageLayout     `json:"storageLayout"`
}

// StorageLayout represents the solc compilers output storage layout for
// a contract.
type StorageLayout struct {
	Storage []StorageLayoutEntry         `json:"storage"`
	Types   map[string]StorageLayoutType `json:"types"`
}

// GetStorageLayoutEntry returns the StorageLayoutEntry where the label matches
// the provided name.
func (s *StorageLayout) GetStorageLayoutEntry(name string) (StorageLayoutEntry, error) {
	for _, entry := range s.Storage {
		if entry.Label == name {
			return entry, nil
		}
	}
	return StorageLayoutEntry{}, fmt.Errorf("%s not found", name)
}

// GetStorageLayoutType returns the StorageLayoutType where the label matches
// the provided name.
func (s *StorageLayout) GetStorageLayoutType(name string) (StorageLayoutType, error) {
	if ty, ok := s.Types[name]; ok {
		return ty, nil
	}
	return StorageLayoutType{}, fmt.Errorf("%s not found", name)
}

type AbiSpecStorageLayoutEntry struct {
	Bytes  uint   `json:"bytes,string"`
	Label  string `json:"label"`
	Offset uint   `json:"offset"`
	Slot   uint   `json:"slot,string"`
	Type   string `json:"type"`
}

type StorageLayoutEntry struct {
	AstId    uint   `json:"astId"`
	Contract string `json:"contract"`
	Label    string `json:"label"`
	Offset   uint   `json:"offset"`
	Slot     uint   `json:"slot,string"`
	Type     string `json:"type"`
}

type StorageLayoutType struct {
	Encoding      string               `json:"encoding"`
	Label         string               `json:"label"`
	NumberOfBytes uint                 `json:"numberOfBytes,string"`
	Key           string               `json:"key,omitempty"`
	Value         string               `json:"value,omitempty"`
	Base          string               `json:"base,omitempty"`
	Members       []StorageLayoutEntry `json:"members,omitempty"`
}

type CompilerOutputEvm struct {
	Bytecode          CompilerOutputBytecode       `json:"bytecode"`
	DeployedBytecode  CompilerOutputBytecode       `json:"deployedBytecode"`
	GasEstimates      map[string]map[string]string `json:"gasEstimates"`
	MethodIdentifiers map[string]string            `json:"methodIdentifiers"`
}

// Object must be a string because its not guaranteed to be
// a hex string
type CompilerOutputBytecode struct {
	Object         string         `json:"object"`
	Opcodes        string         `json:"opcodes"`
	SourceMap      string         `json:"sourceMap"`
	LinkReferences LinkReferences `json:"linkReferences"`
}

type LinkReferences map[string]LinkReference
type LinkReference map[string][]LinkReferenceOffset

type LinkReferenceOffset struct {
	Length uint `json:"length"`
	Start  uint `json:"start"`
}

type CompilerOutputSources map[string]CompilerOutputSource

type CompilerOutputSource struct {
	Id  uint `json:"id"`
	Ast Ast  `json:"ast"`
}

type Ast struct {
	AbsolutePath    string            `json:"absolutePath"`
	ExportedSymbols map[string][]uint `json:"exportedSymbols"`
	Id              uint              `json:"id"`
	License         string            `json:"license"`
	NodeType        string            `json:"nodeType"`
	Nodes           []AstNode         `json:"nodes"`
	Src             string            `json:"src"`
}

type AstNode struct {
	Id                      int               `json:"id"`
	NodeType                string            `json:"nodeType"`
	Src                     string            `json:"src"`
	Nodes                   []AstNode         `json:"nodes,omitempty"`
	Abstract                bool              `json:"abstract,omitempty"`
	BaseContracts           []AstBaseContract `json:"baseContracts,omitempty"`
	CanonicalName           string            `json:"canonicalName,omitempty"`
	ContractDependencies    []int             `json:"contractDependencies,omitempty"`
	ContractKind            string            `json:"contractKind,omitempty"`
	Documentation           interface{}       `json:"documentation,omitempty"`
	FullyImplemented        bool              `json:"fullyImplemented,omitempty"`
	LinearizedBaseContracts []int             `json:"linearizedBaseContracts,omitempty"`
	Name                    string            `json:"name,omitempty"`
	NameLocation            string            `json:"nameLocation,omitempty"`
	Scope                   int               `json:"scope,omitempty"`
	UsedErrors              []int             `json:"usedErrors,omitempty"`
	UsedEvents              []int             `json:"usedEvents,omitempty"`

	// Function specific
	Body             *AstBlock         `json:"body,omitempty"`
	Parameters       *AstParameterList `json:"parameters,omitempty"`
	ReturnParameters *AstParameterList `json:"returnParameters,omitempty"`
	StateMutability  string            `json:"stateMutability,omitempty"`
	Virtual          bool              `json:"virtual,omitempty"`
	Visibility       string            `json:"visibility,omitempty"`

	// Variable specific
	Constant         bool                 `json:"constant,omitempty"`
	Mutability       string               `json:"mutability,omitempty"`
	StateVariable    bool                 `json:"stateVariable,omitempty"`
	StorageLocation  string               `json:"storageLocation,omitempty"`
	TypeDescriptions *AstTypeDescriptions `json:"typeDescriptions,omitempty"`
	TypeName         *AstTypeName         `json:"typeName,omitempty"`

	// Expression specific
	Expression      *Expression `json:"expression,omitempty"`
	IsConstant      bool        `json:"isConstant,omitempty"`
	IsLValue        bool        `json:"isLValue,omitempty"`
	IsPure          bool        `json:"isPure,omitempty"`
	LValueRequested bool        `json:"lValueRequested,omitempty"`

	// Literal specific
	HexValue string      `json:"hexValue,omitempty"`
	Kind     string      `json:"kind,omitempty"`
	Value    interface{} `json:"value,omitempty"`

	// Other fields
	Arguments []Expression `json:"arguments,omitempty"`
	Condition *Expression  `json:"condition,omitempty"`
	TrueBody  *AstBlock    `json:"trueBody,omitempty"`
	FalseBody *AstBlock    `json:"falseBody,omitempty"`
	Operator  string       `json:"operator,omitempty"`
}

type AstBaseContract struct {
	BaseName *AstTypeName `json:"baseName"`
	Id       int          `json:"id"`
	NodeType string       `json:"nodeType"`
	Src      string       `json:"src"`
}

type AstDocumentation struct {
	Id       int    `json:"id"`
	NodeType string `json:"nodeType"`
	Src      string `json:"src"`
	Text     string `json:"text"`
}

type AstBlock struct {
	Id         int       `json:"id"`
	NodeType   string    `json:"nodeType"`
	Src        string    `json:"src"`
	Statements []AstNode `json:"statements"`
}

type AstParameterList struct {
	Id         int       `json:"id"`
	NodeType   string    `json:"nodeType"`
	Parameters []AstNode `json:"parameters"`
	Src        string    `json:"src"`
}

type AstTypeDescriptions struct {
	TypeIdentifier string `json:"typeIdentifier"`
	TypeString     string `json:"typeString"`
}

type AstTypeName struct {
	Id               int                  `json:"id"`
	Name             string               `json:"name"`
	NodeType         string               `json:"nodeType"`
	Src              string               `json:"src"`
	StateMutability  string               `json:"stateMutability,omitempty"`
	TypeDescriptions *AstTypeDescriptions `json:"typeDescriptions,omitempty"`
}

type Expression struct {
	Id                     int                   `json:"id"`
	NodeType               string                `json:"nodeType"`
	Src                    string                `json:"src"`
	TypeDescriptions       *AstTypeDescriptions  `json:"typeDescriptions,omitempty"`
	Name                   string                `json:"name,omitempty"`
	OverloadedDeclarations []int                 `json:"overloadedDeclarations,omitempty"`
	ReferencedDeclaration  int                   `json:"referencedDeclaration,omitempty"`
	ArgumentTypes          []AstTypeDescriptions `json:"argumentTypes,omitempty"`
}

type ForgeArtifact struct {
	Abi               AbiType                `json:"abi"`
	Bytecode          CompilerOutputBytecode `json:"bytecode"`
	DeployedBytecode  CompilerOutputBytecode `json:"deployedBytecode"`
	MethodIdentifiers map[string]string      `json:"methodIdentifiers"`
	RawMetadata       string                 `json:"rawMetadata"`
	Metadata          ForgeCompilerMetadata  `json:"metadata"`
	StorageLayout     *StorageLayout         `json:"storageLayout,omitempty"`
	Ast               Ast                    `json:"ast"`
	Id                int                    `json:"id"`
}

type ForgeCompilerMetadata struct {
	Compiler ForgeCompilerInfo          `json:"compiler"`
	Language string                     `json:"language"`
	Output   ForgeMetadataOutput        `json:"output"`
	Settings CompilerSettings           `json:"settings"`
	Sources  map[string]ForgeSourceInfo `json:"sources"`
	Version  int                        `json:"version"`
}

type ForgeCompilerInfo struct {
	Version string `json:"version"`
}

type ForgeMetadataOutput struct {
	Abi     AbiType        `json:"abi"`
	DevDoc  ForgeDocObject `json:"devdoc"`
	UserDoc ForgeDocObject `json:"userdoc"`
}

type ForgeSourceInfo struct {
	Keccak256 string   `json:"keccak256"`
	License   string   `json:"license"`
	Urls      []string `json:"urls"`
}

type ForgeDocObject struct {
	Kind    string                 `json:"kind"`
	Methods map[string]interface{} `json:"methods"`
	Notice  string                 `json:"notice,omitempty"`
	Version int                    `json:"version"`
}
