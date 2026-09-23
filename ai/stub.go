package ai

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/rating"
)

const (
	maxStubQuestions = 5
	minStubQuestions = 3
	maxTitleRunes    = 80
)

var stubQuestions = map[string]string{
	rating.FieldData:              "Какие данные и материалы есть для работы: таблицы, выгрузки, доступы к системам?",
	rating.FieldContext:           "Опишите контекст: чем занимается компания и как процесс устроен сейчас?",
	rating.FieldNeed:              "Какую проблему нужно решить и почему это важно именно сейчас?",
	rating.FieldExpectedResult:    "Что вы хотите получить на выходе: сервис, отчёт, бота, прототип?",
	rating.FieldSuccessCriteria:   "По каким измеримым показателям вы поймёте, что задача решена?",
	rating.FieldConstraints:       "Какие есть ограничения по срокам, бюджету или технологиям?",
	rating.FieldUsers:             "Кто будет пользоваться результатом и как часто?",
	rating.FieldContact:           "Как команде связаться с вами: имя и email или телефон?",
	rating.FieldInteractionFormat: "В каком формате удобно работать с командой: встречи, чат, как часто?",
}

var stubSignals = map[string][]string{
	rating.FieldData:        {"данн", "таблиц", "excel", "выгруз", "crm", "журнал"},
	rating.FieldUsers:       {"менеджер", "клиент", "сотрудник", "пользоват", "студент", "покупател"},
	rating.FieldConstraints: {"срок", "недел", "месяц", "бюджет", "технолог"},
}

// Stub answers both calls without a model, using only the user's own text.
type Stub struct{}

// NewStub returns the stub client.
func NewStub() Stub {
	return Stub{}
}

// Questions asks about the fields the draft shows no signal for.
func (Stub) Questions(ctx context.Context, draft string) (QuestionsResult, error) {
	if err := ctx.Err(); err != nil {
		return QuestionsResult{}, err
	}
	lower := strings.ToLower(draft)
	selected := map[string]bool{}
	var questions []Question
	for _, field := range AllowedFields {
		if len(questions) == maxStubQuestions {
			break
		}
		if !hasSignal(field, lower) {
			selected[field] = true
			questions = append(questions, Question{Field: field, Question: stubQuestions[field]})
		}
	}
	for _, field := range AllowedFields {
		if len(questions) >= minStubQuestions {
			break
		}
		if !selected[field] {
			selected[field] = true
			questions = append(questions, Question{Field: field, Question: stubQuestions[field]})
		}
	}
	return QuestionsResult{Questions: questions, Source: SourceStub}, nil
}

// Card copies the draft and answers verbatim into the card fields.
func (Stub) Card(ctx context.Context, draft string, qa []QA) (CardResult, error) {
	if err := ctx.Err(); err != nil {
		return CardResult{}, err
	}
	trimmed := strings.TrimSpace(draft)
	card := Card{Title: firstSentence(trimmed), Context: trimmed}
	for _, item := range qa {
		answer := strings.TrimSpace(item.Answer)
		if answer == "" || !isAllowedField(item.Field) {
			continue
		}
		target := cardField(&card, item.Field)
		if *target == "" {
			*target = answer
		} else {
			*target += "\n" + answer
		}
	}
	return CardResult{Card: card, Source: SourceStub}, nil
}

func hasSignal(field, lower string) bool {
	switch field {
	case rating.FieldSuccessCriteria:
		return strings.ContainsAny(lower, "0123456789%")
	case rating.FieldContact:
		return strings.Contains(lower, "@") || countDigits(lower) >= 7
	}
	for _, signal := range stubSignals[field] {
		if strings.Contains(lower, signal) {
			return true
		}
	}
	return false
}

func countDigits(text string) int {
	count := 0
	for _, r := range text {
		if unicode.IsDigit(r) {
			count++
		}
	}
	return count
}

func firstSentence(text string) string {
	if end := strings.IndexAny(text, ".!?\n"); end >= 0 {
		text = text[:end]
	}
	if utf8.RuneCountInString(text) > maxTitleRunes {
		text = string([]rune(text)[:maxTitleRunes])
	}
	return strings.TrimSpace(text)
}

func cardField(card *Card, field string) *string {
	switch field {
	case rating.FieldContext:
		return &card.Context
	case rating.FieldNeed:
		return &card.Need
	case rating.FieldUsers:
		return &card.Users
	case rating.FieldData:
		return &card.Data
	case rating.FieldConstraints:
		return &card.Constraints
	case rating.FieldExpectedResult:
		return &card.ExpectedResult
	case rating.FieldSuccessCriteria:
		return &card.SuccessCriteria
	case rating.FieldContact:
		return &card.Contact
	default:
		return &card.InteractionFormat
	}
}
