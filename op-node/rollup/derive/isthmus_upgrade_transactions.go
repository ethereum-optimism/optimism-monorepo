package derive

import (
	"math/big"

	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

var (
	blockHashDeployerSource     = UpgradeDepositSource{Intent: "Isthmus: EIP-2935 Contract Deployment"}
	blockHashDeploymentBytecode = common.FromHex("0x60538060095f395ff33373fffffffffffffffffffffffffffffffffffffffe14604657602036036042575f35600143038111604257611fff81430311604257611fff9006545f5260205ff35b5f5ffd5b5f35611fff60014303065500")
)

func IsthmusNetworkUpgradeTransactions() ([]hexutil.Bytes, error) {
	deployHistoricalBlockHashesContract, err := types.NewTx(&types.DepositTx{
		SourceHash:          blockHashDeployerSource.SourceHash(),
		From:                predeploys.EIP2935ContractDeployer,
		To:                  nil,
		Mint:                big.NewInt(0),
		Value:               big.NewInt(0),
		Gas:                 250_000,
		IsSystemTransaction: false,
		Data:                blockHashDeploymentBytecode,
	}).MarshalBinary()

	if err != nil {
		return nil, err
	}

	return []hexutil.Bytes{deployHistoricalBlockHashesContract}, nil
}
