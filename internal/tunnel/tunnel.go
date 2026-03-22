// Package tunnel manages the lifecycle of a single proxied connection.
// Each Tunnel bridges a WebSocket client to a raw TCP backend via two
// goroutines — one for each direction of the full-duplex stream.
package tunnel

import (
	"net"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/wiretunnel/wiretunnel/internal/pipeline"
	"github.com/wiretunnel/wiretunnel/pkg/logger"
)

// Tunnel encapsulates the state for one active proxied connection.
type Tunnel struct {
	id      string
	ws      *websocket.Conn
	tcp     net.Conn
	pool    *pipeline.Pool
	log     *logger.Logger
	bufSize int

	// wsMu guards concurrent writes to the WebSocket connection.
	// gorilla/websocket connections allow one concurrent writer only.
	wsMu sync.Mutex
}

// New constructs a Tunnel. id should be a unique string (e.g. a UUID) used
// for log correlation.
func New(
	id string,
	ws *websocket.Conn,
	tcp net.Conn,
	pool *pipeline.Pool,
	log *logger.Logger,
	bufSize int,
) *Tunnel {
	return &Tunnel{
		id:      id,
		ws:      ws,
		tcp:     tcp,
		pool:    pool,
		log:     log.With("tunnel_id", id),
		bufSize: bufSize,
	}
}

// Serve runs the upstream and downstream goroutines concurrently and blocks
// until both have exited. When either goroutine encounters an error, it closes
// both network connections; the resulting I/O error causes the other goroutine
// to exit cleanly without a separate signalling mechanism.
func (t *Tunnel) Serve() {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer t.closeConns()
		t.upstream()
	}()

	go func() {
		defer wg.Done()
		defer t.closeConns()
		t.downstream()
	}()

	wg.Wait()
	t.log.Info("tunnel closed")
}

// closeConns closes both network connections. Safe to call multiple times.
func (t *Tunnel) closeConns() {
	t.ws.Close()
	t.tcp.Close()
}

// upstream is the WS → TCP goroutine. It reads WebSocket frames from the client
// and writes the raw bytes to the TCP backend. After each successful forwarding
// it submits the bytes to the observability pool — this submit is non-blocking.
func (t *Tunnel) upstream() {
	t.log.Debug("upstream goroutine started")

	for {
		// ReadMessage blocks until a full frame arrives or the connection closes.
		_, msg, err := t.ws.ReadMessage()
		if err != nil {
			t.log.Debug("upstream read error", "err", err)
			return
		}

		if _, err := t.tcp.Write(msg); err != nil {
			t.log.Debug("upstream TCP write error", "err", err)
			return
		}

		// Non-blocking tee: submit a copy to the pipeline worker pool.
		// The Submit call returns immediately whether or not the pool accepts it.
		t.pool.Submit(msg)
	}
}

// downstream is the TCP → WS goroutine. It reads raw bytes from the TCP backend
// and wraps them in binary WebSocket frames for delivery to the client.
func (t *Tunnel) downstream() {
	t.log.Debug("downstream goroutine started")

	buf := make([]byte, t.bufSize)

	for {
		n, err := t.tcp.Read(buf)
		if err != nil {
			t.log.Debug("downstream TCP read error", "err", err)
			return
		}

		// Take a copy before handing to the WebSocket writer and the pool so
		// the buffer can be reused immediately in the next iteration.
		data := make([]byte, n)
		copy(data, buf[:n])

		t.wsMu.Lock()
		writeErr := t.ws.WriteMessage(websocket.BinaryMessage, data)
		t.wsMu.Unlock()

		if writeErr != nil {
			t.log.Debug("downstream WS write error", "err", writeErr)
			return
		}

		t.pool.Submit(data)
	}
}
