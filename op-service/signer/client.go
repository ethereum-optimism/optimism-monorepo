package signer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"time"

	optls "github.com/ethereum-optimism/optimism/op-service/tls"
	"github.com/ethereum-optimism/optimism/op-service/tls/certman"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

type SignerClient struct {
	client *rpc.Client
	status string
	logger log.Logger
}

func NewSignerClient(logger log.Logger, endpoint string, headers http.Header, tlsConfig optls.CLIConfig) (*SignerClient, error) {
	var httpClient *http.Client
	if tlsConfig.Enabled {
		logger.Info("tlsConfig specified, loading tls config")
		caCert, err := os.ReadFile(tlsConfig.TLSCaCert)
		if err != nil {
			return nil, fmt.Errorf("failed to read tls.ca: %w", err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		// certman watches for newer client certifictes and automatically reloads them
		cm, err := certman.New(logger, tlsConfig.TLSCert, tlsConfig.TLSKey)
		if err != nil {
			logger.Error("failed to read tls cert or key", "err", err)
			return nil, err
		}
		if err := cm.Watch(); err != nil {
			logger.Error("failed to start certman watcher", "err", err)
			return nil, err
		}

		httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS13,
					RootCAs:    caCertPool,
					GetClientCertificate: func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
						return cm.GetCertificate(nil)
					},
				},
			},
		}
	} else {
		logger.Info("no tlsConfig specified, using default http client")
		httpClient = http.DefaultClient
	}

	rpcClient, err := rpc.DialOptions(context.Background(), endpoint, rpc.WithHTTPClient(httpClient), rpc.WithHeaders(headers))
	if err != nil {
		return nil, err
	}

	signer := &SignerClient{logger: logger, client: rpcClient}
	// Check if reachable
	version, err := signer.pingVersion()
	if err != nil {
		return nil, err
	}
	signer.status = fmt.Sprintf("ok [version=%v]", version)
	return signer, nil
}

func NewSignerClientFromConfig(logger log.Logger, config CLIConfig) (*SignerClient, error) {
	return NewSignerClient(logger, config.Endpoint, config.Headers, config.TLSConfig)
}

func (s *SignerClient) pingVersion() (string, error) {
	var v string
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	if err := s.client.CallContext(ctx, &v, "health_status"); err != nil {
		return "", err
	}
	return v, nil
}

func (s *SignerClient) SignTransaction(ctx context.Context, chainId *big.Int, from common.Address, tx *types.Transaction) (*types.Transaction, error) {
	sidecar := tx.BlobTxSidecar()
	args := NewTransactionArgsFromTransaction(chainId, &from, tx.WithoutBlobTxSidecar())

	var result hexutil.Bytes
	if err := s.client.CallContext(ctx, &result, "eth_signTransaction", args); err != nil {
		return nil, fmt.Errorf("eth_signTransaction failed: %w", err)
	}

	var signed types.Transaction
	if err := signed.UnmarshalBinary(result); err != nil {
		return nil, err
	}
	if sidecar != nil {
		if err := signed.SetBlobTxSidecar(sidecar); err != nil {
			return nil, fmt.Errorf("failed to attach sidecar to signed blob tx: %w", err)
		}
	}

	return &signed, nil
}

func (s *SignerClient) SignBlockPayload(ctx context.Context, args *BlockPayloadArgs) ([65]byte, error) {
	var result hexutil.Bytes

	if err := s.client.CallContext(ctx, &result, "opsigner_signBlockPayload", args); err != nil {
		return [65]byte{}, fmt.Errorf("opsigner_signBlockPayload failed: %w", err)
	}

	if len(result) != 65 {
		return [65]byte{}, fmt.Errorf("invalid signature: %s", result.String())
	}

	signature := [65]byte(result)

	return signature, nil
}
