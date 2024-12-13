package artifacts

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/stretchr/testify/require"
)

func TestDownloadArtifacts_MockArtifacts(t *testing.T) {
	f, err := os.OpenFile("testdata/artifacts.tar.gz", os.O_RDONLY, 0o644)
	require.NoError(t, err)
	defer f.Close()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := io.Copy(w, f)
		require.NoError(t, err)
		// Seek to beginning of file for next request
		_, err = f.Seek(0, 0)
		require.NoError(t, err)
	}))
	defer ts.Close()

	ctx := context.Background()
	artifactsURL, err := url.Parse(ts.URL)
	require.NoError(t, err)
	loc := &Locator{
		URL: artifactsURL,
	}

	t.Run("success", func(t *testing.T) {
		fs, cleanup, err := Download(ctx, loc, nil)
		require.NoError(t, err)
		require.NotNil(t, fs)
		defer func() {
			require.NoError(t, cleanup())
		}()

		info, err := fs.Stat("WETH98.sol/WETH98.json")
		require.NoError(t, err)
		require.Greater(t, info.Size(), int64(0))
	})

	t.Run("bad integrity", func(t *testing.T) {
		_, _, err := downloadURL(ctx, loc.URL, nil, &hashIntegrityChecker{
			hash: common.Hash{'B', 'A', 'D'},
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "integrity check failed")
	})

	t.Run("ok integrity", func(t *testing.T) {
		_, _, err := downloadURL(ctx, loc.URL, nil, &hashIntegrityChecker{
			hash: common.HexToHash("0x0f814df0c4293aaaadd468ac37e6c92f0b40fd21df848076835cb2c21d2a516f"),
		})
		require.NoError(t, err)
	})
}

func TestDownloadArtifacts_TaggedVersions(t *testing.T) {
	tags := []string{
		"op-contracts/v1.6.0",
		"op-contracts/v1.7.0-beta.1+l2-contracts",
	}
	for _, tag := range tags {
		t.Run(tag, func(t *testing.T) {
			t.Parallel()

			loc := MustNewLocatorFromTag(tag)
			_, cleanup, err := Download(context.Background(), loc, nil)
			t.Cleanup(func() {
				require.NoError(t, cleanup())
			})
			require.NoError(t, err)
		})
	}
}
