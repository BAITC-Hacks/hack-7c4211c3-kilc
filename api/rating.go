package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/rating"
)

const maxRatingRequestBytes = 64 * 1024

// Rating scores the card supplied in the request without accessing storage.
func Rating(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRatingRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var card rating.Card
	if err := decoder.Decode(&card); err != nil {
		writeDecodeError(w, err)
		return
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			writeDecodeError(w, err)
			return
		}
		writeError(w, http.StatusBadRequest, "После JSON-объекта обнаружены лишние данные")
		return
	}

	writeJSON(w, http.StatusOK, rating.Score(card))
}

func writeDecodeError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, "Слишком большой запрос: максимум 64 КБ")
		return
	}
	writeError(w, http.StatusBadRequest, "Некорректный JSON: "+err.Error())
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, struct {
		Error string `json:"error"`
	}{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
