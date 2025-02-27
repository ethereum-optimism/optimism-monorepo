package rpc

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-service/httputil"
)

// Server is a convenience util, that wraps an httputil.HTTPServer and provides an RPC Handler
type Server struct {
	httpServer *httputil.HTTPServer

	// embedded, for easy access as caller
	*Handler
}

// Endpoint returns the HTTP endpoint without http / ws protocol prefix.
func (b *Server) Endpoint() string {
	return b.httpServer.Addr().String()
}

func (b *Server) Start() error {
	err := b.httpServer.Start()
	if err != nil {
		return err
	}
	b.log.Info("Started RPC server", "endpoint", b.httpServer.HTTPEndpoint())
	return nil
}

func (b *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = b.httpServer.Shutdown(ctx)
	b.Handler.Stop()
	b.log.Info("Stopped RPC server")
	return nil
}

func (b *Server) AddAPI(api rpc.API) {
	err := b.Handler.AddAPI(api)
	if err != nil {
		panic(fmt.Errorf("invalid API: %w", err))
	}
}

type ServerConfig struct {
	HttpOptions []httputil.Option
	RpcOptions  []Option
	Host        string
	Port        int
	AppVersion  string
}

func NewServer(host string, port int, appVersion string, opts ...Option) *Server {
	return ServerFromConfig(&ServerConfig{
		HttpOptions: nil,
		RpcOptions:  opts,
		Host:        host,
		Port:        port,
		AppVersion:  appVersion,
	})
}

func ServerFromConfig(cfg *ServerConfig) *Server {
	endpoint := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	h := NewHandler(cfg.AppVersion, cfg.RpcOptions...)
	s := httputil.NewHTTPServer(endpoint, h, cfg.HttpOptions...)
	return &Server{httpServer: s, Handler: h}
}
