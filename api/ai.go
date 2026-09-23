package api

import (
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/ai"
)

// RegisterAI exposes generation only; saving and publication remain manual.
func RegisterAI(mux *http.ServeMux, client ai.Client) {
	mux.HandleFunc("POST /api/ai/questions", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			DraftText string `json:"draft_text"`
		}
		if !decodeFrontend(w, r, &body) || !validAIDraft(w, body.DraftText) {
			return
		}
		result, err := client.Questions(r.Context(), body.DraftText)
		if err != nil {
			log.Printf("ai questions: %v", err)
			writeError(w, http.StatusBadGateway, "Не удалось подготовить вопросы. Повторите попытку или заполните карточку вручную.")
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Questions []ai.Question `json:"questions"`
			Source    string        `json:"source"`
		}{result.Questions, result.Source})
	})
	mux.HandleFunc("POST /api/ai/card", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			DraftText string  `json:"draft_text"`
			QA        []ai.QA `json:"qa"`
		}
		if !decodeFrontend(w, r, &body) || !validAIDraft(w, body.DraftText) {
			return
		}
		if len(body.QA) < 3 || len(body.QA) > 5 {
			writeError(w, http.StatusBadRequest, "Передайте от 3 до 5 уточняющих вопросов с ответами; неизвестный ответ можно оставить пустым.")
			return
		}
		allowed := make(map[string]bool, len(ai.AllowedFields))
		for _, field := range ai.AllowedFields {
			allowed[field] = true
		}
		seen := make(map[string]bool)
		for _, item := range body.QA {
			if !allowed[item.Field] || seen[item.Field] || strings.TrimSpace(item.Question) == "" ||
				utf8.RuneCountInString(item.Question) > 2000 || utf8.RuneCountInString(item.Answer) > 2000 {
				writeError(w, http.StatusBadRequest, "Вопросы должны иметь разные допустимые поля, непустой текст и не более 2000 символов в вопросе или ответе.")
				return
			}
			seen[item.Field] = true
		}
		result, err := client.Card(r.Context(), body.DraftText, body.QA)
		if err != nil {
			log.Printf("ai card: %v", err)
			writeError(w, http.StatusBadGateway, "Не удалось сформировать карточку. Ответы сохранены в форме; повторите попытку или заполните карточку вручную.")
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Card   ai.Card `json:"card"`
			Source string  `json:"source"`
		}{result.Card, result.Source})
	})
}

func validAIDraft(w http.ResponseWriter, draft string) bool {
	if strings.TrimSpace(draft) == "" || utf8.RuneCountInString(draft) > 4000 {
		writeError(w, http.StatusBadRequest, "Введите исходное описание задачи от 1 до 4000 символов.")
		return false
	}
	return true
}
