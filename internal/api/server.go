package api

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
)

// Server is the daemon's HTTP API server.
type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

// NewServer creates an API server bound to 127.0.0.1 on the given port.
func NewServer(port int, handlers *Handlers) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handlers.Health)
	mux.HandleFunc("/status", handlers.Status)
	mux.HandleFunc("/chat", handlers.Chat)
	mux.HandleFunc("/invoke", handlers.Invoke)
	mux.HandleFunc("/logs", handlers.Logs)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	return &Server{
		httpServer: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
		logger: handlers.Logger,
	}
}

// Start begins listening. It blocks until the server is shut down.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("binding to %s: %w", s.httpServer.Addr, err)
	}
	s.logger.Info("API server listening", "addr", s.httpServer.Addr)
	return s.httpServer.Serve(ln)
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
