package prestates

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/vm"
	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum/go-ethereum/common"
)

var (
	ErrPrestateUnavailable = errors.New("prestate unavailable")

	// supportedFileTypes lists, in preferred order, the prestate file types to attempt to download
	supportedFileTypes = []string{".bin.gz", ".json.gz", ".json"}
)

type MultiPrestateProvider struct {
	baseUrl        *url.URL
	dataDir        string
	stateConverter vm.StateConverter
}

func NewMultiPrestateProvider(baseUrl *url.URL, dataDir string, stateConverter vm.StateConverter) *MultiPrestateProvider {
	return &MultiPrestateProvider{
		baseUrl:        baseUrl,
		dataDir:        dataDir,
		stateConverter: stateConverter,
	}
}

func (m *MultiPrestateProvider) PrestatePath(ctx context.Context, hash common.Hash) (string, error) {
	// First try to find a previously downloaded prestate
	for _, fileType := range supportedFileTypes {
		path := filepath.Join(m.dataDir, hash.Hex()+fileType)
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			continue // File doesn't exist, try the next file type
		} else if err != nil {
			return "", fmt.Errorf("error checking for existing prestate %v in file %v: %w", hash, path, err)
		}
		return path, nil // Found an existing file so use it
	}

	// Didn't find any available files, try to download one
	var combinedErr error // Keep a track of each download attempt so we can report them if none work
	for _, fileType := range supportedFileTypes {
		path := filepath.Join(m.dataDir, hash.Hex()+fileType)
		if err := m.fetchPrestate(ctx, hash, fileType, path); errors.Is(err, ErrPrestateUnavailable) {
			combinedErr = errors.Join(combinedErr, err)
			continue // Didn't find prestate in this format, try the next
		} else if err != nil {
			return "", fmt.Errorf("error downloading prestate %v to file %v: %w", hash, path, err)
		}
		return path, nil // Successfully downloaded a prestate so use it
	}
	return "", errors.Join(ErrPrestateUnavailable, combinedErr)
}

func (m *MultiPrestateProvider) fetchPrestate(ctx context.Context, hash common.Hash, fileType string, dest string) error {
	if err := os.MkdirAll(m.dataDir, 0755); err != nil {
		return fmt.Errorf("error creating prestate dir: %w", err)
	}
	prestateUrl := m.baseUrl.JoinPath(hash.Hex() + fileType)
	tCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	in, err := m.fetch(tCtx, prestateUrl)
	if err != nil {
		return err
	}
	defer in.Close()
	tmpFile := dest + ".tmp" + fileType // Preserve the file type extension so state decoding is applied correctly
	out, err := ioutil.NewAtomicWriter(tmpFile, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open atomic writer for %v: %w", dest, err)
	}
	defer func() {
		// If errors occur, try to clean up without renaming the file into its final destination as Close() would do
		_ = out.Abort()
	}()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("failed to write file %v: %w", dest, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("failed to close file %v: %w", dest, err)
	}
	// Verify the prestate actually matches the expected hash before moving it into the final destination
	proof, _, _, err := m.stateConverter.ConvertStateToProof(ctx, tmpFile)
	if err != nil || proof.ClaimValue != hash {
		// Treat invalid prestates as unavailable. Often servers return a 404 page with 200 status code
		_ = os.Remove(tmpFile) // Best effort attempt to clean up the temporary file
		return fmt.Errorf("invalid prestate from url: %v, ignoring: %w", prestateUrl, errors.Join(ErrPrestateUnavailable, err))
	}
	if err := os.Rename(tmpFile, dest); err != nil {
		return fmt.Errorf("failed to move temp file to final destination: %w", err)
	}
	return nil
}

func (m *MultiPrestateProvider) fetch(ctx context.Context, prestateUrl *url.URL) (io.ReadCloser, error) {
	if prestateUrl.Scheme == "file" {
		path := prestateUrl.Path
		in, err := os.OpenFile(path, os.O_RDONLY, 0o644)
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w unavailable from path %v", ErrPrestateUnavailable, path)
		}
		return in, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", prestateUrl.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create prestate request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch prestate from %v: %w", prestateUrl, err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close() // Not returning the body so make a best effort to close it now.
		return nil, fmt.Errorf("%w from url %v: status %v", ErrPrestateUnavailable, prestateUrl, resp.StatusCode)
	}
	return resp.Body, nil
}
