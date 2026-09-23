package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/rating"
	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/store"
)

func TestRecommendEligibilityOrderAndLimit(t *testing.T) {
	team := store.Team{Interests: []string{"crm"}}
	tasks := []store.Task{
		{ID: 1, Status: store.StatusPublished, Category: "crm", Score: 35},
		{ID: 2, Status: store.StatusPublished, Category: "crm", Score: 40},
		{ID: 3, Status: store.StatusPublished, Category: "analytics", Score: 95},
	}
	for id, score := range []int{50, 60, 70, 80, 90, 100} {
		tasks = append(tasks, store.Task{ID: int64(id + 10), Status: store.StatusPublished, Category: "crm", Score: score})
	}
	got := recommend(team, tasks, 5)
	if len(got) != 5 {
		t.Fatalf("recommendations = %d, want 5", len(got))
	}
	if got[0].Score != 100 || got[0].ID != 15 {
		t.Errorf("first recommendation = %+v, want highest score first", got[0])
	}
	for _, item := range got {
		if item.ID == 1 || item.ID == 3 {
			t.Errorf("ineligible task %d was recommended", item.ID)
		}
	}
	if got[4].Score != 60 {
		t.Errorf("fifth score = %d, want 60", got[4].Score)
	}
	boundary := recommend(team, []store.Task{{ID: 20, Status: store.StatusPublished, Category: "crm", Score: 40}}, 5)
	if len(boundary) != 1 || boundary[0].Level != rating.LevelWorking {
		t.Errorf("score-40 recommendation = %+v", boundary)
	}
}

func TestRecommendationHandlerWithSeed(t *testing.T) {
	st := seededStore(t)
	ctx := context.Background()
	teams, err := st.ListTeams(ctx)
	if err != nil || len(teams) == 0 {
		t.Fatalf("seed teams: %v, count %d", err, len(teams))
	}
	before, err := st.ListCatalog(ctx, store.CatalogFilter{})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	RegisterRecommendations(mux, st)
	call := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}
	rec := call("/api/teams/" + strconv.FormatInt(teams[0].ID, 10) + "/recommendations")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var items []recommendation
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Level == rating.LevelDraft {
			t.Errorf("draft-level task %d was recommended", item.ID)
		}
		matched := false
		for _, interest := range teams[0].Interests {
			matched = matched || interest == item.Category
		}
		if !matched {
			t.Errorf("task %d category %q is not a team interest", item.ID, item.Category)
		}
	}
	missing := call("/api/teams/999/recommendations")
	if missing.Code != http.StatusNotFound || !strings.Contains(missing.Body.String(), "Команда не найдена") {
		t.Errorf("missing team response: %d %s", missing.Code, missing.Body)
	}
	page := getCatalog(t, st)
	if page.Code != http.StatusOK || strings.Count(page.Body.String(), `class="card level-`) != len(before) {
		t.Errorf("catalog changed after recommendation request: status %d, card count %d, want %d", page.Code, strings.Count(page.Body.String(), `class="card level-`), len(before))
	}
}
