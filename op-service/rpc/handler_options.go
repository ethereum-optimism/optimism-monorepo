package rpc

import (
	"net/http"

	"github.com/ethereum/go-ethereum/log"

	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
)

type Option func(b *Handler)

type Middleware func(next http.Handler) http.Handler

func WithHealthzHandler(hdlr http.Handler) Option {
	return func(b *Handler) {
		b.healthzHandler = hdlr
	}
}

func WithCORSHosts(hosts []string) Option {
	return func(b *Handler) {
		b.corsHosts = hosts
	}
}

func WithVHosts(hosts []string) Option {
	return func(b *Handler) {
		b.vHosts = hosts
	}
}

// WithWebsocketEnabled allows `ws://host:port/`, `ws://host:port/ws` and `ws://host:port/ws/`
// to be upgraded to a websocket JSON RPC connection.
func WithWebsocketEnabled() Option {
	return func(b *Handler) {
		b.wsEnabled = true
	}
}

// WithJWTSecret adds authentication to the RPCs (HTTP, and WS pre-upgrade if enabled).
// The health endpoint is still available without authentication.
func WithJWTSecret(secret []byte) Option {
	return func(b *Handler) {
		b.jwtSecret = secret
	}
}

func WithHTTPRecorder(recorder opmetrics.HTTPRecorder) Option {
	return func(b *Handler) {
		b.httpRecorder = recorder
	}
}

func WithLogger(lgr log.Logger) Option {
	return func(b *Handler) {
		b.log = lgr
	}
}

// WithMiddleware adds an http.Handler to the rpc server handler stack
// The added middleware is invoked directly before the RPC callback
func WithMiddleware(middleware func(http.Handler) (hdlr http.Handler)) Option {
	return func(b *Handler) {
		b.middlewares = append(b.middlewares, middleware)
	}
}
