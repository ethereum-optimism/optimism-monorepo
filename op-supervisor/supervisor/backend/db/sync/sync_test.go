package sync

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/stretchr/testify/require"
)

func TestSyncBasic(t *testing.T) {
	serverRoot, clientRoot := setupTest(t)
	chainID := eth.ChainID{1}

	// Create a test file on the server
	serverFile := filepath.Join(serverRoot, chainID.String(), DBLocalSafe.File())
	createTestFile(t, serverFile, 1024)

	// Setup server
	serverCfg := Config{
		DataDir: serverRoot,
	}
	server, err := NewServer(serverCfg, []eth.ChainID{chainID})
	require.NoError(t, err)
	ts := httptest.NewServer(server)
	defer ts.Close()

	// Setup client
	clientCfg := Config{
		DataDir: clientRoot,
	}
	client, err := NewClient(clientCfg, ts.URL)
	require.NoError(t, err)

	// Perform sync
	err = client.SyncDatabase(context.Background(), chainID, DBLocalSafe, false)
	require.NoError(t, err)
	compareFiles(t, serverFile, filepath.Join(clientRoot, chainID.String(), DBLocalSafe.File()))
}

func TestSyncResume(t *testing.T) {
	serverRoot, clientRoot := setupTest(t)
	chainID := eth.ChainID{1} // Use chain ID 1 for testing

	// Create a test file on the server and partial file on the client
	serverFile := filepath.Join(serverRoot, chainID.String(), DBLocalSafe.File())
	createTestFile(t, serverFile, 2*1024) // 2KB file

	clientFile := filepath.Join(clientRoot, chainID.String(), DBLocalSafe.File())
	createTestFile(t, clientFile, 1024) // 1KB partial file

	// Setup server and client
	serverCfg := Config{
		DataDir: serverRoot,
	}
	server, err := NewServer(serverCfg, []eth.ChainID{chainID})
	require.NoError(t, err)
	ts := httptest.NewServer(server)
	defer ts.Close()

	clientCfg := Config{
		DataDir: clientRoot,
	}
	client, err := NewClient(clientCfg, ts.URL)
	require.NoError(t, err)

	// Perform sync
	err = client.SyncDatabase(context.Background(), chainID, DBLocalSafe, true)
	require.NoError(t, err)
	compareFiles(t, serverFile, clientFile)
}

func TestSyncRetry(t *testing.T) {
	serverRoot, clientRoot := setupTest(t)
	chainID := eth.ChainID{1}

	// Create a test file
	serverFile := filepath.Join(serverRoot, chainID.String(), DBLocalSafe.File())
	createTestFile(t, serverFile, 1024)

	// Setup server with flaky handler that fails twice before succeeding
	serverCfg := Config{
		DataDir: serverRoot,
	}
	server, err := NewServer(serverCfg, []eth.ChainID{chainID})
	require.NoError(t, err)

	failureCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failureCount < 2 {
			failureCount++
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		server.ServeHTTP(w, r)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	clientCfg := Config{
		DataDir: clientRoot,
	}
	client, err := NewClient(clientCfg, ts.URL)
	require.NoError(t, err)

	// Perform sync
	err = client.SyncDatabase(context.Background(), chainID, DBLocalSafe, false)
	require.NoError(t, err)
	require.Equal(t, 2, failureCount, "expected exactly 2 failures")
	compareFiles(t, serverFile, filepath.Join(clientRoot, chainID.String(), DBLocalSafe.File()))
}

func TestSyncErrors(t *testing.T) {
	serverRoot, clientRoot := setupTest(t)
	chainID := eth.ChainID{1}

	serverCfg := Config{
		DataDir: serverRoot,
	}
	server, err := NewServer(serverCfg, []eth.ChainID{chainID})
	require.NoError(t, err)

	ts := httptest.NewServer(server)
	defer ts.Close()

	clientCfg := Config{
		DataDir: clientRoot,
	}
	client, err := NewClient(clientCfg, ts.URL)
	require.NoError(t, err)

	t.Run("NonexistentFile", func(t *testing.T) {
		err := client.SyncDatabase(context.Background(), chainID, "nonexistent", false)
		require.Error(t, err)
	})

	t.Run("CancelledContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		err := client.SyncDatabase(ctx, chainID, DBLocalSafe, false)
		require.ErrorIs(t, err, context.Canceled)
	})
}

// setupTest creates test directories and test files
func setupTest(t *testing.T) (serverRoot, clientRoot string) {
	return t.TempDir(), t.TempDir()
}

// createTestFile creates a file with given size and content
func createTestFile(t *testing.T, path string, size int64) {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(path), 0755)
	require.NoError(t, err)

	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	// Create deterministic content for testing; ASCII A-Za-z0-9
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte((i % 62) + 65)
	}

	// Write the test data over and over until the desired size is reached
	var written int64
	for written < size {
		toWrite := size - written
		if toWrite > int64(len(data)) {
			toWrite = int64(len(data))
		}
		n, err := f.Write(data[:toWrite])
		require.NoError(t, err)
		written += int64(n)
	}
}

// compareFiles verifies two files are identical
func compareFiles(t *testing.T, path1, path2 string) {
	file1, err := os.Open(path1)
	require.NoError(t, err)
	content1, err := io.ReadAll(file1)
	require.NoError(t, err)
	require.NoError(t, file1.Close())

	file2, err := os.Open(path2)
	require.NoError(t, err)
	content2, err := io.ReadAll(file2)
	require.NoError(t, err)
	require.NoError(t, file2.Close())

	require.Equal(t, content1, content2)
}
