package ai

import (
	"context"
	"errors"
	"os"
	"strings"
)

// ChatTurn is one message of a helper conversation.
type ChatTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Chatter answers free-form helper questions in plain text.
type Chatter interface {
	Chat(ctx context.Context, system string, turns []ChatTurn) (string, error)
}

// NewChat returns the model chatter, or nil when the stub is selected:
// the helper has no offline answers, so the UI reports it as unavailable.
func NewChat() Chatter {
	key := os.Getenv("AI_API_KEY")
	if os.Getenv("STUB") != "0" || key == "" {
		return nil
	}
	return Model{BaseURL: envOr("AI_BASE_URL", defaultBaseURL), APIKey: key, Name: envOr("AI_MODEL", defaultModel)}
}

// Chat sends the system prompt and turns and returns the model's text reply.
func (m Model) Chat(ctx context.Context, system string, turns []ChatTurn) (string, error) {
	messages := []chatMessage{{Role: "system", Content: system}}
	for _, turn := range turns {
		messages = append(messages, chatMessage{Role: turn.Role, Content: turn.Content})
	}
	raw, err := m.send(ctx, chatRequest{Model: m.Name, Temperature: 0.3, Messages: messages})
	if err != nil {
		return "", err
	}
	reply := strings.TrimSpace(string(raw))
	if reply == "" {
		return "", errors.New("ai: модель вернула пустой ответ помощника")
	}
	return reply, nil
}
