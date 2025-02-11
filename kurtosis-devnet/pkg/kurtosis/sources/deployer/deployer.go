package deployer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strings"

	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/artifact"
	"github.com/ethereum/go-ethereum/common"
)

const (
	defaultDeployerArtifactName = "op-deployer-configs"
	defaultWalletsName          = "wallets.json"
	defaultStateName            = "state.json"
	defaultGenesisArtifactName  = "el_cl_genesis_data"
	defaultMnemonicsName        = "mnemonics.yaml"
)

// DeploymentAddresses maps contract names to their addresses
type DeploymentAddresses map[string]types.Address

// DeploymentStateAddresses maps chain IDs to their contract addresses
type DeploymentStateAddresses map[string]DeploymentAddresses

type DeploymentState struct {
	Addresses DeploymentAddresses `json:"addresses"`
	Wallets   WalletList          `json:"wallets"`
}

type DeployerState struct {
	Deployments map[string]DeploymentState `json:"l2s"`
	Addresses   DeploymentAddresses        `json:"superchain"`
}

// StateFile represents the structure of the state.json file
type StateFile struct {
	OpChainDeployments        []map[string]interface{} `json:"opChainDeployments"`
	SuperChainDeployment      map[string]interface{}   `json:"superchainDeployment"`
	ImplementationsDeployment map[string]interface{}   `json:"implementationsDeployment"`
}

// Wallet represents a wallet with optional private key and name
type Wallet struct {
	Address    types.Address `json:"address"`
	PrivateKey string        `json:"private_key"`
	Name       string        `json:"name"`
}

// WalletList holds a list of wallets
type WalletList []*Wallet

type DeployerData struct {
	Wallets WalletList     `json:"wallets"`
	State   *DeployerState `json:"state"`
}

type Deployer struct {
	enclave              string
	deployerArtifactName string
	walletsName          string
	stateName            string
	genesisArtifactName  string
	mnemonicsName        string
}

type DeployerOption func(*Deployer)

func WithArtifactName(name string) DeployerOption {
	return func(d *Deployer) {
		d.deployerArtifactName = name
	}
}

func WithWalletsName(name string) DeployerOption {
	return func(d *Deployer) {
		d.walletsName = name
	}
}

func WithStateName(name string) DeployerOption {
	return func(d *Deployer) {
		d.stateName = name
	}
}

func WithGenesisArtifactName(name string) DeployerOption {
	return func(d *Deployer) {
		d.genesisArtifactName = name
	}
}

func WithMnemonicsName(name string) DeployerOption {
	return func(d *Deployer) {
		d.mnemonicsName = name
	}
}

func NewDeployer(enclave string, opts ...DeployerOption) *Deployer {
	d := &Deployer{
		enclave:              enclave,
		deployerArtifactName: defaultDeployerArtifactName,
		walletsName:          defaultWalletsName,
		stateName:            defaultStateName,
		genesisArtifactName:  defaultGenesisArtifactName,
		mnemonicsName:        defaultMnemonicsName,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

// parseWalletsFile parses a JSON file containing wallet information
func parseWalletsFile(r io.Reader) (map[string]WalletList, error) {
	result := make(map[string]WalletList)

	// Read all data from reader
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read wallet file: %w", err)
	}

	// Unmarshal into a map first
	var rawData map[string]map[string]string
	if err := json.Unmarshal(data, &rawData); err != nil {
		return nil, fmt.Errorf("failed to decode wallet file: %w", err)
	}

	for id, chain := range rawData {
		// Create a map to store wallets by name
		walletMap := make(map[string]Wallet)
		hasAddress := make(map[string]bool)

		// First pass: collect addresses
		for key, value := range chain {
			if strings.HasSuffix(key, "Address") {
				name := strings.TrimSuffix(key, "Address")
				wallet := walletMap[name]
				wallet.Address = common.HexToAddress(value)
				wallet.Name = name
				walletMap[name] = wallet
				hasAddress[name] = true
			}
		}

		// Second pass: collect private keys only for wallets with addresses
		for key, value := range chain {
			if strings.HasSuffix(key, "PrivateKey") {
				name := strings.TrimSuffix(key, "PrivateKey")
				if hasAddress[name] {
					wallet := walletMap[name]
					wallet.PrivateKey = value
					walletMap[name] = wallet
				}
			}
		}

		// Convert map to list, only including wallets with addresses
		wl := make(WalletList, 0, len(walletMap))
		for name, wallet := range walletMap {
			if hasAddress[name] {
				wl = append(wl, &wallet)
			}
		}

		result[id] = wl
	}

	return result, nil
}

// hexToDecimal converts a hex string (with or without 0x prefix) to a decimal string
func hexToDecimal(hex string) (string, error) {
	// Remove 0x prefix if present
	hex = strings.TrimPrefix(hex, "0x")

	// Parse hex string to big.Int
	n := new(big.Int)
	if _, ok := n.SetString(hex, 16); !ok {
		return "", fmt.Errorf("invalid hex string: %s", hex)
	}

	// Convert to decimal string
	return n.String(), nil
}

// parseStateFile parses the state.json file and extracts addresses
func parseStateFile(r io.Reader) (*DeployerState, error) {
	var state StateFile
	if err := json.NewDecoder(r).Decode(&state); err != nil {
		return nil, fmt.Errorf("failed to decode state file: %w", err)
	}

	result := &DeployerState{
		Deployments: make(map[string]DeploymentState),
		Addresses:   make(DeploymentAddresses),
	}

	mapDeployment := func(deployment map[string]interface{}) DeploymentAddresses {
		addrSuffix := "Address"
		addresses := make(DeploymentAddresses)
		for key, value := range deployment {
			if strings.HasSuffix(key, addrSuffix) {
				addresses[strings.TrimSuffix(key, addrSuffix)] = common.HexToAddress(value.(string))
			}
		}
		return addresses
	}

	for _, deployment := range state.OpChainDeployments {
		// Get the chain ID
		idValue, ok := deployment["id"]
		if !ok {
			continue
		}
		hexID, ok := idValue.(string)
		if !ok {
			continue
		}

		// Convert hex ID to decimal
		id, err := hexToDecimal(hexID)
		if err != nil {
			continue
		}

		addresses := mapDeployment(deployment)

		if len(addresses) > 0 {
			result.Deployments[id] = DeploymentState{
				Addresses: addresses,
			}
		}
	}

	result.Addresses = mapDeployment(state.ImplementationsDeployment)
	// merge the superchain and implementations addresses
	for key, value := range mapDeployment(state.SuperChainDeployment) {
		result.Addresses[key] = value
	}

	return result, nil
}

// ExtractData downloads and parses the op-deployer state
func (d *Deployer) ExtractData(ctx context.Context) (*DeployerData, error) {
	fs, err := artifact.NewEnclaveFS(ctx, d.enclave)
	if err != nil {
		return nil, err
	}

	a, err := fs.GetArtifact(ctx, d.deployerArtifactName)
	if err != nil {
		return nil, err
	}

	stateBuffer := bytes.NewBuffer(nil)
	walletsBuffer := bytes.NewBuffer(nil)
	if err := a.ExtractFiles(
		artifact.NewArtifactFileWriter(d.stateName, stateBuffer),
		artifact.NewArtifactFileWriter(d.walletsName, walletsBuffer),
	); err != nil {
		return nil, err
	}

	state, err := parseStateFile(stateBuffer)
	if err != nil {
		return nil, err
	}

	wallets, err := parseWalletsFile(walletsBuffer)
	if err != nil {
		return nil, err
	}

	for id, wallets := range wallets {
		if deployment, exists := state.Deployments[id]; exists {
			deployment.Wallets = wallets
			state.Deployments[id] = deployment
		}
	}

	knownWallets, err := d.getKnownWallets(ctx, fs)
	if err != nil {
		return nil, err
	}

	return &DeployerData{
		State:   state,
		Wallets: knownWallets,
	}, nil
}
