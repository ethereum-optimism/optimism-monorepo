package main

import (
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

// osFile wraps os.File to implement File
type osFile struct {
	*os.File
}

func (f *osFile) Readdir(count int) ([]fs.FileInfo, error) {
	return f.File.Readdir(count)
}

// DefaultFileSystem implements testutils.FS using actual OS calls
type DefaultFileSystem struct{}

func (fs *DefaultFileSystem) Open(name string) (File, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	return &osFile{File: file}, nil
}

func (fs *DefaultFileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(name)
}

func (fs *DefaultFileSystem) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (fs *DefaultFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (fs *DefaultFileSystem) Create(name string) (io.WriteCloser, error) {
	return os.Create(name)
}

func (fs *DefaultFileSystem) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (fs *DefaultFileSystem) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

// commandWrapper wraps exec.Cmd to implement CommandRunner
type commandWrapper struct {
	*exec.Cmd
}

func (c *commandWrapper) SetDir(dir string) {
	c.Cmd.Dir = dir
}

// DefaultCommandFactory implements testutils.CommandFactory using actual OS exec
type DefaultCommandFactory struct{}

func (f *DefaultCommandFactory) CreateCommand(name string, args ...string) CommandRunner {
	cmd := exec.Command(name, args...)
	return &commandWrapper{Cmd: cmd}
}
