package constraints

import "github.com/ethereum-optimism/optimism/devnet-sdk/types"

type Constraint interface {
	Met() bool
}

type trivialConstraint struct{}

func (c *trivialConstraint) Met() bool {
	return true
}

func WithFunds(amount types.Balance) Constraint {
	return &trivialConstraint{}
}
