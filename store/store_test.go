package store

import (
	"context"
	"os"
	"path/filepath"
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
