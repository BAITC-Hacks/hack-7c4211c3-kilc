// Package ai defines the two model calls used by the task constructor.
package ai

import (
	"context"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/rating"
)

const (
	SourceModel = "model"
	SourceStub  = "stub"
)

// AllowedFields lists the card keys the rating understands, in question order.
var AllowedFields = []string{
	rating.FieldData,
	rating.FieldContext,
	rating.FieldNeed,
	rating.FieldExpectedResult,
	rating.FieldSuccessCriteria,
	rating.FieldConstraints,
	rating.FieldUsers,
	rating.FieldContact,
	rating.FieldInteractionFormat,
}

// Question is one clarifying question about a card field.
type Question struct {
	Field    string `json:"field"`
	Question string `json:"question"`
}

// QA is a clarifying question together with the user's answer.
type QA struct {
	Field    string `json:"field"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// Card is the structured task card produced from a draft and answers.
type Card struct {
	Title             string `json:"title"`
	Category          string `json:"category"`
	Context           string `json:"context"`
	Need              string `json:"need"`
	Users             string `json:"users"`
	Data              string `json:"data"`
	Constraints       string `json:"constraints"`
	ExpectedResult    string `json:"expected_result"`
	SuccessCriteria   string `json:"success_criteria"`
	Contact           string `json:"contact"`
	InteractionFormat string `json:"interaction_format"`
}

// QuestionsResult carries clarifying questions and where they came from.
type QuestionsResult struct {
	Questions []Question
	Source    string
}

// CardResult carries a generated card and where it came from.
type CardResult struct {
	Card   Card
	Source string
}

// Client performs the two AI calls of the constructor.
type Client interface {
	Questions(ctx context.Context, draft string) (QuestionsResult, error)
	Card(ctx context.Context, draft string, qa []QA) (CardResult, error)
}

func isAllowedField(field string) bool {
	for _, allowed := range AllowedFields {
		if allowed == field {
			return true
		}
	}
	return false
}
