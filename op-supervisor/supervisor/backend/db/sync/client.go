package sync

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

const (
	InitRetryDelay = 1 * time.Second
	MaxRetryDelay  = 30 * time.Minute
)

// Client handles downloading files from a sync server.
type Client struct {
	config  Config
	baseURL string

	httpClient *http.Client
	retryDelay time.Duration
}

// NewClient creates a new Client with the given config and server URL.
func NewClient(config Config, serverURL string, httpClient *http.Client) (*Client, error) {
	// Verify root directory exists and is actually a directory
	root, err := filepath.Abs(config.DataDir)
	if err != nil {
		return nil, fmt.Errorf("invalid root directory: %w", err)
	}
	rootInfo, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("cannot access root directory: %w", err)
	}
	if !rootInfo.IsDir() {
		return nil, fmt.Errorf("root path is not a directory: %s", root)
	}

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		config:     config,
		baseURL:    serverURL,
		httpClient: httpClient,
		retryDelay: InitRetryDelay,
	}, nil
}

// SyncFile downloads the named file from the server.
// If the local file exists, it will attempt to resume the download.
func (c *Client) SyncFile(ctx context.Context, chainID types.ChainID, fileAlias string, resume bool) error {
	// Validate file alias
	filePath, exists := FileAliases[fileAlias]
	if !exists {
		return fmt.Errorf("unknown file alias: %s", fileAlias)
	}

	// Ensure the chain directory exists
	chainDir := filepath.Join(c.config.DataDir, chainID.String())
	if err := os.MkdirAll(chainDir, 0755); err != nil {
		return fmt.Errorf("failed to create chain directory: %w", err)
	}

	filePath = filepath.Join(chainDir, filePath)

	// Get initial file size
	var initialSize int64
	if stat, err := os.Stat(filePath); err == nil {
		initialSize = stat.Size()
	}

	// If we have some data already and don't want to resume then stop now
	if initialSize > 0 && !resume {
		return nil
	}

	// Keep track of the current retry delay
	currentDelay := c.retryDelay

	for {
		// Try to sync and return if successful
		err := c.attemptSync(ctx, chainID, fileAlias, filePath, initialSize)
		if err == nil {
			return nil
		}

		// Check if context was canceled
		if ctx.Err() != nil {
			return ctx.Err()
		}
		c.logError("sync attempt failed", err, fileAlias)

		// Wait before retrying
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(currentDelay):
			// Double the delay for next time, up to max
			currentDelay *= 2
			if currentDelay > MaxRetryDelay {
				currentDelay = MaxRetryDelay
			}
		}
	}
}

// attemptSync makes a single attempt to sync the file
func (c *Client) attemptSync(ctx context.Context, chainID types.ChainID, name, absPath string, initialSize int64) error {
	// First do a HEAD request to get the file size
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.buildURL(chainID, name), nil)
	if err != nil {
		return fmt.Errorf("failed to create HEAD request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HEAD request failed: %w", err)
	}
	if err := resp.Body.Close(); err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HEAD request failed with status %d", resp.StatusCode)
	}
	totalSize, err := parseContentLength(resp.Header)
	if err != nil {
		return fmt.Errorf("invalid Content-Length: %w", err)
	}

	// If we already have the whole file, we're done
	if initialSize == totalSize {
		return nil
	}

	// Create the GET request
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, c.buildURL(chainID, name), nil)
	if err != nil {
		return fmt.Errorf("failed to create GET request: %w", err)
	}

	// If we have partial file, try to resume
	if initialSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", initialSize))
	}

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET request failed: %w", err)
	}
	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			c.logError("failed to close response body", err, name)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("GET request failed with status %d", resp.StatusCode)
	}

	// Open the output file in the appropriate mode
	flag := os.O_CREATE | os.O_WRONLY
	if resp.StatusCode == http.StatusPartialContent {
		flag |= os.O_APPEND
	}

	f, err := os.OpenFile(absPath, flag, 0644)
	if err != nil {
		return fmt.Errorf("failed to open output file: %w", err)
	}
	defer func(f *os.File) {
		if err := f.Close(); err != nil {
			c.logError("failed to close output file", err, name)
		}
	}(f)

	// Copy the data to disk
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}

	return nil
}

// buildURL creates the full URL for a file request
func (c *Client) buildURL(chainID types.ChainID, fileAlias string) string {
	return fmt.Sprintf("%s/dbsync/%s/%s", c.baseURL, chainID.String(), fileAlias)
}

// parseContentLength parses the Content-Length header
func parseContentLength(h http.Header) (int64, error) {
	v := h.Get("Content-Length")
	if v == "" {
		return 0, fmt.Errorf("missing Content-Length header")
	}
	return strconv.ParseInt(v, 10, 64)
}

// logError logs an error if a logger is configured
func (c *Client) logError(msg string, err error, fileName string) {
	if c.config.Logger != nil {
		c.config.Logger.Error(msg,
			"error", err,
			"file", fileName,
		)
	}
}
