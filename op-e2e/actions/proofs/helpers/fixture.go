package helpers

import (
	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

type TestFixture struct {
	Name           string        `toml:"name"`
	ExpectedStatus uint8         `toml:"expected-status"`
	Inputs         FixtureInputs `toml:"inputs"`
}

type FaultProofProgramL2Source struct {
	Node        *helpers.L2Verifier
	Engine      *helpers.L2Engine
	ChainConfig *params.ChainConfig
}

type FixtureInputs struct {
	L2BlockNumber  uint64      `toml:"l2-block-number"`
	L2Claim        common.Hash `toml:"l2-claim"`
	L2Head         common.Hash `toml:"l2-head"`
	L2OutputRoot   common.Hash `toml:"l2-output-root"`
	L2ChainID      eth.ChainID `toml:"l2-chain-id"`
	L1Head         common.Hash `toml:"l1-head"`
	AgreedPrestate []byte      `toml:"agreed-prestate"`
	InteropEnabled bool        `toml:"use-interop"`

	L2Sources []*FaultProofProgramL2Source
}
