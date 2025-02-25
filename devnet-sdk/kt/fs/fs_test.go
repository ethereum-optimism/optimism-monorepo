package fs

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/services"
	"github.com/stretchr/testify/require"
)

type mockEnclaveContext struct {
	artifacts map[string][]byte
	uploaded  map[string]map[string][]byte // artifactName -> path -> content
}

func (m *mockEnclaveContext) DownloadFilesArtifact(_ context.Context, name string) ([]byte, error) {
	return m.artifacts[name], nil
}

func (m *mockEnclaveContext) UploadFiles(pathToUpload string, artifactName string) (services.FilesArtifactUUID, services.FileArtifactName, error) {
	if m.uploaded == nil {
		m.uploaded = make(map[string]map[string][]byte)
	}
	m.uploaded[artifactName] = make(map[string][]byte)

	err := filepath.Walk(pathToUpload, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(pathToUpload, path)
		if err != nil {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		m.uploaded[artifactName][relPath] = content
		return nil
	})

	return "test-uuid", services.FileArtifactName(artifactName), err
}

func createTarGzArtifact(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	for name, content := range files {
		err := tarWriter.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0600,
			Size: int64(len(content)),
		})
		require.NoError(t, err)

		_, err = tarWriter.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzWriter.Close())
	return buf.Bytes()
}

func TestArtifactExtraction(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		requests map[string]string
		wantErr  bool
	}{
		{
			name: "simple path",
			files: map[string]string{
				"file1.txt": "content1",
			},
			requests: map[string]string{
				"file1.txt": "content1",
			},
		},
		{
			name: "path with dot prefix",
			files: map[string]string{
				"./file1.txt": "content1",
			},
			requests: map[string]string{
				"file1.txt": "content1",
			},
		},
		{
			name: "mixed paths",
			files: map[string]string{
				"./file1.txt":  "content1",
				"file2.txt":    "content2",
				"./dir/f3.txt": "content3",
			},
			requests: map[string]string{
				"file1.txt":  "content1",
				"file2.txt":  "content2",
				"dir/f3.txt": "content3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock context with artifact
			mockCtx := &mockEnclaveContext{
				artifacts: map[string][]byte{
					"test-artifact": createTarGzArtifact(t, tt.files),
				},
			}

			fs := NewEnclaveFSWithContext(mockCtx)
			artifact, err := fs.GetArtifact(context.Background(), "test-artifact")
			require.NoError(t, err)

			// Create writers for all requested files
			writers := make([]*ArtifactFileWriter, 0, len(tt.requests))
			buffers := make(map[string]*bytes.Buffer, len(tt.requests))
			for reqPath := range tt.requests {
				buf := &bytes.Buffer{}
				buffers[reqPath] = buf
				writers = append(writers, NewArtifactFileWriter(reqPath, buf))
			}

			// Extract all files at once
			err = artifact.ExtractFiles(writers...)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify contents
			for reqPath, wantContent := range tt.requests {
				require.Equal(t, wantContent, buffers[reqPath].String(), "content mismatch for %s", reqPath)
			}
		})
	}
}

func TestPutArtifact(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		wantErr bool
	}{
		{
			name: "single file",
			files: map[string]string{
				"file1.txt": "content1",
			},
		},
		{
			name: "multiple files",
			files: map[string]string{
				"file1.txt":     "content1",
				"dir/file2.txt": "content2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtx := &mockEnclaveContext{
				artifacts: make(map[string][]byte),
			}

			fs := NewEnclaveFSWithContext(mockCtx)

			// Create readers for all files
			var readers []*ArtifactFileReader
			for path, content := range tt.files {
				readers = append(readers, NewArtifactFileReader(
					path,
					bytes.NewReader([]byte(content)),
				))
			}

			// Put the artifact
			err := fs.PutArtifact(context.Background(), "test-artifact", readers...)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify uploaded contents
			require.NotNil(t, mockCtx.uploaded)
			uploaded := mockCtx.uploaded["test-artifact"]
			require.NotNil(t, uploaded)
			require.Equal(t, len(tt.files), len(uploaded))

			for path, wantContent := range tt.files {
				content, exists := uploaded[path]
				require.True(t, exists, "missing file: %s", path)
				require.Equal(t, wantContent, string(content), "content mismatch for %s", path)
			}
		})
	}
}
