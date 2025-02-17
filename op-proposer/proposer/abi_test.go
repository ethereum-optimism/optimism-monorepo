package proposer

import (
	"crypto/ecdsa"
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum-optimism/optimism/op-proposer/bindings"
	"github.com/ethereum-optimism/optimism/op-proposer/proposer/source"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

func simulatedBackend() (privateKey *ecdsa.PrivateKey, address common.Address, opts *bind.TransactOpts, backend *backends.SimulatedBackend, err error) {
	privateKey, err = crypto.GenerateKey()
	if err != nil {
		return nil, common.Address{}, nil, nil, err
	}
	from := crypto.PubkeyToAddress(privateKey.PublicKey)
	opts, err = bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(1337))
	if err != nil {
		return nil, common.Address{}, nil, nil, err
	}
	backend = backends.NewSimulatedBackend(types.GenesisAlloc{from: {Balance: big.NewInt(params.Ether)}}, 50_000_000) // nolint:staticcheck

	return privateKey, from, opts, backend, nil
}

// setupL2OutputOracle deploys the L2 Output Oracle contract to a simulated backend
func setupL2OutputOracle() (common.Address, *bind.TransactOpts, *backends.SimulatedBackend, *bindings.L2OutputOracle, error) {
	_, from, opts, backend, err := simulatedBackend()
	if err != nil {
		return common.Address{}, nil, nil, nil, err
	}
	_, _, contract, err := bindings.DeployL2OutputOracle(opts, backend)
	if err != nil {
		return common.Address{}, nil, nil, nil, err
	}
	return from, opts, backend, contract, nil
}

// TestManualABIPacking ensure that the manual ABI packing is the same as going through the bound contract.
// We don't use the contract to transact because it does not fit our transaction management scheme, but
// we want to make sure that we don't incorrectly create the transaction data.
func TestManualABIPacking(t *testing.T) {
	// L2OO
	_, opts, _, l2oo, err := setupL2OutputOracle()
	require.NoError(t, err)
	rng := rand.New(rand.NewSource(1234))

	l2ooAbi, err := bindings.L2OutputOracleMetaData.GetAbi()
	require.NoError(t, err)

	proposal := source.Proposal{
		Version:   eth.Bytes32{},
		Root:      testutils.RandomHash(rng),
		CurrentL1: testutils.RandomBlockID(rng),
		Legacy: source.LegacyProposalData{
			BlockRef:    testutils.RandomL2BlockRef(rng),
			HeadL1:      testutils.RandomBlockRef(rng),
			SafeL2:      testutils.RandomL2BlockRef(rng),
			FinalizedL2: testutils.RandomL2BlockRef(rng),
		},
	}

	txData, err := proposeL2OutputTxData(l2ooAbi, proposal)
	require.NoError(t, err)

	// set a gas limit to disable gas estimation. The invariants that the L2OO tries to uphold
	// are not maintained in this test.
	opts.GasLimit = 100_000
	tx, err := l2oo.ProposeL2Output(
		opts,
		proposal.Root,
		new(big.Int).SetUint64(proposal.SequenceNum),
		proposal.CurrentL1.Hash,
		new(big.Int).SetUint64(proposal.CurrentL1.Number))
	require.NoError(t, err)

	require.Equal(t, txData, tx.Data())
}
