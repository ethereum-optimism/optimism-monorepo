package asterisc

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

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/vm"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

//go:embed test_data
var testData embed.FS
var asteriscWitnessLen = 362

func PositionFromTraceIndex(provider *AsteriscTraceProvider, idx *big.Int) types.Position {
	return types.NewPosition(provider.gameDepth, idx)
}

func TestGet(t *testing.T) {
	dataDir, prestate := setupTestData(t)
	t.Run("ExistingProof", func(t *testing.T) {
		provider, generator := setupWithTestData(t, dataDir, prestate)
		value, err := provider.Get(context.Background(), PositionFromTraceIndex(provider, common.Big0))
		require.NoError(t, err)
		require.Equal(t, common.HexToHash("0x034689707b571db46b32c9e433def18e648f4e1fa9e5abd4012e7913031bfc10"), value)
		require.Empty(t, generator.generated)
	})

	t.Run("ErrorsTraceIndexOutOfBounds", func(t *testing.T) {
		provider, generator := setupWithTestData(t, dataDir, prestate)
		largePosition := PositionFromTraceIndex(provider, new(big.Int).Mul(new(big.Int).SetUint64(math.MaxUint64), big.NewInt(2)))
		_, err := provider.Get(context.Background(), largePosition)
		require.ErrorContains(t, err, "trace index out of bounds")
		require.Empty(t, generator.generated)
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
		value, proof, data, err := provider.GetStepData(context.Background(), PositionFromTraceIndex(provider, common.Big0))
		require.NoError(t, err)
		expected := common.FromHex("0x354cfaf28a5b60c3f64f22f9f171b64aa067f90c6de6c96f725f44c5cf9f8ac1000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000080e080000000000000000000000007f0000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")
		require.Equal(t, expected, value)
		expectedProof := common.FromHex("0x000000000000000003350100930581006f00800100000000970f000067800f01000000000000000097c2ffff938282676780020000000000032581009308e0050e1893682c323d6695396f1122b3cb562af8c65cab19978c9246434fda0536c90ca1cfabf684ebce3ad9fbd54000a2b258f8d0e447c1bb6f7e97de47aadfc12cd7b6f466bfd024daa905886c5f638f4692d843709e6c1c0d9eb2e251c626d53d15e04b59735fe0781bc4357a4243fbc28e6981902a8c2669a2d6456f7a964423db5d1585da978861f8b84067654b29490275c82b54083ee09c82eb7aa9ae693911226bb8297ad82c0963ae943f22d0c6086f4f14437e4d1c87ceb17e68caf5eaec77f14b46225b417d2191ca7b49564c896836a95ad4e9c383bd1c8ff9d8e888c64fb3836daa9535e58372e9646b7b144219980a4389aca5da241c3ec11fbc9297bd7a94ac671ccec288604c23a0072b0c1ed069198959cacdc2574aff65b7eceffc391e21778a1775deceb3ec0990836df98d98a4f3f0dc854587230fbf59e4daa60e8240d74caf90f7e2cd014c1d5d707b2e44269d9a9caf133882fe1ebb2f4237f6282abe89639b357e9231418d0c41373229ae9edfa6815bec484cb79772c9e2a7d80912123558f79b539bb45d435f2a4446970f1e2123494740285cec3491b0a41a9fd7403bdc8cd239a87508039a77b48ee39a951a8bd196b583de2b93444aafd456d0cd92050fa6a816d5183c1d75e96df540c8ac3bb8638b971f0cf3fb5b4a321487a1c8992b921de110f3d5bbb87369b25fe743ad7e789ca52d9f9fe62ccb103b78fe65eaa2cd47895022c590639c8f0c6a3999d8a5c71ed94d355815851b479f8d93eae90822294c96b39724b33491f8497b0bf7e1b995b37e4d759ff8a7958d194da6e00c475a6ddcf6efcb5fb4bb383c9b273da18d01e000dbe9c65e9645644786b620e2dd2ad648ddfcbf4a7e5b1a3a4ecfe7f64667a3f0b7e2f4418588ed35a2458cffeb39b93d26f18d2ab13bdce6aee58e7b99359ec2dfd95a9c16dc00d6ef18b7933a6f8dc65ccb55667138776f7dea101070dc8796e3774df84f40ae0c8229d0d6069e5c8f39a7c299677a09d367fc7b05e3bc380ee652cdc72595f74c7b1043d0e1ffbab734648c838dfb0527d971b602bc216c9619ef0abf5ac974a1ed57f4050aa510dd9c74f508277b39d7973bb2dfccc5eeb0618db8cd74046ff337f0a7bf2c8e03e10f642c1886798d71806ab1e888d9e5ee87d0838c5655cb21c6cb83313b5a631175dff4963772cce9108188b34ac87c81c41e662ee4dd2dd7b2bc707961b1e646c4047669dcb6584f0d8d770daf5d7e7deb2e388ab20e2573d171a88108e79d820e98f26c0b84aa8b2f4aa4968dbb818ea32293237c50ba75ee485f4c22adf2f741400bdf8d6a9cc7df7ecae576221665d7358448818bb4ae4562849e949e17ac16e0be16688e156b5cf15e098c627c0056a927ae5ba08d7291c96c8cbddcc148bf48a6d68c7974b94356f53754ef6171d757bf558bebd2ceec7f3c5dce04a4782f88c2c6036ae78ee206d0bc5289d20461a2e21908c2968c0699040a6fd866a577a99a9d2ec88745c815fd4a472c789244daae824d72ddc272aab68a8c3022e36f10454437c1886f3ff9927b64f232df414f27e429a4bef3083bc31a671d046ea5c1f5b8c3094d72868d9dfdc12c7334ac5f743cc5c365a9a6a15c1f240ac25880c7a9d1de290696cb766074a1d83d9278164adcf616c3bfabf63999a01966c998b7bb572774035a63ead49da73b5987f34775786645d0c5dd7c04a2f8a75dcae085213652f5bce3ea8b9b9bedd1cab3c5e9b88b152c9b8a7b79637d35911848b0c41e7cc7cca2ab4fe9a15f9c38bb4bb9390c4e2d8ce834ffd7a6cd85d7113d4521abb857774845c4291e6f6d010d97e3185bc799d83e3bb31501b3da786680df30fbc18eb41cbce611e8c0e9c72f69571ca10d3ef857d04d9c03ead7c6317d797a090fa1271ad9c7addfbcb412e9643d4fb33b1809c42623f474055fa9400a2027a7a885c8dfa4efe20666b4ee27d7529c134d7f28d53f175f6bf4b62faa2110d5b76f0f770c15e628181c1fcc18f970a9c34d24b2fc8c50ca9c07a7156ef4e5ff4bdf002eda0b11c1d359d0b59a54680704dbb9db631457879b27e0dfdbe50158fd9cf9b4cf77605c4ac4c95bd65fc9f6f9295a686647cb999090819cda700820c282c613cedcd218540bbc6f37b01c6567c4a1ea624f092a3a5cca2d6f0f0db231972fce627f0ecca0dee60f17551c5f8fdaeb5ab560b2ceb781cdb339361a0fbee1b9dffad59115138c8d6a70dda9ccc1bf0bbdd7fee15764845db875f6432559ff8dbc9055324431bc34e5b93d15da307317849eccd90c0c7b98870b9317c15a5959dcfb84c76dcc908c4fe6ba92126339bf06e458f6646df5e83ba7c3d35bc263b3222c8e9040068847749ca8e8f95045e4342aeb521eb3a5587ec268ed3aa6faf32b62b0bc41a9d549521f406fc3ec7d4dabb75e0d3e144d7cc882372d13746b6dcd481b1b229bcaec9f7422cdfb84e35c5d92171376cae5c86300822d729cd3a8479583bef09527027dba5f11263c5cbbeb3834b7a5c1cba9aa5fee0c95ec3f17a33ec3d8047fff799187f5ae2040bbe913c226c34c9fbe4389dd728984257a816892b3cae3e43191dd291f0eb50000000000000000420000000000000035000000000000000000000000000000060000000000000000100000000000001900000000000000480000000000001050edbc06b4bfc3ee108b66f7a8f772ca4d90e1a085f4a8398505920f7465bb44b4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d3021ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85e58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a193440eb01ebfc9ed27500cd4dfc979272d1f0913cc9f66540d7e8005811109e1cf2d887c22bd8750d34016ac3c66b5ff102dacdd73f6b014e710b51e8022af9a1968ffd70157e48063fc33c97a050f7f640233bf646cc98d9524c6b92bcf3ab56f839867cc5f7f196b93bae1e27e6320742445d290f2263827498b54fec539f756afcefad4e508c098b9a7e1d8feb19955fb02ba9675585078710969d3440f5054e0f9dc3e7fe016e050eff260334f18a5d4fe391d82092319f5964f2e2eb7c1c3a5f8b13a49e282f609c317a833fb8d976d11517c571d1221a265d25af778ecf8923490c6ceeb450aecdc82e28293031d10c7d73bf85e57bf041a97360aa2c5d99cc1df82d9c4b87413eae2ef048f94b4d3554cea73d92b0f7af96e0271c691e2bb5c67add7c6caf302256adedf7ab114da0acfe870d449a3a489f781d659e8beccda7bce9f4e8618b6bd2f4132ce798cdc7a60e7e1460a7299e3c6342a579626d22733e50f526ec2fa19a22b31e8ed50f23cd1fdf94c9154ed3a7609a2f1ff981fe1d3b5c807b281e4683cc6d6315cf95b9ade8641defcb32372f1c126e398ef7a5a2dce0a8a7f68bb74560f8f71837c2c2ebbcbf7fffb42ae1896f13f7c7479a0b46a28b6f55540f89444f63de0378e3d121be09e06cc9ded1c20e65876d36aa0c65e9645644786b620e2dd2ad648ddfcbf4a7e5b1a3a4ecfe7f64667a3f0b7e2f4418588ed35a2458cffeb39b93d26f18d2ab13bdce6aee58e7b99359ec2dfd95a9c16dc00d6ef18b7933a6f8dc65ccb55667138776f7dea101070dc8796e3774df84f40ae0c8229d0d6069e5c8f39a7c299677a09d367fc7b05e3bc380ee652cdc72595f74c7b1043d0e1ffbab734648c838dfb0527d971b602bc216c9619ef0abf5ac974a1ed57f4050aa510dd9c74f508277b39d7973bb2dfccc5eeb0618db8cd74046ff337f0a7bf2c8e03e10f642c1886798d71806ab1e888d9e5ee87d0838c5655cb21c6cb83313b5a631175dff4963772cce9108188b34ac87c81c41e662ee4dd2dd7b2bc707961b1e646c4047669dcb6584f0d8d770daf5d7e7deb2e388ab20e2573d171a88108e79d820e98f26c0b84aa8b2f4aa4968dbb818ea32293237c50ba75ee485f4c22adf2f741400bdf8d6a9cc7df7ecae576221665d7358448818bb4ae4562849e949e17ac16e0be16688e156b5cf15e098c627c0056a927ae5ba08d7291c96c8cbddcc148bf48a6d68c7974b94356f53754ef6171d757bf558bebd2ceec7f3c5dce04a4782f88c2c6036ae78ee206d0bc5289d20461a2e21908c2968c0699040a6fd866a577a99a9d2ec88745c815fd4a472c789244daae824d72ddc272aab68a8c3022e36f10454437c1886f3ff9927b64f232df414f27e429a4bef3083bc31a671d046ea5c1f5b8c3094d72868d9dfdc12c7334ac5f743cc5c365a9a6a15c1f240ac25880c7a9d1de290696cb766074a1d83d9278164adcf616c3bfabf63999a01966c998b7bb572774035a63ead49da73b5987f34775786645d0c5dd7c04a2f8a75dcae085213652f5bce3ea8b9b9bedd1cab3c5e9b88b152c9b8a7b79637d35911848b0c41e7cc7cca2ab4fe9a15f9c38bb4bb9390c4e2d8ce834ffd7a6cd85d7113d4521abb857774845c4291e6f6d010d97e3185bc799d83e3bb31501b3da786680df30fbc18eb41cbce611e8c0e9c72f69571ca10d3ef857d04d9c03ead7c6317d797a090fa1271ad9c7addfbcb412e9643d4fb33b1809c42623f474055fa9400a2027a7a885c8dfa4efe20666b4ee27d7529c134d7f28d53f175f6bf4b62faa2110d5b76f0f770c15e628181c1fcc18f970a9c34d24b2fc8c50ca9c07a7156ef4e5ff4bdf002eda0b11c1d359d0b59a54680704dbb9db631457879b27e0dfdbe50158fd9cf9b4cf77605c4ac4c95bd65fc9f6f9295a686647cb999090819cda700820c282c613cedcd218540bbc6f37b01c6567c4a1ea624f092a3a5cca2d6f0f0db231972fce627f0ecca0dee60f17551c5f8fdaeb5ab560b2ceb781cdb339361a0fbee1b9dffad59115138c8d6a70dda9ccc1bf0bbdd7fee15764845db875f6432559ff8dbc9055324431bc34e5b93d15da307317849eccd90c0c7b98870b9317c15a5959dcfb84c76dcc908c4fe6ba92126339bf06e458f6646df5e83ba7c3d35bc263b3222c8e9040068847749ca8e8f95045e4342aeb521eb3a5587ec268ed3aa6faf32b62b0bc41a9d549521f406fc30f3e39c5412c30550d1d07fb07ff0e546fbeea1988f6658f04a9b19693e5b99d84e35c5d92171376cae5c86300822d729cd3a8479583bef09527027dba5f11263c5cbbeb3834b7a5c1cba9aa5fee0c95ec3f17a33ec3d8047fff799187f5ae2040bbe913c226c34c9fbe4389dd728984257a816892b3cae3e43191dd291f0eb5")
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
		generator.finalState = &VMState{
			Step:    10,
			Exited:  true,
			Witness: make([]byte, asteriscWitnessLen),
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
		generator.finalState = &VMState{
			Step:    10,
			Exited:  true,
			Witness: make([]byte, asteriscWitnessLen),
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

		witness := generator.finalState.Witness
		require.EqualValues(t, witness, preimage)
		require.Equal(t, []byte{}, proof)
		require.Nil(t, data)
	})

	t.Run("ReadLastStepFromDisk", func(t *testing.T) {
		dataDir, prestate := setupTestData(t)
		provider, initGenerator := setupWithTestData(t, dataDir, prestate)
		initGenerator.finalState = &VMState{
			Step:    10,
			Exited:  true,
			Witness: make([]byte, asteriscWitnessLen),
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
		generator.finalState = &VMState{
			Step:    10,
			Exited:  true,
			Witness: make([]byte, asteriscWitnessLen),
		}
		generator.proof = &utils.ProofData{
			ClaimValue: common.Hash{0xaa},
			StateData:  []byte{0xbb},
			ProofData:  []byte{0xcc},
		}
		preimage, proof, data, err := provider.GetStepData(context.Background(), PositionFromTraceIndex(provider, big.NewInt(7000)))
		require.NoError(t, err)
		require.Empty(t, generator.generated, "should not have to generate the proof again")

		require.EqualValues(t, initGenerator.finalState.Witness, preimage)
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

func setupWithTestData(t *testing.T, dataDir string, prestate string) (*AsteriscTraceProvider, *stubGenerator) {
	generator := &stubGenerator{}
	return &AsteriscTraceProvider{
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
	finalState *VMState
	proof      *utils.ProofData

	finalStatePath string
}

func (e *stubGenerator) ConvertStateToProof(ctx context.Context, statePath string) (*utils.ProofData, uint64, bool, error) {
	if statePath == e.finalStatePath {
		return &utils.ProofData{
			ClaimValue: e.finalState.StateHash,
			StateData:  e.finalState.Witness,
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
