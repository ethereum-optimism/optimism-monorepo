package withdrawals

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient/gethclient"

	"github.com/ethereum-optimism/optimism/op-node/bindings"
	bindingspreview "github.com/ethereum-optimism/optimism/op-node/bindings/preview"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
)

var MessagePassedTopic = crypto.Keccak256Hash([]byte("MessagePassed(uint256,address,address,uint256,uint256,bytes,bytes32)"))

type ProofClient interface {
	GetProof(context.Context, common.Address, []string, *big.Int) (*gethclient.AccountResult, error)
}

type ReceiptClient interface {
	TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
}

type HeaderClient interface {
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
}

// ProvenWithdrawalParameters is the set of parameters to pass to the ProveWithdrawalTransaction
// and FinalizeWithdrawalTransaction functions
type ProvenWithdrawalParameters struct {
	Nonce           *big.Int
	Sender          common.Address
	Target          common.Address
	Value           *big.Int
	GasLimit        *big.Int
	L2OutputIndex   *big.Int
	Data            []byte
	OutputRootProof bindings.TypesOutputRootProof
	WithdrawalProof [][]byte // List of trie nodes to prove L2 storage
}

// ProveWithdrawalParameters calls ProveWithdrawalParametersForBlock with the most recent L2 output after the given header.
func ProveWithdrawalParameters(ctx context.Context, proofCl ProofClient, l2ReceiptCl ReceiptClient, txHash common.Hash, l2Header *types.Header, l2OutputOracleContract *bindings.L2OutputOracleCaller) (ProvenWithdrawalParameters, error) {
	l2OutputIndex, err := l2OutputOracleContract.GetL2OutputIndexAfter(&bind.CallOpts{}, l2Header.Number)
	if err != nil {
		return ProvenWithdrawalParameters{}, fmt.Errorf("failed to get l2OutputIndex: %w", err)
	}
	return ProveWithdrawalParametersForBlock(ctx, proofCl, l2ReceiptCl, txHash, l2Header, l2OutputIndex)
}

// ProveWithdrawalParametersFaultProofs calls ProveWithdrawalParametersForBlock with the most recent L2 output after the latest game.
func ProveWithdrawalParametersFaultProofs(ctx context.Context, proofCl ProofClient, l2ReceiptCl ReceiptClient, l2HeaderCl HeaderClient, txHash common.Hash, disputeGameFactoryContract *bindings.DisputeGameFactoryCaller, optimismPortal2Contract *bindingspreview.OptimismPortal2Caller) (ProvenWithdrawalParameters, error) {
	latestGame, err := FindLatestGame(ctx, disputeGameFactoryContract, optimismPortal2Contract)
	if err != nil {
		return ProvenWithdrawalParameters{}, fmt.Errorf("failed to find latest game: %w", err)
	}

	l2BlockNumber := new(big.Int).SetBytes(latestGame.ExtraData[0:32])
	l2OutputIndex := latestGame.Index
	// Fetch the block header from the L2 node
	l2Header, err := l2HeaderCl.HeaderByNumber(ctx, l2BlockNumber)
	if err != nil {
		return ProvenWithdrawalParameters{}, fmt.Errorf("failed to get l2Block: %w", err)
	}
	return ProveWithdrawalParametersForBlock(ctx, proofCl, l2ReceiptCl, txHash, l2Header, l2OutputIndex)
}

// ProveWithdrawalParametersForBlock queries L1 & L2 to generate all withdrawal parameters and proof necessary to prove a withdrawal on L1.
// The l2Header provided is very important. It should be a block for which there is a submitted output in the L2 Output Oracle
// contract. If not, the withdrawal will fail as it the storage proof cannot be verified if there is no submitted state root.
func ProveWithdrawalParametersForBlock(ctx context.Context, proofCl ProofClient, l2ReceiptCl ReceiptClient, txHash common.Hash, l2Header *types.Header, l2OutputIndex *big.Int) (ProvenWithdrawalParameters, error) {
	// Transaction receipt
	receipt, err := l2ReceiptCl.TransactionReceipt(ctx, txHash)
	if err != nil {
		return ProvenWithdrawalParameters{}, err
	}
	// Parse the receipt
	ev, err := ParseMessagePassed(receipt)
	if err != nil {
		return ProvenWithdrawalParameters{}, err
	}
	return ProveWithdrawalParametersForEvent(ctx, proofCl, ev, l2Header, l2OutputIndex)
}

// ProveWithdrawalParametersForEvent queries L1 to generate all withdrawal parameters and proof necessary to prove a withdrawal on L1.
// The l2Header provided is very important. It should be a block for which there is a submitted output in the L2 Output Oracle
// contract. If not, the withdrawal will fail as it the storage proof cannot be verified if there is no submitted state root.
func ProveWithdrawalParametersForEvent(ctx context.Context, proofCl ProofClient, ev *bindings.L2ToL1MessagePasserMessagePassed, l2Header *types.Header, l2OutputIndex *big.Int) (ProvenWithdrawalParameters, error) {
	// Generate then verify the withdrawal proof
	withdrawalHash, err := WithdrawalHash(ev)
	if !bytes.Equal(withdrawalHash[:], ev.WithdrawalHash[:]) {
		return ProvenWithdrawalParameters{}, errors.New("Computed withdrawal hash incorrectly")
	}
	if err != nil {
		return ProvenWithdrawalParameters{}, err
	}
	slot := StorageSlotOfWithdrawalHash(withdrawalHash)

	p, err := proofCl.GetProof(ctx, predeploys.L2ToL1MessagePasserAddr, []string{slot.String()}, l2Header.Number)
	if err != nil {
		return ProvenWithdrawalParameters{}, err
	}
	if len(p.StorageProof) != 1 {
		return ProvenWithdrawalParameters{}, errors.New("invalid amount of storage proofs")
	}

	err = VerifyProof(l2Header.Root, p)
	if err != nil {
		return ProvenWithdrawalParameters{}, err
	}

	// Encode it as expected by the contract
	trieNodes := make([][]byte, len(p.StorageProof[0].Proof))
	for i, s := range p.StorageProof[0].Proof {
		trieNodes[i] = common.FromHex(s)
	}

	return ProvenWithdrawalParameters{
		Nonce:         ev.Nonce,
		Sender:        ev.Sender,
		Target:        ev.Target,
		Value:         ev.Value,
		GasLimit:      ev.GasLimit,
		L2OutputIndex: l2OutputIndex,
		Data:          ev.Data,
		OutputRootProof: bindings.TypesOutputRootProof{
			Version:                  [32]byte{}, // Empty for version 1
			StateRoot:                l2Header.Root,
			MessagePasserStorageRoot: p.StorageHash,
			LatestBlockhash:          l2Header.Hash(),
		},
		WithdrawalProof: trieNodes,
	}, nil
}

// FindLatestGame finds the latest game in the DisputeGameFactory contract.
func FindLatestGame(ctx context.Context, disputeGameFactoryContract *bindings.DisputeGameFactoryCaller, optimismPortal2Contract *bindingspreview.OptimismPortal2Caller) (*bindings.IDisputeGameFactoryGameSearchResult, error) {
	respectedGameType, err := optimismPortal2Contract.RespectedGameType(&bind.CallOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to get respected game type: %w", err)
	}

	gameCount, err := disputeGameFactoryContract.GameCount(&bind.CallOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to get game count: %w", err)
	}
	if gameCount.Cmp(common.Big0) == 0 {
		return nil, errors.New("no games")
	}

	searchStart := new(big.Int).Sub(gameCount, common.Big1)
	latestGames, err := disputeGameFactoryContract.FindLatestGames(&bind.CallOpts{}, respectedGameType, searchStart, common.Big1)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest games: %w", err)
	}
	if len(latestGames) == 0 {
		return nil, errors.New("no latest games")
	}

	latestGame := latestGames[0]
	return &latestGame, nil
}

// Standard ABI types copied from golang ABI tests
var (
	Uint256Type, _ = abi.NewType("uint256", "", nil)
	BytesType, _   = abi.NewType("bytes", "", nil)
	AddressType, _ = abi.NewType("address", "", nil)
)

// WithdrawalHash computes the hash of the withdrawal that was stored in the L2toL1MessagePasser
// contract state.
// TODO:
//   - I don't like having to use the ABI Generated struct
//   - There should be a better way to run the ABI encoding
//   - These needs to be fuzzed against the solidity
func WithdrawalHash(ev *bindings.L2ToL1MessagePasserMessagePassed) (common.Hash, error) {
	//  abi.encode(nonce, msg.sender, _target, msg.value, _gasLimit, _data)
	args := abi.Arguments{
		{Name: "nonce", Type: Uint256Type},
		{Name: "sender", Type: AddressType},
		{Name: "target", Type: AddressType},
		{Name: "value", Type: Uint256Type},
		{Name: "gasLimit", Type: Uint256Type},
		{Name: "data", Type: BytesType},
	}
	enc, err := args.Pack(ev.Nonce, ev.Sender, ev.Target, ev.Value, ev.GasLimit, ev.Data)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to pack for withdrawal hash: %w", err)
	}
	return crypto.Keccak256Hash(enc), nil
}

// ParseMessagePassed parses MessagePassed events from
// a transaction receipt. It does not support multiple withdrawals
// per receipt.
func ParseMessagePassed(receipt *types.Receipt) (*bindings.L2ToL1MessagePasserMessagePassed, error) {
	events, err := ParseMessagesPassed(receipt)
	if err != nil {
		return nil, err
	}
	return events[0], nil
}

// ParseMessagesPassed parses MessagePassed events from
// a transaction receipt. It supports multiple withdrawals
// per receipt.
func ParseMessagesPassed(receipt *types.Receipt) ([]*bindings.L2ToL1MessagePasserMessagePassed, error) {
	contract, err := bindings.NewL2ToL1MessagePasser(common.Address{}, nil)
	if err != nil {
		return nil, err
	}

	var events []*bindings.L2ToL1MessagePasserMessagePassed
	for _, log := range receipt.Logs {
		if len(log.Topics) == 0 || log.Topics[0] != MessagePassedTopic {
			continue
		}

		ev, err := contract.ParseMessagePassed(*log)
		if err != nil {
			return nil, fmt.Errorf("failed to parse log: %w", err)
		}
		events = append(events, ev)
	}
	if len(events) == 0 {
		return nil, errors.New("unable to find MessagePassed event")
	}
	return events, nil
}

// StorageSlotOfWithdrawalHash determines the storage slot of the L2ToL1MessagePasser contract to look at
// given a WithdrawalHash
func StorageSlotOfWithdrawalHash(hash common.Hash) common.Hash {
	// The withdrawals mapping is the 0th storage slot in the L2ToL1MessagePasser contract.
	// To determine the storage slot, use keccak256(withdrawalHash ++ p)
	// Where p is the 32 byte value of the storage slot and ++ is concatenation
	buf := make([]byte, 64)
	copy(buf, hash[:])
	return crypto.Keccak256Hash(buf)
}
