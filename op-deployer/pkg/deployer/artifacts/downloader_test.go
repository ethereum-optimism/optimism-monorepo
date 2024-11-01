package artifacts

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDownloadArtifacts(t *testing.T) {
	f, err := os.OpenFile("testdata/artifacts.tar.gz", os.O_RDONLY, 0o644)
	require.NoError(t, err)
	defer f.Close()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := io.Copy(w, f)
		require.NoError(t, err)
	}))
	defer ts.Close()

	ctx := context.Background()
	artifactsURL, err := url.Parse(ts.URL)
	require.NoError(t, err)
	loc := &Locator{
		URL: artifactsURL,
	}

	fs, cleanup, err := Download(ctx, loc, nil)
	require.NoError(t, err)
	require.NotNil(t, fs)
	defer func() {
		require.NoError(t, cleanup())
	}()

	info, err := fs.Stat("WETH98.sol/WETH98.json")
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0))
}
