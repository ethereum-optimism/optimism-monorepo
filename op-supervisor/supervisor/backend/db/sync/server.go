package sync

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// Server handles sync requests
type Server struct {
	config      Config
	validChains map[eth.ChainID]struct{}
}

// NewServer creates a new Server with the given config.
func NewServer(config Config, chains []eth.ChainID) (*Server, error) {
	// Convert root to absolute path for security
	root, err := filepath.Abs(config.DataDir)
	if err != nil {
		return nil, fmt.Errorf("invalid root directory: %w", err)
	}

	// Verify root directory exists and is actually a directory
	rootInfo, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("cannot access root directory: %w", err)
	}
	if !rootInfo.IsDir() {
		return nil, fmt.Errorf("root path is not a directory: %s", root)
	}

	// Build map of valid chains for efficient lookup
	validChains := make(map[eth.ChainID]struct{}, len(chains))
	for _, chain := range chains {
		validChains[chain] = struct{}{}
	}

	return &Server{
		config:      config,
		validChains: validChains,
	}, nil
}

func parsePath(path string) (eth.ChainID, string, error) {
	var (
		chainID   eth.ChainID
		fileAlias string
	)

	// Trim leading and trailing slashes and split into segments
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) < 2 {
		return chainID, fileAlias, fmt.Errorf("invalid path: %s", path)
	}
	chainIDStr := segments[len(segments)-2]
	fileAlias = segments[len(segments)-1]

	if err := chainID.UnmarshalText([]byte(chainIDStr)); err != nil {
		return chainID, fileAlias, fmt.Errorf("invalid chainID: %w", err)
	}

	return chainID, fileAlias, nil
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse and validate the path
	chainID, dbName, err := parsePath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, ok := s.validChains[chainID]; !ok {
		http.Error(w, "unsupported chainID", http.StatusNotFound)
		return
	}

	// Get the path to the file based on the database name
	db := Database(dbName)
	fileName, exists := Databases[db]
	if !exists {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	filePath := filepath.Join(s.config.DataDir, chainID.String(), fileName)

	// Open the file for reading
	file, err := os.Open(filePath)
	if err != nil {
		s.logError("error opening file", err, dbName)
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	defer func(file *os.File) {
		if file.Close() != nil {
			s.logError("error closing file", err, dbName)
		}
	}(file)

	// Get file info and set the headers
	fileInfo, err := file.Stat()
	if err != nil {
		s.logError("error stating file", err, dbName)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	w.Header().Set("Last-Modified", fileInfo.ModTime().UTC().Format(http.TimeFormat))

	// Handle HEAD requests by returning and GET requests by streaming the file
	switch r.Method {
	case http.MethodHead:
		return
	case http.MethodGet:
		// Stream the file contents, including handling range requests
		http.ServeContent(w, r, dbName, fileInfo.ModTime(), file)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// logError logs an error iff a logger is configured.
func (s *Server) logError(msg string, err error, fileName string) {
	if s.config.Logger != nil {
		s.config.Logger.Error(msg, "error", err, "file", fileName)
	}
}
