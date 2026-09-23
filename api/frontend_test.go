package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/store"
)

func TestFrontendPersistence(t *testing.T) {
	schema, err := os.ReadFile("../schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"), string(schema))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	mux := http.NewServeMux()
	RegisterFrontend(mux, st)
	call := func(method, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w
	}
	call("POST", "/api/teams", `{"name":"Builders","interests":["web_app"],"skills":"UI","tech":"Go"}`, 201)
	list := call("GET", "/api/teams", "", 200)
	var teams []teamView
	if err := json.Unmarshal(list.Body.Bytes(), &teams); err != nil {
		t.Fatal(err)
	}
	if len(teams) != 1 || teams[0].Name != "Builders" || teams[0].Points != 0 {
		t.Fatalf("teams: %+v", teams)
	}
	call("POST", "/api/teams", `{"name":"Bad","interests":["unknown"]}`, 400)
	call("POST", "/api/teams", `{"name":"Bad","members":[]}`, 400)
	body := `{"title":"Test task","draft_text":"Original draft","context":"The current business process is manual","category":"automation","confirmed":[]}`
	w := call("POST", "/api/tasks", body, 201)
	var task taskView
	if err := json.Unmarshal(w.Body.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	path := "/api/tasks/" + strconv.FormatInt(task.ID, 10)
	if task.Rating.Total != 0 || task.Rating.Potential != 10 {
		t.Fatalf("rating: %+v", task.Rating)
	}
	call("POST", path+"/publish", "{}", 400)
	body = strings.Replace(body, `"confirmed":[]`, `"confirmed":["title","category","context"]`, 1)
	call("PUT", path, body, 200)
	call("POST", path+"/publish", "{}", 200)
	catalog := func() []taskView {
		t.Helper()
		w := call("GET", "/api/tasks?category=automation&level=draft", "", 200)
		var tasks []taskView
		if err := json.Unmarshal(w.Body.Bytes(), &tasks); err != nil {
			t.Fatal(err)
		}
		return tasks
	}
	if tasks := catalog(); len(tasks) != 1 || tasks[0].Rating.Total != 10 || tasks[0].Status != "published" {
		t.Fatalf("catalog: %+v", tasks)
	}
	changed := strings.Replace(body, "is manual", "is manual and slow", 1)
	w = call("PUT", path, changed, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	if task.Rating.Total != 0 || task.Status != "draft" || strings.Join(task.Confirmed, ",") != "title,category" {
		t.Fatalf("changed context kept confirmation: status=%s confirmed=%v rating=%d", task.Status, task.Confirmed, task.Rating.Total)
	}
	if tasks := catalog(); len(tasks) != 0 {
		t.Fatalf("unconfirmed card still in catalog: %+v", tasks)
	}
	call("POST", path+"/publish", "{}", 400)
	w = call("PUT", path, changed, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	if task.Rating.Total != 10 {
		t.Fatalf("reconfirmed context: rating %d, want 10", task.Rating.Total)
	}
	call("POST", path+"/publish", "{}", 200)
	if tasks := catalog(); len(tasks) != 1 {
		t.Fatalf("republished card missing from catalog: %+v", tasks)
	}
	body = changed
	call("GET", path, "", 200)
	call("PUT", path, strings.Replace(body, "Original draft", "Changed draft", 1), 400)
	call("GET", "/api/tasks/nope", "", 400)
	call("GET", "/api/tasks/99999", "", 404)
	call("GET", "/api/tasks?level=unknown", "", 400)
}

func TestRewardTypesEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRewardTypes(mux)
	r := httptest.NewRequest(http.MethodGet, "/api/reward-types", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var items []struct {
		Code  string `json:"code"`
		Label string `json:"label"`
		Bonus int    `json:"bonus"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 || items[0].Code != "" {
		t.Fatalf("reward types = %+v, want 4 items beginning with empty code", items)
	}
}
