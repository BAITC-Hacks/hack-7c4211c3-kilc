package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	modelTimeout        = 10 * time.Second
	maxResponseBytes    = 1 << 20
	chatCompletionsPath = "/chat/completions"
)

// Model calls an OpenAI-compatible chat completions API.
type Model struct {
	BaseURL string
	APIKey  string
	Name    string
	HTTP    *http.Client
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string            `json:"model"`
	Temperature    float64           `json:"temperature"`
	ResponseFormat map[string]string `json:"response_format"`
	Messages       []chatMessage     `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Questions asks the model for clarifying questions about the draft.
func (m Model) Questions(ctx context.Context, draft string) (QuestionsResult, error) {
	raw, err := m.complete(ctx, QuestionsPrompt, map[string]any{"draft": draft})
	if err != nil {
		return QuestionsResult{}, err
	}
	questions, err := ParseQuestions(raw)
	if err != nil {
		return QuestionsResult{}, err
	}
	return QuestionsResult{Questions: questions, Source: SourceModel}, nil
}

// Card asks the model to build a card from the draft and answers.
func (m Model) Card(ctx context.Context, draft string, qa []QA) (CardResult, error) {
	result, _, err := m.cardWithDropped(ctx, draft, qa)
	return result, err
}

func (m Model) cardWithDropped(ctx context.Context, draft string, qa []QA) (CardResult, []string, error) {
	if qa == nil {
		qa = []QA{}
	}
	raw, err := m.complete(ctx, CardPrompt, map[string]any{"draft": draft, "qa": qa})
	if err != nil {
		return CardResult{}, nil, err
	}
	card, dropped, err := ParseCard(raw, draft, qa)
	if err != nil {
		return CardResult{}, nil, err
	}
	return CardResult{Card: card, Source: SourceModel}, dropped, nil
}

// complete sends one chat request and returns the message content.
// Errors never include the API key.
func (m Model) complete(ctx context.Context, prompt string, payload any) ([]byte, error) {
	user, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("ai: кодирование запроса: %w", err)
	}
	body, err := json.Marshal(chatRequest{
		Model:          m.Name,
		Temperature:    0,
		ResponseFormat: map[string]string{"type": "json_object"},
		Messages: []chatMessage{
			{Role: "system", Content: prompt},
			{Role: "user", Content: string(user)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ai: кодирование запроса: %w", err)
	}
	url := strings.TrimRight(m.BaseURL, "/") + chatCompletionsPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ai: создание запроса: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+m.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := m.HTTP
	if client == nil {
		client = &http.Client{Timeout: modelTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ai: запрос к модели: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("ai: чтение ответа модели: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("ai: модель ответила статусом %d", resp.StatusCode)
	}
	var decoded chatResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("%w: ответ API не JSON: %v", ErrInvalidOutput, err)
	}
	if len(decoded.Choices) == 0 {
		return nil, errors.New("ai: модель вернула пустой список choices")
	}
	return []byte(decoded.Choices[0].Message.Content), nil
}
