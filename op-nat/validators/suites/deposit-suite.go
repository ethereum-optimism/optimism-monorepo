package suites

import (
	nat "github.com/ethereum-optimism/optimism/op-nat"
	"github.com/ethereum-optimism/optimism/op-nat/validators/tests"
)

var DepositSuite = nat.Suite{
	ID: "simple-deposit",
	Tests: []nat.Test{
		tests.SimpleDeposit,
	},
}
