package prestates

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPreferSinglePrestate(t *testing.T) {
	uri, err := url.Parse("http://localhost")
	require.NoError(t, err)
	source := NewPrestateSource(uri, "/tmp/path.json", t.TempDir(), nil)
	require.IsType(t, &SinglePrestateSource{}, source)
}
