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
	"time"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/store"
)

func TestProposalsAPI(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()
	if err := st.Seed(ctx, os.DirFS("../data")); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	RegisterProposals(mux, st)
	tasks, err := st.ListCatalog(ctx, store.CatalogFilter{})
	if err != nil || len(tasks) == 0 {
		t.Fatalf("catalog: %v", err)
	}
	teams, err := st.ListTeams(ctx)
	if err != nil || len(teams) == 0 {
		t.Fatalf("teams: %v", err)
	}
	var draftID, lowID int64
	if err := st.DB.QueryRow(`SELECT id FROM tasks WHERE status='draft' LIMIT 1`).Scan(&draftID); err != nil {
		t.Fatal(err)
	}
	for _, task := range tasks {
		if task.Score < 40 {
			lowID = task.ID
			break
		}
	}
	if lowID == 0 {
		t.Fatal("seed has no low-rated published task")
	}
	call := func(method string, id int64, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, fmt.Sprintf("/api/tasks/%d/proposals", id), strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s task %d: got %d want %d: %s", method, id, w.Code, want, w.Body.String())
		}
		return w
	}
	body := func(deadline, url string) string {
		b, err := json.Marshal(proposalBody{TeamID: teams[0].ID, Idea: "Идея решения", Plan: "План реализации", Deadline: deadline, PrototypeURL: url})
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	valid := body("2026-12-31", "https://example.com/prototype")
	var created proposalJSON
	t.Run("published task creates pending proposal", func(t *testing.T) {
		w := call("POST", tasks[0].ID, valid, 201)
		if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
			t.Fatal(err)
		}
		if created.ID == 0 || created.TaskID != tasks[0].ID || created.TeamID != teams[0].ID || created.TeamName != teams[0].Name || created.Status != "pending" || created.PointsAwarded != 0 {
			t.Fatalf("proposal: %+v", created)
		}
		if created.DecidedAt != "" || created.StageConfirmedAt != "" {
			t.Fatalf("unexpected decision timestamps: %+v", created)
		}
		if _, err := time.Parse(time.RFC3339, created.CreatedAt); err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &fields); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"id", "task_id", "team_id", "team_name", "idea", "plan", "deadline", "prototype_url", "status", "points_awarded", "created_at", "decided_at", "stage_confirmed_at"} {
			if _, ok := fields[key]; !ok {
				t.Errorf("missing %s", key)
			}
		}
	})
	for _, tc := range []struct {
		name  string
		id    int64
		body  string
		field string
	}{
		{"draft task", draftID, valid, "task_id"},
		{"wrong date", tasks[0].ID, body("31.12.2026", "https://example.com"), "deadline"},
		{"bare URL", tasks[0].ID, body("2026-12-31", "example.com"), "prototype_url"},
		{"markdown URL", tasks[0].ID, body("2026-12-31", "[example.com](https://example.com/)"), "prototype_url"},
		{"unknown status", tasks[0].ID, strings.TrimSuffix(valid, "}") + `,"status":"accepted"}`, ""},
		{"empty idea", tasks[0].ID, strings.Replace(valid, "Идея решения", " ", 1), "idea"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, err := st.ListProposalsByTask(ctx, tc.id)
			if err != nil {
				t.Fatal(err)
			}
			w := call("POST", tc.id, tc.body, 400)
			var e map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
				t.Fatal(err)
			}
			if e["error"] == "" || e["field"] != tc.field {
				t.Fatalf("error body: %v", e)
			}
			after, err := st.ListProposalsByTask(ctx, tc.id)
			if err != nil {
				t.Fatal(err)
			}
			if len(before) != len(after) {
				t.Fatal("invalid request inserted a proposal")
			}
		})
	}
	t.Run("low rating accepts proposal", func(t *testing.T) { call("POST", lowID, valid, 201) })
	t.Run("list includes new proposal", func(t *testing.T) {
		w := call("GET", tasks[0].ID, "", 200)
		var items []proposalJSON
		if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
			t.Fatal(err)
		}
		for _, p := range items {
			if p.ID == created.ID && p.TeamName == teams[0].Name {
				return
			}
		}
		t.Fatal("created proposal absent from GET")
	})
	t.Run("unknown task", func(t *testing.T) { call("GET", 999999, "", 404); call("POST", 999999, valid, 404) })
	t.Run("empty list is array", func(t *testing.T) {
		task := store.Task{DraftText: "New task"}
		if err := st.CreateTask(ctx, &task); err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSpace(call("GET", task.ID, "", 200).Body.String()); got != "[]" {
			t.Fatalf("got %s", got)
		}
	})
	t.Run("store failure", func(t *testing.T) {
		if err := st.Close(); err != nil {
			t.Fatal(err)
		}
		w := call("GET", tasks[0].ID, "", 500)
		if !strings.Contains(w.Body.String(), "Не удалось") {
			t.Fatal("missing readable error")
		}
	})
}

// Kept here so this feature changes only the seven requested files.
func TestProposalCatalogLinks(t *testing.T) {
	st := openStore(t)
	if err := st.Seed(context.Background(), os.DirFS("../data")); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", Catalog(st, parseTemplates(t)))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 200 {
		t.Fatalf("status %d", w.Code)
	}
	tasks, err := st.ListCatalog(context.Background(), store.CatalogFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range tasks {
		link := fmt.Sprintf(`href="/static/task-builder/task.html?id=%d"`, task.ID)
		if !strings.Contains(w.Body.String(), link) {
			t.Errorf("missing %s", link)
		}
	}
}
