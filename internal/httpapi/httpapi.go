package httpapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/mesutoezdil/tensorwatch/internal/exporter"
	"github.com/mesutoezdil/tensorwatch/internal/pipeline"
)

type Server struct {
	addr   string
	token  string
	pipe   *pipeline.Pipeline
	prom   *exporter.Prometheus
	server *http.Server
}

func New(addr, token string, pipe *pipeline.Pipeline, prom *exporter.Prometheus) *Server {
	return &Server{addr: addr, token: token, pipe: pipe, prom: prom}
}

func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/snapshot", s.auth(s.handleSnapshot))
	mux.HandleFunc("/metrics", s.auth(s.handleMetrics))
	mux.HandleFunc("/", s.handleIndex)

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- s.server.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return s.server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	if s.token == "" {
		return next
	}
	expected := "Bearer " + s.token
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != expected {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) handleSnapshot(w http.ResponseWriter, _ *http.Request) {
	snap := s.pipe.Latest()
	if snap == nil {
		http.Error(w, "no data", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(snap)
}

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = w.Write(s.prom.Render())
}

func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("tensorwatch\n\nendpoints:\n  /health\n  /snapshot\n  /metrics\n"))
}
