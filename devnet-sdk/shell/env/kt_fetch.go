package env

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"

	ktfs "github.com/ethereum-optimism/optimism/devnet-sdk/kt/fs"
)

const (
	KurtosisDevnetEnvArtifactNamePrefix = "devnet-descriptor-"
	KurtosisDevnetEnvArtifactPath       = "env.json"
)

// EnclaveFS is an interface that both our mock and the real implementation satisfy
type EnclaveFS interface {
	GetArtifact(ctx context.Context, name string) (*ktfs.Artifact, error)
	GetAllArtifactNames(ctx context.Context) ([]string, error)
	Close() error
}

// enclaveFSWrapper wraps the artifact.EnclaveFS to implement our EnclaveFS interface
type enclaveFSWrapper struct {
	fs *ktfs.EnclaveFS
}

func (w *enclaveFSWrapper) GetArtifact(ctx context.Context, name string) (*ktfs.Artifact, error) {
	return w.fs.GetArtifact(ctx, name)
}

func (w *enclaveFSWrapper) GetAllArtifactNames(ctx context.Context) ([]string, error) {
	return w.fs.GetAllArtifactNames(ctx)
}

func (w *enclaveFSWrapper) Close() error {
	// The underlying EnclaveFS doesn't have a Close method, but we need it for our interface
	return nil
}

// NewEnclaveFSFunc is the type for functions that create new enclave filesystems
type NewEnclaveFSFunc func(ctx context.Context, enclave string) (EnclaveFS, error)

// NewEnclaveFS is a variable that holds the function to create a new enclave filesystem
// It can be replaced in tests
var NewEnclaveFS NewEnclaveFSFunc = func(ctx context.Context, enclave string) (EnclaveFS, error) {
	fs, err := ktfs.NewEnclaveFS(ctx, enclave)
	if err != nil {
		return nil, err
	}
	return &enclaveFSWrapper{fs: fs}, nil
}

// parseKurtosisURL parses a Kurtosis URL of the form kt://enclave/artifact/file
// If artifact is omitted, it defaults to ""
// If file is omitted, it defaults to "env.json"
func parseKurtosisURL(u *url.URL) (enclave, artifactName, fileName string) {
	enclave = u.Host
	artifactName = ""
	fileName = KurtosisDevnetEnvArtifactPath

	// Trim both prefix and suffix slashes before splitting
	trimmedPath := strings.Trim(u.Path, "/")
	parts := strings.Split(trimmedPath, "/")
	if len(parts) > 0 && parts[0] != "" {
		artifactName = parts[0]
	}
	if len(parts) > 1 && parts[1] != "" {
		fileName = parts[1]
	}

	return
}

func getDefaultDescriptor(ctx context.Context, fs EnclaveFS) (string, error) {
	prefix := KurtosisDevnetEnvArtifactNamePrefix

	names, err := fs.GetAllArtifactNames(ctx)
	if err != nil {
		return "", err
	}

	var maxSuffix int
	var maxName string
	for _, name := range names {
		if !strings.HasPrefix(name, prefix) {
			continue
		}

		// Extract the suffix after the prefix
		suffix := name[len(prefix):]
		// Parse the suffix as a number
		var num int
		if _, err := fmt.Sscanf(suffix, "%d", &num); err != nil {
			continue // Skip if suffix is not a valid number
		}

		// Update maxName if this number is larger
		if maxName == "" || num > maxSuffix {
			maxSuffix = num
			maxName = name
		}
	}

	if maxName == "" {
		return "", fmt.Errorf("no descriptor found with valid numerical suffix")
	}

	return maxName, nil
}

// fetchKurtosisData reads data from a Kurtosis artifact
func fetchKurtosisData(u *url.URL) (string, []byte, error) {
	enclave, artifactName, fileName := parseKurtosisURL(u)

	fs, err := NewEnclaveFS(context.Background(), enclave)
	if err != nil {
		return "", nil, fmt.Errorf("error creating enclave fs: %w", err)
	}

	if artifactName == "" {
		artifactName, err = getDefaultDescriptor(context.Background(), fs)
		if err != nil {
			return "", nil, fmt.Errorf("error getting default descriptor: %w", err)
		}
		fmt.Printf("Using default descriptor: %s\n", artifactName)
	}

	art, err := fs.GetArtifact(context.Background(), artifactName)
	if err != nil {
		return "", nil, fmt.Errorf("error getting artifact: %w", err)
	}

	var buf bytes.Buffer
	writer := ktfs.NewArtifactFileWriter(fileName, &buf)

	if err := art.ExtractFiles(writer); err != nil {
		return "", nil, fmt.Errorf("error extracting file from artifact: %w", err)
	}

	return enclave, buf.Bytes(), nil
}
