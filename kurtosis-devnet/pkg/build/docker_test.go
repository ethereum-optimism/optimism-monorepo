package build

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDockerClient implements the dockerClient interface for testing
type mockDockerClient struct {
	inspectFunc func(ctx context.Context, imageID string) (types.ImageInspect, []byte, error)
	tagFunc     func(ctx context.Context, source, target string) error
}

func (m *mockDockerClient) ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error) {
	return m.inspectFunc(ctx, imageID)
}

func (m *mockDockerClient) ImageTag(ctx context.Context, source, target string) error {
	return m.tagFunc(ctx, source, target)
}

// mockDockerProvider implements the dockerProvider interface for testing
type mockDockerProvider struct {
	client dockerClient
}

func (p *mockDockerProvider) newClient() (dockerClient, error) {
	return p.client, nil
}

// mockCmd is a mock for command execution that always succeeds
type mockCmd struct {
	output []byte
}

func (m *mockCmd) CombinedOutput() ([]byte, error) {
	return m.output, nil
}

// mockCmdFactory creates mock commands for testing
func mockCmdFactory(output []byte) cmdFactory {
	return func(name string, arg ...string) cmdRunner {
		return &mockCmd{output: output}
	}
}

// TestDockerBuilderNaming tests the image naming logic in the DockerBuilder
func TestDockerBuilderNaming(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		imageTag    string
		mockInspect types.ImageInspect
		mockTagErr  error
		wantTag     string
		wantErr     bool
	}{
		{
			name:        "successful image build and tag",
			projectName: "test-project",
			imageTag:    "test-image:latest",
			mockInspect: types.ImageInspect{
				ID: "sha256:abcdef123456789abcdef123456789abcdef1234",
			},
			wantTag: "test-project:abcdef123456",
			wantErr: false,
		},
		{
			name:        "tag error",
			projectName: "test-project",
			imageTag:    "test-image:latest",
			mockInspect: types.ImageInspect{
				ID: "sha256:abcdef123456789abcdef123456789abcdef1234",
			},
			mockTagErr: assert.AnError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockDockerClient{
				inspectFunc: func(ctx context.Context, imageID string) (types.ImageInspect, []byte, error) {
					return tt.mockInspect, nil, nil
				},
				tagFunc: func(ctx context.Context, source, target string) error {
					return tt.mockTagErr
				},
			}

			mockProvider := &mockDockerProvider{
				client: mockClient,
			}

			// Create a builder with our mocks
			builder := NewDockerBuilder(
				withDockerProvider(mockProvider),
				withCmdFactory(mockCmdFactory([]byte("mock build output"))),
			)

			tag, err := builder.Build(tt.projectName, tt.imageTag)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantTag, tag)
		})
	}
}
