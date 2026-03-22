// Package config defines the runtime configuration contract for WireTunnel.
package config

import (
	"fmt"
	"net"
)

// Config holds all tuneable parameters for the proxy. It is populated by the
// CLI layer and passed by value into each subsystem constructor.
type Config struct {
	// ListenPort is the TCP port the HTTP/WebSocket server binds to.
	ListenPort int

	// TargetAddr is the host:port of the backend TCP service.
	TargetAddr string

	// WorkerCount controls how many goroutines process the observability pipeline.
	WorkerCount int

	// LogDir is the filesystem path where compressed log segments are written.
	LogDir string

	// BufferSize is the per-connection read buffer size in bytes for the TCP → WS path.
	BufferSize int

	// PipelineQueueDepth is the capacity of the buffered channel feeding the worker pool.
	// Excess observations are dropped rather than blocking the network path.
	PipelineQueueDepth int
}

// Validate performs semantic validation of the configuration values.
func (c *Config) Validate() error {
	if c.ListenPort <= 0 || c.ListenPort > 65535 {
		return fmt.Errorf("listen port %d is out of range [1, 65535]", c.ListenPort)
	}

	if _, err := net.ResolveTCPAddr("tcp", c.TargetAddr); err != nil {
		return fmt.Errorf("invalid target address %q: %w", c.TargetAddr, err)
	}

	if c.WorkerCount <= 0 {
		return fmt.Errorf("worker count must be positive, got %d", c.WorkerCount)
	}

	if c.BufferSize <= 0 {
		return fmt.Errorf("buffer size must be positive, got %d", c.BufferSize)
	}

	if c.PipelineQueueDepth <= 0 {
		return fmt.Errorf("pipeline queue depth must be positive, got %d", c.PipelineQueueDepth)
	}

	return nil
}
