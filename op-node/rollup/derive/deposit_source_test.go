package derive

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

// TestEcotone4788ContractSourceHash
// cast keccak $(cast concat-hex 0x0000000000000000000000000000000000000000000000000000000000000002 $(cast keccak "Ecotone: L1 Block Proxy Update"))
// # 0x18acb38c5ff1c238a7460ebc1b421fa49ec4874bdf1e0a530d234104e5e67dbc
func TestDeposit(t *testing.T) {
	source := UpgradeDepositSource{
		Intent: "Ecotone: L1 Block Proxy Update",
	}

	actual := source.SourceHash()
	expected := "0x18acb38c5ff1c238a7460ebc1b421fa49ec4874bdf1e0a530d234104e5e67dbc"

	assert.Equal(t, expected, actual.Hex())
}

// TestEcotone4788ContractSourceHash tests that the source-hash of the 4788 deposit deployment tx is correct.
// As per specs, the hash is computed as:
// cast keccak $(cast concat-hex 0x0000000000000000000000000000000000000000000000000000000000000002 $(cast keccak "Ecotone: beacon block roots contract deployment"))
// # 0x69b763c48478b9dc2f65ada09b3d92133ec592ea715ec65ad6e7f3dc519dc00c
func TestEcotone4788ContractSourceHash(t *testing.T) {
	source := UpgradeDepositSource{
		Intent: "Ecotone: beacon block roots contract deployment",
	}

	actual := source.SourceHash()
	expected := "0x69b763c48478b9dc2f65ada09b3d92133ec592ea715ec65ad6e7f3dc519dc00c"

	assert.Equal(t, expected, actual.Hex())
}

// TestL1InfoDepositSource
// cast keccak $(cast concat-hex 0x0000000000000000000000000000000000000000000000000000000000000001 $(cast keccak $(cast concat-hex 0xc00e5d67c2755389aded7d8b151cbd5bcdf7ed275ad5e028b664880fc7581c77 0x0000000000000000000000000000000000000000000000000000000000000004)))
// # 0x0586c503340591999b8b38bc9834bb16aec7d5bc00eb5587ab139c9ddab81977
func TestL1InfoDepositSource(t *testing.T) {
	source := L1InfoDepositSource{
		L1BlockHash: common.HexToHash("0xc00e5d67c2755389aded7d8b151cbd5bcdf7ed275ad5e028b664880fc7581c77"),
		SeqNumber:   4,
	}

	actual := source.SourceHash()
	expected := "0x0586c503340591999b8b38bc9834bb16aec7d5bc00eb5587ab139c9ddab81977"

	assert.Equal(t, expected, actual.Hex())
}

// TestAfterForceIncludeSourceHash
// cast keccak $(cast concat-hex 0x0000000000000000000000000000000000000000000000000000000000000003 $(cast keccak $(cast concat-hex 0xc00e5d67c2755389aded7d8b151cbd5bcdf7ed275ad5e028b664880fc7581c77 0x0000000000000000000000000000000000000000000000000000000000000004)))
// # 0x0d165c391384b29c29f655e3f32315755b8c1e4c1147d1824d1243420dda5ec3
func TestAfterForceIncludeSource(t *testing.T) {
	source := AfterForceIncludeSource{
		L1BlockHash: common.HexToHash("0xc00e5d67c2755389aded7d8b151cbd5bcdf7ed275ad5e028b664880fc7581c77"),
		SeqNumber:   4,
	}

	actual := source.SourceHash()
	expected := "0x0d165c391384b29c29f655e3f32315755b8c1e4c1147d1824d1243420dda5ec3"

	assert.Equal(t, expected, actual.Hex())
}

// TestInvalidatedBlockSource
// cast keccak $(cast concat-hex 0x0000000000000000000000000000000000000000000000000000000000000004 0x1c8aff950685c2ed4bc3174f3472287b56d9517b9c948127319a09a7a36deac8)
// # 0x4a62bcfa03cf778234ae28fa39c9e0748f11099997b19fef9bb3fffc154fe456
func TestInvalidatedBlockSource(t *testing.T) {
	source := InvalidatedBlockSource{
		OutputRoot: common.HexToHash("0x1c8aff950685c2ed4bc3174f3472287b56d9517b9c948127319a09a7a36deac8"),
	}

	actual := source.SourceHash()
	expected := "0x4a62bcfa03cf778234ae28fa39c9e0748f11099997b19fef9bb3fffc154fe456"

	assert.Equal(t, expected, actual.Hex())
}
