package eth

import "github.com/ethereum/go-ethereum/common/hexutil"

type ExecutionWitness struct {
	Keys  map[string]hexutil.Bytes `json:"keys"`
	Codes map[string]hexutil.Bytes `json:"codes"`
	State map[string]hexutil.Bytes `json:"state"`
}
