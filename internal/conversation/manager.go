package conversation

import (
	"sync"

	"github.com/razenry/mcqueen-ai/internal/ai"
)

type Manager struct {
	mu       sync.Mutex
	sessions map[string][]ai.Message
	maxTurns int
}

func NewManager(maxTurns int) *Manager {
	return &Manager{
		sessions: make(map[string][]ai.Message),
		maxTurns: maxTurns,
	}
}

func (m *Manager) Get(key string) []ai.Message {
	m.mu.Lock()
	defer m.mu.Unlock()

	history := m.sessions[key]

	// Return a copy to avoid external mutation.
	result := make([]ai.Message, len(history))
	copy(result, history)

	return result
}

func (m *Manager) Add(
	key string,
	userMessage string,
	assistantMessage string,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	history := m.sessions[key]

	history = append(history,
		ai.Message{
			Role:    "user",
			Content: userMessage,
		},
		ai.Message{
			Role:    "assistant",
			Content: assistantMessage,
		},
	)

	maxMessages := m.maxTurns * 2

	if len(history) > maxMessages {
		history = history[len(history)-maxMessages:]
	}

	m.sessions[key] = history
}

func (m *Manager) Reset(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, key)
}
