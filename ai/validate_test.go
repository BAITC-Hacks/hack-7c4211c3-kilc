package ai

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func mustParseCard(t *testing.T, raw, draft string, qa []QA) (Card, []string) {
	t.Helper()
	card, dropped, err := ParseCard([]byte(raw), draft, qa)
	if err != nil {
		t.Fatalf("ParseCard returned error: %v", err)
	}
	return card, dropped
}

func droppedHas(dropped []string, prefix string) bool {
	for _, item := range dropped {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}

func TestParseQuestionsValid(t *testing.T) {
	raw := `{"questions":[{"field":"data","question":" Какие данные? "},{"field":"users","question":"Кто?"},{"field":"need","question":"Зачем?"}]}`
	questions, err := ParseQuestions([]byte(raw))
	if err != nil {
		t.Fatalf("ParseQuestions returned error: %v", err)
	}
	if len(questions) != 3 {
		t.Fatalf("got %d questions, want 3", len(questions))
	}
	if questions[0].Question != "Какие данные?" {
		t.Fatalf("question not trimmed: %q", questions[0].Question)
	}
}

func TestParseQuestionsUnknownFieldLeavesTooFew(t *testing.T) {
	raw := `{"questions":[{"field":"data","question":"a"},{"field":"users","question":"b"},{"field":"salary","question":"c"}]}`
	if _, err := ParseQuestions([]byte(raw)); !errors.Is(err, ErrInvalidOutput) {
		t.Fatalf("err = %v, want ErrInvalidOutput", err)
	}
}

func TestParseQuestionsTruncatesToFive(t *testing.T) {
	var items []string
	for _, field := range AllowedFields[:7] {
		items = append(items, `{"field":"`+field+`","question":"вопрос"}`)
	}
	questions, err := ParseQuestions([]byte(`{"questions":[` + strings.Join(items, ",") + `]}`))
	if err != nil {
		t.Fatalf("ParseQuestions returned error: %v", err)
	}
	if len(questions) != 5 {
		t.Fatalf("got %d questions, want 5", len(questions))
	}
}

func TestParseQuestionsDedupAndExtraKeys(t *testing.T) {
	raw := `{"note":"x","questions":[{"field":"data","question":"первый","extra":1},{"field":"data","question":"второй"},{"field":"users","question":"b"},{"field":"need","question":"c"}]}`
	questions, err := ParseQuestions([]byte(raw))
	if err != nil {
		t.Fatalf("ParseQuestions returned error: %v", err)
	}
	if len(questions) != 3 || questions[0].Question != "первый" {
		t.Fatalf("dedup failed: %+v", questions)
	}
}

func TestParseQuestionsInvalidJSON(t *testing.T) {
	for _, raw := range []string{`not json`, `{"items":[]}`, `{"questions":"x"}`} {
		if _, err := ParseQuestions([]byte(raw)); !errors.Is(err, ErrInvalidOutput) {
			t.Fatalf("%s: err = %v, want ErrInvalidOutput", raw, err)
		}
	}
}

func TestParseCardDropsUnknownKey(t *testing.T) {
	card, _ := mustParseCard(t, `{"card":{"title":"Нужна CRM","budget":"много"}}`, "Нужна CRM", nil)
	if card.Title != "Нужна CRM" {
		t.Fatalf("title = %q", card.Title)
	}
	if strings.Contains(card.Context+card.Need+card.Constraints, "много") {
		t.Fatalf("budget leaked into card: %+v", card)
	}
}

func TestParseCardDropsRewardFields(t *testing.T) {
	card, dropped := mustParseCard(t, `{"card":{"title":"Задача","context":"Пять важных слов для контекста задачи","reward":"Грант 300 000 ₸","reward_type":"money"}}`, "Задача.\nПять важных слов для контекста задачи.\nГрант 300 000 ₸", nil)
	if card.Title != "Задача" || card.Context != "Пять важных слов для контекста задачи" {
		t.Fatalf("allowed fields were not kept: %+v", card)
	}
	if len(dropped) != 2 {
		t.Fatalf("want both unknown reward fields dropped: %v", dropped)
	}
}

func TestParseCardCategory(t *testing.T) {
	card, dropped := mustParseCard(t, `{"card":{"title":"x","category":"fintech"}}`, "x", nil)
	if card.Category != "" || !droppedHas(dropped, "category:") {
		t.Fatalf("category = %q, dropped = %v", card.Category, dropped)
	}
	card, dropped = mustParseCard(t, `{"card":{"title":"x","category":"crm"}}`, "x", nil)
	if card.Category != "crm" || len(dropped) != 0 {
		t.Fatalf("category = %q, dropped = %v", card.Category, dropped)
	}
}

func TestParseCardInventedNumber(t *testing.T) {
	raw := `{"card":{"title":"Хотим больше продаж","need":"рост продаж на 40%"}}`
	card, dropped := mustParseCard(t, raw, "Хотим больше продаж", nil)
	if card.Need != "" || !droppedHas(dropped, "need:") {
		t.Fatalf("need = %q, dropped = %v", card.Need, dropped)
	}
	card, dropped = mustParseCard(t, raw, "Хотим больше продаж", []QA{{Field: "success_criteria", Answer: "рост продаж на 40%"}})
	if card.Need != "рост продаж на 40%" || len(dropped) != 0 {
		t.Fatalf("need = %q, dropped = %v", card.Need, dropped)
	}
}

func TestParseCardInventedEmail(t *testing.T) {
	card, dropped := mustParseCard(t, `{"card":{"title":"Пишите Ивану","contact":"Иван, ivan@corp.kz"}}`, "Пишите Ивану", nil)
	if card.Contact != "" || !droppedHas(dropped, "contact:") {
		t.Fatalf("contact = %q, dropped = %v", card.Contact, dropped)
	}
	card, _ = mustParseCard(t, `{"card":{"contact":"Ivan@Corp.kz"}}`, "Контакт:\nivan@corp.kz", nil)
	if card.Contact != "Ivan@Corp.kz" {
		t.Fatalf("contact = %q, want kept", card.Contact)
	}
}

func TestParseCardNonStringValue(t *testing.T) {
	card, dropped := mustParseCard(t, `{"card":{"users":12,"data":" Excel "}}`, "Excel", nil)
	if card.Users != "" || !droppedHas(dropped, "users:") {
		t.Fatalf("users = %q, dropped = %v", card.Users, dropped)
	}
	if card.Data != "Excel" {
		t.Fatalf("data = %q, want trimmed", card.Data)
	}
}

func TestParseCardInvalidShape(t *testing.T) {
	for _, raw := range []string{`{"card": "text"}`, `not json`, `{"questions":[]}`, `{"card":null}`, `null`, `[]`, `{"card":[]}`, `{"card":{}}`, `{"card":{"unknown":"x"}}`, `{"card":{"category":"crm"}}`, `{"card":{"title":"  ","context":""}}`} {
		if _, _, err := ParseCard([]byte(raw), "x", nil); !errors.Is(err, ErrInvalidOutput) {
			t.Fatalf("%s: err = %v, want ErrInvalidOutput", raw, err)
		}
	}
}

func TestParseCardRejectsUnsupportedProse(t *testing.T) {
	for _, tt := range []struct{ name, draft, output string }{
		{"invented system", "Нужна CRM", "Компания уже использует SAP для управления складом"},
		{"removed negation", "Мы не используем SAP.", "Мы используем SAP."},
		{"removed condition", "Если будет бюджет, купим CRM.", "Купим CRM."},
		{"question turned into assertion", "Компания использует SAP?", "Компания использует SAP."},
		{"word recombination", "Компания использует Excel. Команда знает SAP.", "Компания использует SAP."},
		{"number substring", "Нужно обработать 140 заявок.", "Нужно обработать 40 заявок."},
		{"email substring", "Контакт: notivan@corp.kz", "ivan@corp.kz"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(map[string]any{"card": map[string]string{"need": tt.output}})
			if err != nil {
				t.Fatal(err)
			}
			card, dropped, err := ParseCard(raw, tt.draft, nil)
			if !errors.Is(err, ErrInvalidOutput) || card.Need != "" || !droppedHas(dropped, "need:") {
				t.Fatalf("want unusable output rejected: card=%+v dropped=%v err=%v", card, dropped, err)
			}
		})
	}
}

func TestParseCardPreservesCompleteSourceExcerpts(t *testing.T) {
	draft := "Нужна CRM. Мы не используем SAP.\nБюджет 1.5 млн.\nhttps://example.com/help"
	qa := []QA{{Field: "data", Question: "Модель придумала вопрос?", Answer: "Обезличенная таблица\nИстория заявок"}}
	raw := `{"card":{"title":"Нужна CRM","context":"Мы не используем SAP.","constraints":"Бюджет 1.5 млн.","data":"Обезличенная   таблица\nИстория заявок","contact":"https://example.com/help","need":"Модель придумала вопрос?"}}`
	card, dropped := mustParseCard(t, raw, draft, qa)
	if card.Title != "Нужна CRM" || card.Context != "Мы не используем SAP." || card.Constraints != "Бюджет 1.5 млн." || card.Data != "Обезличенная   таблица\nИстория заявок" || card.Contact != "https://example.com/help" {
		t.Fatalf("lost supported source text: %+v", card)
	}
	if card.Need != "" || !droppedHas(dropped, "need:") {
		t.Fatalf("question text must not become evidence: %+v %v", card, dropped)
	}
}

func TestParseCardLimitsAndWrongTypes(t *testing.T) {
	for key, limit := range cardLimits {
		if key == "category" {
			continue
		}
		t.Run(key, func(t *testing.T) {
			for _, length := range []int{limit, limit + 1} {
				value := strings.Repeat("я", length)
				raw, _ := json.Marshal(map[string]any{"card": map[string]string{key: value}})
				_, _, err := ParseCard(raw, value, nil)
				if (err == nil) != (length == limit) {
					t.Fatalf("length=%d limit=%d err=%v", length, limit, err)
				}
			}
		})
	}
	for _, value := range []string{"null", "12", "true", "[]", "{}"} {
		_, _, err := ParseCard([]byte(`{"card":{"context":`+value+`}}`), "source", nil)
		if !errors.Is(err, ErrInvalidOutput) {
			t.Fatalf("invalid field type %s should trigger fallback: %v", value, err)
		}
	}
}

func TestParseCardDropReasonsDoNotExposeOutput(t *testing.T) {
	raw := `{"card":{"secret-key":"secret-value","contact":"private-secret@example.com","category":"secret-category"}}`
	_, dropped, err := ParseCard([]byte(raw), "Нужна CRM", nil)
	if err == nil || len(dropped) != 3 || strings.Contains(strings.Join(dropped, " ")+err.Error(), "secret") {
		t.Fatalf("unexpected validation diagnostics: %v %v", dropped, err)
	}
}

func TestParseQuestionsRejectsOversizedText(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"questions": []Question{
		{Field: "data", Question: strings.Repeat("я", 2001)},
		{Field: "need", Question: "Зачем?"},
		{Field: "users", Question: "Кто?"},
	}})
	if _, err := ParseQuestions(raw); !errors.Is(err, ErrInvalidOutput) {
		t.Fatalf("oversized question leaves too few valid questions: %v", err)
	}
}
