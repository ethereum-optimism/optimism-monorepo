package build

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"text/template"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// cmdRunner abstracts command execution for testing
type cmdRunner interface {
	CombinedOutput() ([]byte, error)
}

// defaultCmdRunner is the default implementation that uses exec.Command
type defaultCmdRunner struct {
	*exec.Cmd
}

func (r *defaultCmdRunner) CombinedOutput() ([]byte, error) {
	return r.Cmd.CombinedOutput()
}

// cmdFactory creates commands
type cmdFactory func(name string, arg ...string) cmdRunner

// defaultCmdFactory is the default implementation that uses exec.Command
func defaultCmdFactory(name string, arg ...string) cmdRunner {
	return &defaultCmdRunner{exec.Command(name, arg...)}
}

// dockerClient interface defines the Docker client methods we use
type dockerClient interface {
	ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error)
	ImageTag(ctx context.Context, source, target string) error
}

// dockerProvider abstracts the creation of Docker clients
type dockerProvider interface {
	newClient() (dockerClient, error)
}

// defaultDockerProvider is the default implementation of dockerProvider
type defaultDockerProvider struct{}

func (p *defaultDockerProvider) newClient() (dockerClient, error) {
	return client.NewClientWithOpts(client.FromEnv)
}

// DockerBuilder handles building docker images using just commands
type DockerBuilder struct {
	// Base directory where the build commands should be executed
	baseDir string
	// Template for the build command
	cmdTemplate *template.Template
	// Dry run mode
	dryRun bool
	// Docker provider for creating clients
	dockerProvider dockerProvider
	// Command factory for testing
	cmdFactory cmdFactory

	builtImages map[string]string
}

const cmdTemplateStr = "just {{.ProjectName}}-image {{.ImageTag}}"

var defaultCmdTemplate *template.Template

func init() {
	defaultCmdTemplate = template.Must(template.New("docker_build_cmd").Parse(cmdTemplateStr))
}

type DockerBuilderOptions func(*DockerBuilder)

func WithDockerCmdTemplate(cmdTemplate *template.Template) DockerBuilderOptions {
	return func(b *DockerBuilder) {
		b.cmdTemplate = cmdTemplate
	}
}

func WithDockerBaseDir(baseDir string) DockerBuilderOptions {
	return func(b *DockerBuilder) {
		b.baseDir = baseDir
	}
}

func WithDockerDryRun(dryRun bool) DockerBuilderOptions {
	return func(b *DockerBuilder) {
		b.dryRun = dryRun
	}
}

// withDockerProvider is a package-private option for testing
func withDockerProvider(provider dockerProvider) DockerBuilderOptions {
	return func(b *DockerBuilder) {
		b.dockerProvider = provider
	}
}

// withCmdFactory is a package-private option for testing
func withCmdFactory(factory cmdFactory) DockerBuilderOptions {
	return func(b *DockerBuilder) {
		b.cmdFactory = factory
	}
}

// NewDockerBuilder creates a new DockerBuilder instance
func NewDockerBuilder(opts ...DockerBuilderOptions) *DockerBuilder {
	b := &DockerBuilder{
		baseDir:        ".",
		cmdTemplate:    defaultCmdTemplate,
		dryRun:         false,
		dockerProvider: &defaultDockerProvider{},
		cmdFactory:     defaultCmdFactory,
		builtImages:    make(map[string]string),
	}

	for _, opt := range opts {
		opt(b)
	}

	return b
}

// templateData holds the data for the command template
type templateData struct {
	ImageTag    string
	ProjectName string
}

// Build executes the docker build command for the given project and image tag
// Note: the returned image tag is the image ID, so we don't accidentally
// de-duplicate steps that should not be de-duplicated.
func (b *DockerBuilder) Build(projectName, imageTag string) (string, error) {
	if builtImage, ok := b.builtImages[projectName]; ok {
		return builtImage, nil
	}

	log.Printf("Building docker image for project: %s with tag: %s", projectName, imageTag)

	if b.dryRun {
		b.builtImages[projectName] = imageTag
		return imageTag, nil
	}

	// Prepare template data
	data := templateData{
		ImageTag:    imageTag,
		ProjectName: projectName,
	}

	// Execute template to get command string
	var cmdBuf bytes.Buffer
	if err := b.cmdTemplate.Execute(&cmdBuf, data); err != nil {
		return "", fmt.Errorf("failed to execute command template: %w", err)
	}

	// Create command
	cmd := b.cmdFactory("sh", "-c", cmdBuf.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("build command failed: %w\nOutput: %s", err, string(output))
	}

	dockerClient, err := b.dockerProvider.newClient()
	if err != nil {
		return "", fmt.Errorf("failed to create docker client: %w", err)
	}

	ctx := context.Background()

	// Inspect the image to get its ID
	inspect, _, err := dockerClient.ImageInspectWithRaw(ctx, imageTag)
	if err != nil {
		return "", fmt.Errorf("failed to inspect image: %w", err)
	}

	// Get the short ID (first 12 characters of the SHA256)
	shortID := strings.TrimPrefix(inspect.ID, "sha256:")[:12]

	// Create a new tag with projectName:shortID
	fullTag := fmt.Sprintf("%s:%s", projectName, shortID)
	err = dockerClient.ImageTag(ctx, imageTag, fullTag)
	if err != nil {
		return "", fmt.Errorf("failed to tag image: %w", err)
	}

	b.builtImages[projectName] = fullTag
	return fullTag, nil
}
