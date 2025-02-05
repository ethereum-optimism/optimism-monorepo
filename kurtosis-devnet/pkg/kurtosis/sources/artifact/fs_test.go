package artifact

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockEnclaveContext struct {
	artifacts map[string][]byte
}

func (m *mockEnclaveContext) DownloadFilesArtifact(_ context.Context, name string) ([]byte, error) {
	return m.artifacts[name], nil
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
