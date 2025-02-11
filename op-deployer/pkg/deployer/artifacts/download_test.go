package artifacts

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/minio/sha256-simd"

	"github.com/ethereum/go-ethereum/common"

	"github.com/stretchr/testify/require"
)

func TestDownloadArtifacts_MockArtifacts(t *testing.T) {
	f, err := os.OpenFile("testdata/artifacts.tar.gz", os.O_RDONLY, 0o644)
	require.NoError(t, err)
	defer f.Close()

	var callCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := io.Copy(w, f)
		require.NoError(t, err)
		// Seek to beginning of file for next request
		_, err = f.Seek(0, 0)
		require.NoError(t, err)
		atomic.AddInt32(&callCount, 1)
	}))
	defer ts.Close()

	ctx := context.Background()
	artifactsURL, err := url.Parse(ts.URL)
	require.NoError(t, err)
	loc := &Locator{
		URL: artifactsURL,
	}

	t.Run("success", func(t *testing.T) {
		fs, err := Download(ctx, loc, nil)
		require.NoError(t, err)
		require.NotNil(t, fs)

		info, err := fs.Stat("WETH98.sol/WETH98.json")
		require.NoError(t, err)
		require.Greater(t, info.Size(), int64(0))
	})

	t.Run("bad integrity", func(t *testing.T) {
		_, err := downloadHTTP(ctx, loc.URL, nil, &hashIntegrityChecker{
			hash: common.Hash{'B', 'A', 'D'},
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "integrity check failed")
	})

	correctIntegrity := &hashIntegrityChecker{
		hash: common.HexToHash("0x0f814df0c4293aaaadd468ac37e6c92f0b40fd21df848076835cb2c21d2a516f"),
	}

	t.Run("ok integrity", func(t *testing.T) {
		_, err := downloadHTTP(ctx, loc.URL, nil, correctIntegrity)
		require.NoError(t, err)
	})

	t.Run("caching works", func(t *testing.T) {
		u, err := url.Parse(loc.URL.String())
		require.NoError(t, err)
		u.Path = fmt.Sprintf("/different-path-%d", time.Now().UnixNano())

		startCalls := atomic.LoadInt32(&callCount)
		_, err = downloadHTTP(ctx, u, nil, correctIntegrity)
		require.NoError(t, err)
		startCalls++
		require.Equal(t, startCalls, atomic.LoadInt32(&callCount))

		t.Cleanup(func() {
			require.NoError(t, os.Remove(
				fmt.Sprintf("/tmp/op-deployer-cache/%x.tgz", sha256.Sum256([]byte(u.String()))),
			))
		})

		_, err = downloadHTTP(ctx, u, nil, correctIntegrity)
		require.NoError(t, err)
		require.Equal(t, startCalls, atomic.LoadInt32(&callCount))
	})

	t.Run("caching validates integrity", func(t *testing.T) {
		u, err := url.Parse(loc.URL.String())
		require.NoError(t, err)
		u.Path = fmt.Sprintf("/different-path-%d", time.Now().UnixNano())

		_, err = downloadHTTP(ctx, u, nil, correctIntegrity)
		require.NoError(t, err)

		cacheFile := fmt.Sprintf("/tmp/op-deployer-cache/%x.tgz", sha256.Sum256([]byte(u.String())))
		t.Cleanup(func() {
			require.NoError(t, os.Remove(cacheFile))
		})

		cacheF, err := os.OpenFile(cacheFile, os.O_RDWR, 0o644)
		require.NoError(t, err)
		_, err = cacheF.Write([]byte("bad data"))
		require.NoError(t, err)
		require.NoError(t, cacheF.Close())

		_, err = downloadHTTP(ctx, u, nil, correctIntegrity)
		require.ErrorContains(t, err, "integrity check failed")
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
			_, err := Download(context.Background(), loc, nil)
			require.NoError(t, err)
		})
	}
}
