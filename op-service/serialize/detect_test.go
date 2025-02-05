package serialize

import (
	"io"
	"path/filepath"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"
	"github.com/stretchr/testify/require"
)

func TestRoundtrip(t *testing.T) {
	tests := []struct {
		filename   string
		expectJSON bool
		expectGzip bool
	}{
		{filename: "test.json", expectJSON: true, expectGzip: false},
		{filename: "test.json.gz", expectJSON: true, expectGzip: true},
		{filename: "test.foo", expectJSON: true, expectGzip: false},
		{filename: "test.foo.gz", expectJSON: true, expectGzip: true},
		{filename: "test.bin", expectJSON: false, expectGzip: false},
		{filename: "test.bin.gz", expectJSON: false, expectGzip: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.filename, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), test.filename)

			data := &serializableTestData{A: []byte{0xde, 0xad}, B: 3}
			err := Write[*serializableTestData](path, data, 0644)
			require.NoError(t, err)

			hasGzip, err := hasGzipHeader(path)
			require.NoError(t, err)
			require.Equal(t, test.expectGzip, hasGzip)

			decompressed, err := ioutil.OpenDecompressed(path)
			require.NoError(t, err)
			defer decompressed.Close()
			start := make([]byte, 1)
			_, err = io.ReadFull(decompressed, start)
			require.NoError(t, err)
			var load func(path string) (*serializableTestData, error)
			if test.expectJSON {
				load = jsonutil.LoadJSON[serializableTestData]
				require.Equal(t, "{", string(start))
			} else {
				load = LoadSerializedBinary[serializableTestData]
				require.NotEqual(t, "{", string(start))
			}

			result, err := load(path)
			require.NoError(t, err)
			require.EqualValues(t, data, result)
		})
	}
}
