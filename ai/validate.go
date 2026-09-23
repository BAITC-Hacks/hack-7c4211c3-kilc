package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/store"
)

// ErrInvalidOutput marks model output that cannot be turned into a result.
var ErrInvalidOutput = errors.New("некорректный ответ модели")

var (
	digitRun     = regexp.MustCompile(`\d+`)
	emailPattern = regexp.MustCompile(`[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}`)
)

// cardKeys lists the JSON keys of Card in output order.
var cardKeys = []string{
	"title", "category", "context", "need", "users", "data", "constraints",
	"expected_result", "success_criteria", "contact", "interaction_format",
}

// ParseQuestions keeps only well-formed questions about allowed fields.
func ParseQuestions(raw []byte) ([]Question, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, fmt.Errorf("%w: не JSON-объект: %v", ErrInvalidOutput, err)
	}
	body, ok := top["questions"]
	if !ok {
		return nil, fmt.Errorf("%w: нет ключа questions", ErrInvalidOutput)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("%w: questions не массив", ErrInvalidOutput)
	}
	seen := map[string]bool{}
	var questions []Question
	for _, item := range items {
		var fields map[string]any
		if err := json.Unmarshal(item, &fields); err != nil {
			continue
		}
		field, fieldOK := fields["field"].(string)
		text, textOK := fields["question"].(string)
		field = strings.TrimSpace(field)
		text = strings.TrimSpace(text)
		if !fieldOK || !textOK || text == "" || !isAllowedField(field) || seen[field] {
			continue
		}
		seen[field] = true
		questions = append(questions, Question{Field: field, Question: text})
	}
	if len(questions) < minStubQuestions {
		return nil, fmt.Errorf("%w: допустимых вопросов %d, нужно не меньше %d", ErrInvalidOutput, len(questions), minStubQuestions)
	}
	if len(questions) > maxStubQuestions {
		questions = questions[:maxStubQuestions]
	}
	return questions, nil
}

// ParseCard keeps only card fields whose facts come from the draft and answers.
// The returned list explains every dropped value.
func ParseCard(raw []byte, draft string, qa []QA) (Card, []string, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return Card{}, nil, fmt.Errorf("%w: не JSON-объект: %v", ErrInvalidOutput, err)
	}
	body, ok := top["card"]
	if !ok {
		return Card{}, nil, fmt.Errorf("%w: нет ключа card", ErrInvalidOutput)
	}
	var values map[string]any
	if err := json.Unmarshal(body, &values); err != nil || values == nil {
		return Card{}, nil, fmt.Errorf("%w: card не объект", ErrInvalidOutput)
	}

	input := strings.ToLower(draft)
	for _, item := range qa {
		input += "\n" + strings.ToLower(item.Answer)
	}

	var card Card
	var dropped []string
	for _, key := range cardKeys {
		value, present := values[key]
		if !present || value == nil {
			continue
		}
		text, isString := value.(string)
		if !isString {
			dropped = append(dropped, key+": значение не строка")
			continue
		}
		text = strings.TrimSpace(text)
		if key == "category" {
			if !store.ValidCategory(text) {
				dropped = append(dropped, "category: код "+text+" не из списка")
				continue
			}
		} else if reason := fabricated(text, input); reason != "" {
			dropped = append(dropped, key+": "+reason)
			continue
		}
		*cardKeyField(&card, key) = text
	}
	return card, dropped, nil
}

// fabricated names the first number or email in value that is absent from input.
func fabricated(value, input string) string {
	lower := strings.ToLower(value)
	for _, number := range digitRun.FindAllString(lower, -1) {
		if !strings.Contains(input, number) {
			return "число " + number + " отсутствует во вводе"
		}
	}
	for _, email := range emailPattern.FindAllString(lower, -1) {
		if !strings.Contains(input, email) {
			return "email " + email + " отсутствует во вводе"
		}
	}
	return ""
}

func cardKeyField(card *Card, key string) *string {
	switch key {
	case "title":
		return &card.Title
	case "category":
		return &card.Category
	default:
		return cardField(card, key)
	}
}
