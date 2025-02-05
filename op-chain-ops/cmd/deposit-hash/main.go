package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
)

func main() {
	var rpcURL, txHash string
	flag.StringVar(&rpcURL, "rpc", "", "L1 RPC URL")
	flag.StringVar(&txHash, "tx", "", "Deposit transaction hash on L1")
	flag.Parse()

	depositLogTopic := common.HexToHash("0xb3813568d9991fc951961fcb4c784893574240a28925604d09fc577c55bb7c32")

	ethClient, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Crit("Error creating RPC", "err", err)
	}

	l1Receipt, err := ethClient.TransactionReceipt(context.TODO(), common.HexToHash(txHash))
	if err != nil {
		log.Crit("Error fetching transaction", "err", err)
	}

	for _, ethLog := range l1Receipt.Logs {
		if ethLog.Topics[0].String() == depositLogTopic.String() {

			reconstructedDep, err := derive.UnmarshalDepositLogEvent(ethLog)
			if err != nil {
				log.Crit("Failed to parse deposit event ", "err", err)
			}
			tx := types.NewTx(reconstructedDep)
			fmt.Println("L2 Tx Hash", tx.Hash().String())
		}
	}
}
