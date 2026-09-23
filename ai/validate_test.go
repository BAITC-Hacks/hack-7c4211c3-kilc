package ai

import (
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

func TestParseCardCategory(t *testing.T) {
	card, dropped := mustParseCard(t, `{"card":{"category":"fintech"}}`, "x", nil)
	if card.Category != "" || !droppedHas(dropped, "category:") {
		t.Fatalf("category = %q, dropped = %v", card.Category, dropped)
	}
	card, dropped = mustParseCard(t, `{"card":{"category":"crm"}}`, "x", nil)
	if card.Category != "crm" || len(dropped) != 0 {
		t.Fatalf("category = %q, dropped = %v", card.Category, dropped)
	}
}

func TestParseCardInventedNumber(t *testing.T) {
	raw := `{"card":{"need":"рост продаж на 40%"}}`
	card, dropped := mustParseCard(t, raw, "Хотим больше продаж", nil)
	if card.Need != "" || !droppedHas(dropped, "need: число 40") {
		t.Fatalf("need = %q, dropped = %v", card.Need, dropped)
	}
	card, dropped = mustParseCard(t, raw, "Хотим больше продаж", []QA{{Field: "success_criteria", Answer: "на 40 процентов"}})
	if card.Need != "рост продаж на 40%" || len(dropped) != 0 {
		t.Fatalf("need = %q, dropped = %v", card.Need, dropped)
	}
}

func TestParseCardInventedEmail(t *testing.T) {
	card, dropped := mustParseCard(t, `{"card":{"contact":"Иван, ivan@corp.kz"}}`, "Пишите Ивану", nil)
	if card.Contact != "" || !droppedHas(dropped, "contact: email ivan@corp.kz") {
		t.Fatalf("contact = %q, dropped = %v", card.Contact, dropped)
	}
	card, _ = mustParseCard(t, `{"card":{"contact":"Ivan@Corp.kz"}}`, "Пишите ivan@corp.kz", nil)
	if card.Contact != "Ivan@Corp.kz" {
		t.Fatalf("contact = %q, want kept", card.Contact)
	}
}

func TestParseCardNonStringValue(t *testing.T) {
	card, dropped := mustParseCard(t, `{"card":{"users":12,"data":" Excel "}}`, "Таблица Excel", nil)
	if card.Users != "" || !droppedHas(dropped, "users:") {
		t.Fatalf("users = %q, dropped = %v", card.Users, dropped)
	}
	if card.Data != "Excel" {
		t.Fatalf("data = %q, want trimmed", card.Data)
	}
}

func TestParseCardInvalidShape(t *testing.T) {
	for _, raw := range []string{`{"card": "text"}`, `not json`, `{"questions":[]}`, `{"card":null}`} {
		if _, _, err := ParseCard([]byte(raw), "x", nil); !errors.Is(err, ErrInvalidOutput) {
			t.Fatalf("%s: err = %v, want ErrInvalidOutput", raw, err)
		}
	}
}
