package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"sync"
)

type server struct {
	appRoot       string
	configsDir    string
	buildDir      string
	buildCmd      string
	port          int
	lastBuildHash string
	buildMutex    sync.Mutex
	proofFS       *proofFileSystem
	builder       *Builder
}

func createServer() *server {
	srv := &server{
		appRoot:    *flagAppRoot,
		configsDir: *flagConfigsDir,
		buildDir:   *flagBuildDir,
		buildCmd:   *flagBuildCmd,
		port:       *flagPort,
	}

	// Initialize the proof filesystem
	proofsDir := filepath.Join(srv.appRoot, srv.buildDir, "bin")
	srv.proofFS = newProofFileSystem(proofsDir)

	// Initialize the builder
	srv.builder = NewBuilder(srv.appRoot, srv.configsDir, srv.buildDir, srv.buildCmd)

	return srv
}

func (s *server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("Received upload request from %s", r.RemoteAddr)

	files, currentHash, err := s.processMultipartForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.buildMutex.Lock()
	defer s.buildMutex.Unlock()

	// Check if we need to rebuild
	if currentHash == s.lastBuildHash {
		log.Printf("Hash matches last build, skipping")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Files unchanged, skipping build")
		return
	}

	log.Printf("Hash differs from last build (%s), proceeding with build", s.lastBuildHash)

	// Save the files using the builder
	if err := s.builder.SaveUploadedFiles(files); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Execute the build
	out, err := s.builder.ExecuteBuild()
	if err != nil {
		http.Error(w, fmt.Sprintf("%v\nOutput: %s", err, out), http.StatusInternalServerError)
		return
	}

	// After successful build, scan for new proof files
	if err := s.proofFS.scanProofFiles(); err != nil {
		log.Printf("Warning: failed to scan proof files: %v", err)
	}

	// Update the last successful build hash
	s.lastBuildHash = currentHash

	// Redirect to the proofs endpoint
	http.Redirect(w, r, "/proofs", http.StatusSeeOther)
}

func (s *server) processMultipartForm(r *http.Request) ([]*multipart.FileHeader, string, error) {
	// Parse the multipart form
	if err := r.ParseMultipartForm(1 << 30); err != nil { // 1GB max memory
		return nil, "", fmt.Errorf("failed to parse form: %w", err)
	}

	// Get uploaded files
	files := r.MultipartForm.File["files[]"]
	if len(files) == 0 {
		return nil, "", fmt.Errorf("no files uploaded")
	}

	log.Printf("Processing %d files:", len(files))
	for _, fileHeader := range files {
		log.Printf("  - %s (size: %d bytes)", fileHeader.Filename, fileHeader.Size)
	}

	// Calculate hash of all files
	hash, err := s.calculateFilesHash(files)
	if err != nil {
		return nil, "", fmt.Errorf("failed to calculate files hash: %w", err)
	}

	return files, hash, nil
}

func (s *server) calculateFilesHash(files []*multipart.FileHeader) (string, error) {
	hasher := sha256.New()
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			return "", fmt.Errorf("failed to open file: %w", err)
		}
		if _, err := io.Copy(hasher, file); err != nil {
			file.Close()
			return "", fmt.Errorf("failed to hash file: %w", err)
		}
		file.Close()
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
