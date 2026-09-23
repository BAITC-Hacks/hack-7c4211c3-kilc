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

func TestSessionRoleTeamAndReward(t *testing.T) {
	st := openStore(t)
	if err := st.Seed(context.Background(), os.DirFS("../data")); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	RegisterFrontend(mux, st)
	RegisterProposals(mux, st)
	RegisterBusiness(mux, st)
	RegisterRecommendations(mux, st)
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
	call(nil, "POST", "/api/tasks", `{}`, 401)
	student := call(nil, "POST", "/api/session", `{"role":"student"}`, 201).Result().Cookies()[0]
	business := call(nil, "POST", "/api/session", `{"role":"business"}`, 201).Result().Cookies()[0]
	call(business, "POST", "/api/session/company", `{"company":"Test company"}`, 200)
	if !student.HttpOnly || student.SameSite != http.SameSiteLaxMode {
		t.Fatal("unsafe session cookie")
	}
	call(student, "POST", "/api/session", `{"role":"business"}`, 409)
	call(business, "POST", "/api/session", `{"role":"student"}`, 409)
	call(student, "POST", "/api/tasks", `{}`, 403)
	call(student, "GET", "/api/business/tasks", "", 403)
	call(student, "POST", "/api/proposals/1/decision", `{"status":"accepted"}`, 403)
	call(student, "POST", "/api/proposals/1/stage", `{"points":10}`, 403)
	call(student, "GET", "/static/task-builder/index.html", "", 303)
	call(business, "GET", "/static/task-builder/teams.html", "", 303)
	call(student, "GET", "/static/task-builder/login.html", "", 303)
	call(business, "POST", "/api/session/team", `{"team_id":1}`, 403)
	call(student, "POST", "/api/session/team", `{"team_id":999999}`, 404)
	call(student, "POST", "/api/session/team", `{"team_id":1}`, 200)
	call(student, "POST", "/api/session/team", `{"team_id":1}`, 200)
	call(student, "POST", "/api/session/team", `{"team_id":2}`, 409)
	call(student, "POST", "/api/teams", `{"name":"Other"}`, 403)
	call(student, "GET", "/api/teams/2/recommendations", "", 403)
	var teams []teamView
	json.Unmarshal(call(student, "GET", "/api/teams", "", 200).Body.Bytes(), &teams)
	if len(teams) != 1 || teams[0].ID != 1 {
		t.Fatalf("teams leaked: %+v", teams)
	}
	call(student, "POST", "/api/tasks/6/proposals", `{"team_id":2}`, 403)
	call(business, "POST", "/api/tasks/6/proposals", `{"team_id":1}`, 403)
	call(student, "POST", "/api/tasks/6/proposals", `{"team_id":1,"idea":"Идея","plan":"План","deadline":"2026-12-31","prototype_url":"https://example.com"}`, 201)
	call(student, "GET", "/api/tasks/1", "", 404)
	// Session survives reconstruction of the middleware (as on server restart).
	handler = SessionAccess(st, mux)
	call(student, "POST", "/api/session/team", `{"team_id":2}`, 409)
	body := `{"draft_text":"Исходный черновик","title":"Reward task","reward_type":"money","reward":"Грант 100000 тенге","confirmed":["title","reward"]}`
	var task taskView
	if err := json.Unmarshal(call(business, "POST", "/api/tasks", body, 201).Body.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/api/tasks/%d", task.ID)
	if task.RewardType != "money" || task.Reward != "Грант 100000 тенге" || task.Rating.Bonus != 10 {
		t.Fatalf("reward lost: %+v", task)
	}
	call(business, "POST", path+"/publish", `{}`, 200)
	var loaded taskView
	json.Unmarshal(call(student, "GET", path, "", 200).Body.Bytes(), &loaded)
	if loaded.Reward != task.Reward || loaded.Rating.Position != 10 {
		t.Fatalf("reward read: %+v", loaded)
	}
	updated := strings.ReplaceAll(body, "money", "nonmonetary")
	json.Unmarshal(call(business, "PUT", path, updated, 200).Body.Bytes(), &loaded)
	if loaded.Rating.Bonus != 0 {
		t.Fatalf("changed reward type kept confirmation: %+v", loaded)
	}
	json.Unmarshal(call(business, "PUT", path, updated, 200).Body.Bytes(), &loaded)
	if loaded.Rating.Bonus != 5 {
		t.Fatalf("reward update: %+v", loaded)
	}
	call(business, "PUT", path, strings.ReplaceAll(body, "money", "invalid"), 400)
	call(business, "PUT", path, strings.ReplaceAll(body, "Грант 100000 тенге", ""), 400)
	newStudent := call(nil, "POST", "/api/session", `{"role":"student"}`, 201).Result().Cookies()[0]
	call(newStudent, "POST", "/api/teams", `{"name":"Новая команда","interests":[]}`, 201)
	call(newStudent, "POST", "/api/session/team", `{"team_id":1}`, 409)
}
