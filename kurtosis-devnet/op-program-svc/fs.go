package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// proofFileSystem implements http.FileSystem, mapping hash-based virtual paths to actual files
type proofFileSystem struct {
	root       string
	proofFiles map[string]string // hash -> variable part mapping
	proofMutex sync.RWMutex
}

// proofFile implements http.File, representing a virtual file in our proof filesystem
type proofFile struct {
	*os.File
}

func (f *proofFile) Readdir(count int) ([]fs.FileInfo, error) {
	// For actual files, we don't support directory listing
	return nil, fmt.Errorf("not a directory")
}

// proofDir implements http.File for the root directory
type proofDir struct {
	*proofFileSystem
	pos int
}

func (d *proofDir) Close() error {
	return nil
}

func (d *proofDir) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("cannot read a directory")
}

func (d *proofDir) Seek(offset int64, whence int) (int64, error) {
	return 0, fmt.Errorf("cannot seek a directory")
}

func (d *proofDir) Readdir(count int) ([]fs.FileInfo, error) {
	d.proofMutex.RLock()
	defer d.proofMutex.RUnlock()

	// If we've already read all entries
	if d.pos >= len(d.proofFiles)*2 {
		if count <= 0 {
			return nil, nil
		}
		return nil, io.EOF
	}

	// Convert hashes to virtual file entries
	var entries []fs.FileInfo
	hashes := make([]string, 0, len(d.proofFiles))
	for hash := range d.proofFiles {
		hashes = append(hashes, hash)
	}

	start := d.pos
	end := start + count
	if count <= 0 || end > len(d.proofFiles)*2 {
		end = len(d.proofFiles) * 2
	}

	for i := start; i < end; i++ {
		hash := hashes[i/2]
		isJSON := i%2 == 0

		var name string
		if isJSON {
			name = hash + ".json"
		} else {
			name = hash + ".bin.gz"
		}

		// Create a virtual file info
		entries = append(entries, virtualFileInfo{
			name:    name,
			size:    0, // Size will be determined when actually opening the file
			mode:    0644,
			modTime: time.Now(),
			isDir:   false,
		})
	}

	d.pos = end
	return entries, nil
}

func (d *proofDir) Stat() (fs.FileInfo, error) {
	return virtualFileInfo{
		name:    ".",
		size:    0,
		mode:    0755,
		modTime: time.Now(),
		isDir:   true,
	}, nil
}

// virtualFileInfo implements fs.FileInfo for our virtual files
type virtualFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
}

func (v virtualFileInfo) Name() string       { return v.name }
func (v virtualFileInfo) Size() int64        { return v.size }
func (v virtualFileInfo) Mode() fs.FileMode  { return v.mode }
func (v virtualFileInfo) ModTime() time.Time { return v.modTime }
func (v virtualFileInfo) IsDir() bool        { return v.isDir }
func (v virtualFileInfo) Sys() interface{}   { return nil }

func newProofFileSystem(root string) *proofFileSystem {
	return &proofFileSystem{
		root:       root,
		proofFiles: make(map[string]string),
	}
}

func (fs *proofFileSystem) Open(name string) (http.File, error) {
	if name == "/" || name == "" {
		return &proofDir{proofFileSystem: fs}, nil
	}

	// Clean the path and remove leading slash
	name = strings.TrimPrefix(filepath.Clean(name), "/")

	fs.proofMutex.RLock()
	defer fs.proofMutex.RUnlock()

	var targetFile string
	if strings.HasSuffix(name, ".json") {
		hash := strings.TrimSuffix(name, ".json")
		if variablePart, ok := fs.proofFiles[hash]; ok {
			targetFile = fmt.Sprintf("prestate-proof%s.json", variablePart)
		}
	} else if strings.HasSuffix(name, ".bin.gz") {
		hash := strings.TrimSuffix(name, ".bin.gz")
		if variablePart, ok := fs.proofFiles[hash]; ok {
			targetFile = fmt.Sprintf("prestate%s.bin.gz", variablePart)
		}
	}

	if targetFile == "" {
		return nil, fs.Error("file not found")
	}

	file, err := os.Open(filepath.Join(fs.root, targetFile))
	if err != nil {
		return nil, err
	}

	return &proofFile{File: file}, nil
}

func (fs *proofFileSystem) scanProofFiles() error {
	fs.proofMutex.Lock()
	defer fs.proofMutex.Unlock()

	// Clear existing mappings
	fs.proofFiles = make(map[string]string)

	// Read directory entries
	entries, err := os.ReadDir(fs.root)
	if err != nil {
		return fmt.Errorf("failed to read proofs directory: %v", err)
	}

	// Regexp for matching prestate-proof files and extracting the variable part
	proofRegexp := regexp.MustCompile(`^prestate-proof(.*)\.json$`)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		matches := proofRegexp.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}

		// matches[1] contains the variable part (including the leading hyphen if present)
		variablePart := matches[1]

		// Read and parse the JSON file
		data, err := os.ReadFile(filepath.Join(fs.root, entry.Name()))
		if err != nil {
			log.Printf("Warning: failed to read proof file %s: %v", entry.Name(), err)
			continue
		}

		var proofData struct {
			Pre string `json:"pre"`
		}
		if err := json.Unmarshal(data, &proofData); err != nil {
			log.Printf("Warning: failed to parse proof file %s: %v", entry.Name(), err)
			continue
		}

		// Store the mapping from hash to variable part of filename
		fs.proofFiles[proofData.Pre] = variablePart
		log.Printf("Mapped hash %s to proof file pattern%s", proofData.Pre, variablePart)
	}

	return nil
}

func (fs *proofFileSystem) Error(msg string) error {
	return &os.PathError{Op: "open", Path: "virtual path", Err: errors.New(msg)}
}
