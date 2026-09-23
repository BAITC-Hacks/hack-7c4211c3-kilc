package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/api"
	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/store"
)

//go:embed schema.sql
var schema string

//go:embed static/task-builder/*
var frontend embed.FS

func main() {
	port := envOr("PORT", "8080")
	dbPath := envOr("DB_PATH", "data.db")

	st, err := store.Open(dbPath, schema)
	if err != nil {
		log.Fatalf("старт: %v", err)
	}
	defer st.Close()

	mux := http.NewServeMux()
	api.RegisterFrontend(mux, st)
	assets, err := fs.Sub(frontend, "static/task-builder")
	if err != nil {
		log.Fatalf("интерфейс: %v", err)
	}
	mux.Handle("GET /static/task-builder/", http.StripPrefix("/static/task-builder/", http.FileServer(http.FS(assets))))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/task-builder/login.html", http.StatusSeeOther)
	})
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
