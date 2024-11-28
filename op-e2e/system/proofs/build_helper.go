package proofs

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// BuildOpProgramClient builds the `op-program` client executable and returns the path to the resulting executable
func BuildOpProgramClient(t *testing.T) string {
	clientPath, err := filepath.Abs("../../../op-program/bin/op-program-client")
	require.NoError(t, err)

	_, err = os.Stat(clientPath)
	if err == nil {
		return clientPath
	}
	require.ErrorIs(t, err, os.ErrNotExist)

	t.Log("Building op-program-client")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "make", "op-program-client")
	cmd.Dir = "../../../op-program"
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	require.NoErrorf(t, cmd.Run(), "Failed to build op-program-client: %v", &out)
	t.Log("Built op-program-client successfully")
	return "../../../op-program/bin/op-program-client"
}
