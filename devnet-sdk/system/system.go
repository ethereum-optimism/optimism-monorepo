package system

import (
	"context"
	"math/big"
	"strconv"
	"strings"

	"encoding/json"
	"fmt"
	"os"
	"slices"

	"github.com/ethereum-optimism/optimism/devnet-sdk/constraints"
	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	coreTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/ethereum-optimism/optimism/devnet-sdk/descriptors"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
)

type System interface {
	Identifier() string
	L1() Chain
	L2(uint64) Chain
}

var _ System = (*system)(nil)

type system struct {
	identifier string
	l1         Chain
	l2s        []Chain
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

	// Extract basename without extension from devnetFile path
	basename := devnetFile
	if lastSlash := strings.LastIndex(basename, "/"); lastSlash >= 0 {
		basename = basename[lastSlash+1:]
	}
	if lastDot := strings.LastIndex(basename, "."); lastDot >= 0 {
		basename = basename[:lastDot]
	}

	sys, err := systemFromDevnet(*devnet, basename)
	if err != nil {
		return nil, fmt.Errorf("failed to create system from devnet file: %v", err)
	}
	return sys, nil
}

func (s *system) L1() Chain {
	return s.l1
}

func (s *system) L2(chainID uint64) Chain {
	return s.l2s[chainID]
}

func (s *system) Identifier() string {
	return s.identifier
}

func (s *system) addChains(chains ...*descriptors.Chain) error {
	for _, chainDesc := range chains {
		if chainDesc.ID == "" {
			s.l1 = chainFromDescriptor(chainDesc)
		} else {
			s.l2s = append(s.l2s, chainFromDescriptor(chainDesc))
		}
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

func systemFromDevnet(dn descriptors.DevnetEnvironment, identifier string) (System, error) {
	sys := &system{identifier: identifier}

	if err := sys.addChains(append(dn.L2, dn.L1)...); err != nil {
		return nil, err
	}

	if slices.Contains(dn.Features, "interop") {
		return &interopSystem{system: sys}, nil
	}
	return sys, nil
}

type Chain interface {
	RPCURL() string
	ID() types.ChainID
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
			"SuperchainWETH":             constants.SuperchainWETH,
			"ETHLiquidity":               constants.ETHLiquidity,
			"L2ToL2CrossDomainMessenger": constants.L2ToL2CrossDomainMessenger,
		},
		rpcUrl: rpcUrl,
		user:   user,
	}
}

func chainFromDescriptor(d *descriptors.Chain) Chain {
	firstNodeRPC := d.Nodes[0].Services["el"].Endpoints["rpc"]
	rpcURL := fmt.Sprintf("http://%s:%d", firstNodeRPC.Host, firstNodeRPC.Port)

	user := NewWallet( // for now, we'll just grab the first BIP39 wallet
		"ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
		types.Address("f39Fd6e51aad88F6F4ce6aB8827279cffFb92266"),
		rpcURL,
	)
	c := NewChain(d.ID, rpcURL, user)
	return c
}

func (c chain) ContractAddress(contractID string) types.Address {
	if addr, ok := c.addresses[contractID]; ok {
		return addr
	}
	return types.Address(contractID)
}

func (c chain) RPCURL() string {
	return c.rpcUrl
}

func (c chain) User(ctx context.Context, constraints ...constraints.Constraint) (types.Wallet, error) {
	return c.user, nil
}

func (c chain) ID() types.ChainID {
	if c.id == "" {
		return types.ChainID(0)
	}
	id, _ := strconv.ParseUint(c.id, 10, 64)
	return types.ChainID(id)
}

type InteropSystem interface {
	System
	InteropSet() InteropSet
}

var _ InteropSystem = interopSystem{}

type interopSystem struct {
	*system
}

func (i interopSystem) InteropSet() InteropSet {
	return i.system // TODO
}

type InteropSet interface {
	L2(uint64) Chain
}

type wallet struct {
	privateKey types.Key
	address    types.Address
	rpcURL     string
}

func NewWallet(pk types.Key, addr types.Address, rpcURL string) wallet {
	return wallet{privateKey: pk, address: addr, rpcURL: rpcURL}
}

func (w wallet) PrivateKey() types.Key {
	return strings.TrimPrefix(w.privateKey, "0x")
}

func (w wallet) Address() types.Address {
	return w.address
}

func (w wallet) SendETH(to types.Address, amount types.Balance) types.WriteInvocation[any] {
	return &sendImpl{
		rpcURL: w.rpcURL,
		pk:     w.PrivateKey(),
		to:     to,
		amount: amount,
	}
}

type sendImpl struct {
	rpcURL string
	pk     types.Key
	to     types.Address
	amount types.Balance
}

func (i *sendImpl) Call(ctx context.Context) (any, error) {
	return nil, nil
}

func (i *sendImpl) Send(ctx context.Context) types.InvocationResult {
	tx, err := sendETH(ctx, i.rpcURL, i.pk, i.to, i.amount)
	return &sendResult{
		rpcURL: i.rpcURL,
		tx:     tx,
		err:    err,
	}
}

type sendResult struct {
	rpcURL string
	tx     *coreTypes.Transaction
	err    error
}

func (r *sendResult) Error() error {
	return r.err
}

func (r *sendResult) Wait() error {
	if r.err != nil {
		return r.err
	}
	if r.tx == nil {
		return fmt.Errorf("no transaction to wait for")
	}

	client, err := ethclient.Dial(r.rpcURL)
	if err != nil {
		return fmt.Errorf("failed to connect to ethereum client: %w", err)
	}

	receipt, err := bind.WaitMined(context.Background(), client, r.tx)
	if err != nil {
		return fmt.Errorf("failed waiting for transaction confirmation: %w", err)
	}

	if receipt.Status == 0 {
		return fmt.Errorf("transaction failed")
	}

	return nil
}

func sendETH(ctx context.Context, rpcURL string, privateKey string, to types.Address, amount types.Balance) (*coreTypes.Transaction, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ethereum client: %w", err)
	}

	pk, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	from := crypto.PubkeyToAddress(pk.PublicKey)
	nonce, err := client.PendingNonceAt(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %w", err)
	}

	gasLimit := uint64(210000) // 10x Standard ETH transfer gas limit
	toAddr := common.HexToAddress(string(to))
	tx := coreTypes.NewTransaction(nonce, toAddr, big.NewInt(int64(amount)), gasLimit, gasPrice, nil)

	chainID, err := client.NetworkID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain id: %w", err)
	}

	signedTx, err := coreTypes.SignTx(tx, coreTypes.NewEIP155Signer(chainID), pk)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %w", err)
	}

	return signedTx, nil
}
