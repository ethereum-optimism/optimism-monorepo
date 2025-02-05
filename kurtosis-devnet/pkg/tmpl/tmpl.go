package tmpl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"

	sprig "github.com/go-task/slim-sprig/v3"
	"gopkg.in/yaml.v3"
)

// TemplateFunc represents a function that can be used in templates
type TemplateFunc any

// TemplateContext contains data and functions to be passed to templates
type TemplateContext struct {
	baseDir      string
	Data         interface{}
	Functions    map[string]TemplateFunc
	includeStack []string // Track stack of included files to detect circular includes
}

type TemplateContextOptions func(*TemplateContext)

func WithBaseDir(basedir string) TemplateContextOptions {
	return func(ctx *TemplateContext) {
		ctx.baseDir = basedir
	}
}

func WithFunction(name string, fn TemplateFunc) TemplateContextOptions {
	return func(ctx *TemplateContext) {
		ctx.Functions[name] = fn
	}
}

func WithData(data interface{}) TemplateContextOptions {
	return func(ctx *TemplateContext) {
		ctx.Data = data
	}
}

// NewTemplateContext creates a new TemplateContext with default functions
func NewTemplateContext(opts ...TemplateContextOptions) *TemplateContext {
	ctx := &TemplateContext{
		baseDir:      ".",
		Functions:    make(map[string]TemplateFunc),
		includeStack: make([]string, 0),
	}

	for _, opt := range opts {
		opt(ctx)
	}

	return ctx
}

// includeFile reads and processes a template file relative to the given context's baseDir,
// parses the content as YAML, and returns its JSON representation.
// We use JSON because it can be inlined without worrying about indentation, while remaining valid YAML.
// Note: to protect against infinite recursion, we check for circular includes.
func (ctx *TemplateContext) includeFile(fname string, data ...interface{}) (string, error) {
	// Resolve the file path relative to baseDir
	path := filepath.Join(ctx.baseDir, fname)

	// Check for circular includes
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("error resolving absolute path: %w", err)
	}
	for _, includedFile := range ctx.includeStack {
		if includedFile == absPath {
			return "", fmt.Errorf("circular include detected for file %s", fname)
		}
	}

	// Read the included file
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("error opening include file: %w", err)
	}
	defer file.Close()

	// Create buffer for output
	var buf bytes.Buffer

	var tplData interface{}
	switch len(data) {
	case 0:
		tplData = nil
	case 1:
		tplData = data[0]
	default:
		return "", fmt.Errorf("invalid number of arguments for includeFile: %d", len(data))
	}

	// Create new context with updated baseDir and include stack
	includeCtx := &TemplateContext{
		baseDir:      filepath.Dir(path),
		Data:         tplData,
		Functions:    ctx.Functions,
		includeStack: append(append([]string{}, ctx.includeStack...), absPath),
	}

	// Process the included template
	if err := includeCtx.InstantiateTemplate(file, &buf); err != nil {
		return "", fmt.Errorf("error processing include file: %w", err)
	}

	// Parse the buffer content as YAML
	var yamlData interface{}
	if err := yaml.Unmarshal(buf.Bytes(), &yamlData); err != nil {
		return "", fmt.Errorf("error parsing YAML: %w", err)
	}

	// Convert to JSON
	jsonBytes, err := json.Marshal(yamlData)
	if err != nil {
		return "", fmt.Errorf("error converting to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

// InstantiateTemplate reads a template from the reader, executes it with the context,
// and writes the result to the writer
func (ctx *TemplateContext) InstantiateTemplate(reader io.Reader, writer io.Writer) error {
	// Read template content
	templateBytes, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read template: %w", err)
	}

	// Convert TemplateFunc map to FuncMap
	funcMap := template.FuncMap{
		"include": ctx.includeFile,
	}
	for name, fn := range ctx.Functions {
		funcMap[name] = fn
	}

	// Create template with helper functions and option to error on missing fields
	tmpl := template.New("template").
		Funcs(sprig.TxtFuncMap()).
		Funcs(funcMap).
		Option("missingkey=error")

	// Parse template
	tmpl, err = tmpl.Parse(string(templateBytes))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute template into a buffer
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx.Data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// If this is the top-level rendering, we want to write the output as "pretty" YAML
	if len(ctx.includeStack) == 0 {
		var yamlData interface{}
		// Parse the buffer content as YAML
		if err := yaml.Unmarshal(buf.Bytes(), &yamlData); err != nil {
			return fmt.Errorf("error parsing template output as YAML: %w. Template output: %s", err, buf.String())
		}

		// Create YAML encoder with default indentation
		encoder := yaml.NewEncoder(writer)
		encoder.SetIndent(2)

		// Write the YAML document
		if err := encoder.Encode(yamlData); err != nil {
			return fmt.Errorf("error writing YAML output: %w", err)
		}

	} else {
		// Otherwise, just write the buffer content to the writer
		if _, err := buf.WriteTo(writer); err != nil {
			return fmt.Errorf("failed to write template output: %w", err)
		}
	}

	return nil
}
