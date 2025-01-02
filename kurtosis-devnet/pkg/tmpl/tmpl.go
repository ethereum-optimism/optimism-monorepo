package tmpl

import (
	"fmt"
	"io"
	"text/template"
)

// TemplateFunc represents a function that can be used in templates
type TemplateFunc any

// TemplateContext contains data and functions to be passed to templates
type TemplateContext struct {
	Data      interface{}
	Functions map[string]TemplateFunc
}

type TemplateContextOptions func(*TemplateContext)

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
		Functions: make(map[string]TemplateFunc),
	}

	for _, opt := range opts {
		opt(ctx)
	}

	return ctx
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
	funcMap := template.FuncMap{}
	for name, fn := range ctx.Functions {
		funcMap[name] = fn
	}

	// Create template with helper functions and option to error on missing fields
	tmpl := template.New("template").
		Funcs(funcMap).
		Option("missingkey=error")

	// Parse template
	tmpl, err = tmpl.Parse(string(templateBytes))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute template with context
	if err := tmpl.Execute(writer, ctx.Data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}
