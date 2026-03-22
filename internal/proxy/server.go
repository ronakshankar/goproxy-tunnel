package proxy

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/wiretunnel/wiretunnel/internal/config"
	"github.com/wiretunnel/wiretunnel/internal/pipeline"
	"github.com/wiretunnel/wiretunnel/internal/tunnel"
	"github.com/wiretunnel/wiretunnel/pkg/logger"
)

// Server composes the HTTP server, the observability pipeline, and the tunnel
// registry into a single lifecycle-managed unit.
type Server struct {
	cfg      *config.Config
	pool     *pipeline.Pool
	registry *tunnel.Registry
	http     *http.Server
	log      *logger.Logger
}

// NewServer constructs a Server. Start() must be called to begin serving.
func NewServer(cfg *config.Config, log *logger.Logger) *Server {
	registry := tunnel.NewRegistry()
	pool := pipeline.NewPool(cfg.WorkerCount, cfg.PipelineQueueDepth, cfg.LogDir, log)
	h := newHandler(cfg, pool, registry, log)

	mux := http.NewServeMux()
	mux.HandleFunc("/", h.ServeHTTP)

	httpSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.ListenPort),
		Handler: mux,
		// Prevent slow clients from holding connections open indefinitely
		// during the initial HTTP handshake. After the WebSocket upgrade,
		// timeouts are managed by the tunnel itself.
		ReadHeaderTimeout: 10 * time.Second,
	}

	return &Server{
		cfg:      cfg,
		pool:     pool,
		registry: registry,
		http:     httpSrv,
		log:      log,
	}
}

// Start starts the observability pipeline and the HTTP server. It blocks until
// ctx is cancelled, at which point it performs a graceful shutdown: in-flight
// WebSocket connections are allowed to close naturally, and the worker pool
// drains any queued compression jobs before returning.
func (s *Server) Start(ctx context.Context) error {
	s.pool.Start()

	// Graceful shutdown: when ctx is cancelled, shut down the HTTP server so
	// no new connections are accepted, then stop the pipeline worker pool.
	go func() {
		<-ctx.Done()
		s.log.Info("shutdown signal received; draining connections")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.http.Shutdown(shutdownCtx); err != nil {
			s.log.Error("HTTP server shutdown error", "err", err)
		}

		s.pool.Stop()
	}()

	if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %w", err)
	}

	return nil
}
