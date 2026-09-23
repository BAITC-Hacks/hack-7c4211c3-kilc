package api

import (
	"bytes"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/rating"
	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/store"
)

const needExcerptRunes = 200

var levelLabels = map[string]string{
	rating.LevelDraft:    "Черновик",
	rating.LevelWorking:  "Рабочая",
	rating.LevelReady:    "Готовая",
	rating.LevelPriority: "Приоритетная",
}

type catalogCard struct {
	ID         int64
	Title      string
	Meta       string
	Score      int
	Level      string
	LevelLabel string
	Need       string
	Category   string
}

type catalogPage struct {
	Cards      []catalogCard
	Categories []store.Category
	Industries []string
	Levels     []rating.Level
	Category   string
	Industry   string
	Level      string
	Count      int
	Error      string
}

// Catalog renders every published task, highest rating first. Low-rated
// cards are shown like any other; only their styling differs.
func Catalog(st *store.Store, tpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter := store.CatalogFilter{Category: r.URL.Query().Get("category"), Industry: r.URL.Query().Get("industry"), Level: r.URL.Query().Get("level")}
		tasks, err := st.ListCatalog(r.Context(), filter)
		status := http.StatusOK
		filterError := ""
		if err != nil {
			var validationErr *store.ValidationError
			if !errors.As(err, &validationErr) {
				log.Printf("каталог: %v", err)
				http.Error(w, "Не удалось загрузить каталог: "+err.Error(), http.StatusInternalServerError)
				return
			}
			status = http.StatusBadRequest
			filterError = validationErr.Message
			filter = store.CatalogFilter{}
			tasks, err = st.ListCatalog(r.Context(), filter)
			if err != nil {
				log.Printf("каталог: %v", err)
				http.Error(w, "Не удалось загрузить каталог: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
		industries, err := st.ListIndustries(r.Context())
		if err != nil {
			log.Printf("каталог: %v", err)
			http.Error(w, "Не удалось загрузить каталог: "+err.Error(), http.StatusInternalServerError)
			return
		}
		page := catalogPage{Cards: make([]catalogCard, 0, len(tasks)), Categories: store.Categories, Industries: industries, Levels: rating.Levels(), Category: filter.Category, Industry: filter.Industry, Level: filter.Level, Count: len(tasks)}
		if status == http.StatusBadRequest {
			page.Error = "Неизвестный фильтр: " + filterError
		}
		for _, t := range tasks {
			level := rating.LevelFor(t.Score)
			page.Cards = append(page.Cards, catalogCard{
				ID:         t.ID,
				Title:      t.Title,
				Meta:       joinNonEmpty(" · ", t.Company, t.Industry, store.CategoryLabel(t.Category)),
				Score:      t.Score,
				Level:      level,
				LevelLabel: levelLabels[level],
				Need:       excerpt(t.Need, needExcerptRunes),
				Category:   store.CategoryLabel(t.Category),
			})
		}
		var buf bytes.Buffer
		if err := tpl.ExecuteTemplate(&buf, "layout", page); err != nil {
			log.Printf("каталог: шаблон: %v", err)
			http.Error(w, "Не удалось отобразить каталог: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		if _, err := buf.WriteTo(w); err != nil {
			log.Printf("каталог: ответ: %v", err)
		}
	}
}

func joinNonEmpty(sep string, parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, sep)
}

func excerpt(text string, limit int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= limit {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:limit])) + "…"
}
