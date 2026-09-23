package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/ai"
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

func TestLoadedEnvConfiguresAIClient(t *testing.T) {
	for _, key := range []string{"STUB", "AI_API_KEY", "AI_BASE_URL", "AI_MODEL"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("STUB=0\nAI_API_KEY=fake-test-key\nAI_BASE_URL=http://127.0.0.1:12345/v1\nAI_MODEL=test-model\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := loadEnv(path); err != nil {
		t.Fatal(err)
	}
	client, ok := ai.New().(ai.Fallback)
	if !ok {
		t.Fatal("file configuration should select the model client with fallback")
	}
	model, ok := client.Primary.(ai.Model)
	if !ok || model.BaseURL != "http://127.0.0.1:12345/v1" || model.Name != "test-model" || model.APIKey != "fake-test-key" {
		t.Fatal("model client did not use the file configuration")
	}
	t.Setenv("STUB", "1")
	if err := loadEnv(path); err != nil {
		t.Fatal(err)
	}
	if _, ok := ai.New().(ai.Stub); !ok {
		t.Fatal("shell STUB=1 must override file STUB=0")
	}
}
