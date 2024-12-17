package rpc

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
)

// ObtainJWTSecret attempts to read a JWT secret, and generates one if necessary.
// Unlike the geth rpc.ObtainJWTSecret variant, this uses local logging,
// makes generation optional, and does not blindly overwrite a JWT secret on any read error.
// Generally it is advised to generate a JWT secret if missing, as a server.
// Clients should not generate a JWT secret, and use the secret of the server instead.
func ObtainJWTSecret(logger log.Logger, jwtSecretPath string, generateMissing bool) (eth.Bytes32, error) {
	jwtSecretPath = strings.TrimSpace(jwtSecretPath)
	if jwtSecretPath == "" {
		return eth.Bytes32{}, fmt.Errorf("file-name of jwt secret is empty")
	}
	// Check if the file exists
	_, err := os.Stat(jwtSecretPath)
	exists := !errors.Is(err, fs.ErrNotExist)
	if exists {
		// If the file exists, read the JWT secret from it
		jwtSecret, err := readJWTSecret(jwtSecretPath)
		if err != nil {
			return eth.Bytes32{}, fmt.Errorf("failed to read JWT secret from file path %q: %w", jwtSecretPath, err)
		}
		return jwtSecret, nil
	} else if generateMissing {
		// if the file does not exist, and generation is enabled, generate a new JWT secret
		logger.Warn("JWT secret file not found, generating a new one now.", "path ", jwtSecretPath)
		jwtSecret, err := generateJWTSecret(jwtSecretPath)
		if err != nil {
			return eth.Bytes32{}, fmt.Errorf("failed to generate JWT secret in path %q: %w", jwtSecretPath, err)
		}
		return jwtSecret, nil
	} else {
		// if the file does not exist, and generation is disabled, return an error
		return eth.Bytes32{}, fmt.Errorf("jwt secret file not found at path %q", jwtSecretPath)
	}
}

// generateJWTSecret generates a new JWT secret and writes it to the file at the given path.
// Prior status of the file is not checked, and the file is always overwritten.
// Callers should ensure the file does not exist, or that overwriting is acceptable.
func generateJWTSecret(path string) (eth.Bytes32, error) {
	var secret eth.Bytes32
	if _, err := io.ReadFull(rand.Reader, secret[:]); err != nil {
		return eth.Bytes32{}, fmt.Errorf("failed to generate jwt secret: %w", err)
	}
	if err := os.WriteFile(path, []byte(hexutil.Encode(secret[:])), 0o600); err != nil {
		return eth.Bytes32{}, err
	}
	return secret, nil
}

// readJWTSecret reads a JWT secret from the file at the given path.
// Prior status of the file is not checked, and the file is always read.
// Callers should ensure the file exists
func readJWTSecret(path string) (eth.Bytes32, error) {
	// Read the JWT secret from the file
	data, err := os.ReadFile(path)
	if err != nil {
		return eth.Bytes32{}, fmt.Errorf("failed to read JWT secret from file path %q: %w", path, err)
	}
	// Parse the JWT secret from the file
	jwtSecret := common.FromHex(strings.TrimSpace(string(data))) // FromHex handles optional '0x' prefix
	if len(jwtSecret) != 32 {
		return eth.Bytes32{}, fmt.Errorf("invalid jwt secret in path %q, not 32 hex-formatted bytes", path)
	}
	return eth.Bytes32(jwtSecret), nil
}
