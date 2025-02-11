package artifacts

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"
)

var ErrUnsupportedArtifactsScheme = errors.New("unsupported artifacts URL scheme")

type Downloader interface {
	Download(ctx context.Context, url string, progress DownloadProgressor) (string, error)
}

type Extractor interface {
	Extract(src string, dest string) (string, error)
}

func Download(ctx context.Context, loc *Locator, progressor DownloadProgressor) (foundry.StatDirFs, error) {
	if progressor == nil {
		progressor = NoopProgressor()
	}

	var u *url.URL
	var err error
	var checker integrityChecker
	if loc.IsTag() {
		u, err = standard.ArtifactsURLForTag(loc.Tag)
		if err != nil {
			return nil, fmt.Errorf("failed to get standard artifacts URL for tag %s: %w", loc.Tag, err)
		}

		hash, err := standard.ArtifactsHashForTag(loc.Tag)
		if err != nil {
			return nil, fmt.Errorf("failed to get standard artifacts hash for tag %s: %w", loc.Tag, err)
		}

		checker = &hashIntegrityChecker{hash: hash}
	} else {
		u = loc.URL
		checker = new(noopIntegrityChecker)
	}

	var artifactsFS fs.FS
	switch u.Scheme {
	case "http", "https":
		artifactsFS, err = downloadHTTP(ctx, u, progressor, checker)
		if err != nil {
			return nil, fmt.Errorf("failed to download artifacts: %w", err)
		}
	case "file":
		artifactsFS = os.DirFS(u.Path)
	default:
		return nil, ErrUnsupportedArtifactsScheme
	}
	return artifactsFS.(foundry.StatDirFs), nil
}

func downloadHTTP(ctx context.Context, u *url.URL, progressor DownloadProgressor, checker integrityChecker) (fs.FS, error) {
	cacher := &CachingDownloader{
		d: new(HTTPDownloader),
	}

	tarballPath, err := cacher.Download(ctx, u.String(), progressor)
	if err != nil {
		return nil, fmt.Errorf("failed to download artifacts: %w", err)
	}
	tmpDir, err := os.MkdirTemp("", "op-deployer-artifacts-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	extractor := &TarballExtractor{
		checker: checker,
	}
	if err := extractor.Extract(tarballPath, tmpDir); err != nil {
		return nil, fmt.Errorf("failed to extract tarball: %w", err)
	}
	return os.DirFS(path.Join(tmpDir, "forge-artifacts")), nil
}

type HTTPDownloader struct{}

func (d *HTTPDownloader) Download(ctx context.Context, url string, progress DownloadProgressor) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download artifacts: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download artifacts: invalid status code %s", res.Status)
	}
	defer res.Body.Close()

	tmpFile, err := os.CreateTemp("", "op-deployer-artifacts-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}

	pr := &progressReader{
		r:        res.Body,
		progress: progress,
		total:    res.ContentLength,
	}
	if _, err := io.Copy(tmpFile, pr); err != nil {
		return "", fmt.Errorf("failed to write to temporary file: %w", err)
	}

	return tmpFile.Name(), nil
}

type CachingDownloader struct {
	d   Downloader
	mtx sync.Mutex
}

func (d *CachingDownloader) Download(ctx context.Context, url string, progress DownloadProgressor) (string, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	cachePath := fmt.Sprintf("/tmp/op-deployer-cache/%x.tgz", sha256.Sum256([]byte(url)))
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}
	tmpPath, err := d.d.Download(ctx, url, progress)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	if err := os.MkdirAll("/tmp/op-deployer-cache", 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}
	if err := os.Rename(tmpPath, cachePath); err != nil {
		return "", fmt.Errorf("failed to move downloaded file to cache: %w", err)
	}
	return cachePath, nil
}

type TarballExtractor struct {
	checker integrityChecker
}

func (e *TarballExtractor) Extract(src string, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read tarball: %w", err)
	}

	if err := e.checker.CheckIntegrity(data); err != nil {
		return fmt.Errorf("integrity check failed: %w", err)
	}

	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	if err := untar(dest, tr); err != nil {
		return fmt.Errorf("failed to untar: %w", err)
	}

	return nil
}

func untar(dir string, tr *tar.Reader) error {
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		cleanedName := path.Clean(hdr.Name)
		if strings.Contains(cleanedName, "..") {
			return fmt.Errorf("invalid file path: %s", hdr.Name)
		}
		dst := path.Join(dir, cleanedName)
		if hdr.FileInfo().IsDir() {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		f, err := os.Create(dst)
		buf := bufio.NewWriter(f)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		if _, err := io.Copy(buf, tr); err != nil {
			_ = f.Close()
			return fmt.Errorf("failed to write file: %w", err)
		}
		if err := buf.Flush(); err != nil {
			return fmt.Errorf("failed to flush buffer: %w", err)
		}
		_ = f.Close()
	}
}
