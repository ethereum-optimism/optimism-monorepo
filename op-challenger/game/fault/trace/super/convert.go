package super

import "github.com/ethereum-optimism/optimism/op-service/eth"

func responseToSuper(prevRoot eth.SuperRootResponse) *eth.SuperV1 {
	prevChainOutputs := make([]eth.ChainIDAndOutput, 0, len(prevRoot.Chains))
	for _, chain := range prevRoot.Chains {
		prevChainOutputs = append(prevChainOutputs, eth.ChainIDAndOutput{ChainID: chain.ChainID, Output: chain.Canonical})
	}
	superV1 := eth.NewSuperV1(prevRoot.Timestamp, prevChainOutputs...)
	return superV1
}
