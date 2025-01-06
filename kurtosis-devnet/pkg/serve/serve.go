package serve

import (
	"context"
	"fmt"
	"net"
	"net/http"
)

// Server represents an HTTP server that serves static files
type Server struct {
	server   *http.Server
	listener net.Listener
	url      string
	hostname string
}

type ServerOption func(*Server)

func WithStaticDir(staticDir string) ServerOption {
	return func(s *Server) {
		mux := http.NewServeMux()
		fileServer := http.FileServer(http.Dir(staticDir))
		mux.Handle("/", fileServer)
		s.server.Handler = mux
	}
}

func WithHostname(hostname string) ServerOption {
	return func(s *Server) {
		s.hostname = hostname
	}
}

// NewServer creates a new static file server
func NewServer(opts ...ServerOption) *Server {
	server := &Server{
		server: &http.Server{
			Handler: http.NewServeMux(),
		},
		hostname: "localhost",
	}

	for _, opt := range opts {
		opt(server)
	}

	return server
}

// Start begins serving files in a goroutine and returns when the server is ready
func (s *Server) Start(ctx context.Context) error {
	// Create listener with dynamic port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	s.listener = listener

	// Get the actual address
	addr := listener.Addr().(*net.TCPAddr)
	s.url = fmt.Sprintf("http://%s:%d", s.hostname, addr.Port)

	// Channel to signal server is ready to accept connections
	ready := make(chan struct{})

	go func() {
		// Signal ready right before we start serving
		close(ready)

		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			// Log server errors that occur after startup
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	// Wait for server to be ready or context to be cancelled
	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		listener.Close()
		return ctx.Err()
	}
}

// Stop gracefully shuts down the server
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// URL returns the server's URL
func (s *Server) URL() string {
	return s.url
}
