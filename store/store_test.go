package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadSchema(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../schema.sql")
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	return string(b)
}

func TestOpenUpgradesExistingTasksIdempotently(t *testing.T) {
	legacy := strings.Replace(loadSchema(t), "    reward_type        TEXT    NOT NULL DEFAULT '',\n    reward             TEXT    NOT NULL DEFAULT '',\n", "", 1)
	path := filepath.Join(t.TempDir(), "legacy.db")
	s, err := Open(path, legacy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`INSERT INTO tasks (draft_text) VALUES ('preserved')`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		s, err = Open(path, legacy)
		if err != nil {
			t.Fatalf("reopen #%d: %v", i+1, err)
		}
		var draft, rewardType, reward string
		if err := s.DB.QueryRow(`SELECT draft_text, reward_type, reward FROM tasks`).Scan(&draft, &rewardType, &reward); err != nil {
			t.Fatal(err)
		}
		if draft != "preserved" || rewardType != "" || reward != "" {
			t.Fatalf("row after upgrade = %q %q %q", draft, rewardType, reward)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestOpenIdempotent(t *testing.T) {
	schema := loadSchema(t)
	path := filepath.Join(t.TempDir(), "test.db")
	for i := 0; i < 2; i++ {
		s, err := Open(path, schema)
		if err != nil {
			t.Fatalf("open #%d: %v", i+1, err)
		}
		if err := s.Ping(context.Background()); err != nil {
			t.Fatalf("ping #%d: %v", i+1, err)
		}
		if err := s.Close(); err != nil {
			t.Fatalf("close #%d: %v", i+1, err)
		}
	}
}

func TestCategoryCheck(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"), loadSchema(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	cases := []struct {
		code string
		ok   bool
	}{
		{"xyz", false},
		{"", true},
		{"crm", true},
	}
	for _, c := range cases {
		_, err := s.DB.Exec(`INSERT INTO tasks (category) VALUES (?)`, c.code)
		if c.ok && err != nil {
			t.Errorf("category %q: unexpected error %v", c.code, err)
		}
		if !c.ok && err == nil {
			t.Errorf("category %q: expected CHECK failure", c.code)
		}
	}
}

func TestCategoryHelpers(t *testing.T) {
	cases := []struct {
		code  string
		valid bool
		label string
	}{
		{"", true, ""},
		{"crm", true, "CRM и продажи"},
		{"xyz", false, ""},
	}
	for _, c := range cases {
		if got := ValidCategory(c.code); got != c.valid {
			t.Errorf("ValidCategory(%q) = %v, want %v", c.code, got, c.valid)
		}
		if got := CategoryLabel(c.code); got != c.label {
			t.Errorf("CategoryLabel(%q) = %q, want %q", c.code, got, c.label)
		}
	}
}
