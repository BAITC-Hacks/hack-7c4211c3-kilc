package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseEnv(t *testing.T) {
	values, err := parseEnv(strings.NewReader("\ufeff# local config\r\nexport STUB = 0 # model\r\nAI_API_KEY='a#b=c'\nAI_MODEL=\"gpt-model\" # comment\nEMPTY=\nLITERAL=$(echo secret)\nREFERENCE=${STUB}\nESCAPES=\"a\\nb\\\"c\"\nSTUB=1\n"))
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"STUB": "1", "AI_API_KEY": "a#b=c", "AI_MODEL": "gpt-model", "EMPTY": "",
		"LITERAL": "$(echo secret)", "REFERENCE": "${STUB}", "ESCAPES": "a\nb\"c",
	} {
		if got, ok := values[key]; !ok || got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestParseEnvRejectsInvalidInputWithoutLeakingValues(t *testing.T) {
	for _, line := range []string{"missing-secret", "1KEY=secret", "BAD KEY=secret", "KEY='secret", `KEY="secret" extra`, `KEY="\qsecret"`, "KEY=secret\x00", "KEY=" + strings.Repeat("secret", 12000)} {
		_, err := parseEnv(strings.NewReader("GOOD=ok\n" + line))
		if err == nil || !strings.Contains(err.Error(), "2") || strings.Contains(err.Error(), "secret") {
			t.Fatalf("want line-number-only error, got %v", err)
		}
	}
}

func TestLoadEnvPreservesExistingEnvironment(t *testing.T) {
	const inherited = "TASKLAB_TEST_INHERITED"
	const empty = "TASKLAB_TEST_EMPTY"
	const loaded = "TASKLAB_TEST_LOADED"
	t.Setenv(inherited, "shell")
	t.Setenv(empty, "")
	t.Setenv(loaded, "")
	if err := os.Unsetenv(loaded); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(inherited+"=file\n"+empty+"=file\n"+loaded+"=from-file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := loadEnv(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv(inherited) != "shell" || os.Getenv(empty) != "" || os.Getenv(loaded) != "from-file" {
		t.Fatal("file must fill only absent variables, including preserving explicit empty values")
	}
	if err := loadEnv(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatalf("missing .env should be optional: %v", err)
	}
	if err := loadEnv(t.TempDir()); err == nil {
		t.Fatal("unreadable .env must fail")
	}
}

func TestLoadEnvDoesNotPartiallyApplyMalformedFile(t *testing.T) {
	const key = "TASKLAB_TEST_ATOMIC"
	t.Setenv(key, "")
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(key+"=value\nmalformed-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := loadEnv(path); err == nil {
		t.Fatal("want parse failure")
	}
	if _, exists := os.LookupEnv(key); exists {
		t.Fatal("invalid file partially applied")
	}
}
