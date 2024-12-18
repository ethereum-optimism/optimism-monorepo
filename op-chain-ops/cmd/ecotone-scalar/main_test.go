package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func runMainWithArgs(t *testing.T, args []string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", "run", "main.go")
	cmd.Args = append(cmd.Args, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String() + stderr.String()
	return output, err
}

func TestMain_PreEcotoneScalar(t *testing.T) {
	output, err := runMainWithArgs(t, []string{"-decode=684000"})
	if err != nil {
		t.Fatalf("unexpected error: %v, output: %s", err, output)
	}
	if !strings.Contains(output, "v1 hex encoding  : 0x00000000000000000000000000000000000000000000000000000000000a6fe0") {
		t.Errorf("did not find expected output: %s", output)
	}
}
