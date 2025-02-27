package fs

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/services"
	"github.com/kurtosis-tech/kurtosis/api/golang/engine/lib/kurtosis_context"
)

// EnclaveContextIface abstracts the EnclaveContext for testing
type EnclaveContextIface interface {
	DownloadFilesArtifact(ctx context.Context, name string) ([]byte, error)
	UploadFiles(pathToUpload string, artifactName string) (services.FilesArtifactUUID, services.FileArtifactName, error)
}

type EnclaveFS struct {
	enclaveCtx EnclaveContextIface
}

func NewEnclaveFS(ctx context.Context, enclave string) (*EnclaveFS, error) {
	kurtosisCtx, err := kurtosis_context.NewKurtosisContextFromLocalEngine()
	if err != nil {
		return nil, err
	}

	enclaveCtx, err := kurtosisCtx.GetEnclaveContext(ctx, enclave)
	if err != nil {
		return nil, err
	}

	return &EnclaveFS{enclaveCtx: enclaveCtx}, nil
}

// NewEnclaveFSWithContext creates an EnclaveFS with a provided context (useful for testing)
func NewEnclaveFSWithContext(ctx EnclaveContextIface) *EnclaveFS {
	return &EnclaveFS{enclaveCtx: ctx}
}

type Artifact struct {
	reader      *tar.Reader
	archiveData []byte
	gzipReader  *gzip.Reader
}

func (fs *EnclaveFS) GetArtifact(ctx context.Context, name string) (*Artifact, error) {
	archiveData, err := fs.enclaveCtx.DownloadFilesArtifact(ctx, name)
	if err != nil {
		return nil, err
	}

	// Create a new reader for the archive data
	buffer := bytes.NewBuffer(archiveData)
	gzipReader, err := gzip.NewReader(buffer)
	if err != nil {
		return nil, err
	}
	tarReader := tar.NewReader(gzipReader)

	return &Artifact{
		reader:      tarReader,
		archiveData: archiveData,
		gzipReader:  gzipReader,
	}, nil
}

type ArtifactFileWriter struct {
	path   string
	writer io.Writer
}

func NewArtifactFileWriter(path string, writer io.Writer) *ArtifactFileWriter {
	return &ArtifactFileWriter{
		path:   path,
		writer: writer,
	}
}

// resetReader recreates the tar reader from the stored archive data
func (a *Artifact) resetReader() error {
	// Close the existing gzip reader if it exists
	if a.gzipReader != nil {
		a.gzipReader.Close()
	}

	// Create a new reader from the stored archive data
	buffer := bytes.NewBuffer(a.archiveData)
	gzipReader, err := gzip.NewReader(buffer)
	if err != nil {
		return err
	}

	a.gzipReader = gzipReader
	a.reader = tar.NewReader(gzipReader)
	return nil
}

// ExtractFiles extracts specific files from the artifact to the provided writers.
// This function can be called multiple times on the same Artifact instance.
func (a *Artifact) ExtractFiles(writers ...*ArtifactFileWriter) error {
	// Reset the reader to the beginning of the archive
	if err := a.resetReader(); err != nil {
		return err
	}

	paths := make(map[string]io.Writer)
	for _, writer := range writers {
		canonicalPath := filepath.Clean(writer.path)
		paths[canonicalPath] = writer.writer
	}

	for {
		header, err := a.reader.Next()
		if err == io.EOF {
			break
		}

		headerPath := filepath.Clean(header.Name)
		if _, ok := paths[headerPath]; !ok {
			continue
		}

		writer := paths[headerPath]
		_, err = io.Copy(writer, a.reader)
		if err != nil {
			return err
		}
	}

	return nil
}

func (fs *EnclaveFS) PutArtifact(ctx context.Context, name string, readers ...*ArtifactFileReader) error {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "artifact-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir) // Clean up temp dir when we're done

	// Process each reader
	for _, reader := range readers {
		// Create the full path in the temp directory
		fullPath := filepath.Join(tempDir, reader.path)

		// Ensure the parent directory exists
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return err
		}

		// Create the file
		file, err := os.Create(fullPath)
		if err != nil {
			return err
		}

		// Copy the content
		_, err = io.Copy(file, reader.reader)
		file.Close() // Close file after writing
		if err != nil {
			return err
		}
	}

	// Upload the directory to Kurtosis
	_, _, err = fs.enclaveCtx.UploadFiles(tempDir, name)
	return err
}

type ArtifactFileReader struct {
	path   string
	reader io.Reader
}

func NewArtifactFileReader(path string, reader io.Reader) *ArtifactFileReader {
	return &ArtifactFileReader{
		path:   path,
		reader: reader,
	}
}

// Close closes the gzip reader and releases resources.
func (a *Artifact) Close() error {
	if a.gzipReader != nil {
		return a.gzipReader.Close()
	}
	return nil
}
