package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/rating"
)

func TestRating(t *testing.T) {
	fullCard := `{"context":{"text":"Компания ведет продажи в нескольких регионах","confirmed":true},` +
		`"need":{"text":"Сотрудникам нужна общая система учета обращений","confirmed":true},` +
		`"users":{"text":"Менеджеры отдела продаж работают каждый день","confirmed":true},` +
		`"data":{"text":"Доступны выгрузки сделок и история обращений","confirmed":true},` +
		`"constraints":{"text":"Решение должно работать в существующей инфраструктуре","confirmed":true},` +
		`"expected_result":{"text":"Команда получает отчет по каждому клиенту","confirmed":true},` +
		`"success_criteria":{"text":"Доля обработанных заявок вырастет минимум на 20 процентов","confirmed":true},` +
		`"contact":{"text":"aigerim@example.kz","confirmed":true},` +
		`"interaction_format":{"text":"Команда обсуждает требования с бизнесом каждую неделю","confirmed":true}}`
	cases := []struct {
		name        string
		body        string
		status      int
		checkResult func(*testing.T, rating.Result)
	}{
		{
			name:   "full confirmed card",
			body:   fullCard,
			status: http.StatusOK,
			checkResult: func(t *testing.T, result rating.Result) {
				if result.Total != 100 || result.Level != rating.LevelPriority || len(result.Hints) != 0 {
					t.Errorf("got total=%d level=%q hints=%#v", result.Total, result.Level, result.Hints)
				}
			},
		},
		{
			name:   "empty object",
			body:   `{}`,
			status: http.StatusOK,
			checkResult: func(t *testing.T, result rating.Result) {
				if result.Total != 0 || len(result.Hints) != 9 || len(result.Components) != 7 {
					t.Errorf("got total=%d hints=%d components=%d", result.Total, len(result.Hints), len(result.Components))
				}
			},
		},
		{name: "malformed JSON", body: `{"context":`, status: http.StatusBadRequest},
		{name: "unknown field", body: `{"foo":1}`, status: http.StatusBadRequest},
		{name: "trailing object", body: `{} {}`, status: http.StatusBadRequest},
		{name: "body too large", body: `{"x":"` + strings.Repeat("a", 70*1024) + `"}`, status: http.StatusRequestEntityTooLarge},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/rating", strings.NewReader(tc.body))
			response := httptest.NewRecorder()
			Rating(response, request)

			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, tc.status, response.Body.String())
			}
			if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
				t.Errorf("Content-Type = %q", got)
			}
			if tc.status == http.StatusOK {
				var result rating.Result
				if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				tc.checkResult(t, result)
				return
			}
			var body struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if body.Error == "" {
				t.Error("error response has an empty message")
			}
		})
	}
}

func TestRatingRouteMethod(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/rating", Rating)
	request := httptest.NewRequest(http.MethodGet, "/api/rating", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}
