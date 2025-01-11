package wallet

import (
	// metrics "github.com/ethereum-optimism/optimism/op-node/metrics"
	// "github.com/ethereum-optimism/optimism/op-service/ethclient"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	// rpc "github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/net/context"
)

type Network struct {
	ChainID hexutil.Big
	Name    string
	addr    string
	RPC     *ethclient.Client
	log     log.Logger
}

func NewNetwork(ctx context.Context, log log.Logger, addr, name string) (*Network, error) {
	// rpc, err := dial.DialRPCClientWithTimeout(ctx, time.Second*10, log, addr)
	client, err := ethclient.Dial(addr)
	if err != nil {
		return nil, err
	}
	return &Network{
		RPC:  client,
		addr: addr,
		Name: name,
		log:  log,
	}, nil

}

func (n *Network) DumpInfo(ctx context.Context) error {
	block, err := n.RPC.BlockNumber(ctx)
	if err != nil {
		n.log.Error("error retreving block",
			"network", n.Name,
			"err", err)
	}
	chainID, err := n.RPC.ChainID(ctx)
	if err != nil {
		n.log.Error("error retreving block",
			"network", n.Name,
			"err", err)
	}
	log.Info("Network Dump", "network", n.Name)
	log.Info("Current block", "block", block)
	log.Info("ChainID", "chain-id", chainID.String())
	return nil
}

type Wallet struct {
	privateKey string
	publicKey  string
}

type WalletInterface interface {
	GetBalance(network Network) (int error)
	Send(network Network, to string) error
}

func (w *Wallet) GetBalance(ctx context.Context, network Network) string {
	return w.publicKey
	// if err := network.RPC.CallContext(ctx, &idResult, "eth_chainId"); err != nil {
	// 	return nil, fmt.Errorf("failed to retrieve chain ID: %w", err)
	// }
}

func (w *Wallet) Send() string {
	return w.privateKey
}
