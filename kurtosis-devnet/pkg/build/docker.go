package build

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"text/template"
)

// DockerBuilder handles building docker images using just commands
type DockerBuilder struct {
	// Base directory where the build commands should be executed
	baseDir string
	// Template for the build command
	cmdTemplate *template.Template
	// Dry run mode
	dryRun bool

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

// NewDockerBuilder creates a new DockerBuilder instance
func NewDockerBuilder(opts ...DockerBuilderOptions) *DockerBuilder {
	b := &DockerBuilder{
		baseDir:     ".",
		cmdTemplate: defaultCmdTemplate,
		dryRun:      false,
		builtImages: make(map[string]string),
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
func (b *DockerBuilder) Build(projectName, imageTag string) (string, error) {
	if builtImage, ok := b.builtImages[projectName]; ok {
		return builtImage, nil
	}

	log.Printf("Building docker image for project: %s with tag: %s", projectName, imageTag)
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
	cmd := exec.Command("sh", "-c", cmdBuf.String())
	cmd.Dir = b.baseDir

	if !b.dryRun {
		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("build command failed: %w\nOutput: %s", err, string(output))
		}
	}

	// Return the image tag as confirmation of successful build
	b.builtImages[projectName] = imageTag
	return imageTag, nil
}
