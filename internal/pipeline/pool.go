// Package pipeline implements the non-blocking observability layer described in
// the WireTunnel architecture. Traffic bytes are submitted to a buffered channel
// and consumed by a fixed worker pool that compresses and persists the data.
// The network path is never stalled — if the pool is saturated, the sample is
// silently dropped rather than blocking the caller.
package pipeline

import (
	"bytes"
	"sync"

	"github.com/wiretunnel/wiretunnel/internal/storage"
	"github.com/wiretunnel/wiretunnel/pkg/logger"
)

// sample is a snapshot of bytes captured from the proxy data path.
type sample struct {
	data []byte
}

// Pool manages a fixed number of goroutines that read traffic samples from a
// shared buffered channel, compress each sample with gzip, and write the result
// to rotating log files on disk.
type Pool struct {
	jobs    chan sample
	workers int
	logDir  string
	log     *logger.Logger
	wg      sync.WaitGroup
	once    sync.Once
	quit    chan struct{}
}

// NewPool allocates a Pool. Call Start() before submitting any work and Stop()
// to drain in-flight jobs and release resources.
func NewPool(workers, queueDepth int, logDir string, log *logger.Logger) *Pool {
	return &Pool{
		jobs:    make(chan sample, queueDepth),
		workers: workers,
		logDir:  logDir,
		log:     log,
		quit:    make(chan struct{}),
	}
}

// Start launches the worker goroutines. It must be called exactly once.
func (p *Pool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.runWorker(i)
	}
	p.log.Info("observability pipeline started", "workers", p.workers, "queue_depth", cap(p.jobs))
}

// Stop signals workers to exit after draining any queued samples and waits for
// all goroutines to finish. It is safe to call Stop multiple times.
func (p *Pool) Stop() {
	p.once.Do(func() {
		close(p.quit)
		p.wg.Wait()
		p.log.Info("observability pipeline stopped")
	})
}

// Submit enqueues a copy of data for background compression and storage.
// If the job queue is full, the sample is discarded and a warning is logged —
// the caller is never blocked.
func (p *Pool) Submit(data []byte) {
	if len(data) == 0 {
		return
	}

	// Copy the slice so the caller can reuse its buffer immediately.
	cp := make([]byte, len(data))
	copy(cp, data)

	select {
	case p.jobs <- sample{data: cp}:
	default:
		p.log.Warn("observability pipeline saturated; sample dropped",
			"bytes", len(data))
	}
}

// runWorker is the goroutine body for a single pool worker. Each worker owns
// its own RotatingWriter so there is no contention on file handles.
func (p *Pool) runWorker(id int) {
	defer p.wg.Done()

	w, err := storage.NewRotatingWriter(p.logDir)
	if err != nil {
		p.log.Error("worker failed to open log writer", "worker_id", id, "err", err)
		// Drain the queue without writing rather than stalling other workers.
		for {
			select {
			case <-p.jobs:
			case <-p.quit:
				return
			}
		}
	}
	defer w.Close()

	p.log.Debug("observability worker started", "worker_id", id)

	for {
		select {
		case s := <-p.jobs:
			if err := storage.CompressCopy(w, bytes.NewReader(s.data)); err != nil {
				p.log.Warn("compression error", "worker_id", id, "err", err)
			}

		case <-p.quit:
			// Drain any remaining samples before exiting so we don't lose data
			// that was already queued when Stop() was called.
			for {
				select {
				case s := <-p.jobs:
					storage.CompressCopy(w, bytes.NewReader(s.data)) //nolint:errcheck
				default:
					return
				}
			}
		}
	}
}
