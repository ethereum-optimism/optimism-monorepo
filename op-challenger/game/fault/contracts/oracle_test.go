package contracts

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-challenger/game/keccak/merkle"
	keccakTypes "github.com/ethereum-optimism/optimism/op-challenger/game/keccak/types"
	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	batchingTest "github.com/ethereum-optimism/optimism/op-service/sources/batching/test"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/snapshots"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

const (
	oracle100    = "1.0.0"
	oracleLatest = "1.1.0"
)

var oracleVersions = []contractVersion{
	{
		version: oracle100,
		loadAbi: func() *abi.ABI {
			return mustParseAbi(preimageOracleAbi100)
		},
	},
	{
		version: oracleLatest,
		loadAbi: snapshots.LoadPreimageOracleABI,
	},
}

func TestPreimageOracleContract_AddGlobalDataTx(t *testing.T) {
	for _, version := range oracleVersions {
		version := version
		t.Run(version.version, func(t *testing.T) {

			t.Run("UnknownType", func(t *testing.T) {
				_, oracle := setupPreimageOracleTest(t, version)
				data := types.NewPreimageOracleData(common.Hash{0xcc}.Bytes(), make([]byte, 20), uint32(545))
				_, err := oracle.AddGlobalDataTx(data)
				require.ErrorIs(t, err, ErrUnsupportedKeyType)
			})

			t.Run("Keccak256", func(t *testing.T) {
				stubRpc, oracle := setupPreimageOracleTest(t, version)
				data := types.NewPreimageOracleData(common.Hash{byte(preimage.Keccak256KeyType), 0xcc}.Bytes(), make([]byte, 20), uint32(545))
				stubRpc.SetResponse(oracleAddr, methodLoadKeccak256PreimagePart, rpcblock.Latest, []interface{}{
					new(big.Int).SetUint64(uint64(data.OracleOffset)),
					data.GetPreimageWithoutSize(),
				}, nil)

				tx, err := oracle.AddGlobalDataTx(data)
				require.NoError(t, err)
				stubRpc.VerifyTxCandidate(tx)
			})

			t.Run("Sha256", func(t *testing.T) {
				stubRpc, oracle := setupPreimageOracleTest(t, version)
				data := types.NewPreimageOracleData(common.Hash{byte(preimage.Sha256KeyType), 0xcc}.Bytes(), make([]byte, 20), uint32(545))
				stubRpc.SetResponse(oracleAddr, methodLoadSha256PreimagePart, rpcblock.Latest, []interface{}{
					new(big.Int).SetUint64(uint64(data.OracleOffset)),
					data.GetPreimageWithoutSize(),
				}, nil)

				tx, err := oracle.AddGlobalDataTx(data)
				require.NoError(t, err)
				stubRpc.VerifyTxCandidate(tx)
			})

			t.Run("Blob", func(t *testing.T) {
				stubRpc, oracle := setupPreimageOracleTest(t, version)
				fieldData := testutils.RandomData(rand.New(rand.NewSource(23)), 32)
				data := types.NewPreimageOracleData(common.Hash{byte(preimage.BlobKeyType), 0xcc}.Bytes(), fieldData, uint32(545))
				stubRpc.SetResponse(oracleAddr, methodLoadBlobPreimagePart, rpcblock.Latest, []interface{}{
					new(big.Int).SetUint64(data.BlobFieldIndex),
					new(big.Int).SetBytes(data.GetPreimageWithoutSize()),
					data.BlobCommitment,
					data.BlobProof,
					new(big.Int).SetUint64(uint64(data.OracleOffset)),
				}, nil)

				tx, err := oracle.AddGlobalDataTx(data)
				require.NoError(t, err)
				stubRpc.VerifyTxCandidate(tx)
			})

			t.Run("Precompile", func(t *testing.T) {
				stubRpc, oracle := setupPreimageOracleTest(t, version)
				input := testutils.RandomData(rand.New(rand.NewSource(23)), 200)
				data := types.NewPreimageOracleData(common.Hash{byte(preimage.PrecompileKeyType), 0xcc}.Bytes(), input, uint32(545))
				if version.Is(oracle100) {
					keyData := data.GetPreimageWithoutSize()
					stubRpc.SetResponse(oracleAddr, methodLoadPrecompilePreimagePart, rpcblock.Latest, []interface{}{
						new(big.Int).SetUint64(uint64(data.OracleOffset)),
						common.BytesToAddress(keyData[0:20]),
						keyData[20:],
					}, nil)
				} else {
					stubRpc.SetResponse(oracleAddr, methodLoadPrecompilePreimagePart, rpcblock.Latest, []interface{}{
						new(big.Int).SetUint64(uint64(data.OracleOffset)),
						data.GetPrecompileAddress(),
						data.GetPrecompileRequiredGas(),
						data.GetPrecompileInput(),
					}, nil)
				}

				tx, err := oracle.AddGlobalDataTx(data)
				require.NoError(t, err)
				stubRpc.VerifyTxCandidate(tx)
			})
		})
	}
}

func TestPreimageOracleContract_ChallengePeriod(t *testing.T) {
	for _, version := range oracleVersions {
		version := version
		t.Run(version.version, func(t *testing.T) {

			stubRpc, oracle := setupPreimageOracleTest(t, version)
			stubRpc.SetResponse(oracleAddr, methodChallengePeriod, rpcblock.Latest,
				[]interface{}{},
				[]interface{}{big.NewInt(123)},
			)
			challengePeriod, err := oracle.ChallengePeriod(context.Background())
			require.NoError(t, err)
			require.Equal(t, uint64(123), challengePeriod)

			// Should cache responses
			stubRpc.ClearResponses()
			challengePeriod, err = oracle.ChallengePeriod(context.Background())
			require.NoError(t, err)
			require.Equal(t, uint64(123), challengePeriod)
		})
	}
}

func TestPreimageOracleContract_MinLargePreimageSize(t *testing.T) {
	for _, version := range oracleVersions {
		version := version
		t.Run(version.version, func(t *testing.T) {

			stubRpc, oracle := setupPreimageOracleTest(t, version)
			stubRpc.SetResponse(oracleAddr, methodMinProposalSize, rpcblock.Latest,
				[]interface{}{},
				[]interface{}{big.NewInt(123)},
			)
			minProposalSize, err := oracle.MinLargePreimageSize(context.Background())
			require.NoError(t, err)
			require.Equal(t, uint64(123), minProposalSize)
		})
	}
}

func TestPreimageOracleContract_MinBondSizeLPP(t *testing.T) {
	for _, version := range oracleVersions {
		version := version
		t.Run(version.version, func(t *testing.T) {

			stubRpc, oracle := setupPreimageOracleTest(t, version)
			stubRpc.SetResponse(oracleAddr, methodMinBondSizeLPP, rpcblock.Latest,
				[]interface{}{},
				[]interface{}{big.NewInt(123)},
			)
			minBond, err := oracle.GetMinBondLPP(context.Background())
			require.NoError(t, err)
			require.Equal(t, big.NewInt(123), minBond)

			// Should cache responses
			stubRpc.ClearResponses()
			minBond, err = oracle.GetMinBondLPP(context.Background())
			require.NoError(t, err)
			require.Equal(t, big.NewInt(123), minBond)
		})
	}
}

func TestPreimageOracleContract_PreimageDataExists(t *testing.T) {
	for _, version := range oracleVersions {
		version := version
		t.Run(version.version, func(t *testing.T) {

			t.Run("exists", func(t *testing.T) {
				stubRpc, oracle := setupPreimageOracleTest(t, version)
				data := types.NewPreimageOracleData(common.Hash{0xcc}.Bytes(), make([]byte, 20), 545)
				stubRpc.SetResponse(oracleAddr, methodPreimagePartOk, rpcblock.Latest,
					[]interface{}{common.Hash(data.OracleKey), new(big.Int).SetUint64(uint64(data.OracleOffset))},
					[]interface{}{true},
				)
				exists, err := oracle.GlobalDataExists(context.Background(), data)
				require.NoError(t, err)
				require.True(t, exists)
			})
			t.Run("does not exist", func(t *testing.T) {
				stubRpc, oracle := setupPreimageOracleTest(t, version)
				data := types.NewPreimageOracleData(common.Hash{0xcc}.Bytes(), make([]byte, 20), 545)
				stubRpc.SetResponse(oracleAddr, methodPreimagePartOk, rpcblock.Latest,
					[]interface{}{common.Hash(data.OracleKey), new(big.Int).SetUint64(uint64(data.OracleOffset))},
					[]interface{}{false},
				)
				exists, err := oracle.GlobalDataExists(context.Background(), data)
				require.NoError(t, err)
				require.False(t, exists)
			})
		})
	}
}

func TestPreimageOracleContract_InitLargePreimage(t *testing.T) {
	for _, version := range oracleVersions {
		version := version
		t.Run(version.version, func(t *testing.T) {

			stubRpc, oracle := setupPreimageOracleTest(t, version)

			uuid := big.NewInt(123)
			partOffset := uint32(1)
			claimedSize := uint32(2)
			bond := big.NewInt(42984)
			stubRpc.SetResponse(oracleAddr, methodMinBondSizeLPP, rpcblock.Latest, nil, []interface{}{bond})
			stubRpc.SetResponse(oracleAddr, methodInitLPP, rpcblock.Latest, []interface{}{
				uuid,
				partOffset,
				claimedSize,
			}, nil)

			tx, err := oracle.InitLargePreimage(uuid, partOffset, claimedSize)
			require.NoError(t, err)
			stubRpc.VerifyTxCandidate(tx)
			require.Truef(t, bond.Cmp(tx.Value) == 0, "Expected bond %v got %v", bond, tx.Value)
		})
	}
}

func TestPreimageOracleContract_AddLeaves(t *testing.T) {
	for _, version := range oracleVersions {
		version := version
		t.Run(version.version, func(t *testing.T) {

			stubRpc, oracle := setupPreimageOracleTest(t, version)

			uuid := big.NewInt(123)
			startingBlockIndex := big.NewInt(0)
			input := []byte{0x12}
			commitments := []common.Hash{{0x34}}
			finalize := true
			stubRpc.SetResponse(oracleAddr, methodAddLeavesLPP, rpcblock.Latest, []interface{}{
				uuid,
				startingBlockIndex,
				input,
				commitments,
				finalize,
			}, nil)

			tx, err := oracle.AddLeaves(uuid, startingBlockIndex, input, commitments, finalize)
			require.NoError(t, err)
			stubRpc.VerifyTxCandidate(tx)
		})
	}
}

func TestPreimageOracleContract_Squeeze(t *testing.T) {
	for _, version := range oracleVersions {
		version := version
		t.Run(version.version, func(t *testing.T) {

			stubRpc, oracle := setupPreimageOracleTest(t, version)

			claimant := common.Address{0x12}
			uuid := big.NewInt(123)
			preStateMatrix := keccakTypes.StateSnapshot{0, 1, 2, 3, 4}
			preState := keccakTypes.Leaf{
				Input:           [keccakTypes.BlockSize]byte{0x12},
				Index:           123,
				StateCommitment: common.Hash{0x34},
			}
			preStateProof := merkle.Proof{{0x34}}
			postState := keccakTypes.Leaf{
				Input:           [keccakTypes.BlockSize]byte{0x34},
				Index:           456,
				StateCommitment: common.Hash{0x56},
			}
			postStateProof := merkle.Proof{{0x56}}
			stubRpc.SetResponse(oracleAddr, methodSqueezeLPP, rpcblock.Latest, []interface{}{
				claimant,
				uuid,
				abiEncodeSnapshot(preStateMatrix),
				toPreimageOracleLeaf(preState),
				preStateProof,
				toPreimageOracleLeaf(postState),
				postStateProof,
			}, nil)

			tx, err := oracle.Squeeze(claimant, uuid, preStateMatrix, preState, preStateProof, postState, postStateProof)
			require.NoError(t, err)
			stubRpc.VerifyTxCandidate(tx)
		})
	}
}

func TestGetActivePreimages(t *testing.T) {
	for _, version := range oracleVersions {
		version := version
		t.Run(version.version, func(t *testing.T) {

			blockHash := common.Hash{0xaa}
			_, oracle, proposals := setupPreimageOracleTestWithProposals(t, version, rpcblock.ByHash(blockHash))
			preimages, err := oracle.GetActivePreimages(context.Background(), blockHash)
			require.NoError(t, err)
			require.Equal(t, proposals, preimages)
		})
	}
}

func TestGetProposalMetadata(t *testing.T) {
	for _, version := range oracleVersions {
		version := version
		t.Run(version.version, func(t *testing.T) {

			blockHash := common.Hash{0xaa}
			block := rpcblock.ByHash(blockHash)
			stubRpc, oracle, proposals := setupPreimageOracleTestWithProposals(t, version, block)
			preimages, err := oracle.GetProposalMetadata(
				context.Background(),
				block,
				proposals[0].LargePreimageIdent,
				proposals[1].LargePreimageIdent,
				proposals[2].LargePreimageIdent,
			)
			require.NoError(t, err)
			require.Equal(t, proposals, preimages)

			// Fetching a proposal that doesn't exist should return an empty metadata object.
			ident := keccakTypes.LargePreimageIdent{Claimant: common.Address{0x12}, UUID: big.NewInt(123)}
			meta := new(metadata)
			stubRpc.SetResponse(
				oracleAddr,
				methodProposalMetadata,
				block,
				[]interface{}{ident.Claimant, ident.UUID},
				[]interface{}{meta})
			preimages, err = oracle.GetProposalMetadata(context.Background(), rpcblock.ByHash(blockHash), ident)
			require.NoError(t, err)
			require.Equal(t, []keccakTypes.LargePreimageMetaData{{LargePreimageIdent: ident}}, preimages)
		})
	}
}

func TestGetProposalTreeRoot(t *testing.T) {
	for _, version := range oracleVersions {
		version := version
		t.Run(version.version, func(t *testing.T) {

			blockHash := common.Hash{0xaa}
			expectedRoot := common.Hash{0xbb}
			ident := keccakTypes.LargePreimageIdent{Claimant: common.Address{0x12}, UUID: big.NewInt(123)}
			stubRpc, oracle := setupPreimageOracleTest(t, version)
			stubRpc.SetResponse(oracleAddr, methodGetTreeRootLPP, rpcblock.ByHash(blockHash),
				[]interface{}{ident.Claimant, ident.UUID},
				[]interface{}{expectedRoot})
			actualRoot, err := oracle.GetProposalTreeRoot(context.Background(), rpcblock.ByHash(blockHash), ident)
			require.NoError(t, err)
			require.Equal(t, expectedRoot, actualRoot)
		})
	}
}

func setupPreimageOracleTestWithProposals(t *testing.T, version contractVersion, block rpcblock.Block) (*batchingTest.AbiBasedRpc, PreimageOracleContract, []keccakTypes.LargePreimageMetaData) {
	stubRpc, oracle := setupPreimageOracleTest(t, version)
	stubRpc.SetResponse(
		oracleAddr,
		methodProposalCount,
		block,
		[]interface{}{},
		[]interface{}{big.NewInt(3)})

	preimage1 := keccakTypes.LargePreimageMetaData{
		LargePreimageIdent: keccakTypes.LargePreimageIdent{
			Claimant: common.Address{0xaa},
			UUID:     big.NewInt(1111),
		},
		Timestamp:       1234,
		PartOffset:      1,
		ClaimedSize:     100,
		BlocksProcessed: 10,
		BytesProcessed:  100,
		Countered:       false,
	}
	preimage2 := keccakTypes.LargePreimageMetaData{
		LargePreimageIdent: keccakTypes.LargePreimageIdent{
			Claimant: common.Address{0xbb},
			UUID:     big.NewInt(2222),
		},
		Timestamp:       2345,
		PartOffset:      2,
		ClaimedSize:     200,
		BlocksProcessed: 20,
		BytesProcessed:  200,
		Countered:       true,
	}
	preimage3 := keccakTypes.LargePreimageMetaData{
		LargePreimageIdent: keccakTypes.LargePreimageIdent{
			Claimant: common.Address{0xcc},
			UUID:     big.NewInt(3333),
		},
		Timestamp:       0,
		PartOffset:      3,
		ClaimedSize:     300,
		BlocksProcessed: 30,
		BytesProcessed:  233,
		Countered:       false,
	}

	proposals := []keccakTypes.LargePreimageMetaData{preimage1, preimage2, preimage3}

	for i, proposal := range proposals {
		stubRpc.SetResponse(
			oracleAddr,
			methodProposals,
			block,
			[]interface{}{big.NewInt(int64(i))},
			[]interface{}{
				proposal.Claimant,
				proposal.UUID,
			})
		meta := new(metadata)
		meta.setTimestamp(proposal.Timestamp)
		meta.setPartOffset(proposal.PartOffset)
		meta.setClaimedSize(proposal.ClaimedSize)
		meta.setBlocksProcessed(proposal.BlocksProcessed)
		meta.setBytesProcessed(proposal.BytesProcessed)
		meta.setCountered(proposal.Countered)
		stubRpc.SetResponse(
			oracleAddr,
			methodProposalMetadata,
			block,
			[]interface{}{proposal.Claimant, proposal.UUID},
			[]interface{}{meta})
	}

	return stubRpc, oracle, proposals
}

func setupPreimageOracleTest(t *testing.T, version contractVersion) (*batchingTest.AbiBasedRpc, PreimageOracleContract) {
	stubRpc := batchingTest.NewAbiBasedRpc(t, oracleAddr, version.loadAbi())
	stubRpc.SetResponse(oracleAddr, methodVersion, rpcblock.Latest, nil, []interface{}{version.version})
	oracleContract, err := NewPreimageOracleContract(context.Background(), oracleAddr, batching.NewMultiCaller(stubRpc, batching.DefaultBatchSize))
	require.NoError(t, err)

	return stubRpc, oracleContract
}

func TestMetadata(t *testing.T) {
	uint32Values := []uint32{0, 1, 2, 3252354, math.MaxUint32}
	tests := []struct {
		name   string
		setter func(meta *metadata, val uint32)
		getter func(meta *metadata) uint32
	}{
		{
			name:   "partOffset",
			setter: (*metadata).setPartOffset,
			getter: (*metadata).partOffset,
		},
		{
			name:   "claimedSize",
			setter: (*metadata).setClaimedSize,
			getter: (*metadata).claimedSize,
		},
		{
			name:   "blocksProcessed",
			setter: (*metadata).setBlocksProcessed,
			getter: (*metadata).blocksProcessed,
		},
		{
			name:   "bytesProcessed",
			setter: (*metadata).setBytesProcessed,
			getter: (*metadata).bytesProcessed,
		},
	}
	for _, test := range tests {
		test := test

		for _, version := range oracleVersions {
			version := version
			t.Run(version.version, func(t *testing.T) {

				for _, value := range uint32Values {
					value := value
					t.Run(fmt.Sprintf("%v-%v", test.name, value), func(t *testing.T) {
						meta := new(metadata)
						require.Zero(t, test.getter(meta))
						test.setter(meta, value)
						require.Equal(t, value, test.getter(meta))
					})
				}
			})
		}
	}
}

func TestMetadata_Timestamp(t *testing.T) {
	for _, version := range oracleVersions {
		version := version
		t.Run(version.version, func(t *testing.T) {

			values := []uint64{0, 1, 2, 3252354, math.MaxUint32, math.MaxUint32 + 1, math.MaxUint64}
			var meta metadata
			require.Zero(t, meta.timestamp())
			for _, value := range values {
				meta.setTimestamp(value)
				require.Equal(t, value, meta.timestamp())
			}
		})
	}
}

func TestMetadata_Countered(t *testing.T) {
	for _, version := range oracleVersions {
		version := version
		t.Run(version.version, func(t *testing.T) {

			var meta metadata
			require.False(t, meta.countered())
			meta.setCountered(true)
			require.True(t, meta.countered())
			meta.setCountered(false)
			require.False(t, meta.countered())
		})
	}
}

func TestGetInputDataBlocks(t *testing.T) {
	for _, version := range oracleVersions {
		version := version
		t.Run(version.version, func(t *testing.T) {

			stubRpc, oracle := setupPreimageOracleTest(t, version)
			block := rpcblock.ByHash(common.Hash{0xaa})

			preimage := keccakTypes.LargePreimageIdent{
				Claimant: common.Address{0xbb},
				UUID:     big.NewInt(2222),
			}

			stubRpc.SetResponse(
				oracleAddr,
				methodProposalBlocksLen,
				block,
				[]interface{}{preimage.Claimant, preimage.UUID},
				[]interface{}{big.NewInt(6)})

			blockNums := []uint64{10, 35, 35, 35, 67, 67}
			// Returned block numbers should be deduplicated.
			expectedBlockNums := []uint64{10, 35, 67}

			for i, blockNum := range blockNums {
				stubRpc.SetResponse(
					oracleAddr,
					methodProposalBlocks,
					block,
					[]interface{}{preimage.Claimant, preimage.UUID, big.NewInt(int64(i))},
					[]interface{}{blockNum})
			}

			actual, err := oracle.GetInputDataBlocks(context.Background(), block, preimage)
			require.NoError(t, err)
			require.Len(t, actual, 3)
			require.Equal(t, expectedBlockNums, actual)
		})
	}
}

func TestDecodeInputData(t *testing.T) {
	dataOfLength := func(len int) []byte {
		data := make([]byte, len)
		for i := range data {
			data[i] = byte(i)
		}
		return data
	}
	ident := keccakTypes.LargePreimageIdent{
		Claimant: common.Address{0xaa},
		UUID:     big.NewInt(1111),
	}

	tests := []struct {
		name           string
		input          []byte
		inputData      keccakTypes.InputData
		expectedTxData string
		expectedErr    error
	}{
		{
			name:           "UnknownMethod",
			input:          []byte{0xaa, 0xbb, 0xcc, 0xdd},
			expectedTxData: "aabbccdd",
			expectedErr:    ErrInvalidAddLeavesCall,
		},
		{
			name: "SingleInput",
			inputData: keccakTypes.InputData{
				Input:       dataOfLength(keccakTypes.BlockSize),
				Commitments: []common.Hash{{0xaa}},
				Finalize:    false,
			},
			expectedTxData: "7917de1d0000000000000000000000000000000000000000000000000000000000000457000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000000000000000000000000000016000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000088000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f404142434445464748494a4b4c4d4e4f505152535455565758595a5b5c5d5e5f606162636465666768696a6b6c6d6e6f707172737475767778797a7b7c7d7e7f80818283848586870000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001aa00000000000000000000000000000000000000000000000000000000000000",
		},
		{
			name: "MultipleInputs",
			inputData: keccakTypes.InputData{
				Input:       dataOfLength(2 * keccakTypes.BlockSize),
				Commitments: []common.Hash{{0xaa}, {0xbb}},
				Finalize:    false,
			},
			expectedTxData: "7917de1d0000000000000000000000000000000000000000000000000000000000000457000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000001e000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000110000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f404142434445464748494a4b4c4d4e4f505152535455565758595a5b5c5d5e5f606162636465666768696a6b6c6d6e6f707172737475767778797a7b7c7d7e7f808182838485868788898a8b8c8d8e8f909192939495969798999a9b9c9d9e9fa0a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4b5b6b7b8b9babbbcbdbebfc0c1c2c3c4c5c6c7c8c9cacbcccdcecfd0d1d2d3d4d5d6d7d8d9dadbdcdddedfe0e1e2e3e4e5e6e7e8e9eaebecedeeeff0f1f2f3f4f5f6f7f8f9fafbfcfdfeff000102030405060708090a0b0c0d0e0f000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002aa00000000000000000000000000000000000000000000000000000000000000bb00000000000000000000000000000000000000000000000000000000000000",
		},
		{
			name: "MultipleInputs-InputTooShort",
			inputData: keccakTypes.InputData{
				Input:       dataOfLength(2*keccakTypes.BlockSize - 10),
				Commitments: []common.Hash{{0xaa}, {0xbb}},
				Finalize:    false,
			},
			expectedTxData: "7917de1d0000000000000000000000000000000000000000000000000000000000000457000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000001e000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000106000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f404142434445464748494a4b4c4d4e4f505152535455565758595a5b5c5d5e5f606162636465666768696a6b6c6d6e6f707172737475767778797a7b7c7d7e7f808182838485868788898a8b8c8d8e8f909192939495969798999a9b9c9d9e9fa0a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4b5b6b7b8b9babbbcbdbebfc0c1c2c3c4c5c6c7c8c9cacbcccdcecfd0d1d2d3d4d5d6d7d8d9dadbdcdddedfe0e1e2e3e4e5e6e7e8e9eaebecedeeeff0f1f2f3f4f5f6f7f8f9fafbfcfdfeff00010203040500000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002aa00000000000000000000000000000000000000000000000000000000000000bb00000000000000000000000000000000000000000000000000000000000000",
		},
		{
			name: "MultipleInputs-FinalizeDoesNotPadInput",
			inputData: keccakTypes.InputData{
				Input:       dataOfLength(2*keccakTypes.BlockSize - 15),
				Commitments: []common.Hash{{0xaa}, {0xbb}},
				Finalize:    true,
			},
			expectedTxData: "7917de1d0000000000000000000000000000000000000000000000000000000000000457000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000001e000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000101000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f404142434445464748494a4b4c4d4e4f505152535455565758595a5b5c5d5e5f606162636465666768696a6b6c6d6e6f707172737475767778797a7b7c7d7e7f808182838485868788898a8b8c8d8e8f909192939495969798999a9b9c9d9e9fa0a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4b5b6b7b8b9babbbcbdbebfc0c1c2c3c4c5c6c7c8c9cacbcccdcecfd0d1d2d3d4d5d6d7d8d9dadbdcdddedfe0e1e2e3e4e5e6e7e8e9eaebecedeeeff0f1f2f3f4f5f6f7f8f9fafbfcfdfeff00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002aa00000000000000000000000000000000000000000000000000000000000000bb00000000000000000000000000000000000000000000000000000000000000",
		},
		{
			name: "MultipleInputs-FinalizePadding-FullBlock",
			inputData: keccakTypes.InputData{
				Input:       dataOfLength(2 * keccakTypes.BlockSize),
				Commitments: []common.Hash{{0xaa}, {0xbb}},
				Finalize:    true,
			},
			expectedTxData: "7917de1d0000000000000000000000000000000000000000000000000000000000000457000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000001e000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000110000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f404142434445464748494a4b4c4d4e4f505152535455565758595a5b5c5d5e5f606162636465666768696a6b6c6d6e6f707172737475767778797a7b7c7d7e7f808182838485868788898a8b8c8d8e8f909192939495969798999a9b9c9d9e9fa0a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4b5b6b7b8b9babbbcbdbebfc0c1c2c3c4c5c6c7c8c9cacbcccdcecfd0d1d2d3d4d5d6d7d8d9dadbdcdddedfe0e1e2e3e4e5e6e7e8e9eaebecedeeeff0f1f2f3f4f5f6f7f8f9fafbfcfdfeff000102030405060708090a0b0c0d0e0f000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002aa00000000000000000000000000000000000000000000000000000000000000bb00000000000000000000000000000000000000000000000000000000000000",
		},
		{
			name: "MultipleInputs-FinalizePadding-TrailingZeros",
			inputData: keccakTypes.InputData{
				Input:       make([]byte, 2*keccakTypes.BlockSize),
				Commitments: []common.Hash{{0xaa}, {0xbb}},
				Finalize:    true,
			},
			expectedTxData: "7917de1d0000000000000000000000000000000000000000000000000000000000000457000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000001e0000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000001100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002aa00000000000000000000000000000000000000000000000000000000000000bb00000000000000000000000000000000000000000000000000000000000000",
		},
		{
			name: "MultipleInputs-FinalizePadding-ShorterThanSingleBlock",
			inputData: keccakTypes.InputData{
				Input:       dataOfLength(keccakTypes.BlockSize - 5),
				Commitments: []common.Hash{{0xaa}, {0xbb}},
				Finalize:    true,
			},
			expectedTxData: "7917de1d0000000000000000000000000000000000000000000000000000000000000457000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000000000000000000000000000016000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000083000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f404142434445464748494a4b4c4d4e4f505152535455565758595a5b5c5d5e5f606162636465666768696a6b6c6d6e6f707172737475767778797a7b7c7d7e7f80818200000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002aa00000000000000000000000000000000000000000000000000000000000000bb00000000000000000000000000000000000000000000000000000000000000",
		},
	}
	for _, test := range tests {
		test := test
		for _, version := range oracleVersions {
			version := version
			t.Run(version.version, func(t *testing.T) {

				t.Run(test.name, func(t *testing.T) {
					_, oracle := setupPreimageOracleTest(t, version)
					var input []byte
					if len(test.input) > 0 {
						input = test.input
					} else {
						input = toAddLeavesTxData(t, oracle, ident.UUID, test.inputData)
					}
					require.Equal(t, test.expectedTxData, common.Bytes2Hex(input),
						"ABI has been changed. Add tests to ensure historic transactions can be parsed before updating expectedTxData")
					uuid, leaves, err := oracle.DecodeInputData(input)
					if test.expectedErr != nil {
						require.ErrorIs(t, err, test.expectedErr)
					} else {
						require.NoError(t, err)
						require.Equal(t, ident.UUID, uuid)
						require.Equal(t, test.inputData, leaves)
					}
				})
			})
		}
	}
}

func TestChallenge_First(t *testing.T) {
	for _, version := range oracleVersions {
		version := version
		t.Run(version.version, func(t *testing.T) {

			stubRpc, oracle := setupPreimageOracleTest(t, version)

			ident := keccakTypes.LargePreimageIdent{
				Claimant: common.Address{0xab},
				UUID:     big.NewInt(4829),
			}
			challenge := keccakTypes.Challenge{
				StateMatrix: keccakTypes.StateSnapshot{1, 2, 3, 4, 5},
				Prestate:    keccakTypes.Leaf{},
				Poststate: keccakTypes.Leaf{
					Input:           [136]byte{5, 4, 3, 2, 1},
					Index:           0,
					StateCommitment: common.Hash{0xbb},
				},
				PoststateProof: merkle.Proof{common.Hash{0x01}, common.Hash{0x02}},
			}
			stubRpc.SetResponse(oracleAddr, methodChallengeFirstLPP, rpcblock.Latest,
				[]interface{}{
					ident.Claimant, ident.UUID,
					preimageOracleLeaf{
						Input:           challenge.Poststate.Input[:],
						Index:           new(big.Int).SetUint64(challenge.Poststate.Index),
						StateCommitment: challenge.Poststate.StateCommitment,
					},
					challenge.PoststateProof,
				},
				nil)
			tx, err := oracle.ChallengeTx(ident, challenge)
			require.NoError(t, err)
			stubRpc.VerifyTxCandidate(tx)
		})
	}
}

func TestChallenge_NotFirst(t *testing.T) {
	for _, version := range oracleVersions {
		version := version
		t.Run(version.version, func(t *testing.T) {

			stubRpc, oracle := setupPreimageOracleTest(t, version)

			ident := keccakTypes.LargePreimageIdent{
				Claimant: common.Address{0xab},
				UUID:     big.NewInt(4829),
			}
			challenge := keccakTypes.Challenge{
				StateMatrix: keccakTypes.StateSnapshot{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
				Prestate: keccakTypes.Leaf{
					Input:           [136]byte{9, 8, 7, 6, 5},
					Index:           3,
					StateCommitment: common.Hash{0xcc},
				},
				PrestateProof: merkle.Proof{common.Hash{0x01}, common.Hash{0x02}},
				Poststate: keccakTypes.Leaf{
					Input:           [136]byte{5, 4, 3, 2, 1},
					Index:           4,
					StateCommitment: common.Hash{0xbb},
				},
				PoststateProof: merkle.Proof{common.Hash{0x03}, common.Hash{0x04}},
			}
			stubRpc.SetResponse(oracleAddr, methodChallengeLPP, rpcblock.Latest,
				[]interface{}{
					ident.Claimant, ident.UUID,
					libKeccakStateMatrix{State: challenge.StateMatrix},
					preimageOracleLeaf{
						Input:           challenge.Prestate.Input[:],
						Index:           new(big.Int).SetUint64(challenge.Prestate.Index),
						StateCommitment: challenge.Prestate.StateCommitment,
					},
					challenge.PrestateProof,
					preimageOracleLeaf{
						Input:           challenge.Poststate.Input[:],
						Index:           new(big.Int).SetUint64(challenge.Poststate.Index),
						StateCommitment: challenge.Poststate.StateCommitment,
					},
					challenge.PoststateProof,
				},
				nil)
			tx, err := oracle.ChallengeTx(ident, challenge)
			require.NoError(t, err)
			stubRpc.VerifyTxCandidate(tx)
		})
	}
}

func toAddLeavesTxData(t *testing.T, oracle PreimageOracleContract, uuid *big.Int, inputData keccakTypes.InputData) []byte {
	tx, err := oracle.AddLeaves(uuid, big.NewInt(1), inputData.Input, inputData.Commitments, inputData.Finalize)
	require.NoError(t, err)
	return tx.TxData
}
