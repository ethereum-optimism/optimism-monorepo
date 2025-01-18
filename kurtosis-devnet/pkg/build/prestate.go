package build

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"text/template"
)

// PrestateBuilder handles building prestates using just commands
type PrestateBuilder struct {
	baseDir     string
	cmdTemplate *template.Template
	dryRun      bool

	builtPrestates map[string]interface{}
}

const (
	prestateCmdTemplateStr = "just _prestate-build {{.Path}}"
)

var defaultPrestateTemplate *template.Template

func init() {
	defaultPrestateTemplate = template.Must(template.New("prestate_build_cmd").Parse(prestateCmdTemplateStr))
}

type PrestateBuilderOptions func(*PrestateBuilder)

func WithPrestateBaseDir(baseDir string) PrestateBuilderOptions {
	return func(b *PrestateBuilder) {
		b.baseDir = baseDir
	}
}

func WithPrestateTemplate(cmdTemplate *template.Template) PrestateBuilderOptions {
	return func(b *PrestateBuilder) {
		b.cmdTemplate = cmdTemplate
	}
}

func WithPrestateDryRun(dryRun bool) PrestateBuilderOptions {
	return func(b *PrestateBuilder) {
		b.dryRun = dryRun
	}
}

// NewPrestateBuilder creates a new PrestateBuilder instance
func NewPrestateBuilder(opts ...PrestateBuilderOptions) *PrestateBuilder {
	b := &PrestateBuilder{
		baseDir:        ".",
		cmdTemplate:    defaultPrestateTemplate,
		dryRun:         false,
		builtPrestates: make(map[string]interface{}),
	}

	for _, opt := range opts {
		opt(b)
	}

	return b
}

// templateData holds the data for the command template
type prestateTemplateData struct {
	Path string
}

// Build executes the prestate build command
func (b *PrestateBuilder) Build(path string) error {
	if _, ok := b.builtPrestates[path]; ok {
		return nil
	}

	log.Printf("Building prestate: %s", path)

	// Prepare template data
	data := prestateTemplateData{
		Path: path,
	}

	// Execute template to get command string
	var cmdBuf bytes.Buffer
	if err := b.cmdTemplate.Execute(&cmdBuf, data); err != nil {
		return fmt.Errorf("failed to execute command template: %w", err)
	}

	// Create command
	cmd := exec.Command("sh", "-c", cmdBuf.String())
	cmd.Dir = b.baseDir

	if !b.dryRun {
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("prestate build command failed: %w\nOutput: %s", err, string(output))
		}
	}

	b.builtPrestates[path] = struct{}{}
	return nil
}
