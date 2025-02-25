package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MockFile implements both File and fs.FileInfo interfaces for testing
type MockFile struct {
	name     string
	contents []byte
	pos      int64
	isDir    bool
}

func NewMockFile(name string, contents []byte) *MockFile {
	return &MockFile{
		name:     name,
		contents: contents,
	}
}

func (m *MockFile) Close() error { return nil }
func (m *MockFile) Read(p []byte) (n int, err error) {
	if m.pos >= int64(len(m.contents)) {
		return 0, io.EOF
	}
	n = copy(p, m.contents[m.pos:])
	m.pos += int64(n)
	return n, nil
}

func (m *MockFile) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = m.pos + offset
	case io.SeekEnd:
		abs = int64(len(m.contents)) + offset
	default:
		return 0, fmt.Errorf("invalid whence")
	}
	if abs < 0 {
		return 0, fmt.Errorf("negative position")
	}
	m.pos = abs
	return abs, nil
}

func (m *MockFile) Stat() (fs.FileInfo, error) { return m, nil }
func (m *MockFile) Name() string               { return m.name }
func (m *MockFile) Size() int64                { return int64(len(m.contents)) }
func (m *MockFile) Mode() fs.FileMode          { return 0644 }
func (m *MockFile) ModTime() time.Time         { return time.Now() }
func (m *MockFile) IsDir() bool                { return m.isDir }
func (m *MockFile) Sys() interface{}           { return nil }
func (m *MockFile) Readdir(count int) ([]fs.FileInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

// MockFS implements FS interface for testing
type MockFS struct {
	Files         map[string]*MockFile
	ShouldFail    bool
	StatFailPaths map[string]bool // Paths that should fail for Stat
	JoinCalls     [][]string
	MkdirCalls    []string
	CreateCalls   []string
}

func NewMockFS() *MockFS {
	return &MockFS{
		Files:         make(map[string]*MockFile),
		StatFailPaths: make(map[string]bool),
		JoinCalls:     make([][]string, 0),
		MkdirCalls:    make([]string, 0),
		CreateCalls:   make([]string, 0),
	}
}

// FS interface methods
func (m *MockFS) Open(name string) (File, error) {
	if m.ShouldFail {
		return nil, fmt.Errorf("mock open error")
	}
	if file, ok := m.Files[name]; ok {
		file.pos = 0 // Reset position for new reads
		return file, nil
	}
	return nil, fs.ErrNotExist
}

func (m *MockFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if m.ShouldFail {
		return nil, fmt.Errorf("mock readdir error")
	}
	var entries []fs.DirEntry
	for path, file := range m.Files {
		if filepath.Dir(path) == name {
			entries = append(entries, fs.FileInfoToDirEntry(file))
		}
	}
	return entries, nil
}

func (m *MockFS) ReadFile(name string) ([]byte, error) {
	if m.ShouldFail {
		return nil, fmt.Errorf("mock readfile error")
	}
	if file, ok := m.Files[name]; ok {
		return file.contents, nil
	}
	return nil, fs.ErrNotExist
}

// FileSystem interface methods
func (m *MockFS) MkdirAll(path string, perm os.FileMode) error {
	if m.ShouldFail {
		return fmt.Errorf("mock mkdir error")
	}
	m.MkdirCalls = append(m.MkdirCalls, path)
	return nil
}

func (m *MockFS) Create(name string) (io.WriteCloser, error) {
	if m.ShouldFail {
		return nil, fmt.Errorf("mock create error")
	}
	m.CreateCalls = append(m.CreateCalls, name)
	return &MockWriteCloser{}, nil
}

func (m *MockFS) Join(elem ...string) string {
	m.JoinCalls = append(m.JoinCalls, elem)
	return filepath.Join(elem...)
}

func (m *MockFS) Stat(name string) (fs.FileInfo, error) {
	if m.ShouldFail {
		return nil, fmt.Errorf("mock stat error")
	}
	if m.StatFailPaths[name] {
		return nil, fmt.Errorf("file not found: %s", name)
	}
	return m.Files[name], nil
}

// MockWriteCloser implements io.WriteCloser for testing
type MockWriteCloser struct {
	bytes.Buffer
}

func (m *MockWriteCloser) Close() error {
	return nil
}

// MockUploadedFile implements UploadedFile for testing
type MockUploadedFile struct {
	filename string
	content  []byte
}

func NewMockUploadedFile(filename string, content []byte) *MockUploadedFile {
	return &MockUploadedFile{
		filename: filename,
		content:  content,
	}
}

func (m *MockUploadedFile) Open() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(m.content)), nil
}

func (m *MockUploadedFile) GetFilename() string {
	return m.filename
}

// MockCommandRunner implements CommandRunner for testing
type MockCommandRunner struct {
	StartCalled bool
	WaitCalled  bool
	ShouldFail  bool
	Stdout      string
	Stderr      string
	Dir         string
}

func (m *MockCommandRunner) Start() error {
	if m.ShouldFail {
		return fmt.Errorf("mock start error")
	}
	m.StartCalled = true
	return nil
}

func (m *MockCommandRunner) Wait() error {
	if m.ShouldFail {
		return fmt.Errorf("mock wait error")
	}
	m.WaitCalled = true
	return nil
}

func (m *MockCommandRunner) StdoutPipe() (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(m.Stdout)), nil
}

func (m *MockCommandRunner) StderrPipe() (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(m.Stderr)), nil
}

func (m *MockCommandRunner) SetDir(dir string) {
	m.Dir = dir
}

// MockCommandFactory implements CommandFactory for testing
type MockCommandFactory struct {
	Runner *MockCommandRunner
}

func (f *MockCommandFactory) CreateCommand(name string, args ...string) CommandRunner {
	return f.Runner
}

// MockProofFS is a mock implementation of ProofFS
type MockProofFS struct {
	scanProofFilesFn func() error
	fs               *MockFS
}

func NewMockProofFS() *MockProofFS {
	return &MockProofFS{
		fs: NewMockFS(),
	}
}

func (m *MockProofFS) scanProofFiles() error {
	if m.scanProofFilesFn != nil {
		return m.scanProofFilesFn()
	}
	return nil
}

func (m *MockProofFS) Open(name string) (http.File, error) {
	file, err := m.fs.Open(name)
	if err != nil {
		return nil, err
	}
	// MockFile implements http.File (including Seek)
	return file.(http.File), nil
}

// AddFile adds a file to the mock filesystem
func (m *MockProofFS) AddFile(name string, contents []byte) {
	m.fs.Files[name] = NewMockFile(name, contents)
}

// MockBuilder is a mock implementation of BuildSystem
type MockBuilder struct {
	saveUploadedFilesFn func(files []UploadedFile) error
	executeBuildFn      func() ([]byte, error)
}

func (m *MockBuilder) SaveUploadedFiles(files []UploadedFile) error {
	if m.saveUploadedFilesFn != nil {
		return m.saveUploadedFilesFn(files)
	}
	return nil
}

func (m *MockBuilder) ExecuteBuild() ([]byte, error) {
	if m.executeBuildFn != nil {
		return m.executeBuildFn()
	}
	return []byte("mock build output"), nil
}
