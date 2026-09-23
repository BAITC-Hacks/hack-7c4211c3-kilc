package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// loadEnv reads an optional local file without overriding the process environment.
// Values are parsed as data: no variable interpolation or shell execution occurs.
func loadEnv(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("открыть файл окружения: %w", err)
	}
	defer f.Close()
	values, err := parseEnv(f)
	if err != nil {
		return err
	}
	for key, value := range values {
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("не удалось установить переменную окружения %s", key)
		}
	}
	return nil
}

func parseEnv(r io.Reader) (map[string]string, error) {
	values := make(map[string]string)
	scanner := bufio.NewScanner(r)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if lineNumber == 1 {
			line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") || strings.HasPrefix(line, "export\t") {
			line = strings.TrimSpace(line[len("export"):])
		}
		key, raw, found := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !found || !validEnvKey(key) {
			return nil, fmt.Errorf("строка %d: ожидается ИМЯ=значение", lineNumber)
		}
		value, ok := envValue(strings.TrimSpace(raw))
		if !ok || strings.ContainsRune(value, 0) {
			return nil, fmt.Errorf("строка %d: некорректное значение или незакрытая кавычка", lineNumber)
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("не удалось прочитать строку %d файла окружения", lineNumber+1)
	}
	return values, nil
}

func validEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, c := range key {
		if c != '_' && !(c >= 'A' && c <= 'Z') && !(c >= 'a' && c <= 'z') && !(i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func envValue(raw string) (string, bool) {
	if raw == "" {
		return "", true
	}
	quote := raw[0]
	if quote == '\'' || quote == '"' {
		for i := 1; i < len(raw); i++ {
			if quote == '"' && raw[i] == '\\' {
				i++
				continue
			}
			if raw[i] != quote {
				continue
			}
			tail := strings.TrimSpace(raw[i+1:])
			if tail != "" && !strings.HasPrefix(tail, "#") {
				return "", false
			}
			if quote == '\'' {
				return raw[1:i], true
			}
			value, err := strconv.Unquote(raw[:i+1])
			return value, err == nil
		}
		return "", false
	}
	for i, c := range raw {
		if c == '#' && (i == 0 || raw[i-1] == ' ' || raw[i-1] == '\t') {
			return strings.TrimSpace(raw[:i]), true
		}
	}
	return raw, true
}
