package gates

import (
	nat "github.com/ethereum-optimism/optimism/op-nat"
	"github.com/ethereum-optimism/optimism/op-nat/validators/suites"
)

var Alphanet = nat.Gate{
	ID: "alphanet",
	Validators: []nat.Validator{
		suites.Network,
		suites.DepositSuite,
		suites.SimpleTransfer,
		suites.LoadTest,
	},
}
