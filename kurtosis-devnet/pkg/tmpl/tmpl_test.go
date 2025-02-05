package tmpl

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewTemplateContext(t *testing.T) {
	t.Run("creates empty context", func(t *testing.T) {
		ctx := NewTemplateContext()
		require.Nil(t, ctx.Data, "expected nil Data in new context")
		require.Empty(t, ctx.Functions, "expected empty Functions map in new context")
	})

	t.Run("adds data with WithData option", func(t *testing.T) {
		data := map[string]string{"key": "value"}
		ctx := NewTemplateContext(WithData(data))
		require.NotNil(t, ctx.Data, "expected non-nil Data in context")
		d, ok := ctx.Data.(map[string]string)
		require.True(t, ok)
		require.Equal(t, "value", d["key"])
	})

	t.Run("adds function with WithFunction option", func(t *testing.T) {
		fn := func(s string) (string, error) { return s + "test", nil }
		ctx := NewTemplateContext(WithFunction("testfn", fn))
		require.Len(t, ctx.Functions, 1, "expected one function in context")
		_, ok := ctx.Functions["testfn"]
		require.True(t, ok, "function not added with correct name")
	})
}

func TestInstantiateTemplate(t *testing.T) {
	t.Run("simple template substitution", func(t *testing.T) {
		data := map[string]string{"name": "world"}
		ctx := NewTemplateContext(WithData(data))

		input := strings.NewReader("Hello {{.name}}!")
		var output bytes.Buffer

		err := ctx.InstantiateTemplate(input, &output)
		require.NoError(t, err)

		expected := "Hello world!\n"
		require.Equal(t, expected, output.String())
	})

	t.Run("template with custom function", func(t *testing.T) {
		upper := func(s string) (string, error) { return strings.ToUpper(s), nil }
		ctx := NewTemplateContext(
			WithData(map[string]string{"name": "world"}),
			WithFunction("upper", upper),
		)

		input := strings.NewReader("Hello {{upper .name}}!")
		var output bytes.Buffer

		err := ctx.InstantiateTemplate(input, &output)
		require.NoError(t, err)

		expected := "Hello WORLD!\n"
		require.Equal(t, expected, output.String())
	})

	t.Run("invalid template syntax", func(t *testing.T) {
		ctx := NewTemplateContext()
		input := strings.NewReader("Hello {{.name")
		var output bytes.Buffer

		err := ctx.InstantiateTemplate(input, &output)
		require.Error(t, err, "expected error for invalid template syntax")
	})

	t.Run("missing data field", func(t *testing.T) {
		ctx := NewTemplateContext()
		input := strings.NewReader("Hello {{.name}}!")
		var output bytes.Buffer

		err := ctx.InstantiateTemplate(input, &output)
		require.Error(t, err, "expected error for missing data field")
	})

	t.Run("multiple functions and data fields", func(t *testing.T) {
		upper := func(s string) (string, error) { return strings.ToUpper(s), nil }
		lower := func(s string) (string, error) { return strings.ToLower(s), nil }

		data := map[string]string{
			"greeting": "Hello",
			"name":     "World",
		}

		ctx := NewTemplateContext(
			WithData(data),
			WithFunction("upper", upper),
			WithFunction("lower", lower),
		)

		input := strings.NewReader("{{upper .greeting}} {{lower .name}}!")
		var output bytes.Buffer

		err := ctx.InstantiateTemplate(input, &output)
		require.NoError(t, err)

		expected := "HELLO world!\n"
		require.Equal(t, expected, output.String())
	})
}
