package deployer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os/exec"
	"strings"
)

const (
	defaultDeployerArtifactName = "op-deployer-configs"
	defaultWalletsName          = "wallets.json"
	defaultStateName            = "state.json"
	defaultGenesisArtifactName  = "el_cl_genesis_data"
	defaultMnemonicsName        = "mnemonics.yaml"
)

// DeploymentAddresses maps contract names to their addresses
type DeploymentAddresses map[string]string

// DeploymentStateAddresses maps chain IDs to their contract addresses
type DeploymentStateAddresses map[string]DeploymentAddresses

// StateFile represents the structure of the state.json file
type StateFile struct {
	OpChainDeployments []map[string]interface{} `json:"opChainDeployments"`
}

// Wallet represents a wallet with optional private key and name
type Wallet struct {
	Address    string
	PrivateKey string
	Name       string
}

// WalletList holds a list of wallets
type WalletList []*Wallet

type DeployerData struct {
	Wallets WalletList
	State   DeploymentStateAddresses
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
func parseWalletsFile(r io.Reader) (WalletList, error) {
	// Read all data from reader
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read wallet file: %w", err)
	}

	// Unmarshal into a map first
	var rawData map[string]string
	if err := json.Unmarshal(data, &rawData); err != nil {
		return nil, fmt.Errorf("failed to decode wallet file: %w", err)
	}

	// Create a map to store wallets by name
	walletMap := make(map[string]Wallet)

	// Process each key-value pair
	for key, value := range rawData {
		if strings.HasSuffix(key, "Address") {
			name := strings.TrimSuffix(key, "Address")
			wallet := walletMap[name]
			wallet.Address = value
			wallet.Name = name
			walletMap[name] = wallet
		} else if strings.HasSuffix(key, "PrivateKey") {
			name := strings.TrimSuffix(key, "PrivateKey")
			wallet := walletMap[name]
			wallet.PrivateKey = value
			wallet.Name = name
			walletMap[name] = wallet
		}
	}

	// Convert map to list
	result := make(WalletList, 0, len(walletMap))

	for _, wallet := range walletMap {
		// Only include wallets that have at least an address
		if wallet.Address != "" {
			result = append(result, &wallet)
		}
	}

	return result, nil
}

// downloadArtifact downloads a kurtosis artifact to a temporary directory
// TODO: reimplement this using the kurtosis SDK
func downloadArtifact(enclave, artifact, destDir string) error {
	cmd := exec.Command("kurtosis", "files", "download", enclave, artifact, destDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to download artifact %s: %w", artifact, err)
	}
	return nil
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
func parseStateFile(r io.Reader) (DeploymentStateAddresses, error) {
	var state StateFile
	if err := json.NewDecoder(r).Decode(&state); err != nil {
		return nil, fmt.Errorf("failed to decode state file: %w", err)
	}

	result := make(DeploymentStateAddresses)

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

		addresses := make(DeploymentAddresses)

		// Look for address fields in the deployment map
		for key, value := range deployment {
			if strings.HasSuffix(key, "Address") {
				key = strings.TrimSuffix(key, "Address")
				addresses[key] = value.(string)
			}
		}

		if len(addresses) > 0 {
			result[id] = addresses
		}
	}

	return result, nil
}

// ExtractData downloads and parses the op-deployer state
func (d *Deployer) ExtractData(ctx context.Context) (*DeployerData, error) {
	fs, err := NewEnclaveFS(ctx, d.enclave)
	if err != nil {
		return nil, err
	}

	artifact, err := fs.GetArtifact(ctx, d.deployerArtifactName)
	if err != nil {
		return nil, err
	}

	stateBuffer := bytes.NewBuffer(nil)
	walletsBuffer := bytes.NewBuffer(nil)
	if err := artifact.ExtractFiles(
		&ArtifactFileWriter{path: d.stateName, writer: stateBuffer},
		&ArtifactFileWriter{path: d.walletsName, writer: walletsBuffer},
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

	knownWallets, err := d.getKnownWallets(ctx, fs)
	if err != nil {
		return nil, err
	}

	wallets = append(wallets, knownWallets...)

	return &DeployerData{State: state, Wallets: wallets}, nil
}
