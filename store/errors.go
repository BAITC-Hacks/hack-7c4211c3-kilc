package store

import "errors"

var (
	ErrNotFound          = errors.New("запись не найдена")
	ErrInvalidTransition = errors.New("недопустимая смена статуса")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
