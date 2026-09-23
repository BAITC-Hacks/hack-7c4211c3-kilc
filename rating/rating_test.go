package rating

import (
	"strings"
	"testing"
)

var componentKeys = []string{
	"context_and_need", "data_and_materials", "expected_result",
	"success_criteria", "constraints", "users", "business_contact",
}

func fullCard(confirmed bool) Card {
	return Card{
		Context:           Field{"Компания ведет продажи в нескольких регионах", confirmed},
		Need:              Field{"Сотрудникам нужна общая система учета обращений", confirmed},
		Users:             Field{"Менеджеры отдела продаж работают каждый день", confirmed},
		Data:              Field{"Доступны выгрузки сделок и история обращений", confirmed},
		Constraints:       Field{"Решение должно работать в существующей инфраструктуре", confirmed},
		ExpectedResult:    Field{"Команда получает отчет по каждому клиенту", confirmed},
		SuccessCriteria:   Field{"Доля обработанных заявок вырастет минимум на 20 процентов", confirmed},
		Contact:           Field{"Свяжитесь с Айгерим по адресу aigerim@example.kz", confirmed},
		InteractionFormat: Field{"Команда обсуждает требования с бизнесом каждую неделю", confirmed},
	}
}

func componentByKey(t *testing.T, result Result, key string) Component {
	t.Helper()
	for _, component := range result.Components {
		if component.Key == key {
			return component
		}
	}
	t.Fatalf("component %q not found", key)
	return Component{}
}

func hintByField(t *testing.T, result Result, field string) (Hint, bool) {
	t.Helper()
	for _, hint := range result.Hints {
		if hint.Field == field {
			return hint, true
		}
	}
	return Hint{}, false
}

func TestScoreEmptyCard(t *testing.T) {
	result := Score(Card{})
	if result.Total != 0 || result.Potential != 0 || result.Level != LevelDraft {
		t.Fatalf("got total=%d potential=%d level=%q", result.Total, result.Potential, result.Level)
	}
	if result.Components == nil || len(result.Components) != 7 {
		t.Fatalf("components must contain 7 entries, got %#v", result.Components)
	}
	maxTotal := 0
	for i, component := range result.Components {
		if component.Key != componentKeys[i] {
			t.Errorf("component %d key = %q, want %q", i, component.Key, componentKeys[i])
		}
		maxTotal += component.Max
	}
	if maxTotal != 100 {
		t.Errorf("component maxima sum to %d, want 100", maxTotal)
	}
	if len(result.Hints) != 9 {
		t.Fatalf("got %d hints, want 9", len(result.Hints))
	}
	for _, hint := range result.Hints {
		if hint.Reason != ReasonEmpty {
			t.Errorf("hint for %q has reason %q, want empty", hint.Field, hint.Reason)
		}
		if hint.Message == "" {
			t.Errorf("hint for %q has empty message", hint.Field)
		}
	}
}

func TestScoreCompleteConfirmedAndUnconfirmed(t *testing.T) {
	allConfirmed := Score(fullCard(true))
	if allConfirmed.Total != 100 || allConfirmed.Potential != 100 || allConfirmed.Level != LevelPriority {
		t.Errorf("complete card: got total=%d potential=%d level=%q", allConfirmed.Total, allConfirmed.Potential, allConfirmed.Level)
	}
	if allConfirmed.Hints == nil || len(allConfirmed.Hints) != 0 {
		t.Errorf("complete card hints must be a non-nil empty slice, got %#v", allConfirmed.Hints)
	}

	allUnconfirmed := Score(fullCard(false))
	if allUnconfirmed.Total != 0 || allUnconfirmed.Potential != 100 {
		t.Errorf("unconfirmed card: got total=%d potential=%d", allUnconfirmed.Total, allUnconfirmed.Potential)
	}
	if len(allUnconfirmed.Hints) != 9 {
		t.Fatalf("unconfirmed card has %d hints, want 9", len(allUnconfirmed.Hints))
	}
	for _, hint := range allUnconfirmed.Hints {
		if hint.Reason != ReasonUnconfirmed {
			t.Errorf("hint for %q has reason %q, want unconfirmed", hint.Field, hint.Reason)
		}
	}
}

func TestScoreLevelThresholds(t *testing.T) {
	cases := []struct {
		name  string
		card  Card
		total int
		level string
	}{
		{"context need data", Card{Context: fullCard(true).Context, Need: fullCard(true).Need, Data: fullCard(true).Data}, 40, LevelWorking},
		{"plus result and criteria", Card{Context: fullCard(true).Context, Need: fullCard(true).Need, Data: fullCard(true).Data, ExpectedResult: fullCard(true).ExpectedResult, SuccessCriteria: fullCard(true).SuccessCriteria}, 70, LevelReady},
		{"plus constraints and users", Card{Context: fullCard(true).Context, Need: fullCard(true).Need, Data: fullCard(true).Data, ExpectedResult: fullCard(true).ExpectedResult, SuccessCriteria: fullCard(true).SuccessCriteria, Constraints: fullCard(true).Constraints, Users: fullCard(true).Users}, 90, LevelPriority},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := Score(tc.card)
			if result.Total != tc.total || result.Level != tc.level {
				t.Errorf("got total=%d level=%q, want total=%d level=%q", result.Total, result.Level, tc.total, tc.level)
			}
		})
	}
}

func TestScoreFieldQualityHints(t *testing.T) {
	tests := []struct {
		name       string
		field      Field
		component  string
		wantScore  int
		wantReason string
		wantGain   int
		wantNoHint bool
	}{
		{"criteria without number", Field{"Доля заявок с ответом вырастет за первый месяц", true}, "success_criteria", 7, ReasonNoNumber, 8, false},
		{"contact without contact details", Field{"Позвоните менеджеру", true}, "business_contact", 2, ReasonNoContact, 3, false},
		{"contact email", Field{"aigerim@example.kz", true}, "business_contact", 5, "", 0, true},
		{"short context", Field{"Нужна CRM", true}, "context_and_need", 5, ReasonShort, 5, false},
		{"whitespace is empty", Field{"   ", true}, "context_and_need", 0, ReasonEmpty, 10, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			card := Card{}
			switch tc.name {
			case "criteria without number":
				card.SuccessCriteria = tc.field
			case "contact without contact details", "contact email":
				card.Contact = tc.field
			default:
				card.Context = tc.field
			}
			result := Score(card)
			if got := componentByKey(t, result, tc.component).Score; got != tc.wantScore {
				t.Errorf("component score = %d, want %d", got, tc.wantScore)
			}
			hint, ok := hintByField(t, result, map[string]string{
				"criteria without number":         FieldSuccessCriteria,
				"contact without contact details": FieldContact,
				"contact email":                   FieldContact,
				"short context":                   FieldContext,
				"whitespace is empty":             FieldContext,
			}[tc.name])
			if tc.wantNoHint {
				if ok {
					t.Errorf("unexpected hint: %#v", hint)
				}
				return
			}
			if !ok {
				t.Fatal("expected field hint")
			}
			if hint.Reason != tc.wantReason || hint.Gain != tc.wantGain {
				t.Errorf("hint = reason %q gain %d, want reason %q gain %d", hint.Reason, hint.Gain, tc.wantReason, tc.wantGain)
			}
		})
	}
	// A hint must identify the field in Russian, while exact wording stays flexible.
	result := Score(Card{Context: Field{"Нужна CRM", true}})
	hint, ok := hintByField(t, result, FieldContext)
	if !ok || !strings.Contains(hint.Message, "Контекст") {
		t.Errorf("context hint message %q should contain its Russian field label", hint.Message)
	}
}

func TestScoreHintGainOrder(t *testing.T) {
	result := Score(Card{Data: Field{}, Users: Field{}})
	if len(result.Hints) < 2 {
		t.Fatalf("got %d hints, want at least 2", len(result.Hints))
	}
	if result.Hints[0].Field != FieldData || result.Hints[0].Gain != 20 {
		t.Errorf("first hint = %#v, want empty data gain 20", result.Hints[0])
	}
	if result.Hints[1].Field != FieldUsers || result.Hints[1].Gain != 10 {
		t.Errorf("second hint = %#v, want empty users gain 10", result.Hints[1])
	}
}

func TestLevelFor(t *testing.T) {
	cases := []struct {
		score int
		want  string
	}{
		{-5, LevelDraft}, {0, LevelDraft}, {39, LevelDraft},
		{40, LevelWorking}, {69, LevelWorking},
		{70, LevelReady}, {89, LevelReady},
		{90, LevelPriority}, {100, LevelPriority}, {150, LevelPriority},
	}
	for _, tc := range cases {
		if got := LevelFor(tc.score); got != tc.want {
			t.Errorf("LevelFor(%d) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

func TestLevelRange(t *testing.T) {
	cases := []struct {
		level    string
		min, max int
		ok       bool
	}{
		{LevelDraft, 0, 39, true},
		{LevelWorking, 40, 69, true},
		{LevelReady, 70, 89, true},
		{LevelPriority, 90, 100, true},
		{"unknown", 0, 0, false},
	}
	for _, tc := range cases {
		min, max, ok := LevelRange(tc.level)
		if min != tc.min || max != tc.max || ok != tc.ok {
			t.Errorf("LevelRange(%q) = (%d, %d, %t), want (%d, %d, %t)", tc.level, min, max, ok, tc.min, tc.max, tc.ok)
		}
	}
}
