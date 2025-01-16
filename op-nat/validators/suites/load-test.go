package suites

import (
	nat "github.com/ethereum-optimism/optimism/op-nat"
	"github.com/ethereum-optimism/optimism/op-nat/validators/tests"
)

var LoadTest = nat.Suite{
	ID: "load-test",
	Tests: []nat.Test{
		tests.TxFuzz,
	},
	TestsParams: map[string]interface{}{
		"tx-fuzz": tests.TxFuzzParams{
			NSlotsToRunFor:     1,
			TxPerAccount:       2,
			GenerateAccessList: false,
		},
	},
}
