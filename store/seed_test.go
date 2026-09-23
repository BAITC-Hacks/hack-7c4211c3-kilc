package store

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/rating"
)

func countRows(t *testing.T, s *Store, query string) int {
	t.Helper()
	var n int
	if err := s.DB.QueryRow(query).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func TestSeedFixtures(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	fixtures := os.DirFS("../data")
	if err := s.Seed(ctx, fixtures); err != nil {
		t.Fatalf("seed: %v", err)
	}

	counts := map[string]int{
		`SELECT COUNT(*) FROM tasks WHERE status = 'draft'`:        5,
		`SELECT COUNT(*) FROM tasks WHERE status = 'published'`:    5,
		`SELECT COUNT(*) FROM teams`:                               5,
		`SELECT COUNT(*) FROM proposals`:                           5,
		`SELECT COUNT(*) FROM proposals WHERE status = 'pending'`:  3,
		`SELECT COUNT(*) FROM proposals WHERE status = 'accepted'`: 1,
		`SELECT COUNT(*) FROM proposals WHERE status = 'rejected'`: 1,
	}
	for query, want := range counts {
		if got := countRows(t, s, query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}

	catalog, err := s.ListCatalog(ctx, CatalogFilter{})
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	levels := map[string]bool{}
	for _, task := range catalog {
		level := rating.LevelFor(task.Score)
		levels[level] = true
		t.Logf("%-50s %3d %s", task.Title, task.Score, level)
	}
	for _, level := range []string{rating.LevelDraft, rating.LevelWorking, rating.LevelReady, rating.LevelPriority} {
		if !levels[level] {
			t.Errorf("no published card at level %q", level)
		}
	}

	teams, err := s.ListTeams(ctx)
	if err != nil {
		t.Fatalf("teams: %v", err)
	}
	total := 0
	for _, team := range teams {
		total += team.Points
	}
	if total != 10 {
		t.Errorf("total team points = %d, want 10", total)
	}

	if err := s.Seed(ctx, fixtures); !errors.Is(err, ErrSeedSkipped) {
		t.Fatalf("second seed: got %v, want ErrSeedSkipped", err)
	}
	if got := countRows(t, s, `SELECT COUNT(*) FROM tasks`); got != 10 {
		t.Errorf("tasks after second seed = %d, want 10", got)
	}
	if got := countRows(t, s, `SELECT COUNT(*) FROM proposals`); got != 5 {
		t.Errorf("proposals after second seed = %d, want 5", got)
	}
}

func TestSeedUnknownKeyRollsBack(t *testing.T) {
	s := openTestStore(t)
	fixtures := fstest.MapFS{
		"tasks.json":     {Data: []byte(`[{"key":"a","draft_text":"Нужна CRM","publish":false}]`)},
		"teams.json":     {Data: []byte(`[{"key":"t","name":"Альфа","members":["x"]}]`)},
		"proposals.json": {Data: []byte(`[]`)},
	}
	err := s.Seed(context.Background(), fixtures)
	if err == nil || !strings.Contains(err.Error(), "teams.json") {
		t.Fatalf("got %v, want error mentioning teams.json", err)
	}
	if got := countRows(t, s, `SELECT COUNT(*) FROM tasks`); got != 0 {
		t.Errorf("tasks after failed seed = %d, want 0 (rolled back)", got)
	}
}

func TestSeedInvalidProposalRollsBack(t *testing.T) {
	s := openTestStore(t)
	fixtures := fstest.MapFS{
		"tasks.json":     {Data: []byte(`[{"key":"a","title":"CRM","draft_text":"Нужна CRM","publish":true}]`)},
		"teams.json":     {Data: []byte(`[{"key":"t","name":"Альфа"}]`)},
		"proposals.json": {Data: []byte(`[{"task_key":"a","team_key":"t","idea":"x","plan":"y","deadline":"31.12.2026","prototype_url":"https://example.com"}]`)},
	}
	err := s.Seed(context.Background(), fixtures)
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "deadline" || !strings.Contains(err.Error(), "proposals.json[0]") {
		t.Fatalf("got %v, want deadline ValidationError at proposals.json[0]", err)
	}
	for _, table := range []string{"tasks", "teams", "proposals"} {
		if got := countRows(t, s, `SELECT COUNT(*) FROM `+table); got != 0 {
			t.Errorf("%s after failed seed = %d, want 0", table, got)
		}
	}
}
