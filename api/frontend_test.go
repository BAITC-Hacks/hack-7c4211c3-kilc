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
	body = strings.Replace(body, `"confirmed":[]`, `"confirmed":["context"]`, 1)
	call("PUT", path, body, 200)
	call("POST", path+"/publish", "{}", 200)
	w = call("GET", "/api/tasks?category=automation&level=draft", "", 200)
	var tasks []taskView
	if err := json.Unmarshal(w.Body.Bytes(), &tasks); err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Rating.Total != 10 || tasks[0].Status != "published" {
		t.Fatalf("catalog: %+v", tasks)
	}
	call("GET", path, "", 200)
	call("PUT", path, strings.Replace(body, "Original draft", "Changed draft", 1), 400)
	call("GET", "/api/tasks/nope", "", 400)
	call("GET", "/api/tasks/99999", "", 404)
	call("GET", "/api/tasks?level=unknown", "", 400)
}
