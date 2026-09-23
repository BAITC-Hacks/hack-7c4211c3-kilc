package ai

import (
	"log"
	"net/http"
	"os"
)

const (
	defaultBaseURL = "https://api.deepseek.com"
	defaultModel   = "deepseek-chat"
)

// New picks the stub or the model with fallback from the environment.
func New() Client {
	key := os.Getenv("AI_API_KEY")
	if os.Getenv("STUB") != "0" || key == "" {
		log.Print("ai: режим заглушки")
		return NewStub()
	}
	model := Model{
		BaseURL: envOr("AI_BASE_URL", defaultBaseURL),
		APIKey:  key,
		Name:    envOr("AI_MODEL", defaultModel),
		HTTP:    &http.Client{Timeout: modelTimeout},
	}
	log.Printf("ai: модель %s", model.Name)
	return Fallback{Primary: model, Stub: NewStub(), Log: log.Default()}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
