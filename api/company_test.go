package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestBusinessCompanyIsolation(t *testing.T) {
	st := openStore(t)
	if err := st.Seed(context.Background(), os.DirFS("../data")); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	RegisterFrontend(mux, st)
	RegisterBusiness(mux, st)
	RegisterProposals(mux, st)
	handler := SessionAccess(st, mux)
	call := func(cookie *http.Cookie, method, path, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s = %d want %d: %s", method, path, w.Code, want, w.Body.String())
		}
		return w
	}
	owner := call(nil, "POST", "/api/session", `{"role":"business"}`, 201).Result().Cookies()[0]
	other := call(nil, "POST", "/api/session", `{"role":"business"}`, 201).Result().Cookies()[0]
	call(owner, "GET", "/api/business/tasks", "", 403)
	call(owner, "GET", "/static/task-builder/business.html", "", 303)
	task, err := st.GetTask(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"company": task.Company})
	call(owner, "POST", "/api/session/company", string(body), 200)
	call(owner, "POST", "/api/session/company", `{"company":"Другой бизнес"}`, 409)
	call(other, "POST", "/api/session/company", `{"company":"Другой бизнес"}`, 200)
	var tasks []businessTaskJSON
	json.Unmarshal(call(owner, "GET", "/api/business/tasks?company=Другой", "", 200).Body.Bytes(), &tasks)
	if len(tasks) != 2 {
		t.Fatalf("own tasks = %d", len(tasks))
	}
	for _, item := range tasks {
		if item.Company != task.Company {
			t.Fatal("foreign task leaked")
		}
	}
	json.Unmarshal(call(other, "GET", "/api/business/tasks", "", 200).Body.Bytes(), &tasks)
	if len(tasks) != 0 {
		t.Fatalf("new company sees tasks: %+v", tasks)
	}
	call(other, "GET", "/api/tasks/7", "", 403)
	call(other, "GET", "/api/tasks/7/proposals/full", "", 403)
	call(other, "GET", "/api/tasks/7/proposals", "", 403)
	call(other, "PUT", "/api/tasks/7", `{}`, 403)
	call(other, "POST", "/api/tasks/7/publish", `{}`, 403)
	var id int64
	if err := st.DB.QueryRow(`SELECT id FROM proposals WHERE task_id=7 AND status='pending'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	decision := fmt.Sprintf("/api/proposals/%d/decision", id)
	stage := fmt.Sprintf("/api/proposals/%d/stage", id)
	call(other, "POST", decision, `{"status":"accepted"}`, 403)
	call(other, "POST", stage, `{"points":10}`, 403)
	call(owner, "GET", "/api/tasks/7/proposals/full", "", 200)
	call(owner, "POST", decision, `{"status":"accepted"}`, 200)
	call(owner, "POST", stage, `{"points":10}`, 200)
	call(other, "POST", "/api/tasks", `{"draft_text":"Draft","company":"Wrong"}`, 403)
	var created taskView
	json.Unmarshal(call(other, "POST", "/api/tasks", `{"draft_text":"Draft","title":"Our task"}`, 201).Body.Bytes(), &created)
	if created.Company != "Другой бизнес" {
		t.Fatalf("company not attached: %+v", created)
	}
	call(owner, "GET", fmt.Sprintf("/api/tasks/%d", created.ID), "", 403)
	call(other, "PUT", fmt.Sprintf("/api/tasks/%d", created.ID), `{"draft_text":"Draft","company":"Wrong"}`, 403)
	call(other, "GET", "/api/teams", "", 200)
	// The binding remains after server restart.
	handler = SessionAccess(st, mux)
	call(owner, "POST", "/api/session/company", `{"company":"Wrong"}`, 409)
}
