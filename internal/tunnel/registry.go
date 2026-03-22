package tunnel

import (
	"sync"
	"sync/atomic"
)

// Registry tracks all currently active Tunnel instances. It is safe for
// concurrent use and is designed for high read/low write access patterns
// (many goroutines querying Count, infrequent Register/Deregister calls).
type Registry struct {
	tunnels sync.Map
	count   atomic.Int64
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register stores t under id. Panics if id is already registered to encourage
// callers to use unique identifiers.
func (r *Registry) Register(id string, t *Tunnel) {
	if _, loaded := r.tunnels.LoadOrStore(id, t); loaded {
		panic("tunnel: duplicate registration for id " + id)
	}
	r.count.Add(1)
}

// Deregister removes the tunnel with the given id. It is a no-op if id is not
// present, making it safe to call from deferred cleanup code.
func (r *Registry) Deregister(id string) {
	if _, loaded := r.tunnels.LoadAndDelete(id); loaded {
		r.count.Add(-1)
	}
}

// Count returns the number of currently active tunnels. The value is maintained
// by an atomic counter rather than iterating the map, so it is O(1).
func (r *Registry) Count() int64 {
	return r.count.Load()
}
