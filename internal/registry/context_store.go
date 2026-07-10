package registry

import "sync"

// ContextStore is a thread-safe key-value store, mirroring Node-RED's
// flow/global context (`flow.get`/`flow.set`, `global.get`/`global.set`).
// The engine owns one global ContextStore shared by every flow; each active
// flow owns its own, private to that flow. Nodes reach both through
// NodeRuntime (see runtime.go).
type ContextStore struct {
    mu   sync.RWMutex
    data map[string]interface{}
}

// NewContextStore creates an empty ContextStore.
func NewContextStore() *ContextStore {
    return &ContextStore{data: make(map[string]interface{})}
}

// Get returns the value stored under key, and whether it was present.
func (s *ContextStore) Get(key string) (interface{}, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    val, ok := s.data[key]
    return val, ok
}

// Set stores value under key, overwriting any existing value.
func (s *ContextStore) Set(key string, value interface{}) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.data[key] = value
}

// Delete removes key from the store. It is a no-op if key is not present.
func (s *ContextStore) Delete(key string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    delete(s.data, key)
}

// Keys returns the currently stored keys, in no particular order.
func (s *ContextStore) Keys() []string {
    s.mu.RLock()
    defer s.mu.RUnlock()
    keys := make([]string, 0, len(s.data))
    for k := range s.data {
        keys = append(keys, k)
    }
    return keys
}
