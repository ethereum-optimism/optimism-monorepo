package system

import (
	"context"

	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"

	"github.com/ethereum-optimism/optimism/devnet-sdk/constraints"

	"github.com/ethereum-optimism/optimism/devnet-sdk/descriptors"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
)

type System interface {
	Chain(chainID types.ChainID) Chain
}

var _ System = system{}

type system struct {
	chains map[types.ChainID]Chain
}

func NewSystemFromEnv(envVar string) (System, error) {
	devnetFile := os.Getenv(envVar)
	if devnetFile == "" {
		return nil, fmt.Errorf("env var '%s' is unset", envVar)
	}
	devnet, err := devnetFromFile(devnetFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse devnet file: %v", err)
	}
	return systemFromDevnet(*devnet)
}

func (s system) Chain(chainID types.ChainID) Chain {
	return s.chains[chainID]
}

func (s system) addChains(chains ...*descriptors.Chain) error {
	for _, chainDesc := range chains {
		chainID, err := strconv.ParseUint(chainDesc.ID, 10, 64)
		if err != nil {
			return err
		}
		s.chains[types.ChainID(chainID)] = chainFromDescriptor(chainDesc)
	}
	return nil
}

// devnetFromFile reads a DevnetEnvironment from a JSON file.
func devnetFromFile(devnetFile string) (*descriptors.DevnetEnvironment, error) {
	data, err := os.ReadFile(devnetFile)
	if err != nil {
		return nil, fmt.Errorf("error reading devnet file: %w", err)
	}

	var config descriptors.DevnetEnvironment
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}
	return &config, nil
}

func systemFromDevnet(dn descriptors.DevnetEnvironment) (System, error) {
	chains := make(map[types.ChainID]Chain)
	sys := system{chains: chains}

	if err := sys.addChains(append(dn.L2, dn.L1)...); err != nil {
		return nil, err
	}

	if slices.Contains(dn.Features, "interop") {
		return interopSystem{system: sys}, nil
	}
	return sys, nil
}

type Chain interface {
	RPCURL() string
	ContractAddress(contractID string) types.Address
	User(ctx context.Context, constraints ...constraints.Constraint) (types.Wallet, error)
}

type chain struct {
	id     string
	rpcUrl string

	addresses map[string]types.Address
	user      types.Wallet
}

func NewChain(chainID string, rpcUrl string, user types.Wallet) chain {
	return chain{
		id: chainID,
		addresses: map[string]types.Address{
			"SuperchainWETH":             "0x4200000000000000000000000000000000000024",
			"ETHLiquidity":               "0x4200000000000000000000000000000000000025",
			"L2ToL2CrossDomainMessenger": "0x4200000000000000000000000000000000000023",
		},
		user: user,
	}
}

func chainFromDescriptor(d *descriptors.Chain) Chain {
	firstNodeRPC := d.Nodes[0].Services["el"].Endpoints["rpc"]
	rpcURL := fmt.Sprintf("%s:%d", firstNodeRPC.Host, firstNodeRPC.Port)
	var user wallet // for now, we'll just grab the first wallet
	for _, walletDescriptor := range d.Wallets {
		user = NewWallet(
			walletDescriptor.PrivateKey,
			types.Address(walletDescriptor.Address),
		)
		break
	}
	return NewChain(d.ID, rpcURL, user)
}

func (c chain) ContractAddress(contractID string) types.Address {
	return c.addresses[contractID]
}

func (c chain) RPCURL() string {
	return c.rpcUrl
}

func (c chain) User(ctx context.Context, constraints ...constraints.Constraint) (types.Wallet, error) {
	return c.user, nil
}

type InteropSystem interface {
	System
	InteropSet() InteropSet
}

var _ InteropSystem = interopSystem{}

type interopSystem struct {
	system
}

func (i interopSystem) InteropSet() InteropSet {
	return i.system // TODO
}

type InteropSet interface {
	Chain(chainID types.ChainID) Chain
}

type wallet struct {
	privateKey types.Key
	address    types.Address
}

func NewWallet(pk types.Key, addr types.Address) wallet {
	return wallet{privateKey: pk, address: addr}
}

func (w wallet) PrivateKey() types.Key {
	return w.privateKey
}

func (w wallet) Address() types.Address {
	return w.address
}
