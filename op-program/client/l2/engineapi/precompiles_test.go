package engineapi

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

var (
	stubRequiredGas     = uint64(29382938)
	stubResult          = []byte{1, 2, 3, 6, 4, 3, 6, 6}
	defaultOracleResult = []byte{9, 9, 9, 10, 10, 10}
)

func TestOverriddenPrecompiles(t *testing.T) {
	tests := []struct {
		name         string
		addr         common.Address
		rules        params.Rules
		overrideWith any
	}{
		{name: "ecrecover", addr: ecrecoverPrecompileAddress, overrideWith: &ecrecoverOracle{}},
		{name: "bn256Pairing", addr: bn256PairingPrecompileAddress, overrideWith: &bn256PairingOracle{}},
		{name: "bn256PairingGranite", addr: bn256PairingPrecompileAddress, rules: params.Rules{IsOptimismGranite: true}, overrideWith: &bn256PairingOracleGranite{}},
		{name: "kzgPointEvaluation", addr: kzgPointEvaluationPrecompileAddress, overrideWith: &kzgPointEvaluationOracle{}},

		{name: "blsG1Add", addr: blsG1AddPrecompileAddress, overrideWith: &blsOperationOracle{}},
		{name: "blsG1MSM", addr: blsG1MSMPrecompileAddress, overrideWith: &blsOperationOracleWithSizeLimit{}},
		{name: "blsG2Add", addr: blsG2AddPrecompileAddress, overrideWith: &blsOperationOracle{}},
		{name: "blsG2MSM", addr: blsG2MSMPrecompileAddress, overrideWith: &blsOperationOracleWithSizeLimit{}},
		{name: "blsPairingCheck", addr: blsPairingPrecompileAddress, overrideWith: &blsOperationOracleWithSizeLimit{}},
		{name: "blsMapFpToG1", addr: blsMapToG1PrecompileAddress, overrideWith: &blsOperationOracle{}},
		{name: "blsMapFp2ToG2", addr: blsMapToG2PrecompileAddress, overrideWith: &blsOperationOracle{}},

		// Actual precompiles but not overridden
		{name: "identity", addr: common.Address{0x04}},
		{name: "ripemd160", addr: common.BytesToAddress([]byte{0x03})},
		{name: "blake2F", addr: common.BytesToAddress([]byte{0x09})},
		{name: "sha256", addr: common.BytesToAddress([]byte{0x02})},

		// Not a precompile, not overridden
		{name: "unknown", addr: common.Address{0xdd, 0xff, 0x33, 0xaa}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			orig := &stubPrecompile{}
			oracle := &stubPrecompileOracle{}
			overrides := CreatePrecompileOverrides(oracle)

			actual := overrides(test.rules, orig, test.addr)
			if test.overrideWith != nil {
				require.NotSame(t, orig, actual, "should have overridden precompile")
				require.IsType(t, test.overrideWith, actual, "should have overridden with correct type")
			} else {
				require.Same(t, orig, actual, "should not have overridden precompile")
			}
		})
	}

	// Ensures that if the pre-compile isn't present in the active fork, we don't add an override that enables it
	t.Run("nil-orig", func(t *testing.T) {
		oracle := &stubPrecompileOracle{}
		overrides := CreatePrecompileOverrides(oracle)

		actual := overrides(params.Rules{}, nil, ecrecoverPrecompileAddress)
		require.Nil(t, actual, "should not add new pre-compiles")
	})
}

func TestEcrecover(t *testing.T) {
	setup := func() (vm.PrecompiledContract, *stubPrecompileOracle) {
		orig := &stubPrecompile{}
		oracle := &stubPrecompileOracle{}
		overrides := CreatePrecompileOverrides(oracle)
		override := overrides(params.Rules{}, orig, ecrecoverPrecompileAddress)
		return override, oracle
	}
	validInput := common.FromHex("18c547e4f7b0f325ad1e56f57e26c745b09a3e503d86e00e5255ff7f715d3d1c000000000000000000000000000000000000000000000000000000000000001c73b1693892219d736caba55bdb67216e485557ea6b6af75f37096c9aa6a5a75feeb940b1d03b21e36b0e47e79769f095fe2ab855bd91e3a38756b7d75a9c4549")

	t.Run("RequiredGas", func(t *testing.T) {
		impl, _ := setup()
		require.Equal(t, stubRequiredGas, impl.RequiredGas(validInput))
	})

	t.Run("Valid", func(t *testing.T) {
		impl, oracle := setup()
		result, err := impl.Run(validInput)
		require.NoError(t, err)
		require.Equal(t, defaultOracleResult, result)
		require.Equal(t, oracle.calledAddr, ecrecoverPrecompileAddress)
		require.Equal(t, oracle.calledInput, validInput)
		require.Equal(t, oracle.calledRequiredGas, stubRequiredGas)
	})

	t.Run("OracleRevert", func(t *testing.T) {
		impl, oracle := setup()
		oracle.failureResponse = true
		result, err := impl.Run(validInput)
		require.ErrorIs(t, err, errInvalidEcrecoverInput)
		require.Nil(t, result)
		require.Equal(t, oracle.calledAddr, ecrecoverPrecompileAddress)
		require.Equal(t, oracle.calledInput, validInput)
		require.Equal(t, oracle.calledRequiredGas, stubRequiredGas)
	})

	t.Run("NotAllZeroV", func(t *testing.T) {
		impl, oracle := setup()
		input := make([]byte, 128)
		copy(input, validInput)
		input[33] = 1
		result, err := impl.Run(input)
		require.NoError(t, err)
		require.Nil(t, result)
		require.Equal(t, oracle.calledAddr, common.Address{}, "should not call oracle")
	})

	t.Run("InvalidSignatureValues", func(t *testing.T) {
		impl, oracle := setup()
		input := []byte{1, 2, 3, 4} // Rubbish input that doesn't pass the sanity checks.
		result, err := impl.Run(input)
		require.NoError(t, err)
		require.Nil(t, result)
		require.Equal(t, oracle.calledAddr, common.Address{}, "should not call oracle")
	})

	t.Run("RightPadInput", func(t *testing.T) {
		impl, oracle := setup()
		// No expected hash, but valid r,s,v values
		input := validInput[:len(validInput)-2]
		paddedInput := make([]byte, len(validInput))
		copy(paddedInput, validInput)
		paddedInput[len(paddedInput)-1] = 0
		paddedInput[len(paddedInput)-2] = 0
		result, err := impl.Run(input)
		require.NoError(t, err)
		require.Equal(t, defaultOracleResult, result)
		require.Equal(t, oracle.calledAddr, ecrecoverPrecompileAddress)
		require.Equal(t, oracle.calledInput, paddedInput)
		require.Equal(t, oracle.calledRequiredGas, stubRequiredGas)
	})
}

func TestBn256Pairing(t *testing.T) {
	setup := func(enableGranite bool) (vm.PrecompiledContract, *stubPrecompileOracle) {
		orig := &stubPrecompile{}
		oracle := &stubPrecompileOracle{result: true32Byte}
		overrides := CreatePrecompileOverrides(oracle)
		override := overrides(params.Rules{IsOptimismGranite: enableGranite}, orig, bn256PairingPrecompileAddress)
		return override, oracle
	}
	validInput := common.FromHex("1c76476f4def4bb94541d57ebba1193381ffa7aa76ada664dd31c16024c43f593034dd2920f673e204fee2811c678745fc819b55d3e9d294e45c9b03a76aef41209dd15ebff5d46c4bd888e51a93cf99a7329636c63514396b4a452003a35bf704bf11ca01483bfa8b34b43561848d28905960114c8ac04049af4b6315a416782bb8324af6cfc93537a2ad1a445cfd0ca2a71acd7ac41fadbf933c2a51be344d120a2a4cf30c1bf9845f20c6fe39e07ea2cce61f0c9bb048165fe5e4de877550111e129f1cf1097710d41c4ac70fcdfa5ba2023c6ff1cbeac322de49d1b6df7c2032c61a830e3c17286de9462bf242fca2883585b93870a73853face6a6bf411198e9393920d483a7260bfb731fb5d25f1aa493335a9e71297e485b7aef312c21800deef121f1e76426a00665e5c4479674322d4f75edadd46debd5cd992f6ed090689d0585ff075ec9e99ad690c3395bc4b313370b38ef355acdadcd122975b12c85ea5db8c6deb4aab71808dcb408fe3d1e7690c43d37b4ce6cc0166fa7daa")

	for _, enableGranite := range []bool{true, false} {
		enableGranite := enableGranite
		name := "Pre-Granite"
		if enableGranite {
			name = "Granite"
		}
		t.Run(name, func(t *testing.T) {
			t.Run("RequiredGas", func(t *testing.T) {
				impl, _ := setup(enableGranite)
				require.Equal(t, stubRequiredGas, impl.RequiredGas(validInput))
			})

			t.Run("Valid", func(t *testing.T) {
				impl, oracle := setup(enableGranite)
				result, err := impl.Run(validInput)
				require.NoError(t, err)
				require.Equal(t, true32Byte, result)
				require.Equal(t, oracle.calledAddr, bn256PairingPrecompileAddress)
				require.Equal(t, oracle.calledInput, validInput)
				require.Equal(t, oracle.calledRequiredGas, stubRequiredGas)
			})

			t.Run("OracleRevert", func(t *testing.T) {
				impl, oracle := setup(enableGranite)
				oracle.failureResponse = true
				result, err := impl.Run(validInput)
				require.ErrorIs(t, err, errInvalidBn256PairingCheck)
				require.Nil(t, result)
				require.Equal(t, oracle.calledAddr, bn256PairingPrecompileAddress)
				require.Equal(t, oracle.calledInput, validInput)
				require.Equal(t, oracle.calledRequiredGas, stubRequiredGas)
			})

			t.Run("LengthNotMultipleOf192", func(t *testing.T) {
				impl, oracle := setup(enableGranite)
				input := make([]byte, 193)
				result, err := impl.Run(input)
				require.ErrorIs(t, err, errBadPairingInput)
				require.Nil(t, result)
				require.Equal(t, oracle.calledAddr, common.Address{}, "should not call oracle")
			})
		})
	}

	t.Run("LongInputPreGranite", func(t *testing.T) {
		impl, oracle := setup(false)
		input := make([]byte, (params.Bn256PairingMaxInputSizeGranite/192+1)*192)
		result, err := impl.Run(input)
		require.NoError(t, err)
		require.Equal(t, true32Byte, result)
		require.Equal(t, oracle.calledAddr, bn256PairingPrecompileAddress)
		require.Equal(t, oracle.calledInput, input)
		require.Equal(t, oracle.calledRequiredGas, stubRequiredGas)
	})

	t.Run("LongInputPostGranite", func(t *testing.T) {
		impl, oracle := setup(true)
		input := make([]byte, params.Bn256PairingMaxInputSizeGranite+1)
		result, err := impl.Run(input)
		require.ErrorIs(t, err, errBadPairingInputSize)
		require.Nil(t, result)
		require.Equal(t, oracle.calledAddr, common.Address{}, "should not call oracle")
	})
}

func TestKzgPointEvaluationPrecompile(t *testing.T) {
	oracleResult := common.FromHex(blobPrecompileReturnValue)
	setup := func() (vm.PrecompiledContract, *stubPrecompileOracle) {
		orig := &stubPrecompile{}
		oracle := &stubPrecompileOracle{result: oracleResult}
		overrides := CreatePrecompileOverrides(oracle)
		override := overrides(params.Rules{}, orig, kzgPointEvaluationPrecompileAddress)
		return override, oracle
	}
	validInput := common.FromHex("01e798154708fe7789429634053cbf9f99b619f9f084048927333fce637f549b564c0a11a0f704f4fc3e8acfe0f8245f0ad1347b378fbf96e206da11a5d3630624d25032e67a7e6a4910df5834b8fe70e6bcfeeac0352434196bdf4b2485d5a18f59a8d2a1a625a17f3fea0fe5eb8c896db3764f3185481bc22f91b4aaffcca25f26936857bc3a7c2539ea8ec3a952b7873033e038326e87ed3e1276fd140253fa08e9fc25fb2d9a98527fc22a2c9612fbeafdad446cbc7bcdbdcd780af2c16a")

	t.Run("RequiredGas", func(t *testing.T) {
		impl, _ := setup()
		require.Equal(t, stubRequiredGas, impl.RequiredGas(validInput))
	})

	t.Run("Valid", func(t *testing.T) {
		impl, oracle := setup()
		result, err := impl.Run(validInput)
		require.NoError(t, err)
		require.Equal(t, oracleResult, result)
		require.Equal(t, oracle.calledAddr, kzgPointEvaluationPrecompileAddress)
		require.Equal(t, oracle.calledInput, validInput)
		require.Equal(t, oracle.calledRequiredGas, stubRequiredGas)
	})

	t.Run("OracleRevert", func(t *testing.T) {
		impl, oracle := setup()
		oracle.failureResponse = true
		result, err := impl.Run(validInput)
		require.ErrorIs(t, err, errBlobVerifyKZGProof)
		require.Nil(t, result)
		require.Equal(t, oracle.calledAddr, kzgPointEvaluationPrecompileAddress)
		require.Equal(t, oracle.calledInput, validInput)
		require.Equal(t, oracle.calledRequiredGas, stubRequiredGas)
	})

	t.Run("IncorrectVersionedHash", func(t *testing.T) {
		impl, oracle := setup()
		input := make([]byte, len(validInput))
		copy(input, validInput)
		input[3] = 74 // Change part of the versioned hash so it doesn't match the commitment
		result, err := impl.Run(input)
		require.ErrorIs(t, err, errBlobVerifyMismatchedVersion)
		require.Nil(t, result)
		require.Equal(t, oracle.calledAddr, common.Address{}, "should not call oracle")
	})

	t.Run("IncorrectLength", func(t *testing.T) {
		impl, oracle := setup()
		input := make([]byte, 193)
		result, err := impl.Run(input)
		require.ErrorIs(t, err, errBlobVerifyInvalidInputLength)
		require.Nil(t, result)
		require.Equal(t, oracle.calledAddr, common.Address{}, "should not call oracle")
	})
}

func TestBLSPrecompileWithSizeLimit(t *testing.T) {
	testCases := []struct {
		name        string
		validInput  []byte
		validOutput []byte
		address     common.Address
		maxLength   int
	}{
		{
			name:        "blsG1Add",
			address:     blsG1AddPrecompileAddress,
			validInput:  common.FromHex("0000000000000000000000000000000017f1d3a73197d7942695638c4fa9ac0fc3688c4f9774b905a14e3a3f171bac586c55e83ff97a1aeffb3af00adb22c6bb0000000000000000000000000000000008b3f481e3aaa0f1a09e30ed741d8ae4fcf5e095d5d00af600db18cb2c04b3edd03cc744a2888ae40caa232946c5e7e100000000000000000000000000000000112b98340eee2777cc3c14163dea3ec97977ac3dc5c70da32e6e87578f44912e902ccef9efe28d4a78b8999dfbca942600000000000000000000000000000000186b28d92356c4dfec4b5201ad099dbdede3781f8998ddf929b4cd7756192185ca7b8f4ef7088f813270ac3d48868a21"),
			validOutput: common.FromHex("000000000000000000000000000000000a40300ce2dec9888b60690e9a41d3004fda4886854573974fab73b046d3147ba5b7a5bde85279ffede1b45b3918d82d0000000000000000000000000000000006d3d887e9f53b9ec4eb6cedf5607226754b07c01ace7834f57f3e7315faefb739e59018e22c492006190fba4a870025"),
			maxLength:   256,
		},
		{
			name:        "blsG1MSM",
			address:     blsG1MSMPrecompileAddress,
			validInput:  common.FromHex("0000000000000000000000000000000017f1d3a73197d7942695638c4fa9ac0fc3688c4f9774b905a14e3a3f171bac586c55e83ff97a1aeffb3af00adb22c6bb0000000000000000000000000000000008b3f481e3aaa0f1a09e30ed741d8ae4fcf5e095d5d00af600db18cb2c04b3edd03cc744a2888ae40caa232946c5e7e1000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000112b98340eee2777cc3c14163dea3ec97977ac3dc5c70da32e6e87578f44912e902ccef9efe28d4a78b8999dfbca942600000000000000000000000000000000186b28d92356c4dfec4b5201ad099dbdede3781f8998ddf929b4cd7756192185ca7b8f4ef7088f813270ac3d48868a210000000000000000000000000000000000000000000000000000000000000002"),
			validOutput: common.FromHex("00000000000000000000000000000000148f92dced907361b4782ab542a75281d4b6f71f65c8abf94a5a9082388c64662d30fd6a01ced724feef3e284752038c0000000000000000000000000000000015c3634c3b67bc18e19150e12bfd8a1769306ed010f59be645a0823acb5b38f39e8e0d86e59b6353fdafc59ca971b769"),
			maxLength:   513760,
		},
		{
			name:        "blsG2Add",
			address:     blsG2AddPrecompileAddress,
			validInput:  common.FromHex("00000000000000000000000000000000024aa2b2f08f0a91260805272dc51051c6e47ad4fa403b02b4510b647ae3d1770bac0326a805bbefd48056c8c121bdb80000000000000000000000000000000013e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e000000000000000000000000000000000ce5d527727d6e118cc9cdc6da2e351aadfd9baa8cbdd3a76d429a695160d12c923ac9cc3baca289e193548608b82801000000000000000000000000000000000606c4a02ea734cc32acd2b02bc28b99cb3e287e85a763af267492ab572e99ab3f370d275cec1da1aaa9075ff05f79be00000000000000000000000000000000103121a2ceaae586d240843a398967325f8eb5a93e8fea99b62b9f88d8556c80dd726a4b30e84a36eeabaf3592937f2700000000000000000000000000000000086b990f3da2aeac0a36143b7d7c824428215140db1bb859338764cb58458f081d92664f9053b50b3fbd2e4723121b68000000000000000000000000000000000f9e7ba9a86a8f7624aa2b42dcc8772e1af4ae115685e60abc2c9b90242167acef3d0be4050bf935eed7c3b6fc7ba77e000000000000000000000000000000000d22c3652d0dc6f0fc9316e14268477c2049ef772e852108d269d9c38dba1d4802e8dae479818184c08f9a569d878451"),
			validOutput: common.FromHex("000000000000000000000000000000000b54a8a7b08bd6827ed9a797de216b8c9057b3a9ca93e2f88e7f04f19accc42da90d883632b9ca4dc38d013f71ede4db00000000000000000000000000000000077eba4eecf0bd764dce8ed5f45040dd8f3b3427cb35230509482c14651713282946306247866dfe39a8e33016fcbe520000000000000000000000000000000014e60a76a29ef85cbd69f251b9f29147b67cfe3ed2823d3f9776b3a0efd2731941d47436dc6d2b58d9e65f8438bad073000000000000000000000000000000001586c3c910d95754fef7a732df78e279c3d37431c6a2b77e67a00c7c130a8fcd4d19f159cbeb997a178108fffffcbd20"),
			maxLength:   512,
		},
		{
			name:        "blsG2MSM",
			address:     blsG2MSMPrecompileAddress,
			validInput:  common.FromHex("00000000000000000000000000000000024aa2b2f08f0a91260805272dc51051c6e47ad4fa403b02b4510b647ae3d1770bac0326a805bbefd48056c8c121bdb80000000000000000000000000000000013e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e000000000000000000000000000000000ce5d527727d6e118cc9cdc6da2e351aadfd9baa8cbdd3a76d429a695160d12c923ac9cc3baca289e193548608b82801000000000000000000000000000000000606c4a02ea734cc32acd2b02bc28b99cb3e287e85a763af267492ab572e99ab3f370d275cec1da1aaa9075ff05f79be000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000103121a2ceaae586d240843a398967325f8eb5a93e8fea99b62b9f88d8556c80dd726a4b30e84a36eeabaf3592937f2700000000000000000000000000000000086b990f3da2aeac0a36143b7d7c824428215140db1bb859338764cb58458f081d92664f9053b50b3fbd2e4723121b68000000000000000000000000000000000f9e7ba9a86a8f7624aa2b42dcc8772e1af4ae115685e60abc2c9b90242167acef3d0be4050bf935eed7c3b6fc7ba77e000000000000000000000000000000000d22c3652d0dc6f0fc9316e14268477c2049ef772e852108d269d9c38dba1d4802e8dae479818184c08f9a569d8784510000000000000000000000000000000000000000000000000000000000000002"),
			validOutput: common.FromHex("00000000000000000000000000000000009cc9ed6635623ba19b340cbc1b0eb05c3a58770623986bb7e041645175b0a38d663d929afb9a949f7524656043bccc000000000000000000000000000000000c0fb19d3f083fd5641d22a861a11979da258003f888c59c33005cb4a2df4df9e5a2868832063ac289dfa3e997f21f8a00000000000000000000000000000000168bf7d87cef37cf1707849e0a6708cb856846f5392d205ae7418dd94d94ef6c8aa5b424af2e99d957567654b9dae1d90000000000000000000000000000000017e0fa3c3b2665d52c26c7d4cea9f35443f4f9007840384163d3aa3c7d4d18b21b65ff4380cf3f3b48e94b5eecb221dd"),
			maxLength:   488448,
		},
		{
			name:        "blsPairingCheck",
			address:     blsPairingPrecompileAddress,
			validInput:  common.FromHex("0000000000000000000000000000000017f1d3a73197d7942695638c4fa9ac0fc3688c4f9774b905a14e3a3f171bac586c55e83ff97a1aeffb3af00adb22c6bb0000000000000000000000000000000008b3f481e3aaa0f1a09e30ed741d8ae4fcf5e095d5d00af600db18cb2c04b3edd03cc744a2888ae40caa232946c5e7e100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000024aa2b2f08f0a91260805272dc51051c6e47ad4fa403b02b4510b647ae3d1770bac0326a805bbefd48056c8c121bdb80000000000000000000000000000000013e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e000000000000000000000000000000000ce5d527727d6e118cc9cdc6da2e351aadfd9baa8cbdd3a76d429a695160d12c923ac9cc3baca289e193548608b82801000000000000000000000000000000000606c4a02ea734cc32acd2b02bc28b99cb3e287e85a763af267492ab572e99ab3f370d275cec1da1aaa9075ff05f79be"),
			validOutput: common.FromHex("0000000000000000000000000000000000000000000000000000000000000001"),
			maxLength:   235008,
		},
		{
			name:        "blsMapToG1",
			address:     blsMapToG1PrecompileAddress,
			validInput:  common.FromHex("00000000000000000000000000000000156c8a6a2c184569d69a76be144b5cdc5141d2d2ca4fe341f011e25e3969c55ad9e9b9ce2eb833c81a908e5fa4ac5f03"),
			validOutput: common.FromHex("00000000000000000000000000000000184bb665c37ff561a89ec2122dd343f20e0f4cbcaec84e3c3052ea81d1834e192c426074b02ed3dca4e7676ce4ce48ba0000000000000000000000000000000004407b8d35af4dacc809927071fc0405218f1401a6d15af775810e4e460064bcc9468beeba82fdc751be70476c888bf3"),
			maxLength:   64,
		},
		{
			name:        "blsMapToG2",
			address:     blsMapToG2PrecompileAddress,
			validInput:  common.FromHex("0000000000000000000000000000000007355d25caf6e7f2f0cb2812ca0e513bd026ed09dda65b177500fa31714e09ea0ded3a078b526bed3307f804d4b93b040000000000000000000000000000000002829ce3c021339ccb5caf3e187f6370e1e2a311dec9b75363117063ab2015603ff52c3d3b98f19c2f65575e99e8b78c"),
			validOutput: common.FromHex("0000000000000000000000000000000000e7f4568a82b4b7dc1f14c6aaa055edf51502319c723c4dc2688c7fe5944c213f510328082396515734b6612c4e7bb700000000000000000000000000000000126b855e9e69b1f691f816e48ac6977664d24d99f8724868a184186469ddfd4617367e94527d4b74fc86413483afb35b000000000000000000000000000000000caead0fd7b6176c01436833c79d305c78be307da5f6af6c133c47311def6ff1e0babf57a0fb5539fce7ee12407b0a42000000000000000000000000000000001498aadcf7ae2b345243e281ae076df6de84455d766ab6fcdaad71fab60abb2e8b980a440043cd305db09d283c895e3d"),
			maxLength:   128,
		},
	}

	for _, testCase := range testCases {
		oracleResult := testCase.validOutput
		setup := func() (vm.PrecompiledContract, *stubPrecompileOracle) {
			orig := &stubPrecompile{}
			oracle := &stubPrecompileOracle{result: oracleResult}
			overrides := CreatePrecompileOverrides(oracle)
			override := overrides(params.Rules{}, orig, testCase.address)
			return override, oracle
		}
		validInput := testCase.validInput

		t.Run(fmt.Sprintf("%s-RequiredGas", testCase.name), func(t *testing.T) {
			impl, _ := setup()
			require.Equal(t, stubRequiredGas, impl.RequiredGas(validInput))
		})

		t.Run(fmt.Sprintf("%s-Valid", testCase.name), func(t *testing.T) {
			impl, oracle := setup()
			result, err := impl.Run(validInput)
			require.NoError(t, err)
			require.Equal(t, oracleResult, result)
			require.Equal(t, oracle.calledAddr, testCase.address)
			require.Equal(t, oracle.calledInput, validInput)
			require.Equal(t, oracle.calledRequiredGas, stubRequiredGas)
		})

		t.Run(fmt.Sprintf("%s-OracleRevert", testCase.name), func(t *testing.T) {
			impl, oracle := setup()
			oracle.failureResponse = true
			result, err := impl.Run(validInput)
			require.ErrorIs(t, err, errInvalidBlsOperation)
			require.Nil(t, result)
			require.Equal(t, oracle.calledAddr, testCase.address)
			require.Equal(t, oracle.calledInput, validInput)
			require.Equal(t, oracle.calledRequiredGas, stubRequiredGas)
		})

		t.Run(fmt.Sprintf("%s-IncorrectLength", testCase.name), func(t *testing.T) {
			impl, oracle := setup()
			input := make([]byte, testCase.maxLength+1)
			result, err := impl.Run(input)
			require.ErrorIs(t, err, errInvalidBlsSize)
			require.Nil(t, result)
			require.Equal(t, oracle.calledAddr, common.Address{}, "should not call oracle")
		})
	}

}

type stubPrecompile struct{}

func (s *stubPrecompile) RequiredGas(_ []byte) uint64 {
	return stubRequiredGas
}

func (s *stubPrecompile) Run(_ []byte) ([]byte, error) {
	return stubResult, nil
}

type stubPrecompileOracle struct {
	result            []byte
	failureResponse   bool
	calledAddr        common.Address
	calledInput       []byte
	calledRequiredGas uint64
}

func (s *stubPrecompileOracle) Precompile(addr common.Address, input []byte, requiredGas uint64) ([]byte, bool) {
	s.calledAddr = addr
	s.calledInput = input
	s.calledRequiredGas = requiredGas
	result := defaultOracleResult
	if s.result != nil {
		result = s.result
	}
	return result, !s.failureResponse
}
