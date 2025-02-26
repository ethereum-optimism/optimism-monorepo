package contracts

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sync/atomic"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-challenger/game/keccak/merkle"
	keccakTypes "github.com/ethereum-optimism/optimism/op-challenger/game/keccak/types"
	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/snapshots"
	"github.com/ethereum/go-ethereum/common"
)

const (
	methodInitLPP                    = "initLPP"
	methodAddLeavesLPP               = "addLeavesLPP"
	methodSqueezeLPP                 = "squeezeLPP"
	methodLoadKeccak256PreimagePart  = "loadKeccak256PreimagePart"
	methodLoadSha256PreimagePart     = "loadSha256PreimagePart"
	methodLoadBlobPreimagePart       = "loadBlobPreimagePart"
	methodLoadPrecompilePreimagePart = "loadPrecompilePreimagePart"
	methodProposalCount              = "proposalCount"
	methodProposals                  = "proposals"
	methodProposalMetadata           = "proposalMetadata"
	methodProposalBlocksLen          = "proposalBlocksLen"
	methodProposalBlocks             = "proposalBlocks"
	methodPreimagePartOk             = "preimagePartOk"
	methodMinProposalSize            = "minProposalSize"
	methodChallengeFirstLPP          = "challengeFirstLPP"
	methodChallengeLPP               = "challengeLPP"
	methodChallengePeriod            = "challengePeriod"
	methodGetTreeRootLPP             = "getTreeRootLPP"
	methodMinBondSizeLPP             = "MIN_BOND_SIZE"
)

var (
	ErrInvalidAddLeavesCall = errors.New("tx is not a valid addLeaves call")
	ErrInvalidPreimageKey   = errors.New("invalid preimage key")
	ErrUnsupportedKeyType   = errors.New("unsupported preimage key type")
)

// preimageOracleLeaf matches the contract representation of a large preimage leaf
type preimageOracleLeaf struct {
	Input           []byte
	Index           *big.Int
	StateCommitment [32]byte
}

// libKeccakStateMatrix matches the contract representation of a keccak state matrix
type libKeccakStateMatrix struct {
	State [25]uint64
}

// PreimageOracleContractLatest is a binding that works with contracts implementing the IPreimageOracle interface
type PreimageOracleContractLatest struct {
	addr        common.Address
	multiCaller *batching.MultiCaller
	contract    *batching.BoundContract

	// challengePeriod caches the challenge period from the contract once it has been loaded.
	// 0 indicates the period has not been loaded yet.
	challengePeriod atomic.Uint64
	// minBondSizeLPP caches the minimum bond size for large preimages from the contract once it has been loaded.
	// 0 indicates the value has not been loaded yet.
	minBondSizeLPP atomic.Uint64
}

// toPreimageOracleLeaf converts a Leaf to the contract format.
func toPreimageOracleLeaf(l keccakTypes.Leaf) preimageOracleLeaf {
	return preimageOracleLeaf{
		Input:           l.Input[:],
		Index:           new(big.Int).SetUint64(l.Index),
		StateCommitment: l.StateCommitment,
	}
}

func NewPreimageOracleContract(ctx context.Context, addr common.Address, caller *batching.MultiCaller) (PreimageOracleContract, error) {
	oracleAbi := snapshots.LoadPreimageOracleABI()

	var builder VersionedBuilder[PreimageOracleContract]
	builder.AddVersion(1, 0, func() (PreimageOracleContract, error) {
		legacyAbi := mustParseAbi(preimageOracleAbi100)
		return &PreimageOracleContract100{
			PreimageOracleContractLatest{
				addr:        addr,
				multiCaller: caller,
				contract:    batching.NewBoundContract(legacyAbi, addr),
			},
		}, nil
	})

	return builder.Build(ctx, caller, oracleAbi, addr, func() (PreimageOracleContract, error) {
		return &PreimageOracleContractLatest{
			addr:        addr,
			multiCaller: caller,
			contract:    batching.NewBoundContract(oracleAbi, addr),
		}, nil
	})

}

func (c *PreimageOracleContractLatest) Addr() common.Address {
	return c.addr
}

func (c *PreimageOracleContractLatest) AddGlobalDataTx(data *types.PreimageOracleData) (txmgr.TxCandidate, error) {
	if len(data.OracleKey) == 0 {
		return txmgr.TxCandidate{}, ErrInvalidPreimageKey
	}
	keyType := preimage.KeyType(data.OracleKey[0])
	switch keyType {
	case preimage.Keccak256KeyType:
		call := c.contract.Call(methodLoadKeccak256PreimagePart, new(big.Int).SetUint64(uint64(data.OracleOffset)), data.GetPreimageWithoutSize())
		return call.ToTxCandidate()
	case preimage.Sha256KeyType:
		call := c.contract.Call(methodLoadSha256PreimagePart, new(big.Int).SetUint64(uint64(data.OracleOffset)), data.GetPreimageWithoutSize())
		return call.ToTxCandidate()
	case preimage.BlobKeyType:
		call := c.contract.Call(methodLoadBlobPreimagePart,
			new(big.Int).SetUint64(data.BlobFieldIndex),
			new(big.Int).SetBytes(data.GetPreimageWithoutSize()),
			data.BlobCommitment,
			data.BlobProof,
			new(big.Int).SetUint64(uint64(data.OracleOffset)))
		return call.ToTxCandidate()
	case preimage.PrecompileKeyType:
		call := c.contract.Call(methodLoadPrecompilePreimagePart,
			new(big.Int).SetUint64(uint64(data.OracleOffset)),
			data.GetPrecompileAddress(),
			data.GetPrecompileRequiredGas(),
			data.GetPrecompileInput())
		return call.ToTxCandidate()
	default:
		return txmgr.TxCandidate{}, fmt.Errorf("%w: %v", ErrUnsupportedKeyType, keyType)
	}
}

func (c *PreimageOracleContractLatest) InitLargePreimage(uuid *big.Int, partOffset uint32, claimedSize uint32) (txmgr.TxCandidate, error) {
	bond, err := c.GetMinBondLPP(context.Background())
	if err != nil {
		return txmgr.TxCandidate{}, fmt.Errorf("failed to get min bond for large preimage proposal: %w", err)
	}
	call := c.contract.Call(methodInitLPP, uuid, partOffset, claimedSize)
	candidate, err := call.ToTxCandidate()
	if err != nil {
		return txmgr.TxCandidate{}, fmt.Errorf("failed to create initLPP tx candidate: %w", err)
	}
	candidate.Value = bond
	return candidate, nil
}

func (c *PreimageOracleContractLatest) AddLeaves(uuid *big.Int, startingBlockIndex *big.Int, input []byte, commitments []common.Hash, finalize bool) (txmgr.TxCandidate, error) {
	call := c.contract.Call(methodAddLeavesLPP, uuid, startingBlockIndex, input, commitments, finalize)
	return call.ToTxCandidate()
}

// MinLargePreimageSize returns the minimum size of a large preimage.
func (c *PreimageOracleContractLatest) MinLargePreimageSize(ctx context.Context) (uint64, error) {
	result, err := c.multiCaller.SingleCall(ctx, rpcblock.Latest, c.contract.Call(methodMinProposalSize))
	if err != nil {
		return 0, fmt.Errorf("failed to fetch min lpp size bytes: %w", err)
	}
	return result.GetBigInt(0).Uint64(), nil
}

// ChallengePeriod returns the challenge period for large preimages.
func (c *PreimageOracleContractLatest) ChallengePeriod(ctx context.Context) (uint64, error) {
	if period := c.challengePeriod.Load(); period != 0 {
		return period, nil
	}
	result, err := c.multiCaller.SingleCall(ctx, rpcblock.Latest, c.contract.Call(methodChallengePeriod))
	if err != nil {
		return 0, fmt.Errorf("failed to fetch challenge period: %w", err)
	}
	period := result.GetBigInt(0).Uint64()
	c.challengePeriod.Store(period)
	return period, nil
}

func (c *PreimageOracleContractLatest) CallSqueeze(
	ctx context.Context,
	claimant common.Address,
	uuid *big.Int,
	prestateMatrix keccakTypes.StateSnapshot,
	preState keccakTypes.Leaf,
	preStateProof merkle.Proof,
	postState keccakTypes.Leaf,
	postStateProof merkle.Proof,
) error {
	call := c.contract.Call(methodSqueezeLPP, claimant, uuid, abiEncodeSnapshot(prestateMatrix), toPreimageOracleLeaf(preState), preStateProof, toPreimageOracleLeaf(postState), postStateProof)
	_, err := c.multiCaller.SingleCall(ctx, rpcblock.Latest, call)
	if err != nil {
		return fmt.Errorf("failed to call squeeze: %w", err)
	}
	return nil
}

func (c *PreimageOracleContractLatest) Squeeze(
	claimant common.Address,
	uuid *big.Int,
	prestateMatrix keccakTypes.StateSnapshot,
	preState keccakTypes.Leaf,
	preStateProof merkle.Proof,
	postState keccakTypes.Leaf,
	postStateProof merkle.Proof,
) (txmgr.TxCandidate, error) {
	call := c.contract.Call(
		methodSqueezeLPP,
		claimant,
		uuid,
		abiEncodeSnapshot(prestateMatrix),
		toPreimageOracleLeaf(preState),
		preStateProof,
		toPreimageOracleLeaf(postState),
		postStateProof,
	)
	return call.ToTxCandidate()
}

func abiEncodeSnapshot(packedState keccakTypes.StateSnapshot) libKeccakStateMatrix {
	return libKeccakStateMatrix{State: packedState}
}

func (c *PreimageOracleContractLatest) GetActivePreimages(ctx context.Context, blockHash common.Hash) ([]keccakTypes.LargePreimageMetaData, error) {
	block := rpcblock.ByHash(blockHash)
	results, err := batching.ReadArray(ctx, c.multiCaller, block, c.contract.Call(methodProposalCount), func(i *big.Int) *batching.ContractCall {
		return c.contract.Call(methodProposals, i)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load claims: %w", err)
	}

	var idents []keccakTypes.LargePreimageIdent
	for _, result := range results {
		idents = append(idents, c.decodePreimageIdent(result))
	}

	return c.GetProposalMetadata(ctx, block, idents...)
}

func (c *PreimageOracleContractLatest) GetProposalMetadata(ctx context.Context, block rpcblock.Block, idents ...keccakTypes.LargePreimageIdent) ([]keccakTypes.LargePreimageMetaData, error) {
	var calls []batching.Call
	for _, ident := range idents {
		calls = append(calls, c.contract.Call(methodProposalMetadata, ident.Claimant, ident.UUID))
	}
	results, err := c.multiCaller.Call(ctx, block, calls...)
	if err != nil {
		return nil, fmt.Errorf("failed to load proposal metadata: %w", err)
	}
	var proposals []keccakTypes.LargePreimageMetaData
	for i, result := range results {
		meta := metadata(result.GetBytes32(0))
		proposals = append(proposals, keccakTypes.LargePreimageMetaData{
			LargePreimageIdent: idents[i],
			Timestamp:          meta.timestamp(),
			PartOffset:         meta.partOffset(),
			ClaimedSize:        meta.claimedSize(),
			BlocksProcessed:    meta.blocksProcessed(),
			BytesProcessed:     meta.bytesProcessed(),
			Countered:          meta.countered(),
		})
	}
	return proposals, nil
}

func (c *PreimageOracleContractLatest) GetProposalTreeRoot(ctx context.Context, block rpcblock.Block, ident keccakTypes.LargePreimageIdent) (common.Hash, error) {
	call := c.contract.Call(methodGetTreeRootLPP, ident.Claimant, ident.UUID)
	result, err := c.multiCaller.SingleCall(ctx, block, call)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get tree root: %w", err)
	}
	return result.GetHash(0), nil
}

func (c *PreimageOracleContractLatest) GetInputDataBlocks(ctx context.Context, block rpcblock.Block, ident keccakTypes.LargePreimageIdent) ([]uint64, error) {
	results, err := batching.ReadArray(ctx, c.multiCaller, block,
		c.contract.Call(methodProposalBlocksLen, ident.Claimant, ident.UUID),
		func(i *big.Int) *batching.ContractCall {
			return c.contract.Call(methodProposalBlocks, ident.Claimant, ident.UUID, i)
		})
	if err != nil {
		return nil, fmt.Errorf("failed to load proposal blocks: %w", err)
	}
	blockNums := make([]uint64, 0, len(results))
	for _, result := range results {
		num := result.GetUint64(0)
		if len(blockNums) > 0 && blockNums[len(blockNums)-1] == num {
			// Deduplicate block numbers. The contract guarantees they are in order so we just need to check if the
			// previous block number is the same as this one.
			// Duplicate entries happen when there are two addLeavesLPP calls in the same block.
			continue
		}
		blockNums = append(blockNums, num)
	}
	return blockNums, nil
}

// DecodeInputData returns the UUID and [keccakTypes.InputData] being added to the preimage via a addLeavesLPP call.
// An [ErrInvalidAddLeavesCall] error is returned if the call is not a valid call to addLeavesLPP.
// Otherwise, the uuid and input data is returned. The raw data supplied is returned so long as it can be parsed.
// Specifically the length of the input data is not validated to ensure it is consistent with the number of commitments.
func (c *PreimageOracleContractLatest) DecodeInputData(data []byte) (*big.Int, keccakTypes.InputData, error) {
	method, args, err := c.contract.DecodeCall(data)
	if errors.Is(err, batching.ErrUnknownMethod) {
		return nil, keccakTypes.InputData{}, ErrInvalidAddLeavesCall
	} else if err != nil {
		return nil, keccakTypes.InputData{}, err
	}
	if method != methodAddLeavesLPP {
		return nil, keccakTypes.InputData{}, fmt.Errorf("%w: %v", ErrInvalidAddLeavesCall, method)
	}
	uuid := args.GetBigInt(0)
	// Arg 1 is the starting block index which we don't current use
	input := args.GetBytes(2)
	stateCommitments := args.GetBytes32Slice(3)
	finalize := args.GetBool(4)

	commitments := make([]common.Hash, 0, len(stateCommitments))
	for _, c := range stateCommitments {
		commitments = append(commitments, c)
	}
	return uuid, keccakTypes.InputData{
		Input:       input,
		Commitments: commitments,
		Finalize:    finalize,
	}, nil
}

func (c *PreimageOracleContractLatest) GlobalDataExists(ctx context.Context, data *types.PreimageOracleData) (bool, error) {
	call := c.contract.Call(methodPreimagePartOk, common.Hash(data.OracleKey), new(big.Int).SetUint64(uint64(data.OracleOffset)))
	results, err := c.multiCaller.SingleCall(ctx, rpcblock.Latest, call)
	if err != nil {
		return false, fmt.Errorf("failed to get preimagePartOk: %w", err)
	}
	return results.GetBool(0), nil
}

func (c *PreimageOracleContractLatest) ChallengeTx(ident keccakTypes.LargePreimageIdent, challenge keccakTypes.Challenge) (txmgr.TxCandidate, error) {
	var call *batching.ContractCall
	if challenge.Prestate == (keccakTypes.Leaf{}) {
		call = c.contract.Call(
			methodChallengeFirstLPP,
			ident.Claimant,
			ident.UUID,
			toPreimageOracleLeaf(challenge.Poststate),
			challenge.PoststateProof)
	} else {
		call = c.contract.Call(
			methodChallengeLPP,
			ident.Claimant,
			ident.UUID,
			abiEncodeSnapshot(challenge.StateMatrix),
			toPreimageOracleLeaf(challenge.Prestate),
			challenge.PrestateProof,
			toPreimageOracleLeaf(challenge.Poststate),
			challenge.PoststateProof)
	}
	return call.ToTxCandidate()
}

func (c *PreimageOracleContractLatest) GetMinBondLPP(ctx context.Context) (*big.Int, error) {
	if bondSize := c.minBondSizeLPP.Load(); bondSize != 0 {
		return big.NewInt(int64(bondSize)), nil
	}
	result, err := c.multiCaller.SingleCall(ctx, rpcblock.Latest, c.contract.Call(methodMinBondSizeLPP))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch min bond size for LPPs: %w", err)
	}
	period := result.GetBigInt(0)
	c.minBondSizeLPP.Store(period.Uint64())
	return period, nil
}

func (c *PreimageOracleContractLatest) decodePreimageIdent(result *batching.CallResult) keccakTypes.LargePreimageIdent {
	return keccakTypes.LargePreimageIdent{
		Claimant: result.GetAddress(0),
		UUID:     result.GetBigInt(1),
	}
}

// metadata is the packed preimage metadata
// ┌─────────────┬────────────────────────────────────────────┐
// │ Bit Offsets │                Description                 │
// ├─────────────┼────────────────────────────────────────────┤
// │ [0, 64)     │ Timestamp (Finalized - All data available) │
// │ [64, 96)    │ Part Offset                                │
// │ [96, 128)   │ Claimed Size                               │
// │ [128, 160)  │ Blocks Processed (Inclusive of Padding)    │
// │ [160, 192)  │ Bytes Processed (Non-inclusive of Padding) │
// │ [192, 256)  │ Countered                                  │
// └─────────────┴────────────────────────────────────────────┘
type metadata [32]byte

func (m *metadata) setTimestamp(timestamp uint64) {
	binary.BigEndian.PutUint64(m[0:8], timestamp)
}

func (m *metadata) timestamp() uint64 {
	return binary.BigEndian.Uint64(m[0:8])
}

func (m *metadata) setPartOffset(value uint32) {
	binary.BigEndian.PutUint32(m[8:12], value)
}

func (m *metadata) partOffset() uint32 {
	return binary.BigEndian.Uint32(m[8:12])
}

func (m *metadata) setClaimedSize(value uint32) {
	binary.BigEndian.PutUint32(m[12:16], value)
}

func (m *metadata) claimedSize() uint32 {
	return binary.BigEndian.Uint32(m[12:16])
}

func (m *metadata) setBlocksProcessed(value uint32) {
	binary.BigEndian.PutUint32(m[16:20], value)
}

func (m *metadata) blocksProcessed() uint32 {
	return binary.BigEndian.Uint32(m[16:20])
}

func (m *metadata) setBytesProcessed(value uint32) {
	binary.BigEndian.PutUint32(m[20:24], value)
}

func (m *metadata) bytesProcessed() uint32 {
	return binary.BigEndian.Uint32(m[20:24])
}

func (m *metadata) setCountered(value bool) {
	v := uint64(0)
	if value {
		v = math.MaxUint64
	}
	binary.BigEndian.PutUint64(m[24:32], v)
}

func (m *metadata) countered() bool {
	return binary.BigEndian.Uint64(m[24:32]) != 0
}

type PreimageOracleContract interface {
	Addr() common.Address
	AddGlobalDataTx(data *types.PreimageOracleData) (txmgr.TxCandidate, error)
	InitLargePreimage(uuid *big.Int, partOffset uint32, claimedSize uint32) (txmgr.TxCandidate, error)
	AddLeaves(uuid *big.Int, startingBlockIndex *big.Int, input []byte, commitments []common.Hash, finalize bool) (txmgr.TxCandidate, error)
	MinLargePreimageSize(ctx context.Context) (uint64, error)
	ChallengePeriod(ctx context.Context) (uint64, error)
	CallSqueeze(
		ctx context.Context,
		claimant common.Address,
		uuid *big.Int,
		prestateMatrix keccakTypes.StateSnapshot,
		preState keccakTypes.Leaf,
		preStateProof merkle.Proof,
		postState keccakTypes.Leaf,
		postStateProof merkle.Proof,
	) error
	Squeeze(
		claimant common.Address,
		uuid *big.Int,
		prestateMatrix keccakTypes.StateSnapshot,
		preState keccakTypes.Leaf,
		preStateProof merkle.Proof,
		postState keccakTypes.Leaf,
		postStateProof merkle.Proof,
	) (txmgr.TxCandidate, error)
	GetActivePreimages(ctx context.Context, blockHash common.Hash) ([]keccakTypes.LargePreimageMetaData, error)
	GetProposalMetadata(ctx context.Context, block rpcblock.Block, idents ...keccakTypes.LargePreimageIdent) ([]keccakTypes.LargePreimageMetaData, error)
	GetProposalTreeRoot(ctx context.Context, block rpcblock.Block, ident keccakTypes.LargePreimageIdent) (common.Hash, error)
	GetInputDataBlocks(ctx context.Context, block rpcblock.Block, ident keccakTypes.LargePreimageIdent) ([]uint64, error)
	DecodeInputData(data []byte) (*big.Int, keccakTypes.InputData, error)
	GlobalDataExists(ctx context.Context, data *types.PreimageOracleData) (bool, error)
	ChallengeTx(ident keccakTypes.LargePreimageIdent, challenge keccakTypes.Challenge) (txmgr.TxCandidate, error)
	GetMinBondLPP(ctx context.Context) (*big.Int, error)
}
