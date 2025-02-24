package cannon

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/memory"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/singlethreaded"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/vm"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

//go:embed test_data
var testData embed.FS

func PositionFromTraceIndex(provider *CannonTraceProvider, idx *big.Int) types.Position {
	return types.NewPosition(provider.gameDepth, idx)
}

func TestGet(t *testing.T) {
	dataDir, prestate := setupTestData(t)
	t.Run("ExistingProof", func(t *testing.T) {
		provider, generator := setupWithTestData(t, dataDir, prestate)
		value, err := provider.Get(context.Background(), PositionFromTraceIndex(provider, common.Big0))
		require.NoError(t, err)
		require.Equal(t, common.HexToHash("0x45fd9aa59768331c726e719e76aa343e73123af888804604785ae19506e65e87"), value)
		require.Empty(t, generator.generated)
	})

	t.Run("ErrorsTraceIndexOutOfBounds", func(t *testing.T) {
		provider, generator := setupWithTestData(t, dataDir, prestate)
		largePosition := PositionFromTraceIndex(provider, new(big.Int).Mul(new(big.Int).SetUint64(math.MaxUint64), big.NewInt(2)))
		_, err := provider.Get(context.Background(), largePosition)
		require.ErrorContains(t, err, "trace index out of bounds")
		require.Empty(t, generator.generated)
	})

	t.Run("ProofAfterEndOfTrace", func(t *testing.T) {
		provider, generator := setupWithTestData(t, dataDir, prestate)
		generator.finalState = &singlethreaded.State{
			Memory: memory.NewMemory(),
			Step:   10,
			Exited: true,
		}
		value, err := provider.Get(context.Background(), PositionFromTraceIndex(provider, big.NewInt(7000)))
		require.NoError(t, err)
		require.Contains(t, generator.generated, 7000, "should have tried to generate the proof")
		_, stateHash := generator.finalState.EncodeWitness()
		require.Equal(t, stateHash, value)
	})

	t.Run("MissingPostHash", func(t *testing.T) {
		provider, generator := setupWithTestData(t, dataDir, prestate)
		_, err := provider.Get(context.Background(), PositionFromTraceIndex(provider, big.NewInt(1)))
		require.ErrorContains(t, err, "missing post hash")
		require.Empty(t, generator.generated)
	})

	t.Run("IgnoreUnknownFields", func(t *testing.T) {
		provider, generator := setupWithTestData(t, dataDir, prestate)
		value, err := provider.Get(context.Background(), PositionFromTraceIndex(provider, big.NewInt(2)))
		require.NoError(t, err)
		expected := common.HexToHash("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
		require.Equal(t, expected, value)
		require.Empty(t, generator.generated)
	})
}

func TestGetStepData(t *testing.T) {
	t.Run("ExistingProof", func(t *testing.T) {
		dataDir, prestate := setupTestData(t)
		provider, generator := setupWithTestData(t, dataDir, prestate)
		value, proof, data, err := provider.GetStepData(context.Background(), PositionFromTraceIndex(provider, new(big.Int)))
		require.NoError(t, err)
		expected := common.FromHex("b8f068de604c85ea0e2acd437cdb47add074a2d70b81d018390c504b71fe26f400000000000000000000000000000000000000000000000000000000000000000000000000")
		require.Equal(t, expected, value)
		expectedProof := common.FromHex("08028e3c0000000000000000000000003c01000a24210b7c00200008000000008fa40004")
		require.Equal(t, expectedProof, proof)
		// TODO: Need to add some oracle data
		require.Nil(t, data)
		require.Empty(t, generator.generated)
	})

	t.Run("ErrorsTraceIndexOutOfBounds", func(t *testing.T) {
		dataDir, prestate := setupTestData(t)
		provider, generator := setupWithTestData(t, dataDir, prestate)
		largePosition := PositionFromTraceIndex(provider, new(big.Int).Mul(new(big.Int).SetUint64(math.MaxUint64), big.NewInt(2)))
		_, _, _, err := provider.GetStepData(context.Background(), largePosition)
		require.ErrorContains(t, err, "trace index out of bounds")
		require.Empty(t, generator.generated)
	})

	t.Run("GenerateProof", func(t *testing.T) {
		dataDir, prestate := setupTestData(t)
		provider, generator := setupWithTestData(t, dataDir, prestate)
		generator.finalState = &singlethreaded.State{
			Memory: memory.NewMemory(),
			Step:   10,
			Exited: true,
		}
		generator.proof = &utils.ProofData{
			ClaimValue:   common.Hash{0xaa},
			StateData:    []byte{0xbb},
			ProofData:    []byte{0xcc},
			OracleKey:    common.Hash{0xdd}.Bytes(),
			OracleValue:  []byte{0xdd},
			OracleOffset: 10,
		}
		preimage, proof, data, err := provider.GetStepData(context.Background(), PositionFromTraceIndex(provider, big.NewInt(4)))
		require.NoError(t, err)
		require.Contains(t, generator.generated, 4, "should have tried to generate the proof")

		require.EqualValues(t, generator.proof.StateData, preimage)
		require.EqualValues(t, generator.proof.ProofData, proof)
		expectedData := types.NewPreimageOracleData(generator.proof.OracleKey, generator.proof.OracleValue, generator.proof.OracleOffset)
		require.EqualValues(t, expectedData, data)
	})

	t.Run("ProofAfterEndOfTrace", func(t *testing.T) {
		dataDir, prestate := setupTestData(t)
		provider, generator := setupWithTestData(t, dataDir, prestate)
		generator.finalState = &singlethreaded.State{
			Memory: memory.NewMemory(),
			Step:   10,
			Exited: true,
		}
		generator.proof = &utils.ProofData{
			ClaimValue:   common.Hash{0xaa},
			StateData:    []byte{0xbb},
			ProofData:    []byte{0xcc},
			OracleKey:    common.Hash{0xdd}.Bytes(),
			OracleValue:  []byte{0xdd},
			OracleOffset: 10,
		}
		preimage, proof, data, err := provider.GetStepData(context.Background(), PositionFromTraceIndex(provider, big.NewInt(7000)))
		require.NoError(t, err)
		require.Contains(t, generator.generated, 7000, "should have tried to generate the proof")

		witness, _ := generator.finalState.EncodeWitness()
		require.EqualValues(t, witness, preimage)
		require.Equal(t, []byte{}, proof)
		require.Nil(t, data)
	})

	t.Run("ReadLastStepFromDisk", func(t *testing.T) {
		dataDir, prestate := setupTestData(t)
		provider, initGenerator := setupWithTestData(t, dataDir, prestate)
		initGenerator.finalState = &singlethreaded.State{
			Memory: memory.NewMemory(),
			Step:   10,
			Exited: true,
		}
		initGenerator.proof = &utils.ProofData{
			ClaimValue:   common.Hash{0xaa},
			StateData:    []byte{0xbb},
			ProofData:    []byte{0xcc},
			OracleKey:    common.Hash{0xdd}.Bytes(),
			OracleValue:  []byte{0xdd},
			OracleOffset: 10,
		}
		_, _, _, err := provider.GetStepData(context.Background(), PositionFromTraceIndex(provider, big.NewInt(7000)))
		require.NoError(t, err)
		require.Contains(t, initGenerator.generated, 7000, "should have tried to generate the proof")

		provider, generator := setupWithTestData(t, dataDir, prestate)
		generator.finalState = &singlethreaded.State{
			Memory: memory.NewMemory(),
			Step:   10,
			Exited: true,
		}
		generator.proof = &utils.ProofData{
			ClaimValue: common.Hash{0xaa},
			StateData:  []byte{0xbb},
			ProofData:  []byte{0xcc},
		}
		preimage, proof, data, err := provider.GetStepData(context.Background(), PositionFromTraceIndex(provider, big.NewInt(7000)))
		require.NoError(t, err)
		require.Empty(t, generator.generated, "should not have to generate the proof again")

		encodedWitness, _ := initGenerator.finalState.EncodeWitness()
		require.EqualValues(t, encodedWitness, preimage)
		require.Empty(t, proof)
		require.Nil(t, data)
	})

	t.Run("MissingStateData", func(t *testing.T) {
		dataDir, prestate := setupTestData(t)
		provider, generator := setupWithTestData(t, dataDir, prestate)
		_, _, _, err := provider.GetStepData(context.Background(), PositionFromTraceIndex(provider, big.NewInt(1)))
		require.ErrorContains(t, err, "missing state data")
		require.Empty(t, generator.generated)
	})

	t.Run("IgnoreUnknownFields", func(t *testing.T) {
		dataDir, prestate := setupTestData(t)
		provider, generator := setupWithTestData(t, dataDir, prestate)
		value, proof, data, err := provider.GetStepData(context.Background(), PositionFromTraceIndex(provider, big.NewInt(2)))
		require.NoError(t, err)
		expected := common.FromHex("cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
		require.Equal(t, expected, value)
		expectedProof := common.FromHex("dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")
		require.Equal(t, expectedProof, proof)
		require.Empty(t, generator.generated)
		require.Nil(t, data)
	})
}

func setupTestData(t *testing.T) (string, string) {
	srcDir := filepath.Join("test_data", "proofs")
	entries, err := testData.ReadDir(srcDir)
	require.NoError(t, err)
	dataDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dataDir, utils.ProofsDir), 0o777))
	for _, entry := range entries {
		path := filepath.Join(srcDir, entry.Name())
		file, err := testData.ReadFile(path)
		require.NoErrorf(t, err, "reading %v", path)
		proofFile := filepath.Join(dataDir, utils.ProofsDir, entry.Name()+".gz")
		err = ioutil.WriteCompressedBytes(proofFile, file, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0o644)
		require.NoErrorf(t, err, "writing %v", path)
	}
	return dataDir, "state.json"
}

func setupWithTestData(t *testing.T, dataDir string, prestate string) (*CannonTraceProvider, *stubGenerator) {
	generator := &stubGenerator{}
	return &CannonTraceProvider{
		logger:         testlog.Logger(t, log.LevelInfo),
		dir:            dataDir,
		generator:      generator,
		prestate:       filepath.Join(dataDir, prestate),
		gameDepth:      63,
		stateConverter: generator,
	}, generator
}

type stubGenerator struct {
	generated  []int // Using int makes assertions easier
	finalState *singlethreaded.State
	proof      *utils.ProofData

	finalStatePath string
}

func (e *stubGenerator) ConvertStateToProof(ctx context.Context, statePath string) (*utils.ProofData, uint64, bool, error) {
	if statePath == e.finalStatePath {
		witness, hash := e.finalState.EncodeWitness()
		return &utils.ProofData{
			ClaimValue: hash,
			StateData:  witness,
			ProofData:  []byte{},
		}, e.finalState.Step, e.finalState.Exited, nil
	} else {
		return nil, 0, false, fmt.Errorf("loading unexpected state: %s, only support: %s", statePath, e.finalStatePath)
	}
}

func (e *stubGenerator) GenerateProof(ctx context.Context, dir string, i uint64) error {
	e.generated = append(e.generated, int(i))
	var proofFile string
	var data []byte
	var err error
	if e.finalState != nil && e.finalState.Step <= i {
		// Requesting a trace index past the end of the trace
		proofFile = vm.FinalStatePath(dir, false)
		e.finalStatePath = proofFile
		data, err = json.Marshal(e.finalState)
		if err != nil {
			return err
		}
		return ioutil.WriteCompressedBytes(proofFile, data, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0o644)
	}
	if e.proof != nil {
		proofFile = filepath.Join(dir, utils.ProofsDir, fmt.Sprintf("%d.json.gz", i))
		data, err = json.Marshal(e.proof)
		if err != nil {
			return err
		}
		return ioutil.WriteCompressedBytes(proofFile, data, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0o644)
	}
	return nil
}
