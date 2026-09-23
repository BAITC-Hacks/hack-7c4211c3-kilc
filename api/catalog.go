package api

import (
	"bytes"
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
}

type catalogPage struct {
	Cards []catalogCard
}

// Catalog renders every published task, highest rating first. Low-rated
// cards are shown like any other; only their styling differs.
func Catalog(st *store.Store, tpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tasks, err := st.ListCatalog(r.Context(), store.CatalogFilter{})
		if err != nil {
			log.Printf("каталог: %v", err)
			http.Error(w, "Не удалось загрузить каталог: "+err.Error(), http.StatusInternalServerError)
			return
		}
		page := catalogPage{Cards: make([]catalogCard, 0, len(tasks))}
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
			})
		}
		var buf bytes.Buffer
		if err := tpl.ExecuteTemplate(&buf, "layout", page); err != nil {
			log.Printf("каталог: шаблон: %v", err)
			http.Error(w, "Не удалось отобразить каталог: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
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
