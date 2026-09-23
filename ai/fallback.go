package ai

import (
	"context"
	"log"
)

// cardDropper is implemented by clients that report sanitized-away values.
type cardDropper interface {
	cardWithDropped(ctx context.Context, draft string, qa []QA) (CardResult, []string, error)
}

// Fallback uses Primary and answers with the stub when Primary fails.
type Fallback struct {
	Primary Client
	Stub    Stub
	Log     *log.Logger
}

// Questions asks Primary, falling back to the stub on any error except cancellation.
func (f Fallback) Questions(ctx context.Context, draft string) (QuestionsResult, error) {
	result, err := f.Primary.Questions(ctx, draft)
	if err == nil {
		return result, nil
	}
	if ctx.Err() != nil {
		return QuestionsResult{}, ctx.Err()
	}
	f.logger().Printf("ai: переход на заглушку: %v", err)
	return f.Stub.Questions(ctx, draft)
}

// Card asks Primary, logging dropped values, and falls back to the stub on error.
func (f Fallback) Card(ctx context.Context, draft string, qa []QA) (CardResult, error) {
	var result CardResult
	var err error
	if dropper, ok := f.Primary.(cardDropper); ok {
		var dropped []string
		result, dropped, err = dropper.cardWithDropped(ctx, draft, qa)
		for _, item := range dropped {
			f.logger().Printf("ai: отброшено из ответа модели: %s", item)
		}
	} else {
		result, err = f.Primary.Card(ctx, draft, qa)
	}
	if err == nil {
		return result, nil
	}
	if ctx.Err() != nil {
		return CardResult{}, ctx.Err()
	}
	f.logger().Printf("ai: переход на заглушку: %v", err)
	return f.Stub.Card(ctx, draft, qa)
}

func (f Fallback) logger() *log.Logger {
	if f.Log != nil {
		return f.Log
	}
	return log.Default()
}
