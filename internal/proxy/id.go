package proxy

import (
	"fmt"
	"sync/atomic"
)

// idGenerator produces monotonically increasing, zero-padded hex connection
// identifiers. It is safe for concurrent use without a mutex.
type idGenerator struct {
	counter atomic.Uint64
}

func newIDGenerator() *idGenerator {
	return &idGenerator{}
}

// Next returns the next unique identifier in the form "conn-000000000000001".
func (g *idGenerator) Next() string {
	n := g.counter.Add(1)
	return fmt.Sprintf("conn-%015d", n)
}
