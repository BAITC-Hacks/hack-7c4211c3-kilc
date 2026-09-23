package main

import (
	_ "embed"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/store"
)

//go:embed schema.sql
var schema string

func main() {
	port := envOr("PORT", "8080")
	dbPath := envOr("DB_PATH", "data.db")

	st, err := store.Open(dbPath, schema)
	if err != nil {
		log.Fatalf("старт: %v", err)
	}
	defer st.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if err := st.Ping(r.Context()); err != nil {
			log.Printf("health: %v", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "База данных недоступна"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	addr := ":" + port
	log.Printf("слушаю %s, база %s", addr, dbPath)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("сервер: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("ответ: %v", err)
	}
}
