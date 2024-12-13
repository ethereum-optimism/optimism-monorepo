package engineapi

import (
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
