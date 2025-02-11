package standard

import (
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/stretchr/testify/require"
)

func TestDefaultHardforkScheduleForTag(t *testing.T) {
	sched := DefaultHardforkScheduleForTag(ContractsV160Tag)
	require.Nil(t, sched.HoloceneTime(0))

	sched = DefaultHardforkScheduleForTag(ContractsV180Tag)
	require.NotNil(t, sched.HoloceneTime(0))
}

func TestStandardAddresses(t *testing.T) {
	type addressReturner func(uint64) (common.Address, error)

	tests := []struct {
		f           addressReturner
		mainnetAddr common.Address
		sepoliaAddr common.Address
	}{
		{
			GuardianAddressFor,
			common.HexToAddress("0x09f7150D8c019BeF34450d6920f6B3608ceFdAf2"),
			common.HexToAddress("0x7a50f00e8D05b95F98fE38d8BeE366a7324dCf7E"),
		},
		{
			ChallengerAddressFor,
			common.HexToAddress("0x9BA6e03D8B90dE867373Db8cF1A58d2F7F006b3A"),
			common.HexToAddress("0xfd1D2e729aE8eEe2E146c033bf4400fE75284301"),
		},
		{
			L1ProxyAdminOwner,
			common.HexToAddress("0x5a0Aae59D09fccBdDb6C6CcEB07B7279367C3d2A"),
			common.HexToAddress("0x1Eb2fFc903729a0F03966B917003800b145F56E2"),
		},
		{
			ProtocolVersionsOwner,
			common.HexToAddress("0x847B5c174615B1B7fDF770882256e2D3E95b9D92"),
			common.HexToAddress("0xfd1D2e729aE8eEe2E146c033bf4400fE75284301"),
		},
	}
	for _, test := range tests {
		fname := runtime.FuncForPC(reflect.ValueOf(test.f).Pointer()).Name()
		parts := strings.Split(fname, ".")
		t.Run(parts[len(parts)-1], func(t *testing.T) {
			mainnetAddr, err := test.f(1)
			require.NoError(t, err)
			require.Equal(t, test.mainnetAddr, mainnetAddr)

			sepoliaAddr, err := test.f(11155111)
			require.NoError(t, err)
			require.Equal(t, test.sepoliaAddr, sepoliaAddr)
		})
	}
}
