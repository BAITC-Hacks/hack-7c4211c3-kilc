package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/ai"
)

type fakeChatter struct {
	system string
	turns  []ai.ChatTurn
}

func (f *fakeChatter) Chat(_ context.Context, system string, turns []ai.ChatTurn) (string, error) {
	f.system, f.turns = system, turns
	return "Добавьте измеримый критерий успеха.", nil
}

func TestChatAccessAndValidation(t *testing.T) {
	st := openStore(t)
	if err := st.Seed(context.Background(), os.DirFS("../data")); err != nil {
		t.Fatal(err)
	}
	chatter := &fakeChatter{}
	mux := http.NewServeMux()
	RegisterChat(mux, st, chatter)
	handler := SessionAccess(st, mux)
	call := func(cookie *http.Cookie, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest("POST", "/api/chat", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("POST /api/chat %s = %d want %d: %s", body, w.Code, want, w.Body.String())
		}
		return w
	}
	session := func(role string) *http.Cookie {
		r := httptest.NewRequest("POST", "/api/session", strings.NewReader(`{"role":"`+role+`"}`))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		SessionAccess(st, http.NewServeMux()).ServeHTTP(w, r)
		return w.Result().Cookies()[0]
	}

	call(nil, `{"message":"Привет","history":[],"context":{"page":"/","task_id":null}}`, 401)
	student := session("student")
	call(student, `{"message":"","history":[],"context":{"page":"/","task_id":null}}`, 400)
	call(student, `{"message":"Привет","history":[{"role":"system","content":"x"}],"context":{"page":"/","task_id":null}}`, 400)
	call(student, `{"message":"Привет","role":"business","history":[],"context":{"page":"/","task_id":null}}`, 400)
	call(student, `{"message":"Как улучшить?","history":[],"context":{"page":"/","task_id":1}}`, 403)
	w := call(student, `{"message":"Как улучшить?","history":[{"role":"user","content":"Что такое рейтинг?"},{"role":"assistant","content":"Полнота."}],"context":{"page":"/static/task-builder/task.html","task_id":6}}`, 200)
	if !strings.Contains(w.Body.String(), `"reply"`) || len(chatter.turns) != 3 || !strings.Contains(chatter.system, "студенческая команда") || !strings.Contains(chatter.system, "Открытая задача") {
		t.Fatalf("reply %s, turns %+v, system %q", w.Body.String(), chatter.turns, chatter.system)
	}
	call(student, `{"message":"Ещё","history":[],"context":{"page":"/","task_id":null}}`, 429)

	mux2 := http.NewServeMux()
	RegisterChat(mux2, st, nil)
	r := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"message":"Привет","history":[],"context":{"page":"/","task_id":null}}`))
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(session("student"))
	rec := httptest.NewRecorder()
	SessionAccess(st, mux2).ServeHTTP(rec, r)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("stub mode chat = %d, want 503", rec.Code)
	}
}
