package deployer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/enclaves"
	"github.com/kurtosis-tech/kurtosis/api/golang/engine/lib/kurtosis_context"
)

type EnclaveFS struct {
	enclaveCtx *enclaves.EnclaveContext
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

func (a *Artifact) ExtractFiles(writers ...*ArtifactFileWriter) error {
	paths := make(map[string]io.Writer)
	for _, writer := range writers {
		canonicalPath := filepath.Clean(writer.path)
		canonicalPath = fmt.Sprintf("./%s", canonicalPath)
		paths[canonicalPath] = writer.writer
	}

	for {
		header, err := a.reader.Next()
		if err == io.EOF {
			break
		}

		if _, ok := paths[header.Name]; !ok {
			continue
		}

		writer := paths[header.Name]
		_, err = io.Copy(writer, a.reader)
		if err != nil {
			return err
		}
	}

	return nil
}
