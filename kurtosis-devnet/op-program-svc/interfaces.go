package main

import (
	"io"
	"io/fs"
	"os"
)

// File interface abstracts file operations
type File interface {
	fs.File
	io.Seeker
	Readdir(count int) ([]fs.FileInfo, error)
}

// FS defines the interface for all filesystem operations
type FS interface {
	// Core FS operations
	Open(name string) (File, error)
	ReadDir(name string) ([]fs.DirEntry, error)
	ReadFile(name string) ([]byte, error)
	Join(elem ...string) string
	Stat(name string) (fs.FileInfo, error)

	// Additional FileSystem operations
	MkdirAll(path string, perm os.FileMode) error
	Create(name string) (io.WriteCloser, error)
}

// UploadedFile represents a file that has been uploaded
type UploadedFile interface {
	Open() (io.ReadCloser, error)
	GetFilename() string
}

// CommandRunner abstracts command execution for testing
type CommandRunner interface {
	Start() error
	Wait() error
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
	SetDir(dir string)
}

// CommandFactory creates commands
type CommandFactory interface {
	CreateCommand(name string, args ...string) CommandRunner
}
