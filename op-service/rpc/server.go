package rpc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"

	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	optls "github.com/ethereum-optimism/optimism/op-service/tls"
)

var wildcardHosts = []string{"*"}

type Server struct {
	endpoint       string
	apis           []rpc.API
	appVersion     string
	healthzHandler http.Handler
	corsHosts      []string
	vHosts         []string
	jwtSecret      []byte
	wsEnabled      bool
	rpcPath        string
	healthzPath    string
	httpRecorder   opmetrics.HTTPRecorder
	httpServer     *http.Server
	listener       net.Listener
	log            log.Logger
	tls            *ServerTLSConfig
	middlewares    []Middleware
	rpcServer      *rpc.Server
	handlers       map[string]http.Handler
}

type ServerTLSConfig struct {
	Config    *tls.Config
	CLIConfig *optls.CLIConfig // paths to certificate and key files
}

type ServerOption func(b *Server)

type Middleware func(next http.Handler) http.Handler

func WithAPIs(apis []rpc.API) ServerOption {
	return func(b *Server) {
		b.apis = apis
	}
}

func WithHealthzHandler(hdlr http.Handler) ServerOption {
	return func(b *Server) {
		b.healthzHandler = hdlr
	}
}

func WithCORSHosts(hosts []string) ServerOption {
	return func(b *Server) {
		b.corsHosts = hosts
	}
}

func WithVHosts(hosts []string) ServerOption {
	return func(b *Server) {
		b.vHosts = hosts
	}
}

// WithWebsocketEnabled allows `ws://host:port/`, `ws://host:port/ws` and `ws://host:port/ws/`
// to be upgraded to a websocket JSON RPC connection.
func WithWebsocketEnabled() ServerOption {
	return func(b *Server) {
		b.wsEnabled = true
	}
}

// WithJWTSecret adds authentication to the RPCs (HTTP, and WS pre-upgrade if enabled).
// The health endpoint is still available without authentication.
func WithJWTSecret(secret []byte) ServerOption {
	return func(b *Server) {
		b.jwtSecret = secret
	}
}

func WithRPCPath(path string) ServerOption {
	return func(b *Server) {
		b.rpcPath = path
	}
}

func WithHealthzPath(path string) ServerOption {
	return func(b *Server) {
		b.healthzPath = path
	}
}

func WithHTTPRecorder(recorder opmetrics.HTTPRecorder) ServerOption {
	return func(b *Server) {
		b.httpRecorder = recorder
	}
}

func WithLogger(lgr log.Logger) ServerOption {
	return func(b *Server) {
		b.log = lgr
	}
}

// WithTLSConfig configures TLS for the RPC server
// If this option is passed, the server will use ListenAndServeTLS
func WithTLSConfig(tls *ServerTLSConfig) ServerOption {
	return func(b *Server) {
		b.tls = tls
	}
}

// WithMiddleware adds an http.Handler to the rpc server handler stack
// The added middleware is invoked directly before the RPC callback
func WithMiddleware(middleware func(http.Handler) (hdlr http.Handler)) ServerOption {
	return func(b *Server) {
		b.middlewares = append(b.middlewares, middleware)
	}
}

func NewServer(host string, port int, appVersion string, opts ...ServerOption) *Server {
	endpoint := net.JoinHostPort(host, strconv.Itoa(port))
	bs := &Server{
		endpoint:       endpoint,
		appVersion:     appVersion,
		healthzHandler: defaultHealthzHandler(appVersion),
		corsHosts:      wildcardHosts,
		vHosts:         wildcardHosts,
		rpcPath:        "/",
		healthzPath:    "/healthz",
		httpRecorder:   opmetrics.NoopHTTPRecorder,
		httpServer: &http.Server{
			Addr: endpoint,
		},
		log:       log.Root(),
		rpcServer: rpc.NewServer(),
		handlers:  make(map[string]http.Handler),
	}
	for _, opt := range opts {
		opt(bs)
	}
	if bs.tls != nil {
		bs.httpServer.TLSConfig = bs.tls.Config
	}
	bs.AddAPI(rpc.API{
		Namespace: "health",
		Service: &healthzAPI{
			appVersion: appVersion,
		},
	})
	return bs
}

// Endpoint returns the HTTP endpoint without http / ws protocol prefix.
func (b *Server) Endpoint() string {
	if b.listener == nil {
		panic("Server has not started yet, no endpoint is known")
	}
	return b.listener.Addr().String()
}

func (b *Server) AddAPI(api rpc.API) {
	b.apis = append(b.apis, api)
}

// AddHandler adds a custom http.Handler to the server, mapped to an absolute path
func (b *Server) AddHandler(path string, handler http.Handler) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	b.handlers[path] = handler
}

func (b *Server) Start() error {
	// Register all APIs to the RPC server.
	for _, api := range b.apis {
		if err := b.rpcServer.RegisterName(api.Namespace, api.Service); err != nil {
			return fmt.Errorf("failed to register API %s: %w", api.Namespace, err)
		}
		b.log.Info("registered API", "namespace", api.Namespace)
	}

	// http handler stack.
	var handler http.Handler

	// default to 404 not-found
	handler = http.HandlerFunc(http.NotFound)

	// Health endpoint is lowest priority.
	handler = b.newHealthMiddleware(handler)

	// serve RPC on configured RPC path (but not on arbitrary paths)
	handler = b.newHttpRPCMiddleware(handler)

	// Conditionally enable Websocket support.
	if b.wsEnabled { // prioritize WS RPC, if it's an upgrade request
		handler = b.newWsMiddleWare(handler)
	}

	// Apply user middlewares
	for _, middleware := range b.middlewares {
		handler = middleware(handler)
	}

	// Outer-most middlewares: logging, metrics, TLS
	handler = optls.NewPeerTLSMiddleware(handler)
	handler = opmetrics.NewHTTPRecordingMiddleware(b.httpRecorder, handler)
	handler = oplog.NewLoggingMiddleware(b.log, handler)

	// Add custom handlers
	handler = b.newUserHandlersMiddleware(handler)

	b.httpServer.Handler = handler

	listener, err := net.Listen("tcp", b.endpoint)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	b.listener = listener
	// override endpoint with the actual listener address, in case the port was 0 during test.
	b.httpServer.Addr = listener.Addr().String()
	b.endpoint = listener.Addr().String()
	errCh := make(chan error, 1)
	go func() {
		if b.tls != nil {
			if err := b.httpServer.ServeTLS(b.listener, "", ""); err != nil {
				errCh <- err
			}
		} else {
			if err := b.httpServer.Serve(b.listener); err != nil {
				errCh <- err
			}
		}
	}()

	// verify that the server comes up
	tick := time.NewTimer(10 * time.Millisecond)
	defer tick.Stop()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server failed: %w", err)
	case <-tick.C:
		return nil
	}
}

func (b *Server) newHealthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == b.healthzPath {
			b.healthzHandler.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (b *Server) newHttpRPCMiddleware(next http.Handler) http.Handler {
	// Only allow RPC handlers behind the appropriate CORS / vhost / JWT (optional) setup.
	// Note that websockets have their own handler-stack, also configured with CORS and JWT, separately.
	httpHandler := node.NewHTTPHandlerStack(b.rpcServer, b.corsHosts, b.vHosts, b.jwtSecret)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == b.rpcPath {
			httpHandler.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (b *Server) newWsMiddleWare(next http.Handler) http.Handler {
	wsHandler := node.NewWSHandlerStack(b.rpcServer.WebsocketHandler(b.corsHosts), b.jwtSecret)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isWebsocket(r) && (r.URL.Path == "/" || r.URL.Path == "/ws" || r.URL.Path == "/ws/") {
			wsHandler.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (b *Server) newUserHandlersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for path, handler := range b.handlers {
			if strings.HasPrefix(r.URL.Path, path) {
				handler.ServeHTTP(w, r)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (b *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = b.httpServer.Shutdown(ctx)
	b.rpcServer.Stop()
	return nil
}

type HealthzResponse struct {
	Version string `json:"version"`
}

func defaultHealthzHandler(appVersion string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		_ = enc.Encode(&HealthzResponse{Version: appVersion})
	}
}

type healthzAPI struct {
	appVersion string
}

func (h *healthzAPI) Status() string {
	return h.appVersion
}

func isWebsocket(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}
