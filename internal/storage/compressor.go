// Package storage handles compression and durable log storage for the
// observability pipeline. It is intentionally decoupled from the network layer
// so the CPU-intensive gzip work never touches the hot data path.
package storage

import (
	"compress/gzip"
	"io"
)

// Compressor wraps a destination writer and compresses incoming data with gzip.
// Each Compressor owns its own gzip.Writer; it is NOT safe for concurrent use.
type Compressor struct {
	gz *gzip.Writer
	w  io.Writer
}

// NewCompressor wraps dst with a gzip compressor. Callers must call Close()
// to flush any pending gzip data and write the gzip footer.
func NewCompressor(dst io.Writer) (*Compressor, error) {
	gz, err := gzip.NewWriterLevel(dst, gzip.BestSpeed)
	if err != nil {
		return nil, err
	}
	return &Compressor{gz: gz, w: dst}, nil
}

// Write compresses p and writes the compressed bytes to the underlying writer.
func (c *Compressor) Write(p []byte) (int, error) {
	return c.gz.Write(p)
}

// Flush flushes any buffered compressed data to the underlying writer without
// closing the gzip stream. Use this for chunked, streaming writes.
func (c *Compressor) Flush() error {
	return c.gz.Flush()
}

// Close flushes remaining data, writes the gzip footer, and closes the gzip
// stream. It does NOT close the underlying writer.
func (c *Compressor) Close() error {
	return c.gz.Close()
}

// CompressCopy reads all bytes from src, compresses them with gzip at best-speed
// level, and writes the compressed output to dst. It is a convenience wrapper
// for the common case of compressing a discrete byte stream.
//
// io.EOF and io.ErrClosedPipe from src are treated as clean end-of-stream and
// are not returned as errors, matching the expected lifecycle of an io.Pipe reader.
func CompressCopy(dst io.Writer, src io.Reader) error {
	c, err := NewCompressor(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(c, src); err != nil && err != io.ErrClosedPipe {
		c.Close()
		return err
	}

	return c.Close()
}
