package api

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/store"
)

func parseTemplates(t *testing.T) *template.Template {
	t.Helper()
	tpl, err := template.ParseFS(os.DirFS(".."), "templates/*.html")
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}
	return tpl
}

func openStore(t *testing.T) *store.Store {
	t.Helper()
	schema, err := os.ReadFile("../schema.sql")
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"), string(schema))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func getCatalog(t *testing.T, st *store.Store) *httptest.ResponseRecorder {
	return getCatalogURL(t, st, "/")
}

func getCatalogURL(t *testing.T, st *store.Store, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	Catalog(st, parseTemplates(t)).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func seededStore(t *testing.T) *store.Store {
	t.Helper()
	st := openStore(t)
	if err := st.Seed(context.Background(), os.DirFS("../data")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return st
}

func TestCatalogFilters(t *testing.T) {
	st := seededStore(t)
	ctx := context.Background()
	all, err := st.ListCatalog(ctx, store.CatalogFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) == 0 {
		t.Fatal("seed produced no published tasks")
	}

	priority := getCatalogURL(t, st, "/?level=priority")
	if priority.Code != http.StatusOK {
		t.Fatalf("priority status %d: %s", priority.Code, priority.Body)
	}
	wantPriority := 0
	for _, task := range all {
		if task.Score >= 90 {
			wantPriority++
		}
	}
	if got := strings.Count(priority.Body.String(), `class="card level-priority"`); got != wantPriority {
		t.Errorf("priority cards = %d, want %d", got, wantPriority)
	}
	if !strings.Contains(priority.Body.String(), `value="priority" selected`) {
		t.Error("priority selection is not selected")
	}

	category := all[0].Category
	filtered, err := st.ListCatalog(ctx, store.CatalogFilter{Category: category})
	if err != nil {
		t.Fatal(err)
	}
	byCategory := getCatalogURL(t, st, "/?category="+category)
	body := byCategory.Body.String()
	if byCategory.Code != http.StatusOK {
		t.Fatalf("category status %d", byCategory.Code)
	}
	if !strings.Contains(body, `value="`+category+`" selected`) {
		t.Error("category selection is not selected")
	}
	prev := -1
	for _, task := range filtered {
		idx := strings.Index(body, template.HTMLEscapeString(task.Title))
		if idx < 0 {
			t.Errorf("category task %q absent", task.Title)
			continue
		}
		if idx < prev {
			t.Errorf("category task %q is out of score order", task.Title)
		}
		prev = idx
	}
	if got := strings.Count(body, `class="card level-`); got != len(filtered) {
		t.Errorf("category cards %d, want %d", got, len(filtered))
	}

	draft := getCatalogURL(t, st, "/?level=draft")
	if draft.Code != http.StatusOK {
		t.Fatalf("draft status %d", draft.Code)
	}
	if !strings.Contains(draft.Body.String(), "Требует уточнения") {
		t.Error("draft filter lacks clarification text")
	}
	if !strings.Contains(draft.Body.String(), `value="draft" selected`) {
		t.Error("draft selection is not selected")
	}

	invalid := getCatalogURL(t, st, "/?category=xyz")
	if invalid.Code != http.StatusBadRequest || !strings.Contains(invalid.Body.String(), "Неизвестный фильтр") {
		t.Errorf("invalid filter response: status %d, body %s", invalid.Code, invalid.Body)
	}
}

func TestCatalogSeeded(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()
	if err := st.Seed(ctx, os.DirFS("../data")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	rec := getCatalog(t, st)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()

	published, err := st.ListCatalog(ctx, store.CatalogFilter{})
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if len(published) == 0 {
		t.Fatal("seed produced no published tasks")
	}
	prev, prevTitle, hasLow := -1, "", false
	for _, task := range published {
		idx := strings.Index(body, template.HTMLEscapeString(task.Title))
		if idx < 0 {
			t.Errorf("published %q (score %d) missing from page", task.Title, task.Score)
			continue
		}
		if idx < prev {
			t.Errorf("%q appears before %q despite lower score", task.Title, prevTitle)
		}
		prev, prevTitle = idx, task.Title
		hasLow = hasLow || task.Score < 40
	}
	if !hasLow {
		t.Error("seed has no published task with score < 40")
	}
	if !strings.Contains(body, "Требует уточнения") {
		t.Error("page lacks the draft-level note «Требует уточнения»")
	}
	if got := strings.Count(body, `class="card level-`); got != len(published) {
		t.Errorf("rendered %d cards, want %d", got, len(published))
	}

	rows, err := st.DB.Query(`SELECT title FROM tasks WHERE status = 'draft' AND title <> ''`)
	if err != nil {
		t.Fatalf("drafts: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if strings.Contains(body, template.HTMLEscapeString(title)) {
			t.Errorf("draft %q appears in the catalog", title)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("drafts: %v", err)
	}
}

func TestCatalogEmpty(t *testing.T) {
	rec := getCatalog(t, openStore(t))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Опубликованных задач пока нет") {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
}

func TestCatalogStoreError(t *testing.T) {
	st := openStore(t)
	st.Close()
	rec := getCatalog(t, st)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Не удалось загрузить каталог: ") {
		t.Errorf("body = %q, want readable error", rec.Body)
	}
}

func TestExcerpt(t *testing.T) {
	long := strings.Repeat("я", 250)
	if got := []rune(excerpt(long, 200)); len(got) != 201 || got[200] != '…' {
		t.Errorf("excerpt length %d, want 200 runes plus ellipsis", len(got))
	}
	if got := excerpt("  коротко  ", 200); got != "коротко" {
		t.Errorf("excerpt = %q", got)
	}
}
