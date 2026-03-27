package storage

import (
	"fmt"
	"sync"
)

// Manager holds the active storage driver and allows swapping it at runtime.
// It is intentionally simple: callers should resolve tokens/env before Set.
type Manager struct {
	mu  sync.RWMutex
	st  Storage
	cfg DriverConfig
}

func NewManager(st Storage, cfg DriverConfig) *Manager {
	return &Manager{st: st, cfg: cfg}
}

func (m *Manager) Get() Storage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.st
}

func (m *Manager) GetConfig() DriverConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

func (m *Manager) Set(cfg DriverConfig) error {
	st, err := New(cfg)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.st = st
	m.cfg = cfg
	return nil
}

func (m *Manager) SetWithResolvedEnv(cfg DriverConfig, resolve func(DriverConfig) DriverConfig) error {
	if resolve == nil {
		return fmt.Errorf("resolve func is nil")
	}
	return m.Set(resolve(cfg))
}

