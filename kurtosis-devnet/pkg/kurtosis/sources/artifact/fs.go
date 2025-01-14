package artifact

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"path/filepath"

	"github.com/kurtosis-tech/kurtosis/api/golang/engine/lib/kurtosis_context"
)

// EnclaveContextIface abstracts the EnclaveContext for testing
type EnclaveContextIface interface {
	DownloadFilesArtifact(ctx context.Context, name string) ([]byte, error)
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
	reader *tar.Reader
}

func (fs *EnclaveFS) GetArtifact(ctx context.Context, name string) (*Artifact, error) {
	artifact, err := fs.enclaveCtx.DownloadFilesArtifact(ctx, name)
	if err != nil {
		return nil, err
	}

	buffer := bytes.NewBuffer(artifact)
	zipReader, err := gzip.NewReader(buffer)
	if err != nil {
		return nil, err
	}
	tarReader := tar.NewReader(zipReader)
	return &Artifact{reader: tarReader}, nil
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

func (a *Artifact) ExtractFiles(writers ...*ArtifactFileWriter) error {
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
