package storage

import (
	"context"
	"sync"
)

// MemoryRepository is a fast, race-free in-memory Repository implementation
// used for unit tests and local single-node execution.
type MemoryRepository struct {
	mu          sync.RWMutex
	state       *AppState
	hasState    bool
	clickstream []ClickstreamRecord
}

// NewMemoryRepository creates an initialized MemoryRepository populated with DefaultAppState.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		state:       DefaultAppState(),
		hasState:    true,
		clickstream: make([]ClickstreamRecord, 0, 64),
	}
}

// LoadState returns a deep copy of the currently stored AppState.
func (m *MemoryRepository) LoadState(_ context.Context) (*AppState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.hasState || m.state == nil {
		return nil, ErrNotFound
	}
	return m.state.Clone(), nil
}

// SaveState persists a deep copy of state under write lock.
func (m *MemoryRepository) SaveState(_ context.Context, state *AppState) error {
	if state == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = state.Clone()
	m.hasState = true
	return nil
}

// AppendClickstream stores clickstream records in a bounded slice (max 500 entries).
func (m *MemoryRepository) AppendClickstream(_ context.Context, record ClickstreamRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if record.Metadata != nil {
		meta := make(map[string]string, len(record.Metadata))
		for k, v := range record.Metadata {
			meta[k] = v
		}
		record.Metadata = meta
	}
	if len(m.clickstream) >= 500 {
		copy(m.clickstream, m.clickstream[1:])
		m.clickstream[len(m.clickstream)-1] = record
	} else {
		m.clickstream = append(m.clickstream, record)
	}
	return nil
}

// ClickstreamCount returns the number of stored clickstream records (useful for tests/diagnostics).
func (m *MemoryRepository) ClickstreamCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clickstream)
}

// Mode identifies the storage driver.
func (m *MemoryRepository) Mode() string {
	return "memory"
}

// Healthy always returns true for MemoryRepository.
func (m *MemoryRepository) Healthy(_ context.Context) bool {
	return true
}

// Close satisfies Repository without external handles.
func (m *MemoryRepository) Close() error {
	return nil
}
