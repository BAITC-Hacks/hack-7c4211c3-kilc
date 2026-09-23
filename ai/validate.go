package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/store"
)

// ErrInvalidOutput marks model output that cannot be turned into a result.
var ErrInvalidOutput = errors.New("некорректный ответ модели")

// cardKeys lists the JSON keys of Card in output order.
var cardKeys = []string{
	"title", "category", "context", "need", "users", "data", "constraints",
	"expected_result", "success_criteria", "contact", "interaction_format",
}

var cardLimits = map[string]int{
	"title": 120, "category": 32, "context": 2000, "need": 2000,
	"users": 1000, "data": 2000, "constraints": 2000,
	"expected_result": 2000, "success_criteria": 2000,
	"contact": 250, "interaction_format": 1000,
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
		if !fieldOK || !textOK || text == "" || utf8.RuneCountInString(text) > 2000 || !isAllowedField(field) || seen[field] {
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

// ParseCard accepts extractive text from the draft/answers, never free paraphrases.
// It drops unknown/invalid fields and rejects cards with no usable source text.
// Reasons contain schema keys only, not user/model text or credentials.
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

	sources := []string{draft}
	for _, item := range qa {
		if isAllowedField(item.Field) {
			sources = append(sources, item.Answer)
		}
	}
	evidence := sourceUnits(sources)

	var card Card
	var dropped []string
	for key := range values {
		if _, known := cardLimits[key]; !known {
			dropped = append(dropped, "неизвестное поле отброшено")
		}
	}
	usable := false
	for _, key := range cardKeys {
		value, present := values[key]
		if !present {
			continue
		}
		text, isString := value.(string)
		if !isString {
			dropped = append(dropped, key+": значение не строка")
			continue
		}
		text = strings.TrimSpace(text)
		if utf8.RuneCountInString(text) > cardLimits[key] {
			dropped = append(dropped, key+": превышена допустимая длина")
			continue
		}
		if key == "category" {
			if !store.ValidCategory(text) {
				dropped = append(dropped, "category: код не из списка")
				continue
			}
		} else if text != "" && !groundedText(text, evidence) {
			dropped = append(dropped, key+": текст не совпадает с предложением или строкой исходных данных")
			continue
		}
		*cardKeyField(&card, key) = text
		if key != "category" && text != "" {
			usable = true
		}
	}
	if !usable {
		return Card{}, dropped, fmt.Errorf("%w: карточка не содержит подтверждённого исходными данными текста", ErrInvalidOutput)
	}
	return card, dropped, nil
}

// sourceUnits matches whole inputs and complete sentences/lines rather than
// individual words or substrings. Human review still determines their meaning.
func sourceUnits(sources []string) map[string]bool {
	units := make(map[string]bool)
	for _, source := range sources {
		units[normalizeExcerpt(source)] = true
		for _, unit := range textUnits(source) {
			units[normalizeExcerpt(unit)] = true
		}
	}
	delete(units, "")
	return units
}

func normalizeExcerpt(text string) string {
	return strings.TrimRight(strings.ToLower(strings.Join(strings.Fields(text), " ")), ".")
}

func groundedText(text string, evidence map[string]bool) bool {
	if evidence[normalizeExcerpt(text)] {
		return true
	}
	units := textUnits(text)
	if len(units) == 0 {
		return false
	}
	for _, unit := range units {
		if !evidence[normalizeExcerpt(unit)] {
			return false
		}
	}
	return true
}

func textUnits(text string) []string {
	runes := []rune(text)
	var units []string
	start := 0
	for i, r := range runes {
		lineEnd := r == '\n' || r == '\r'
		sentenceEnd := strings.ContainsRune(".!?", r) && (i+1 == len(runes) || unicode.IsSpace(runes[i+1]))
		if lineEnd || sentenceEnd {
			if unit := strings.TrimSpace(string(runes[start : i+1])); unit != "" {
				units = append(units, unit)
			}
			start = i + 1
		}
	}
	if unit := strings.TrimSpace(string(runes[start:])); unit != "" {
		units = append(units, unit)
	}
	return units
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
