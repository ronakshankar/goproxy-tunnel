package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// maxSegmentSize is the maximum number of compressed bytes written to a single
// log segment before rotation. 256 MiB keeps individual files manageable.
const maxSegmentSize int64 = 256 * 1024 * 1024

// RotatingWriter writes bytes to time-stamped files under a configured directory.
// When the current file exceeds maxSegmentSize it is closed and a new segment is
// opened transparently.
//
// RotatingWriter is safe for concurrent use; multiple pipeline workers share one
// instance per worker (each worker creates its own).
type RotatingWriter struct {
	dir     string
	mu      sync.Mutex
	file    *os.File
	written int64
}

// NewRotatingWriter opens a fresh log segment in dir, creating the directory if
// it does not exist. The caller must call Close() when done.
func NewRotatingWriter(dir string) (*RotatingWriter, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log directory %q: %w", dir, err)
	}

	w := &RotatingWriter{dir: dir}
	if err := w.rotate(); err != nil {
		return nil, err
	}
	return w, nil
}

// Write appends p to the current segment, rotating to a new file if the segment
// size limit would be exceeded.
func (w *RotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.written+int64(len(p)) > maxSegmentSize {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}

	n, err := w.file.Write(p)
	w.written += int64(n)
	return n, err
}

// Close flushes and closes the current segment.
func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		return err
	}
	return nil
}

// rotate closes the current file (if any) and opens a new time-stamped segment.
// Must be called with w.mu held.
func (w *RotatingWriter) rotate() error {
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return fmt.Errorf("close log segment: %w", err)
		}
	}

	// Segments are named with millisecond precision to avoid collisions when
	// multiple workers rotate near-simultaneously.
	ts := time.Now().UTC().Format("20060102-150405.000")
	name := filepath.Join(w.dir, fmt.Sprintf("wt-%s.gz", ts))

	f, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("create log segment %q: %w", name, err)
	}

	w.file = f
	w.written = 0
	return nil
}
