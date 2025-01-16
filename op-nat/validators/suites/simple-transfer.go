package suites

import (
	nat "github.com/ethereum-optimism/optimism/op-nat"
	"github.com/ethereum-optimism/optimism/op-nat/validators/tests"
)

var SimpleTransfer = nat.Suite{
	ID: "simple-transfer",
	Tests: []nat.Test{
		tests.SimpleTransfer,
	},
	TestsParams: map[string]interface{}{},
}
