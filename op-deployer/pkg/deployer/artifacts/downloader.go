package artifacts

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
)

var ErrUnsupportedArtifactsScheme = errors.New("unsupported artifacts URL scheme")

type DownloadProgressor func(current, total int64)

func NoopDownloadProgressor(current, total int64) {}

type CleanupFunc func() error

var noopCleanup = func() error { return nil }

func LogProgressor(lgr log.Logger) DownloadProgressor {
	return func(curr, total int64) {
		lgr.Info("artifacts download progress", "current", curr, "total", total)
	}
}

func Download(ctx context.Context, loc *Locator, progress DownloadProgressor) (foundry.StatDirFs, CleanupFunc, error) {
	var u *url.URL
	var err error
	if loc.IsTag() {
		u, err = standard.ArtifactsURLForTag(loc.Tag)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get standard artifacts URL for tag %s: %w", loc.Tag, err)
		}
	} else {
		u = loc.URL
	}

	switch u.Scheme {
	case "http", "https":
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to download artifacts: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, nil, fmt.Errorf("failed to download artifacts: invalid status code %s", resp.Status)
		}

		tmpDir, err := os.MkdirTemp("", "op-deployer-artifacts-*")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create temp dir: %w", err)
		}

		pr := &progressReader{
			r:        resp.Body,
			progress: progress,
			total:    resp.ContentLength,
		}

		gr, err := gzip.NewReader(pr)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gr.Close()

		tr := tar.NewReader(gr)
		if err := untar(tmpDir, tr); err != nil {
			return nil, nil, fmt.Errorf("failed to untar: %w", err)
		}

		fs := os.DirFS(path.Join(tmpDir, "forge-artifacts"))
		cleanup := func() error {
			return os.RemoveAll(tmpDir)
		}
		return fs.(foundry.StatDirFs), cleanup, nil
	case "file":
		fs := os.DirFS(u.Path)
		return fs.(foundry.StatDirFs), noopCleanup, nil
	default:
		return nil, nil, ErrUnsupportedArtifactsScheme
	}
}

type progressReader struct {
	r         io.Reader
	progress  DownloadProgressor
	curr      int64
	total     int64
	lastPrint time.Time
}

func (pr *progressReader) Read(p []byte) (int, error) {

	n, err := pr.r.Read(p)
	pr.curr += int64(n)
	if pr.progress != nil && time.Since(pr.lastPrint) > 1*time.Second {
		pr.progress(pr.curr, pr.total)
		pr.lastPrint = time.Now()
	}
	return n, err
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
