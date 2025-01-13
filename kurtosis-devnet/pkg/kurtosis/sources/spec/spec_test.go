package spec

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSpec(t *testing.T) {
	yamlContent := `
optimism_package:
  chains:
    - participants:
        - el_type: op-geth
      network_params:
        name: op-rollup-one
        network_id: "3151909"
      additional_services:
        - blockscout
    - participants:
        - el_type: op-geth
      network_params:
        name: op-rollup-two
        network_id: "3151910"
      additional_services:
        - blockscout
ethereum_package:
  participants:
    - el_type: geth
    - el_type: reth
  network_params:
    preset: minimal
    genesis_delay: 5
`

	result, err := NewSpec().ExtractData(strings.NewReader(yamlContent))
	require.NoError(t, err)

	expectedChains := []ChainSpec{
		{
			Name:      "op-rollup-one",
			NetworkID: "3151909",
		},
		{
			Name:      "op-rollup-two",
			NetworkID: "3151910",
		},
	}

	require.Len(t, result.Chains, len(expectedChains))

	for i, expected := range expectedChains {
		actual := result.Chains[i]
		require.Equal(t, expected.Name, actual.Name, "Chain %d: name mismatch", i)
		require.Equal(t, expected.NetworkID, actual.NetworkID, "Chain %d: network ID mismatch", i)
	}
}

func TestParseSpecErrors(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name:    "empty yaml",
			yaml:    "",
			wantErr: true,
		},
		{
			name:    "invalid yaml",
			yaml:    "invalid: [yaml: content",
			wantErr: true,
		},
		{
			name: "missing network params",
			yaml: `
optimism_package:
  chains:
    - participants:
        - el_type: op-geth
      additional_services:
        - blockscout`,
		},
		{
			name: "missing chains",
			yaml: `
optimism_package:
  other_field: value`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSpec().ExtractData(strings.NewReader(tt.yaml))
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
