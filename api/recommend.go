package api

import (
	"errors"
	"log"
	"net/http"
	"sort"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/rating"
	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/store"
)

type recommendation struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	Company       string `json:"company"`
	Category      string `json:"category"`
	CategoryLabel string `json:"category_label"`
	Score         int    `json:"score"`
	Level         string `json:"level"`
	LevelLabel    string `json:"level_label"`
	Reason        string `json:"reason"`
}

func recommend(team store.Team, tasks []store.Task, limit int) []recommendation {
	if limit <= 0 {
		return []recommendation{}
	}
	interests := make(map[string]struct{}, len(team.Interests))
	for _, category := range team.Interests {
		interests[category] = struct{}{}
	}
	eligible := make([]store.Task, 0, len(tasks))
	for _, task := range tasks {
		if task.Status != store.StatusPublished || task.Score < 40 {
			continue
		}
		if _, ok := interests[task.Category]; !ok {
			continue
		}
		eligible = append(eligible, task)
	}
	sort.Slice(eligible, func(i, j int) bool {
		if eligible[i].Score == eligible[j].Score {
			return eligible[i].ID > eligible[j].ID
		}
		return eligible[i].Score > eligible[j].Score
	})
	if limit > 5 {
		limit = 5
	}
	if len(eligible) > limit {
		eligible = eligible[:limit]
	}
	result := make([]recommendation, 0, len(eligible))
	for _, task := range eligible {
		label := store.CategoryLabel(task.Category)
		level := rating.LevelFor(task.Score)
		result = append(result, recommendation{
			ID: task.ID, Title: task.Title, Company: task.Company, Category: task.Category,
			CategoryLabel: label, Score: task.Score, Level: level,
			LevelLabel: rating.LevelLabel(level), Reason: "Совпадает интерес: " + label,
		})
	}
	return result
}

func RegisterRecommendations(mux *http.ServeMux, st *store.Store) {
	mux.HandleFunc("GET /api/teams/{id}/recommendations", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		team, err := st.GetTeam(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Команда не найдена")
			return
		}
		if err != nil {
			frontendError(w, err)
			return
		}
		tasks, err := st.ListCatalog(r.Context(), store.CatalogFilter{})
		if err != nil {
			log.Printf("рекомендации: %v", err)
			writeError(w, http.StatusInternalServerError, "Не удалось загрузить рекомендации")
			return
		}
		writeJSON(w, http.StatusOK, recommend(team, tasks, 5))
	})
}
