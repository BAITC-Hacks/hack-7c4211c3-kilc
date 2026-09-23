package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

const testAPIKey = "sk-test-secret-key-123"

type capturedRequest struct {
	auth string
	body chatRequest
}

// modelServer answers every chat request with the given status and content.
func modelServer(t *testing.T, status int, content string, captured *capturedRequest) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if captured != nil {
			captured.auth = r.Header.Get("Authorization")
			if err := json.NewDecoder(r.Body).Decode(&captured.body); err != nil {
				t.Errorf("decode request: %v", err)
			}
		}
		w.WriteHeader(status)
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func fallbackFor(t *testing.T, url string, timeout time.Duration) (Fallback, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	model := Model{BaseURL: url, APIKey: testAPIKey, Name: "test-model", HTTP: &http.Client{Timeout: timeout}}
	t.Cleanup(func() {
		if strings.Contains(logs.String(), testAPIKey) {
			t.Errorf("log contains API key: %s", logs.String())
		}
	})
	return Fallback{Primary: model, Stub: NewStub(), Log: log.New(&logs, "", 0)}, &logs
}

func TestModelQuestionsSuccess(t *testing.T) {
	var captured capturedRequest
	content := `{"questions":[{"field":"data","question":"Какие данные?"},{"field":"users","question":"Кто?"},{"field":"need","question":"Зачем?"}]}`
	server := modelServer(t, http.StatusOK, content, &captured)
	client, _ := fallbackFor(t, server.URL, time.Second)

	result, err := client.Questions(context.Background(), "Хотим автоматизировать заявки")
	if err != nil {
		t.Fatalf("Questions returned error: %v", err)
	}
	if result.Source != SourceModel || len(result.Questions) < 3 {
		t.Fatalf("result = %+v, want model source and at least 3 questions", result)
	}
	if captured.auth != "Bearer "+testAPIKey {
		t.Fatalf("authorization header = %q", captured.auth)
	}
	if captured.body.Model != "test-model" || captured.body.ResponseFormat["type"] != "json_object" {
		t.Fatalf("request body = %+v", captured.body)
	}
	if len(captured.body.Messages) != 2 || captured.body.Messages[0].Content != QuestionsPrompt {
		t.Fatalf("messages = %+v", captured.body.Messages)
	}
	if !strings.Contains(captured.body.Messages[1].Content, `"draft":"Хотим автоматизировать заявки"`) {
		t.Fatalf("user payload = %s", captured.body.Messages[1].Content)
	}
}

func TestFallbackOnInvalidContent(t *testing.T) {
	server := modelServer(t, http.StatusOK, "not json", nil)
	client, logs := fallbackFor(t, server.URL, time.Second)

	result, err := client.Questions(context.Background(), "Хотим автоматизировать заявки")
	if err != nil {
		t.Fatalf("Questions returned error: %v", err)
	}
	if result.Source != SourceStub {
		t.Fatalf("source = %q, want stub", result.Source)
	}
	if !strings.Contains(logs.String(), "ai: переход на заглушку") {
		t.Fatalf("fallback not logged: %s", logs.String())
	}
}

func TestFallbackOnServerError(t *testing.T) {
	server := modelServer(t, http.StatusInternalServerError, "", nil)
	client, logs := fallbackFor(t, server.URL, time.Second)

	result, err := client.Card(context.Background(), "Нужна CRM", nil)
	if err != nil {
		t.Fatalf("Card returned error: %v", err)
	}
	if result.Source != SourceStub {
		t.Fatalf("source = %q, want stub", result.Source)
	}
	if !strings.Contains(logs.String(), "500") {
		t.Fatalf("status not logged: %s", logs.String())
	}
}

func TestFallbackOnTimeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(11 * time.Second):
		case <-release:
		}
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })
	client, _ := fallbackFor(t, server.URL, 100*time.Millisecond)

	result, err := client.Questions(context.Background(), "Хотим автоматизировать заявки")
	if err != nil {
		t.Fatalf("Questions returned error: %v", err)
	}
	if result.Source != SourceStub {
		t.Fatalf("source = %q, want stub", result.Source)
	}
}

func TestFallbackKeepsCallerCancellation(t *testing.T) {
	server := modelServer(t, http.StatusOK, "not json", nil)
	client, _ := fallbackFor(t, server.URL, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Questions(ctx, "x"); err == nil {
		t.Fatal("want error for cancelled context")
	}
}

func TestModelCardDropsInventedNumber(t *testing.T) {
	var captured capturedRequest
	content := `{"card":{"title":"Нужна CRM","need":"рост продаж на 40%","data":"Excel"}}`
	server := modelServer(t, http.StatusOK, content, &captured)
	client, logs := fallbackFor(t, server.URL, time.Second)
	qa := []QA{{Field: "data", Question: "Какие данные?", Answer: "Excel"}}

	result, err := client.Card(context.Background(), "Нужна CRM", qa)
	if err != nil {
		t.Fatalf("Card returned error: %v", err)
	}
	if result.Source != SourceModel || result.Card.Need != "" || result.Card.Data != "Excel" {
		t.Fatalf("result = %+v", result)
	}
	if !strings.Contains(logs.String(), "need: текст не совпадает") {
		t.Fatalf("dropped item not logged: %s", logs.String())
	}
	if !strings.Contains(captured.body.Messages[1].Content, `"qa":[{"field":"data"`) {
		t.Fatalf("user payload = %s", captured.body.Messages[1].Content)
	}
}

func TestCardFallsBackWhenNoGroundedTextRemains(t *testing.T) {
	for _, content := range []string{
		`{"card":{}}`,
		`{"card":{"unknown":"anything"}}`,
		`{"card":{"category":"crm"}}`,
		`{"card":{"need":"Компания использует SAP"}}`,
		`{"card":{"context":true}}`,
	} {
		t.Run(content, func(t *testing.T) {
			server := modelServer(t, http.StatusOK, content, nil)
			client, logs := fallbackFor(t, server.URL, time.Second)
			result, err := client.Card(context.Background(), "Нужна CRM", []QA{{Field: "data", Answer: "Excel"}})
			if err != nil || result.Source != SourceStub || result.Card.Context != "Нужна CRM" || result.Card.Data != "Excel" {
				t.Fatalf("want source-backed stub: %+v err=%v", result, err)
			}
			if !strings.Contains(logs.String(), "переход на заглушку") {
				t.Fatalf("fallback not logged: %s", logs.String())
			}
		})
	}
}

func TestNewSelectsClient(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		if strings.Contains(logs.String(), testAPIKey) {
			t.Errorf("log contains API key: %s", logs.String())
		}
	})

	t.Setenv("STUB", "")
	os.Unsetenv("STUB")
	t.Setenv("AI_API_KEY", testAPIKey)
	if _, ok := New().(Stub); !ok {
		t.Fatal("STUB unset: want Stub")
	}

	t.Setenv("STUB", "0")
	t.Setenv("AI_API_KEY", "")
	if _, ok := New().(Stub); !ok {
		t.Fatal("empty key: want Stub")
	}

	t.Setenv("AI_API_KEY", testAPIKey)
	t.Setenv("AI_MODEL", "")
	fallback, ok := New().(Fallback)
	if !ok {
		t.Fatal("STUB=0 with key: want Fallback")
	}
	model, ok := fallback.Primary.(Model)
	if !ok || model.Name != defaultModel || model.BaseURL != defaultBaseURL {
		t.Fatalf("primary = %+v", fallback.Primary)
	}
	if !strings.Contains(logs.String(), "ai: режим заглушки") || !strings.Contains(logs.String(), "ai: модель deepseek-chat") {
		t.Fatalf("mode not logged: %s", logs.String())
	}
}
