package session

import (
	"sync"
)

// Session represents a user session with isolated state
type Session struct {
	mu   sync.Mutex
	data map[string]interface{}
}

// Manager handles session creation and lifecycle
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewManager creates a new session manager
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

// GetOrCreateSession retrieves existing session or creates new one
func (m *Manager) GetOrCreateSession(token string) *Session {
	m.mu.RLock()
	session, exists := m.sessions[token]
	m.mu.RUnlock()
	
	if exists {
		return session
	}
	
	// Create new session
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Double-check after acquiring write lock
	if session, exists = m.sessions[token]; exists {
		return session
	}
	
	session = &Session{
		data: make(map[string]interface{}),
	}
	m.sessions[token] = session
	
	return session
}

// GetSession retrieves a session without creating
func (m *Manager) GetSession(token string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[token]
}

// ResetSession clears all data for a token
func (m *Manager) ResetSession(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	delete(m.sessions, token)
}

// Session methods

// Execute runs a function within the session's lock
func (s *Session) Execute(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn()
}

// SetData stores data in the session
func (s *Session) SetData(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

// GetData retrieves data from the session
func (s *Session) GetData(key string) interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data[key]
}

// Clear removes all data from the session
func (s *Session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string]interface{})
}