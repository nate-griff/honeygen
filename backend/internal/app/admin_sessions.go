package app

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
)

type AdminSessionManager struct {
	mu       sync.RWMutex
	sessions map[string]struct{}
}

func NewAdminSessionManager() *AdminSessionManager {
	return &AdminSessionManager{sessions: make(map[string]struct{})}
}

func (m *AdminSessionManager) Create() (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate admin session token: %w", err)
	}

	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	m.mu.Lock()
	m.sessions[token] = struct{}{}
	m.mu.Unlock()
	return token, nil
}

func (m *AdminSessionManager) Valid(token string) bool {
	m.mu.RLock()
	_, ok := m.sessions[token]
	m.mu.RUnlock()
	return ok
}

func (m *AdminSessionManager) Delete(token string) {
	m.mu.Lock()
	delete(m.sessions, token)
	m.mu.Unlock()
}
