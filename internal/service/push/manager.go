package push

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
)

var ErrManagerClosed = errors.New("push manager is closed")

// Manager serializes candidate publication, retirement, and shutdown.
type Manager struct {
	mu      sync.Mutex
	current *Generation
	closed  bool
}

func NewManager() *Manager { return &Manager{} }

// Replace validates before starting workers, publishes the complete snapshot,
// then drains the old generation without blocking any handler submitter.
func (m *Manager) Replace(configs map[string]EndpointConfig, publish func(*Generation)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrManagerClosed
	}
	for name, config := range configs {
		if config.Sink == nil {
			return fmt.Errorf("push endpoint %q: sink is required", name)
		}
	}
	candidate := NewGeneration(configs)
	previous := m.current
	publish(candidate)
	m.current = candidate
	added, removed := endpointDiff(previous, candidate)
	slog.Info("Push generation reloaded", "added", added, "removed", removed)
	if previous != nil {
		previous.Retire()
	}
	return nil
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	m.closed = true
	if m.current != nil {
		m.current.Retire()
	}
}

func endpointDiff(old, next *Generation) (added, removed []string) {
	for name := range next.endpoints {
		if old == nil || old.endpoints[name] == nil {
			added = append(added, name)
		}
	}
	if old != nil {
		for name := range old.endpoints {
			if next.endpoints[name] == nil {
				removed = append(removed, name)
			}
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return
}
