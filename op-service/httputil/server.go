package httputil

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// HTTPServer wraps a http.Server, while providing conveniences
// like exposing the running state and address.
//
// It can be started with HTTPServer.Start and closed with
// HTTPServer.Stop, HTTPServer.Close and HTTPServer.Shutdown (convenience functions for different gracefulness).
//
// The addr contains both host and port. A 0 port may be used to make the system bind to an available one.
// The resulting address can be retrieved with HTTPServer.Addr or HTTPServer.HTTPEndpoint.
//
// The server may be started, stopped and started back up.
type HTTPServer struct {
	// mu is the lock used for bringing the server online/offline, and accessing the address of the server.
	mu sync.RWMutex

	// listener that the server is bound to. Nil if online.
	listener net.Listener

	srv *http.Server

	// used as BaseContext of the http.Server
	srvCtx    context.Context
	srvCancel context.CancelFunc

	config *config
}

// NewHTTPServer creates an HTTPServer that serves the given HTTP handler.
// The server is inactive and has to be started explicitly.
func NewHTTPServer(addr string, handler http.Handler, opts ...Option) *HTTPServer {
	cfg := &config{
		listenAddr: addr,
		tls:        nil,
		handler:    handler,
		httpOpts:   nil,
	}
	cfg.ApplyOptions(opts...)
	return &HTTPServer{config: cfg}
}

func StartHTTPServer(addr string, handler http.Handler, opts ...Option) (*HTTPServer, error) {
	out := NewHTTPServer(addr, handler, opts...)
	return out, out.Start()
}

// Start starts the server, and checks if it comes online fully.
func (s *HTTPServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv != nil {
		return errors.New("already have existing server")
	}

	srvCtx, srvCancel := context.WithCancel(context.Background())
	srv := &http.Server{
		Handler:           s.config.handler,
		ReadTimeout:       DefaultTimeouts.ReadTimeout,
		ReadHeaderTimeout: DefaultTimeouts.ReadHeaderTimeout,
		WriteTimeout:      DefaultTimeouts.WriteTimeout,
		IdleTimeout:       DefaultTimeouts.IdleTimeout,
		BaseContext: func(listener net.Listener) context.Context {
			return srvCtx
		},
	}

	if s.config.tls != nil && s.config.tls.CLIConfig.Enabled {
		srv.TLSConfig = s.config.tls.Config
	}

	for _, opt := range s.config.httpOpts {
		if err := opt(srv); err != nil {
			srvCancel()
			return fmt.Errorf("failed to apply HTTP option: %w", err)
		}
	}

	listener, err := net.Listen("tcp", s.config.listenAddr)
	if err != nil {
		srvCancel()
		return fmt.Errorf("failed to bind to address %q: %w", s.config.listenAddr, err)
	}
	s.listener = listener

	s.srv = srv
	s.srvCtx = srvCtx
	s.srvCancel = srvCancel

	// cap of 1, to not block on non-immediate shutdown
	errCh := make(chan error, 1)
	go func() {
		if s.config.tls != nil {
			errCh <- s.srv.ServeTLS(s.listener, "", "")
		} else {
			errCh <- s.srv.Serve(s.listener)
		}
	}()

	// verify that the server comes up
	standupTimer := time.NewTimer(10 * time.Millisecond)
	defer standupTimer.Stop()

	select {
	case err := <-errCh:
		s.cleanup()
		return fmt.Errorf("http server failed: %w", err)
	case <-standupTimer.C:
		return nil
	}
}

func (s *HTTPServer) Closed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.srv == nil
}

// Stop is a convenience method to gracefully shut down the server, but force-close if the ctx is cancelled.
// The ctx error is not returned when the force-close is successful.
func (s *HTTPServer) Stop(ctx context.Context) error {
	if err := s.Shutdown(ctx); err != nil {
		if errors.Is(err, ctx.Err()) { // force-close connections if we cancelled the stopping
			return s.Close()
		}
		return err
	}
	return nil
}

func (s *HTTPServer) cleanup() {
	s.srv = nil
	s.listener = nil
	s.srvCtx = nil
	s.srvCancel = nil
}

// Shutdown shuts down the HTTP server and its listener,
// but allows active connections to close gracefully.
// If the function exits due to a ctx cancellation the listener is closed but active connections may remain,
// a call to Close() can force-close any remaining active connections.
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv == nil {
		return nil
	}
	s.srvCancel()
	// closes the underlying listener too.
	err := s.srv.Shutdown(ctx)
	if err != nil {
		return err
	}
	s.cleanup()
	return nil
}

// Close force-closes the HTTPServer, its listener, and all its active connections.
func (s *HTTPServer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv == nil {
		return nil
	}
	s.srvCancel()
	// closes the underlying listener too
	err := s.srv.Close()
	if err != nil {
		return err
	}
	s.cleanup()
	return nil
}

// Addr returns the address that the server is listening on.
// It returns nil if the server is not online.
func (s *HTTPServer) Addr() net.Addr {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listener == nil {
		return nil
	}
	return s.listener.Addr()
}

// HTTPEndpoint returns the http(s) endpoint the server is serving.
// It returns an empty string if the server is not online.
func (s *HTTPServer) HTTPEndpoint() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listener == nil {
		return ""
	}
	addr := s.listener.Addr().String()
	if s.config.tls != nil {
		return "https://" + addr
	} else {
		return "http://" + addr
	}
}
