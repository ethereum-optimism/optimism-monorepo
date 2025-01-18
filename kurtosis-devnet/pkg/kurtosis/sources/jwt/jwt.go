package jwt

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/artifact"
)

const (
	jwtSecretFileName = "jwtsecret"
)

// Data holds the JWT secrets for L1 and L2
type Data struct {
	L1JWT string
	L2JWT string
}

// extractor implements the interfaces.JWTExtractor interface
type extractor struct {
	enclave string
}

// NewExtractor creates a new JWT extractor
func NewExtractor(enclave string) *extractor {
	return &extractor{
		enclave: enclave,
	}
}

// ExtractData extracts JWT secrets from their respective artifacts
func (e *extractor) ExtractData(ctx context.Context) (*Data, error) {
	fs, err := artifact.NewEnclaveFS(ctx, e.enclave)
	if err != nil {
		return nil, err
	}

	// Get L1 JWT
	l1JWT, err := extractJWTFromArtifact(ctx, fs, "jwt_file")
	if err != nil {
		return nil, fmt.Errorf("failed to get L1 JWT: %w", err)
	}

	// Get L2 JWT
	l2JWT, err := extractJWTFromArtifact(ctx, fs, "op_jwt_file")
	if err != nil {
		return nil, fmt.Errorf("failed to get L2 JWT: %w", err)
	}

	return &Data{
		L1JWT: l1JWT,
		L2JWT: l2JWT,
	}, nil
}

func extractJWTFromArtifact(ctx context.Context, fs *artifact.EnclaveFS, artifactName string) (string, error) {
	a, err := fs.GetArtifact(ctx, artifactName)
	if err != nil {
		return "", fmt.Errorf("failed to get artifact: %w", err)
	}

	buffer := &bytes.Buffer{}
	if err := a.ExtractFiles(artifact.NewArtifactFileWriter(jwtSecretFileName, buffer)); err != nil {
		return "", fmt.Errorf("failed to extract JWT: %w", err)
	}

	return parseJWT(buffer)
}

func parseJWT(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("failed to read JWT file: %w", err)
	}
	return string(data), nil
}
