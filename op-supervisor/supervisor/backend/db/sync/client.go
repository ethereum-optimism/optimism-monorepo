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

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/retry"
)

var (
	maxRetries    = 25
	retryStrategy = &retry.ExponentialStrategy{
		Min:       1 * time.Second,
		Max:       30 * time.Second,
		MaxJitter: 250 * time.Millisecond,
	}
)

// Client handles downloading files from a sync server.
type Client struct {
	config     Config
	baseURL    string
	httpClient *client.BasicHTTPClient
}

// NewClient creates a new Client with the given config and server URL.
func NewClient(config Config, serverURL string) (*Client, error) {
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

	// Create the HTTP client
	httpClient := client.NewBasicHTTPClient(serverURL, config.Logger)

	return &Client{
		config:     config,
		baseURL:    serverURL,
		httpClient: httpClient,
	}, nil
}

// SyncAll syncs all known databases for the given chains.
func (c *Client) SyncAll(ctx context.Context, chains []eth.ChainID, resume bool) error {
	for _, chain := range chains {
		for fileAlias := range Databases {
			if err := c.SyncDatabase(ctx, chain, fileAlias, resume); err != nil {
				return fmt.Errorf("failed to sync %s for chain %s: %w", fileAlias, chain, err)
			}
		}
	}
	return nil
}

// SyncDatabase downloads the named file from the server.
// If the local file exists, it will attempt to resume the download if resume is true.
func (c *Client) SyncDatabase(ctx context.Context, chainID eth.ChainID, database Database, resume bool) error {
	// Validate file alias
	filePath, exists := Databases[database]
	if !exists {
		return fmt.Errorf("unknown file alias: %s", database)
	}

	// Ensure the chain directory exists
	chainDir := filepath.Join(c.config.DataDir, chainID.String())
	if err := os.MkdirAll(chainDir, 0755); err != nil {
		return fmt.Errorf("failed to create chain directory: %w", err)
	}

	// Ensure the database file exists and get initial size
	filePath = filepath.Join(chainDir, filePath)
	var initialSize int64
	if stat, err := os.Stat(filePath); err == nil {
		initialSize = stat.Size()
	}

	// If we have data already and don't want to resume then stop now
	if initialSize > 0 && !resume {
		return nil
	}

	// Attempt to sync the file and retry until successful
	err := retry.Do0(ctx, maxRetries, retryStrategy, func() error {
		err := c.attemptSync(ctx, chainID, database, filePath, initialSize)
		if err != nil {
			c.logError("sync attempt failed", err, database)
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}
	return nil
}

// attemptSync makes a single attempt to sync the file
func (c *Client) attemptSync(ctx context.Context, chainID eth.ChainID, database Database, absPath string, initialSize int64) error {
	// First do a HEAD request to get the file size
	path := c.buildURLPath(chainID, database)
	resp, err := c.httpClient.Get(ctx, path, nil, http.Header{"X-HTTP-Method-Override": []string{"HEAD"}})
	if err != nil {
		return fmt.Errorf("HEAD request failed: %w", err)
	}
	if err := resp.Body.Close(); err != nil {
		return fmt.Errorf("HEAD request body failed to close: %w", err)
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
	headers := make(http.Header)
	if initialSize > 0 {
		headers.Set("Range", fmt.Sprintf("bytes=%d-", initialSize))
	}
	resp, err = c.httpClient.Get(ctx, path, nil, headers)
	if err != nil {
		return fmt.Errorf("GET request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logError("failed to close response body", err, database)
		}
	}()
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
			c.logError("failed to close output file", err, database)
		}
	}(f)

	// Copy the data to disk
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to copy data: %s", database)
	}

	return nil
}

// buildURLPath creates the URL path for a given database download request
func (c *Client) buildURLPath(chainID eth.ChainID, database Database) string {
	return fmt.Sprintf("dbsync/%s/%s", chainID.String(), database)
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
func (c *Client) logError(msg string, err error, database Database) {
	if c.config.Logger != nil {
		c.config.Logger.Error(msg,
			"error", err,
			"database", database,
		)
	}
}
