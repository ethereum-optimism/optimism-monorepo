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
	data, err := os.ReadFile(jwtSecretPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if !generateMissing {
				return eth.Bytes32{}, fmt.Errorf("JWT-secret in path %q does not exist: %w", jwtSecretPath, err)
			}
			logger.Warn("Failed to read JWT secret from file, generating a new one now.", "path", jwtSecretPath)
			return generateJWTSecret(jwtSecretPath)
		} else {
			return eth.Bytes32{}, fmt.Errorf("failed to read JWT secret from file path %q", jwtSecretPath)
		}
	}
	// Parse the JWT secret data we just read
	jwtSecret := common.FromHex(strings.TrimSpace(string(data))) // FromHex handles optional '0x' prefix
	if len(jwtSecret) != 32 {
		return eth.Bytes32{}, fmt.Errorf("invalid jwt secret in path %q, not 32 hex-formatted bytes", jwtSecretPath)
	}
	return eth.Bytes32(jwtSecret), nil
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
