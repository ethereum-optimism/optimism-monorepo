package withdrawals

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/op-bindings/bindings"
	"github.com/ethereum-optimism/optimism/op-bindings/predeploys"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/gethclient"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
)

var WithdrawalInitiatedTopic = common.HexToHash("0x87bf7b546c8de873abb0db5b579ec131f8d0cf5b14f39933551cf9ced23a6136")
var WithdrawalInitiatedExtension1Topic = common.HexToHash("0x2ef6ceb1668fdd882b1f89ddd53a666b0c1113d14cf90c0fbf97c7b1ad880fbb")

// WaitForFinalizationPeriod waits until there is OutputProof for an L2 block number larger than the supplied l2BlockNumber
// and that the output is finalized.
// This functions polls and can block for a very long time if used on mainnet.
// This returns the block number to use for the proof generation.
func WaitForFinalizationPeriod(ctx context.Context, client *ethclient.Client, portalAddr common.Address, l2BlockNumber *big.Int) (uint64, error) {
	l2BlockNumber = new(big.Int).Set(l2BlockNumber) // Don't clobber caller owned l2BlockNumber
	opts := &bind.CallOpts{Context: ctx}

	portal, err := bindings.NewOptimismPortalCaller(portalAddr, client)
	if err != nil {
		return 0, err
	}
	l2OOAddress, err := portal.L2ORACLE(opts)
	if err != nil {
		return 0, err
	}
	l2OO, err := bindings.NewL2OutputOracleCaller(l2OOAddress, client)
	if err != nil {
		return 0, err
	}
	submissionInterval, err := l2OO.SUBMISSIONINTERVAL(opts)
	if err != nil {
		return 0, err
	}
	// Convert blockNumber to submission interval boundary
	rem := new(big.Int)
	l2BlockNumber, rem = l2BlockNumber.DivMod(l2BlockNumber, submissionInterval, rem)
	if rem.Cmp(common.Big0) != 0 {
		l2BlockNumber = l2BlockNumber.Add(l2BlockNumber, common.Big1)
	}
	l2BlockNumber = l2BlockNumber.Mul(l2BlockNumber, submissionInterval)

	finalizationPeriod, err := portal.FINALIZATIONPERIODSECONDS(opts)
	if err != nil {
		return 0, err
	}

	latest, err := l2OO.LatestBlockNumber(opts)
	if err != nil {
		return 0, err
	}

	// Now poll for the output to be submitted on chain
	var ticker *time.Ticker
	diff := new(big.Int).Sub(l2BlockNumber, latest)
	if diff.Cmp(big.NewInt(10)) > 0 {
		ticker = time.NewTicker(time.Minute)
	} else {
		ticker = time.NewTicker(time.Second)
	}

loop:
	for {
		select {
		case <-ticker.C:
			latest, err = l2OO.LatestBlockNumber(opts)
			if err != nil {
				return 0, err
			}
			// Already passed the submitted block (likely just equals rather than >= here).
			if latest.Cmp(l2BlockNumber) >= 0 {
				break loop
			}
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}

	// Now wait for it to be finalized
	output, err := l2OO.GetL2Output(opts, l2BlockNumber)
	if err != nil {
		return 0, err
	}
	if output.OutputRoot == [32]byte{} {
		return 0, errors.New("empty output root. likely no proposal at timestamp")
	}
	targetTimestamp := new(big.Int).Add(output.Timestamp, finalizationPeriod)
	targetTime := time.Unix(targetTimestamp.Int64(), 0)
	// Assume clock is relatively correct
	time.Sleep(time.Until(targetTime))
	// Poll for L1 Block to have a time greater than the target time
	ticker = time.NewTicker(time.Second)
	for {
		select {
		case <-ticker.C:
			header, err := client.HeaderByNumber(ctx, nil)
			if err != nil {
				return 0, err
			}
			if header.Time > targetTimestamp.Uint64() {
				return l2BlockNumber.Uint64(), nil
			}
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}

}

type ProofClient interface {
	TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
	GetProof(context.Context, common.Address, []string, *big.Int) (*gethclient.AccountResult, error)
}

type ec = *ethclient.Client
type gc = *gethclient.Client

type Client struct {
	ec
	gc
}

// Ensure that ProofClient and Client interfaces are valid
var _ ProofClient = &Client{}

// NewClient wraps a RPC client with both ethclient and gethclient methods.
// Implements ProofClient
func NewClient(client *rpc.Client) *Client {
	return &Client{
		ethclient.NewClient(client),
		gethclient.New(client),
	}

}

// FinalizedWithdrawalParameters is the set of parameters to pass to the FinalizedWithdrawal function
type FinalizedWithdrawalParameters struct {
	Nonce           *big.Int
	Sender          common.Address
	Target          common.Address
	Value           *big.Int
	GasLimit        *big.Int
	BlockNumber     *big.Int
	Data            []byte
	OutputRootProof bindings.TypesOutputRootProof
	WithdrawalProof []byte // RLP Encoded list of trie nodes to prove L2 storage
}

// FinalizeWithdrawalParameters queries L2 to generate all withdrawal parameters and proof necessary to finalize an withdrawal on L1.
// The header provided is very important. It should be a block (timestamp) for which there is a submitted output in the L2 Output Oracle
// contract. If not, the withdrawal will fail as it the storage proof cannot be verified if there is no submitted state root.
func FinalizeWithdrawalParameters(ctx context.Context, l2client ProofClient, txHash common.Hash, header *types.Header) (FinalizedWithdrawalParameters, error) {
	// Transaction receipt
	receipt, err := l2client.TransactionReceipt(ctx, txHash)
	if err != nil {
		return FinalizedWithdrawalParameters{}, err
	}
	// Parse the receipt
	ev, err := ParseWithdrawalInitiated(receipt)
	if err != nil {
		return FinalizedWithdrawalParameters{}, err
	}
	ev1, err := ParseWithdrawalInitiatedExtension1(receipt)
	if err != nil {
		return FinalizedWithdrawalParameters{}, err
	}
	// Generate then verify the withdrawal proof
	withdrawalHash, err := WithdrawalHash(ev)
	if !bytes.Equal(withdrawalHash[:], ev1.Hash[:]) {
		return FinalizedWithdrawalParameters{}, errors.New("Computed withdrawal hash incorrectly")
	}
	if err != nil {
		return FinalizedWithdrawalParameters{}, err
	}
	slot := StorageSlotOfWithdrawalHash(withdrawalHash)
	p, err := l2client.GetProof(ctx, predeploys.L2ToL1MessagePasserAddr, []string{slot.String()}, header.Number)
	if err != nil {
		return FinalizedWithdrawalParameters{}, err
	}
	// TODO: Could skip this step, but it's nice to double check it
	err = VerifyProof(header.Root, p)
	if err != nil {
		return FinalizedWithdrawalParameters{}, err
	}
	if len(p.StorageProof) != 1 {
		return FinalizedWithdrawalParameters{}, errors.New("invalid amount of storage proofs")
	}

	// Encode it as expected by the contract
	trieNodes := make([][]byte, len(p.StorageProof[0].Proof))
	for i, s := range p.StorageProof[0].Proof {
		trieNodes[i] = common.FromHex(s)
	}

	withdrawalProof, err := rlp.EncodeToBytes(trieNodes)
	if err != nil {
		return FinalizedWithdrawalParameters{}, err
	}

	return FinalizedWithdrawalParameters{
		Nonce:       ev.Nonce,
		Sender:      ev.Sender,
		Target:      ev.Target,
		Value:       ev.Value,
		GasLimit:    ev.GasLimit,
		BlockNumber: new(big.Int).Set(header.Number),
		Data:        ev.Data,
		OutputRootProof: bindings.TypesOutputRootProof{
			Version:                  [32]byte{}, // Empty for version 1
			StateRoot:                header.Root,
			MessagePasserStorageRoot: p.StorageHash,
			LatestBlockhash:          header.Hash(),
		},
		WithdrawalProof: withdrawalProof,
	}, nil
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
func WithdrawalHash(ev *bindings.L2ToL1MessagePasserWithdrawalInitiated) (common.Hash, error) {
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

// ParseWithdrawalInitiated parses
func ParseWithdrawalInitiated(receipt *types.Receipt) (*bindings.L2ToL1MessagePasserWithdrawalInitiated, error) {
	contract, err := bindings.NewL2ToL1MessagePasser(common.Address{}, nil)
	if err != nil {
		return nil, err
	}

	for _, log := range receipt.Logs {
		if len(log.Topics) == 0 || log.Topics[0] != WithdrawalInitiatedTopic {
			continue
		}

		ev, err := contract.ParseWithdrawalInitiated(*log)
		if err != nil {
			return nil, fmt.Errorf("failed to parse log: %w", err)
		}
		return ev, nil
	}
	return nil, errors.New("Unable to find WithdrawalInitiated event")
}

// ParseWithdrawalInitiatedExtension1 parses
func ParseWithdrawalInitiatedExtension1(receipt *types.Receipt) (*bindings.L2ToL1MessagePasserWithdrawalInitiatedExtension1, error) {
	contract, err := bindings.NewL2ToL1MessagePasser(common.Address{}, nil)
	if err != nil {
		return nil, err
	}

	for _, log := range receipt.Logs {
		if len(log.Topics) == 0 || log.Topics[0] != WithdrawalInitiatedExtension1Topic {
			continue
		}

		ev, err := contract.ParseWithdrawalInitiatedExtension1(*log)
		if err != nil {
			return nil, fmt.Errorf("failed to parse log: %w", err)
		}
		return ev, nil
	}
	return nil, errors.New("Unable to find WithdrawalInitiatedExtension1 event")
}

// StorageSlotOfWithdrawalHash determines the storage slot of the Withdrawer contract to look at
// given a WithdrawalHash
func StorageSlotOfWithdrawalHash(hash common.Hash) common.Hash {
	// The withdrawals mapping is the second (0 indexed) storage element in the Withdrawer contract.
	// To determine the storage slot, use keccak256(withdrawalHash ++ p)
	// Where p is the 32 byte value of the storage slot and ++ is concatenation
	buf := make([]byte, 64)
	copy(buf, hash[:])
	return crypto.Keccak256Hash(buf)
}
