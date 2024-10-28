package prestates

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestDownloadPrestateHTTP(t *testing.T) {
	for _, ext := range supportedFileTypes {
		t.Run(ext, func(t *testing.T) {
			dir := t.TempDir()
			mkContent := func(path string) []byte {
				// Large enough to be bigger than a single write buffer.
				out := make([]byte, 16192)
				copy(out, path)
				return out
			}
			server := prestateHTTPServer(ext, mkContent)
			defer server.Close()
			hash := common.Hash{0xaa}
			provider := NewMultiPrestateProvider(parseURL(t, server.URL), dir, &stubStateConverter{hash: hash})
			path, err := provider.PrestatePath(context.Background(), hash)
			require.NoError(t, err)
			in, err := os.Open(path)
			require.NoError(t, err)
			defer in.Close()
			content, err := io.ReadAll(in)
			require.NoError(t, err)
			require.Equal(t, mkContent("/"+hash.Hex()+ext), content)
		})
	}
}

func TestDownloadPrestateFile(t *testing.T) {
	for _, ext := range supportedFileTypes {
		t.Run(ext, func(t *testing.T) {
			sourceDir := t.TempDir()
			dir := t.TempDir()
			hash := common.Hash{0xaa}
			sourcePath := filepath.Join(sourceDir, hash.Hex()+ext)
			expectedContent := "/" + hash.Hex() + ext
			require.NoError(t, os.WriteFile(sourcePath, []byte(expectedContent), 0600))
			provider := NewMultiPrestateProvider(parseURL(t, "file:"+sourceDir), dir, &stubStateConverter{hash: hash})
			path, err := provider.PrestatePath(context.Background(), hash)
			require.NoError(t, err)
			in, err := os.Open(path)
			require.NoError(t, err)
			defer in.Close()
			content, err := io.ReadAll(in)
			require.NoError(t, err)
			require.Equal(t, expectedContent, string(content))
		})
	}
}

func TestCreateDirectory(t *testing.T) {
	for _, ext := range supportedFileTypes {
		t.Run(ext, func(t *testing.T) {
			dir := t.TempDir()
			dir = filepath.Join(dir, "test")
			server := prestateHTTPServer(ext, func(path string) []byte { return []byte(path) })
			defer server.Close()
			hash := common.Hash{0xaa}
			provider := NewMultiPrestateProvider(parseURL(t, server.URL), dir, &stubStateConverter{hash: hash})
			path, err := provider.PrestatePath(context.Background(), hash)
			require.NoError(t, err)
			in, err := os.Open(path)
			require.NoError(t, err)
			defer in.Close()
			content, err := io.ReadAll(in)
			require.NoError(t, err)
			require.Equal(t, "/"+hash.Hex()+ext, string(content))
		})
	}
}

func TestExistingPrestate(t *testing.T) {
	dir := t.TempDir()
	hash := common.Hash{0xaa}
	provider := NewMultiPrestateProvider(parseURL(t, "http://127.0.0.1:1"), dir, &stubStateConverter{hash: hash})
	expectedFile := filepath.Join(dir, hash.Hex()+".json.gz")
	err := ioutil.WriteCompressedBytes(expectedFile, []byte("expected content"), os.O_WRONLY|os.O_CREATE, 0o644)
	require.NoError(t, err)

	path, err := provider.PrestatePath(context.Background(), hash)
	require.NoError(t, err)
	require.Equal(t, expectedFile, path)
	in, err := ioutil.OpenDecompressed(path)
	require.NoError(t, err)
	defer in.Close()
	content, err := io.ReadAll(in)
	require.NoError(t, err)
	require.Equal(t, "expected content", string(content))
}

func TestMissingPrestate(t *testing.T) {
	dir := t.TempDir()
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		w.WriteHeader(404)
	}))
	defer server.Close()
	hash := common.Hash{0xaa}
	provider := NewMultiPrestateProvider(parseURL(t, server.URL), dir, &stubStateConverter{hash: hash})
	path, err := provider.PrestatePath(context.Background(), hash)
	require.ErrorIs(t, err, ErrPrestateUnavailable)
	_, err = os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)
	expectedRequests := []string{
		"/" + hash.Hex() + ".bin.gz",
		"/" + hash.Hex() + ".json.gz",
		"/" + hash.Hex() + ".json",
	}
	require.Equal(t, expectedRequests, requests)
}

func TestStorePrestateWithCorrectExtension(t *testing.T) {
	extensions := []string{".bin.gz", ".json.gz", ".json"}
	for _, ext := range extensions {
		ext := ext
		t.Run(ext, func(t *testing.T) {
			dir := t.TempDir()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, ext) {
					w.WriteHeader(404)
					return
				}
				_, _ = w.Write([]byte("content"))
			}))
			defer server.Close()
			hash := common.Hash{0xaa}
			provider := NewMultiPrestateProvider(parseURL(t, server.URL), dir, &stubStateConverter{hash: hash})
			path, err := provider.PrestatePath(context.Background(), hash)
			require.NoError(t, err)
			require.Truef(t, strings.HasSuffix(path, ext), "Expected path %v to have extension %v", path, ext)
			in, err := os.Open(path)
			require.NoError(t, err)
			defer in.Close()
			content, err := io.ReadAll(in)
			require.NoError(t, err)
			require.Equal(t, "content", string(content))
		})
	}
}

func TestDetectInvalidPrestate(t *testing.T) {
	dir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("content"))
	}))
	defer server.Close()
	hash := common.Hash{0xaa}
	provider := NewMultiPrestateProvider(parseURL(t, server.URL), dir, &stubStateConverter{hash: hash, err: errors.New("boom")})
	_, err := provider.PrestatePath(context.Background(), hash)
	require.ErrorIs(t, err, ErrPrestateUnavailable)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries, "should not leave any files in temp dir")
}

func TestDetectPrestateWithWrongHash(t *testing.T) {
	dir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("content"))
	}))
	defer server.Close()
	hash := common.Hash{0xaa}
	actualHash := common.Hash{0xbb}
	provider := NewMultiPrestateProvider(parseURL(t, server.URL), dir, &stubStateConverter{hash: actualHash})
	_, err := provider.PrestatePath(context.Background(), hash)
	require.ErrorIs(t, err, ErrPrestateUnavailable)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries, "should not leave any files in temp dir")
}

func parseURL(t *testing.T, str string) *url.URL {
	parsed, err := url.Parse(str)
	require.NoError(t, err)
	return parsed
}

func prestateHTTPServer(ext string, content func(path string) []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ext) {
			_, _ = w.Write(content(r.URL.Path))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

type stubStateConverter struct {
	err  error
	hash common.Hash
}

func (s *stubStateConverter) ConvertStateToProof(_ context.Context, path string) (*utils.ProofData, uint64, bool, error) {
	// Return an error if we're given the wrong path
	if _, err := os.Stat(path); err != nil {
		return nil, 0, false, err
	}
	return &utils.ProofData{ClaimValue: s.hash}, 0, false, s.err
}
