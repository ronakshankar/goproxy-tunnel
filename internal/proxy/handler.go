// Package proxy contains the HTTP server, WebSocket upgrade handler, and the
// glue that connects incoming connections to tunnel goroutines.
package proxy

import (
	"net"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/wiretunnel/wiretunnel/internal/config"
	"github.com/wiretunnel/wiretunnel/internal/pipeline"
	"github.com/wiretunnel/wiretunnel/internal/tunnel"
	"github.com/wiretunnel/wiretunnel/pkg/logger"
)

var upgrader = websocket.Upgrader{
	// Allow all origins for a general-purpose proxy. In a production deployment
	// this should be restricted to known client origins.
	CheckOrigin: func(r *http.Request) bool { return true },

	ReadBufferSize:  32 * 1024,
	WriteBufferSize: 32 * 1024,
}

// handler implements http.Handler. It upgrades every incoming HTTP request to a
// WebSocket connection, dials the configured TCP backend, and starts a Tunnel.
type handler struct {
	cfg      *config.Config
	pool     *pipeline.Pool
	registry *tunnel.Registry
	log      *logger.Logger
	idGen    *idGenerator
}

func newHandler(cfg *config.Config, pool *pipeline.Pool, registry *tunnel.Registry, log *logger.Logger) *handler {
	return &handler{
		cfg:      cfg,
		pool:     pool,
		registry: registry,
		log:      log,
		idGen:    newIDGenerator(),
	}
}

// ServeHTTP upgrades the connection to WebSocket, dials the TCP target, and
// runs the tunnel in the current goroutine. The HTTP server invokes this in its
// own goroutine per connection, so blocking here is correct and expected.
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// upgrader has already written the error response; just log and return.
		h.log.Error("WebSocket upgrade failed", "remote", r.RemoteAddr, "err", err)
		return
	}

	tcpConn, err := net.Dial("tcp", h.cfg.TargetAddr)
	if err != nil {
		wsConn.Close()
		h.log.Error("TCP dial failed",
			"remote", r.RemoteAddr,
			"target", h.cfg.TargetAddr,
			"err", err,
		)
		return
	}

	id := h.idGen.Next()
	t := tunnel.New(id, wsConn, tcpConn, h.pool, h.log, h.cfg.BufferSize)

	h.registry.Register(id, t)
	defer h.registry.Deregister(id)

	h.log.Info("tunnel opened",
		"tunnel_id", id,
		"remote", r.RemoteAddr,
		"target", h.cfg.TargetAddr,
		"active_tunnels", h.registry.Count(),
	)

	// Serve blocks until both upstream and downstream goroutines have exited.
	t.Serve()

	h.log.Info("tunnel deregistered",
		"tunnel_id", id,
		"active_tunnels", h.registry.Count(),
	)
}
