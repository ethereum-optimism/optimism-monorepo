package main

import (
	"bytes"
	"encoding/json"
	"math/big"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)

	o := new(outputTy)
	err = json.Unmarshal([]byte(output), o)
	require.NoError(t, err)
	require.Equal(t, "0x00000000000000000000000000000000000000000000000000000000000a6fe0", o.ScalarHex)
}

func TestMain_PostEcotoneScalar(t *testing.T) {
	longScalar := "452312848583266388373324160190187140051835877600158453279135543542576845931"
	output, err := runMainWithArgs(t, []string{"-decode=" + longScalar})
	require.NoError(t, err)

	o := new(outputTy)
	err = json.Unmarshal([]byte(output), o)
	if err != nil {
		t.Fatal(err)
	}

	expected := &outputTy{
		BaseFee:           5227,
		BlobbaseFeeScalar: 1014213,
		ScalarHex:         "0x010000000000000000000000000000000000000000000000000f79c50000146b",
		Scalar:            new(big.Int),
	}
	_, ok := expected.Scalar.SetString(longScalar, 0)

	require.True(t, ok)
	require.Equal(t, expected, o)
}
