package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/ai"
	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/api"
	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/store"
)

//go:embed schema.sql
var schema string

//go:embed data/*.json
var seedFiles embed.FS

//go:embed templates/*.html static/*
var assets embed.FS

func main() {
	if err := loadEnv(".env"); err != nil {
		log.Fatalf("старт: .env: %v", err)
	}
	port := envOr("PORT", "8080")
	dbPath := envOr("DB_PATH", "data.db")

	st, err := store.Open(dbPath, schema)
	if err != nil {
		log.Fatalf("старт: %v", err)
	}
	defer st.Close()

	fixtures, err := fs.Sub(seedFiles, "data")
	if err != nil {
		log.Fatalf("старт: открыть данные для seed: %v", err)
	}
	switch err := st.Seed(context.Background(), fixtures); {
	case errors.Is(err, store.ErrSeedSkipped):
		log.Print("seed skipped: database not empty")
	case err != nil:
		log.Fatalf("старт: seed: %v", err)
	default:
		log.Print("seeded")
	}

	tpl, err := template.ParseFS(assets, "templates/*.html")
	if err != nil {
		log.Fatalf("старт: шаблоны: %v", err)
	}

	mux := http.NewServeMux()
	api.RegisterAI(mux, ai.New())
	api.RegisterChat(mux, st, ai.NewChat())
	api.RegisterFrontend(mux, st)
	api.RegisterRewardTypes(mux)
	api.RegisterProposals(mux, st)
	api.RegisterBusiness(mux, st)
	api.RegisterRecommendations(mux, st)
	mux.HandleFunc("GET /{$}", api.Catalog(st, tpl))
	mux.Handle("GET /static/", http.FileServerFS(assets))
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
	if err := http.ListenAndServe(addr, api.SessionAccess(st, mux)); err != nil {
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
