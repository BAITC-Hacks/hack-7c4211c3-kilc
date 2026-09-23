package ai

import (
	"context"
	"strings"
	"testing"
)

var _ Client = NewStub()

func questionFields(t *testing.T, draft string) []string {
	t.Helper()
	result, err := NewStub().Questions(context.Background(), draft)
	if err != nil {
		t.Fatalf("Questions returned error: %v", err)
	}
	if result.Source != SourceStub {
		t.Fatalf("source = %q, want %q", result.Source, SourceStub)
	}
	seen := map[string]bool{}
	var fields []string
	for _, question := range result.Questions {
		if !isAllowedField(question.Field) {
			t.Fatalf("field %q is not allowed", question.Field)
		}
		if seen[question.Field] {
			t.Fatalf("duplicate field %q", question.Field)
		}
		if strings.TrimSpace(question.Question) == "" {
			t.Fatalf("empty question for field %q", question.Field)
		}
		seen[question.Field] = true
		fields = append(fields, question.Field)
	}
	return fields
}

func TestQuestionsBareDraft(t *testing.T) {
	fields := questionFields(t, "Хотим автоматизировать заявки")
	if len(fields) != 5 {
		t.Fatalf("got %d questions, want 5: %v", len(fields), fields)
	}
	if fields[0] != "data" {
		t.Fatalf("first field = %q, want data", fields[0])
	}
}

func TestQuestionsSkipSignalledFields(t *testing.T) {
	draft := "Заявки ведём в таблица Excel, менеджеры теряют время. Срок 2 недели, ждём рост на 20%. Пишите a@b.kz"
	fields := questionFields(t, draft)
	if len(fields) < 3 {
		t.Fatalf("got %d questions, want at least 3: %v", len(fields), fields)
	}
	for _, field := range fields {
		switch field {
		case "data", "users", "constraints", "success_criteria", "contact":
			t.Fatalf("unexpected question for %q: %v", field, fields)
		}
	}
}

func TestQuestionsTopUpToThree(t *testing.T) {
	fields := questionFields(t, "Данные в CRM, клиенты и сотрудники, срок месяц, рост 30%, тел 87011234567")
	if len(fields) < 3 {
		t.Fatalf("got %d questions, want at least 3: %v", len(fields), fields)
	}
}

func TestCardCopiesAnswersVerbatim(t *testing.T) {
	draft := "  Нужна CRM. Сейчас всё в тетради  "
	qa := []QA{
		{Field: "data", Question: "q", Answer: "  Тетрадь с заказами за год "},
		{Field: "users", Question: "q", Answer: "Три менеджера"},
		{Field: "success_criteria", Question: "q", Answer: "Ни одной потерянной заявки"},
		{Field: "unknown", Question: "q", Answer: "лишнее"},
		{Field: "need", Question: "q", Answer: "   "},
	}
	result, err := NewStub().Card(context.Background(), draft, qa)
	if err != nil {
		t.Fatalf("Card returned error: %v", err)
	}
	card := result.Card
	if result.Source != SourceStub {
		t.Fatalf("source = %q, want %q", result.Source, SourceStub)
	}
	if card.Data != "Тетрадь с заказами за год" || card.Users != "Три менеджера" || card.SuccessCriteria != "Ни одной потерянной заявки" {
		t.Fatalf("answers not copied verbatim: %+v", card)
	}
	for name, value := range map[string]string{
		"need": card.Need, "constraints": card.Constraints, "contact": card.Contact,
		"interaction_format": card.InteractionFormat, "expected_result": card.ExpectedResult,
		"category": card.Category,
	} {
		if value != "" {
			t.Fatalf("%s = %q, want empty", name, value)
		}
	}
	if card.Title != "Нужна CRM" {
		t.Fatalf("title = %q, want %q", card.Title, "Нужна CRM")
	}
	if card.Context != strings.TrimSpace(draft) {
		t.Fatalf("context = %q, want trimmed draft", card.Context)
	}

	source := draft
	for _, item := range qa {
		source += "\n" + item.Answer
	}
	for name, value := range map[string]string{
		"title": card.Title, "category": card.Category, "context": card.Context,
		"need": card.Need, "users": card.Users, "data": card.Data,
		"constraints": card.Constraints, "expected_result": card.ExpectedResult,
		"success_criteria": card.SuccessCriteria, "contact": card.Contact,
		"interaction_format": card.InteractionFormat,
	} {
		if value != "" && !strings.Contains(source, value) {
			t.Fatalf("%s = %q is not in the user's text", name, value)
		}
	}
}

func TestCardJoinsAnswersForSameField(t *testing.T) {
	qa := []QA{{Field: "data", Answer: "Excel"}, {Field: "data", Answer: "Выгрузка 1С"}}
	result, err := NewStub().Card(context.Background(), "Задача", qa)
	if err != nil {
		t.Fatalf("Card returned error: %v", err)
	}
	if result.Card.Data != "Excel\nВыгрузка 1С" {
		t.Fatalf("data = %q", result.Card.Data)
	}
}

func TestTitleLimitedTo80Runes(t *testing.T) {
	title := firstSentence(strings.Repeat("ж", 100))
	if len([]rune(title)) != 80 {
		t.Fatalf("title has %d runes, want 80", len([]rune(title)))
	}
}

func TestCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewStub().Questions(ctx, "x"); err == nil {
		t.Fatal("Questions: want error on cancelled context")
	}
	if _, err := NewStub().Card(ctx, "x", nil); err == nil {
		t.Fatal("Card: want error on cancelled context")
	}
}
