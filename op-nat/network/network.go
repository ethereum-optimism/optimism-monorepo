package network

import (
	"errors"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/net/context"
)

type Network struct {
	ChainID *big.Int
	Name    string
	// Addr is the Address the client is dialing
	Addr string
	RPC  *ethclient.Client
	log  log.Logger
	// BlockTime time.Duration
}

func NewNetwork(ctx context.Context, log log.Logger, addr, name string, chainid *big.Int) (*Network, error) {
	client, err := ethclient.Dial(addr)
	if err != nil {
		return nil, err
	}
	return &Network{
		RPC:     client,
		Addr:    addr,
		Name:    name,
		log:     log,
		ChainID: chainid,
	}, nil

}

// DumpInfo will print a current networks information
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

func (n *Network) PollForConfirmation(ctx context.Context, log log.Logger, confs uint64, txhash common.Hash) error {
	for i := 0; i < 10; i++ {
		receipt, err := n.RPC.TransactionReceipt(context.Background(), txhash)
		if err != nil {
			log.Warn("error getting tx reciept for tx",
				"tx", txhash.String(),
				"error", err,
			)
		}

		// Check if the transaction has been mined
		if receipt != nil {
			// Get the current block number
			blockNumber, err := n.RPC.BlockNumber(context.Background())
			if err != nil {
				log.Error("error getting block number",
					"tx", txhash.String(),
					"error", err,
				)
			}

			// Calculate the number of confirmations
			confirmations := blockNumber - receipt.BlockNumber.Uint64()
			log.Debug("current confirmations",
				"confirmations", confirmations,
				"required_confirmations", confs,
				"tx", txhash,
			)

			// Check if the required number of confirmations has been reached
			if confirmations >= confs {
				log.Debug("transaction confirmed",
					"tx", txhash,
					"confs", confirmations,
				)
				return nil
			}
		} else {
			log.Debug("transaction not yet mined",
				"tx", txhash,
			)
		}
		// Wait for a certain duration before polling again
		time.Sleep(10 * time.Second)
	}
	return errors.New("tx was not confirmed")
}
