package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"bufio"
)

// MultipartUploadedFile adapts multipart.FileHeader to UploadedFile
type MultipartUploadedFile struct {
	header *multipart.FileHeader
}

func NewMultipartUploadedFile(header *multipart.FileHeader) *MultipartUploadedFile {
	return &MultipartUploadedFile{header: header}
}

func (f *MultipartUploadedFile) Open() (io.ReadCloser, error) {
	return f.header.Open()
}

func (f *MultipartUploadedFile) GetFilename() string {
	return f.header.Filename
}

type Builder struct {
	appRoot    string
	configsDir string
	buildDir   string
	buildCmd   string
	fs         FS
	cmdFactory CommandFactory
}

func NewBuilder(appRoot, configsDir, buildDir, buildCmd string) *Builder {
	return &Builder{
		appRoot:    appRoot,
		configsDir: configsDir,
		buildDir:   buildDir,
		buildCmd:   buildCmd,
		fs:         &DefaultFileSystem{},
		cmdFactory: &DefaultCommandFactory{},
	}
}

func (b *Builder) SaveUploadedFiles(files []UploadedFile) error {
	// Create configs directory if it doesn't exist
	fullConfigsDir := b.fs.Join(b.appRoot, b.buildDir, b.configsDir)
	if err := b.fs.MkdirAll(fullConfigsDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Save the files
	for _, file := range files {
		reader, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer reader.Close()

		destPath := b.fs.Join(fullConfigsDir, b.normalizeFilename(file.GetFilename()))
		dst, err := b.fs.Create(destPath)
		if err != nil {
			return fmt.Errorf("failed to create destination file: %w", err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, reader); err != nil {
			return fmt.Errorf("failed to save file: %w", err)
		}
		log.Printf("Saved file: %s", destPath)
	}

	return nil
}

func (b *Builder) ExecuteBuild() ([]byte, error) {
	log.Printf("Starting build...")
	cmdParts := strings.Fields(b.buildCmd)
	cmd := b.cmdFactory.CreateCommand(cmdParts[0], cmdParts[1:]...)

	// Set working directory
	cmd.SetDir(b.fs.Join(b.appRoot, b.buildDir))

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Buffer to store complete output for error reporting
	var output bytes.Buffer
	output.WriteString("Build output:\n")

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start build: %w", err)
	}

	// Create a WaitGroup to wait for both stdout and stderr to be processed
	var wg sync.WaitGroup
	wg.Add(2)

	// Stream stdout
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("[build] %s", line)
			output.WriteString(line + "\n")
		}
	}()

	// Stream stderr
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("[build][stderr] %s", line)
			output.WriteString(line + "\n")
		}
	}()

	// Wait for both streams to complete
	wg.Wait()

	// Wait for the command to complete
	if err := cmd.Wait(); err != nil {
		return output.Bytes(), fmt.Errorf("build failed: %w", err)
	}

	log.Printf("Build completed successfully")
	return output.Bytes(), nil
}

// This is a convenience hack to natively support the file format of op-deployer
func (b *Builder) normalizeFilename(filename string) string {
	// Get just the filename without directories
	filename = filepath.Base(filename)

	// Check if filename matches PREFIX-NUMBER.json pattern
	if parts := strings.Split(filename, "-"); len(parts) == 2 {
		if numStr := strings.TrimSuffix(parts[1], ".json"); numStr != parts[1] {
			// Check if the number part is actually numeric
			if _, err := strconv.Atoi(numStr); err == nil {
				// Handle specific cases
				if parts[0] == "genesis" {
					return numStr + "-" + parts[0] + "-l2.json"
				} else if parts[0] == "rollup" {
					return numStr + "-" + "rollup.json"
				}
				// For all other cases, leave the filename unchanged
			}
		}
	}

	return filename
}
