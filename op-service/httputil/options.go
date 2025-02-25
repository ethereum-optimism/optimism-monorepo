package httputil

import (
	"crypto/tls"
	"net/http"

	optls "github.com/ethereum-optimism/optimism/op-service/tls"
)

type config struct {
	// listenAddr is the configured address to listen to when started.
	// use listener.Addr to retrieve the address when online.
	listenAddr string

	tls *ServerTLSConfig

	handler http.Handler

	httpOpts []HTTPOption
}

func (c *config) ApplyOptions(opts ...Option) {
	for _, opt := range opts {
		opt(c)
	}
}

// Option is a general config option.
type Option func(cfg *config)

// HTTPOption applies a change to an HTTP server, just before standup.
// HTTPOption options are be re-executed on server shutdown/startup cycles,
// for each new underlying Go *http.Server instance.
type HTTPOption func(config *http.Server) error

func WithHTTPOptions(options ...HTTPOption) Option {
	return func(cfg *config) {
		cfg.httpOpts = append(cfg.httpOpts, options...)
	}
}

func WithMaxHeaderBytes(max int) HTTPOption {
	return func(srv *http.Server) error {
		srv.MaxHeaderBytes = max
		return nil
	}
}

type ServerTLSConfig struct {
	Config    *tls.Config
	CLIConfig *optls.CLIConfig // paths to certificate and key files
}

func WithServerTLS(tlsCfg *ServerTLSConfig) Option {
	return func(cfg *config) {
		cfg.tls = tlsCfg
	}
}
