package state

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBase64BytesMarshaling(t *testing.T) {
	tests := []struct {
		name string
		in   Base64Bytes
		out  string
	}{
		{
			name: "empty",
			in:   Base64Bytes{},
			out:  "null",
		},
		{
			name: "non-empty",
			in:   Base64Bytes{0x01, 0x02, 0x03},
			out:  `"AQID"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.in.MarshalJSON()
			require.NoError(t, err)
			require.Equal(t, tt.out, string(data))

			var b Base64Bytes
			err = b.UnmarshalJSON(data)
			require.NoError(t, err)
			require.Equal(t, tt.in, b)
		})
	}
}
