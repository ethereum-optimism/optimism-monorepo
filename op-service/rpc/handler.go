package rpc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"

	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	optls "github.com/ethereum-optimism/optimism/op-service/tls"
)

// the root is "", since the "/" prefix is already assumed to be stripped.
const rootRoute = ""

var wildcardHosts = []string{"*"}

// Handler is an http Handler, serving a default RPC server on the root path.
//
// Additional RPC servers can be attached to this on sub-routes using AddRPC.
// Each sub-route has its own RPCs that can be served, registered with AddAPIToRPC.
// These inherit the same RPC settings, and each have their own health handlers,
// and websocket support if configured.
//
// Custom routes can also be added with AddHandler, these are registered to the underlying http.ServeMux.
//
// If more customization is needed, this Handler can be composed in a HTTP stack of your own.
// Use http.StripPrefix to clean the URL route prefix that this Handler is registered on (leave no prefix).
type Handler struct {
	appVersion     string
	healthzHandler http.Handler
	corsHosts      []string
	vHosts         []string
	jwtSecret      []byte
	wsEnabled      bool
	httpRecorder   opmetrics.HTTPRecorder

	log         log.Logger
	middlewares []Middleware

	// rpcRoutes is a collection of RPC servers
	rpcRoutes     map[string]*rpc.Server
	rpcRoutesLock sync.Mutex

	mux *http.ServeMux

	// What we serve to users of this Handler, see ServeHTTP
	outer http.Handler
}

func NewHandler(appVersion string, opts ...Option) *Handler {
	bs := &Handler{
		appVersion:     appVersion,
		healthzHandler: defaultHealthzHandler(appVersion),
		corsHosts:      wildcardHosts,
		vHosts:         wildcardHosts,
		httpRecorder:   opmetrics.NoopHTTPRecorder,
		log:            log.Root(),
		mux:            &http.ServeMux{},
		rpcRoutes:      make(map[string]*rpc.Server),
	}
	for _, opt := range opts {
		opt(bs)
	}
	bs.log.Debug("Creating RPC handler")

	var handler http.Handler
	handler = bs.mux
	// Outer-most middlewares: logging, metrics, TLS
	handler = optls.NewPeerTLSMiddleware(handler)
	handler = opmetrics.NewHTTPRecordingMiddleware(bs.httpRecorder, handler)
	handler = oplog.NewLoggingMiddleware(bs.log, handler)
	bs.outer = handler

	if err := bs.AddRPC(rootRoute); err != nil {
		panic(fmt.Errorf("failed to register root RPC server: %w", err))
	}

	return bs
}

var _ http.Handler = (*Handler)(nil)

// ServeHTTP implements http.Handler
func (b *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	b.outer.ServeHTTP(writer, request)
}

// AddAPI adds a backend to the given RPC namespace, on the default RPC route of the server.
func (b *Handler) AddAPI(api rpc.API) error {
	return b.AddAPIToRPC(rootRoute, api)
}

// AddAPIToRPC adds a backend to the given RPC namespace, on the RPC corresponding to the given route.
func (b *Handler) AddAPIToRPC(route string, api rpc.API) error {
	b.rpcRoutesLock.Lock()
	defer b.rpcRoutesLock.Unlock()
	server, ok := b.rpcRoutes[route]
	if !ok {
		return fmt.Errorf("route %q not found", route)
	}
	if err := server.RegisterName(api.Namespace, api.Service); err != nil {
		return fmt.Errorf("failed to register API namespace %s on route %q: %w", api.Namespace, route, err)
	}
	b.log.Info("registered API", "route", route, "namespace", api.Namespace)
	return nil
}

// AddHandler adds a custom http.Handler, mapped to an absolute path
func (b *Handler) AddHandler(path string, handler http.Handler) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	b.mux.Handle(path, handler)
}

// AddRPC creates a default RPC handler at the given route,
// with a health sub-route, HTTP endpoint, and websocket endpoint if configured.
// Once the route is added, RPC namespaces can be registered with AddAPIToRPC.
// The route must not have a "/" suffix, since the trailing "/" is ambiguous.
func (b *Handler) AddRPC(route string) error {
	b.rpcRoutesLock.Lock()
	defer b.rpcRoutesLock.Unlock()
	if strings.HasSuffix(route, "/") {
		return fmt.Errorf("routes must not have a / suffix, got %q", route)
	}
	_, ok := b.rpcRoutes[route]
	if ok {
		return fmt.Errorf("route %q already exists", route)
	}

	srv := rpc.NewServer()

	if err := srv.RegisterName("health", &healthzAPI{
		appVersion: b.appVersion,
	}); err != nil {
		return fmt.Errorf("failed to setup default health RPC namespace")
	}

	// http handler stack.
	var handler http.Handler

	// default to 404 not-found
	handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.NotFound(writer, request)
	})

	// serve RPC on configured RPC path (but not on arbitrary paths)
	handler = b.newHttpRPCMiddleware(srv, handler)

	// Conditionally enable Websocket support.
	if b.wsEnabled { // prioritize WS RPC, if it's an upgrade request
		handler = b.newWsMiddleWare(srv, handler)
	}

	// Apply user middlewares
	for _, middleware := range b.middlewares {
		handler = middleware(handler)
	}

	// Health endpoint applies before user middleware
	handler = b.newHealthMiddleware(handler)

	b.rpcRoutes[route] = srv

	b.mux.Handle(route+"/", http.StripPrefix(route+"/", handler))
	if route != "" {
		b.mux.Handle(route, http.StripPrefix(route, handler))
	}
	return nil
}

func (b *Handler) newHealthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// URL is already stripped with http.StripPrefix
		if r.URL.Path == "healthz" || r.URL.Path == "healthz/" {
			b.healthzHandler.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (b *Handler) newHttpRPCMiddleware(server *rpc.Server, next http.Handler) http.Handler {
	// Only allow RPC handlers behind the appropriate CORS / vhost / JWT (optional) setup.
	// Note that websockets have their own handler-stack, also configured with CORS and JWT, separately.
	httpHandler := node.NewHTTPHandlerStack(server, b.corsHosts, b.vHosts, b.jwtSecret)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// URL is already stripped with http.StripPrefix
		if r.URL.Path == "" {
			httpHandler.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (b *Handler) newWsMiddleWare(server *rpc.Server, next http.Handler) http.Handler {
	wsHandler := node.NewWSHandlerStack(server.WebsocketHandler(b.corsHosts), b.jwtSecret)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// URL is already stripped with http.StripPrefix
		if isWebsocket(r) && (r.URL.Path == "" || r.URL.Path == "ws" || r.URL.Path == "ws/") {
			wsHandler.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (b *Handler) Stop() {
	for route, s := range b.rpcRoutes {
		b.log.Debug("Stopping RPC", "route", route)
		s.Stop()
	}
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
