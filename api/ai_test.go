package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/ai"
	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/store"
)

func callAI(t *testing.T, mux *http.ServeMux, path, body string, status int) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != status {
		t.Fatalf("%s: status %d, want %d: %s", path, w.Code, status, w.Body.String())
	}
	return w
}

func TestAIStubWorkflow(t *testing.T) {
	t.Setenv("STUB", "1")
	t.Setenv("AI_API_KEY", "")
	schema, err := os.ReadFile("../schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "workflow.db"), string(schema))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	mux := http.NewServeMux()
	RegisterAI(mux, ai.New())
	RegisterFrontend(mux, st)
	draft := "Нужно сократить ручную обработку заявок клиентов."
	encoded, _ := json.Marshal(map[string]string{"draft_text": draft})
	w := callAI(t, mux, "/api/ai/questions", string(encoded), 200)
	var questions struct {
		Questions []ai.Question `json:"questions"`
		Source    string        `json:"source"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &questions); err != nil {
		t.Fatal(err)
	}
	if len(questions.Questions) < 3 || questions.Source != ai.SourceStub {
		t.Fatalf("questions = %+v", questions)
	}
	qa := make([]store.QA, len(questions.Questions))
	for i, q := range questions.Questions {
		qa[i] = store.QA{Field: q.Field, Question: q.Question}
		if q.Field == "data" {
			qa[i].Answer = "Обезличенная таблица заявок за прошлый месяц"
		}
	}
	encoded, _ = json.Marshal(map[string]any{"draft_text": draft, "qa": qa})
	w = callAI(t, mux, "/api/ai/card", string(encoded), 200)
	var generated struct {
		Card   ai.Card `json:"card"`
		Source string  `json:"source"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &generated); err != nil {
		t.Fatal(err)
	}
	if generated.Source != ai.SourceStub || generated.Card.Context != draft || generated.Card.Data != qa[0].Answer || generated.Card.Category != "" {
		t.Fatalf("generated = %+v", generated)
	}
	var count int
	if err := st.DB.QueryRow("SELECT COUNT(*) FROM tasks").Scan(&count); err != nil || count != 0 {
		t.Fatalf("generation must not save tasks: count=%d err=%v", count, err)
	}
	cardJSON, _ := json.Marshal(generated.Card)
	var body taskBody
	if err := json.Unmarshal(cardJSON, &body); err != nil {
		t.Fatal(err)
	}
	body.DraftText, body.QA = draft, qa
	encoded, _ = json.Marshal(body)
	w = callAI(t, mux, "/api/tasks", string(encoded), 201)
	var saved taskView
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	stored, err := st.GetTask(context.Background(), saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.DraftText != draft || len(stored.QA) != len(qa) || stored.QA[0] != qa[0] || len(stored.Confirmed) != 0 || stored.Status != store.StatusDraft || saved.Rating.Total != 0 || saved.Rating.Potential == 0 {
		t.Fatalf("unexpected saved card: %+v; rating=%+v", stored, saved.Rating)
	}
	callAI(t, mux, "/api/tasks/"+strconv.FormatInt(saved.ID, 10)+"/publish", "{}", 400)
}

func TestAIRequestValidation(t *testing.T) {
	mux := http.NewServeMux()
	RegisterAI(mux, ai.NewStub())
	for _, tt := range []struct{ name, path, body string }{
		{"empty draft", "questions", `{"draft_text":"  "}`},
		{"unknown property", "questions", `{"draft_text":"CRM","confirmed":true}`},
		{"trailing JSON", "questions", `{"draft_text":"CRM"}{}`},
		{"null body", "questions", `null`},
		{"too long", "questions", `{"draft_text":"` + strings.Repeat("я", 4001) + `"}`},
		{"no questions", "card", `{"draft_text":"CRM","qa":[]}`},
		{"invalid field", "card", `{"draft_text":"CRM","qa":[{"field":"status","question":"?"},{"field":"need","question":"?"},{"field":"users","question":"?"}]}`},
		{"duplicate field", "card", `{"draft_text":"CRM","qa":[{"field":"need","question":"?"},{"field":"need","question":"?"},{"field":"users","question":"?"}]}`},
		{"empty question", "card", `{"draft_text":"CRM","qa":[{"field":"data","question":" "},{"field":"need","question":"?"},{"field":"users","question":"?"}]}`},
		{"answer too long", "card", `{"draft_text":"CRM","qa":[{"field":"data","question":"?","answer":"` + strings.Repeat("я", 2001) + `"},{"field":"need","question":"?"},{"field":"users","question":"?"}]}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := callAI(t, mux, "/api/ai/"+tt.path, tt.body, 400)
			var result map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result["error"] == "" {
				t.Fatalf("want readable error: %s", w.Body.String())
			}
		})
	}
	r := httptest.NewRequest("POST", "/api/ai/questions", strings.NewReader(`{"draft_text":"CRM"}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 415 {
		t.Fatalf("missing content type: %d", w.Code)
	}
	callAI(t, mux, "/api/ai/questions", `{"draft_text":"`+strings.Repeat("x", 65536)+`"}`, 413)
}

type failingAI struct{}

func (failingAI) Questions(context.Context, string) (ai.QuestionsResult, error) {
	return ai.QuestionsResult{}, errors.New("unavailable")
}
func (failingAI) Card(context.Context, string, []ai.QA) (ai.CardResult, error) {
	return ai.CardResult{}, errors.New("unavailable")
}

func TestAIReportsFailure(t *testing.T) {
	mux := http.NewServeMux()
	RegisterAI(mux, failingAI{})
	callAI(t, mux, "/api/ai/questions", `{"draft_text":"CRM"}`, 502)
	callAI(t, mux, "/api/ai/card", `{"draft_text":"CRM","qa":[{"field":"data","question":"?"},{"field":"need","question":"?"},{"field":"users","question":"?"}]}`, 502)
}

func TestAIHTTPFallbackOnMalformedModelOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"not JSON"}}]}`))
	}))
	defer server.Close()
	var logs bytes.Buffer
	client := ai.Fallback{Primary: ai.Model{BaseURL: server.URL, HTTP: server.Client()}, Stub: ai.NewStub(), Log: log.New(&logs, "", 0)}
	mux := http.NewServeMux()
	RegisterAI(mux, client)
	for _, tt := range []struct{ path, body string }{
		{"questions", `{"draft_text":"CRM"}`},
		{"card", `{"draft_text":"CRM","qa":[{"field":"data","question":"?","answer":"Excel"},{"field":"need","question":"?"},{"field":"users","question":"?"}]}`},
	} {
		w := callAI(t, mux, "/api/ai/"+tt.path, tt.body, 200)
		var result struct {
			Source string `json:"source"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Source != ai.SourceStub {
			t.Fatalf("want explicit fallback source: %s", w.Body.String())
		}
	}
	if !strings.Contains(logs.String(), "переход на заглушку") {
		t.Fatalf("fallback was not logged: %s", logs.String())
	}
}
