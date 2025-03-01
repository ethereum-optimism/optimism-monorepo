package frontend

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockAdminBackend struct{}

func (m *mockAdminBackend) Hello(ctx context.Context, name string) (string, error) {
	return "hello " + name, nil
}

var _ AdminBackend = (*mockAdminBackend)(nil)

func TestAdmin(t *testing.T) {
	b := &mockAdminBackend{}
	front := &AdminFrontend{Backend: b}
	out, err := front.Hello(context.Background(), "world")
	require.NoError(t, err)
	require.Equal(t, "hello world", out)
}
