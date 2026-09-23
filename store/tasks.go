package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/rating"
)

type QA struct {
	Field    string `json:"field"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type Task struct {
	ID                int64
	Company           string
	Title             string
	Industry          string
	Category          string
	DraftText         string
	QA                []QA
	Context           string
	Need              string
	Users             string
	Data              string
	Constraints       string
	ExpectedResult    string
	SuccessCriteria   string
	Contact           string
	InteractionFormat string
	Confirmed         []string
	Status            string
	Score             int
	CreatedAt         time.Time
	PublishedAt       time.Time
}

type CatalogFilter struct {
	Category string
	Industry string
	Level    string
}

const (
	StatusDraft     = "draft"
	StatusPublished = "published"
)

const (
	FieldTitle    = "title"
	FieldCategory = "category"
)

// confirmableFields lists every key accepted in Task.Confirmed, in the order
// used for publication errors. Title and category are confirmed but never scored.
var confirmableFields = []string{
	FieldTitle, FieldCategory,
	rating.FieldContext, rating.FieldNeed, rating.FieldUsers, rating.FieldData,
	rating.FieldConstraints, rating.FieldExpectedResult, rating.FieldSuccessCriteria,
	rating.FieldContact, rating.FieldInteractionFormat, rating.FieldReward,
}

func isConfirmable(key string) bool {
	for _, f := range confirmableFields {
		if f == key {
			return true
		}
	}
	return false
}

const taskColumns = `id, company, title, industry, category, draft_text, qa,
	context, need, users, data, constraints, expected_result, success_criteria,
	contact, interaction_format, confirmed, status, score, created_at, published_at`

func (t Task) Card() rating.Card {
	confirmed := make(map[string]bool, len(t.Confirmed))
	for _, f := range t.Confirmed {
		confirmed[f] = true
	}
	field := func(key, text string) rating.Field {
		return rating.Field{Text: text, Confirmed: confirmed[key]}
	}
	return rating.Card{
		Context:           field(rating.FieldContext, t.Context),
		Need:              field(rating.FieldNeed, t.Need),
		Users:             field(rating.FieldUsers, t.Users),
		Data:              field(rating.FieldData, t.Data),
		Constraints:       field(rating.FieldConstraints, t.Constraints),
		ExpectedResult:    field(rating.FieldExpectedResult, t.ExpectedResult),
		SuccessCriteria:   field(rating.FieldSuccessCriteria, t.SuccessCriteria),
		Contact:           field(rating.FieldContact, t.Contact),
		InteractionFormat: field(rating.FieldInteractionFormat, t.InteractionFormat),
	}
}

func (t Task) fieldTexts() map[string]string {
	return map[string]string{
		rating.FieldContext:           t.Context,
		rating.FieldNeed:              t.Need,
		rating.FieldUsers:             t.Users,
		rating.FieldData:              t.Data,
		rating.FieldConstraints:       t.Constraints,
		rating.FieldExpectedResult:    t.ExpectedResult,
		rating.FieldSuccessCriteria:   t.SuccessCriteria,
		rating.FieldContact:           t.Contact,
		rating.FieldInteractionFormat: t.InteractionFormat,
	}
}

// confirmableTexts returns the stored value of every confirmable field the
// task holds. Reward is absent until the task stores a reward.
func (t Task) confirmableTexts() map[string]string {
	texts := t.fieldTexts()
	texts[FieldTitle] = t.Title
	texts[FieldCategory] = t.Category
	return texts
}

// keepUnchangedConfirmations returns the requested confirmations whose field
// value equals the stored one. A changed field must be confirmed again by a
// later save.
func keepUnchangedConfirmations(t, stored Task) []string {
	next, prev := t.confirmableTexts(), stored.confirmableTexts()
	confirmed := make([]string, 0, len(t.Confirmed))
	for _, f := range t.Confirmed {
		if next[f] == prev[f] {
			confirmed = append(confirmed, f)
		}
	}
	return confirmed
}

// publicationError reports why the task cannot be visible in the catalog:
// every filled confirmable field must be confirmed and the title is required.
func publicationError(t Task) error {
	confirmed := make(map[string]bool, len(t.Confirmed))
	for _, f := range t.Confirmed {
		confirmed[f] = true
	}
	texts := t.confirmableTexts()
	var unconfirmed []string
	for _, key := range confirmableFields {
		if strings.TrimSpace(texts[key]) != "" && !confirmed[key] {
			unconfirmed = append(unconfirmed, key)
		}
	}
	if len(unconfirmed) > 0 {
		return &ValidationError{Field: "confirmed", Message: "подтвердите поля перед публикацией: " + strings.Join(unconfirmed, ", ")}
	}
	if strings.TrimSpace(t.Title) == "" {
		return &ValidationError{Field: "title", Message: "укажите название задачи перед публикацией"}
	}
	return nil
}

func validateTask(t *Task) error {
	if strings.TrimSpace(t.DraftText) == "" {
		return &ValidationError{Field: "draft_text", Message: "исходное описание задачи не может быть пустым"}
	}
	if !ValidCategory(t.Category) {
		return &ValidationError{Field: "category", Message: "неизвестная категория «" + t.Category + "»"}
	}
	seen := make(map[string]bool, len(t.Confirmed))
	confirmed := make([]string, 0, len(t.Confirmed))
	for _, f := range t.Confirmed {
		if !isConfirmable(f) {
			return &ValidationError{Field: "confirmed", Message: "неизвестное поле «" + f + "»"}
		}
		if seen[f] {
			continue
		}
		seen[f] = true
		confirmed = append(confirmed, f)
	}
	t.Confirmed = confirmed
	if t.QA == nil {
		t.QA = []QA{}
	}
	return nil
}

func encodeLists(t *Task) (qa, confirmed string, err error) {
	qaJSON, err := json.Marshal(t.QA)
	if err != nil {
		return "", "", fmt.Errorf("кодировать qa: %w", err)
	}
	confirmedJSON, err := json.Marshal(t.Confirmed)
	if err != nil {
		return "", "", fmt.Errorf("кодировать confirmed: %w", err)
	}
	return string(qaJSON), string(confirmedJSON), nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("разобрать время %q: %w", s, err)
	}
	return t.UTC(), nil
}

func (s *Store) CreateTask(ctx context.Context, t *Task) error {
	if err := validateTask(t); err != nil {
		return err
	}
	qa, confirmed, err := encodeLists(t)
	if err != nil {
		return err
	}
	t.Status = StatusDraft
	t.Score = rating.Score(t.Card()).Total
	t.CreatedAt = time.Now().UTC().Truncate(time.Second)
	t.PublishedAt = time.Time{}

	res, err := s.DB.ExecContext(ctx, `INSERT INTO tasks (
		company, title, industry, category, draft_text, qa,
		context, need, users, data, constraints, expected_result, success_criteria,
		contact, interaction_format, confirmed, status, score, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.Company, t.Title, t.Industry, t.Category, t.DraftText, qa,
		t.Context, t.Need, t.Users, t.Data, t.Constraints, t.ExpectedResult, t.SuccessCriteria,
		t.Contact, t.InteractionFormat, confirmed, t.Status, t.Score, formatTime(t.CreatedAt))
	if err != nil {
		return fmt.Errorf("создать задачу: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("получить id задачи: %w", err)
	}
	t.ID = id
	return nil
}

// UpdateTask saves the editable fields of a task. Within one transaction it
// compares them with the stored values: draft_text cannot change, a changed
// field loses its confirmation, and a published task left with unconfirmed
// filled fields returns to draft until it is confirmed and republished.
func (s *Store) UpdateTask(ctx context.Context, t *Task) error {
	if err := validateTask(t); err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("обновить задачу %d: начать транзакцию: %w", t.ID, err)
	}
	defer tx.Rollback()

	stored, err := scanTask(tx.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM tasks WHERE id = ?`, t.ID))
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("прочитать задачу %d: %w", t.ID, err)
	}
	if t.DraftText != stored.DraftText {
		return &ValidationError{Field: "draft_text", Message: "исходное описание задачи нельзя изменять после сохранения"}
	}
	t.Confirmed = keepUnchangedConfirmations(*t, stored)
	qa, confirmed, err := encodeLists(t)
	if err != nil {
		return err
	}
	t.Score = rating.Score(t.Card()).Total
	t.Status = stored.Status
	t.CreatedAt = stored.CreatedAt
	t.PublishedAt = stored.PublishedAt
	if t.Status == StatusPublished && publicationError(*t) != nil {
		t.Status = StatusDraft
		t.PublishedAt = time.Time{}
	}
	publishedAt := sql.NullString{}
	if !t.PublishedAt.IsZero() {
		publishedAt = sql.NullString{String: formatTime(t.PublishedAt), Valid: true}
	}

	_, err = tx.ExecContext(ctx, `UPDATE tasks SET
		company = ?, title = ?, industry = ?, category = ?, qa = ?,
		context = ?, need = ?, users = ?, data = ?, constraints = ?, expected_result = ?,
		success_criteria = ?, contact = ?, interaction_format = ?, confirmed = ?, score = ?,
		status = ?, published_at = ?
		WHERE id = ?`,
		t.Company, t.Title, t.Industry, t.Category, qa,
		t.Context, t.Need, t.Users, t.Data, t.Constraints, t.ExpectedResult,
		t.SuccessCriteria, t.Contact, t.InteractionFormat, confirmed, t.Score,
		t.Status, publishedAt, t.ID)
	if err != nil {
		return fmt.Errorf("обновить задачу %d: %w", t.ID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("обновить задачу %d: завершить транзакцию: %w", t.ID, err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTask(row rowScanner) (Task, error) {
	var t Task
	var qa, confirmed, createdAt string
	var publishedAt sql.NullString
	err := row.Scan(&t.ID, &t.Company, &t.Title, &t.Industry, &t.Category, &t.DraftText, &qa,
		&t.Context, &t.Need, &t.Users, &t.Data, &t.Constraints, &t.ExpectedResult, &t.SuccessCriteria,
		&t.Contact, &t.InteractionFormat, &confirmed, &t.Status, &t.Score, &createdAt, &publishedAt)
	if err != nil {
		return Task{}, err
	}
	if err := json.Unmarshal([]byte(qa), &t.QA); err != nil {
		return Task{}, fmt.Errorf("разобрать qa задачи %d: %w", t.ID, err)
	}
	if err := json.Unmarshal([]byte(confirmed), &t.Confirmed); err != nil {
		return Task{}, fmt.Errorf("разобрать confirmed задачи %d: %w", t.ID, err)
	}
	if t.CreatedAt, err = parseTime(createdAt); err != nil {
		return Task{}, err
	}
	if publishedAt.Valid {
		if t.PublishedAt, err = parseTime(publishedAt.String); err != nil {
			return Task{}, err
		}
	}
	return t, nil
}

func (s *Store) GetTask(ctx context.Context, id int64) (Task, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM tasks WHERE id = ?`, id)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("прочитать задачу %d: %w", id, err)
	}
	return t, nil
}

// PublishTask validates the stored task and makes it visible in the catalog.
// Publishing an already published task keeps its published_at.
func (s *Store) PublishTask(ctx context.Context, id int64) error {
	t, err := s.GetTask(ctx, id)
	if err != nil {
		return err
	}
	if err := publicationError(t); err != nil {
		return err
	}
	if t.Status == StatusPublished {
		return nil
	}
	_, err = s.DB.ExecContext(ctx, `UPDATE tasks SET status = ?, published_at = ? WHERE id = ?`,
		StatusPublished, formatTime(time.Now()), id)
	if err != nil {
		return fmt.Errorf("опубликовать задачу %d: %w", id, err)
	}
	return nil
}

func (s *Store) ListCatalog(ctx context.Context, f CatalogFilter) ([]Task, error) {
	query := `SELECT ` + taskColumns + ` FROM tasks WHERE status = ?`
	args := []any{StatusPublished}
	if f.Category != "" {
		if !ValidCategory(f.Category) {
			return nil, &ValidationError{Field: "category", Message: "неизвестная категория «" + f.Category + "»"}
		}
		query += ` AND category = ?`
		args = append(args, f.Category)
	}
	if f.Industry != "" {
		query += ` AND industry = ?`
		args = append(args, f.Industry)
	}
	if f.Level != "" {
		min, max, ok := rating.LevelRange(f.Level)
		if !ok {
			return nil, &ValidationError{Field: "level", Message: "неизвестный уровень «" + f.Level + "»"}
		}
		query += ` AND score BETWEEN ? AND ?`
		args = append(args, min, max)
	}
	query += ` ORDER BY score DESC, published_at DESC, id DESC`

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("прочитать каталог: %w", err)
	}
	defer rows.Close()
	tasks := []Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("прочитать каталог: %w", err)
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("прочитать каталог: %w", err)
	}
	return tasks, nil
}

func (s *Store) ListIndustries(ctx context.Context) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT DISTINCT industry FROM tasks
		WHERE status = ? AND industry <> '' ORDER BY industry`, StatusPublished)
	if err != nil {
		return nil, fmt.Errorf("прочитать отрасли: %w", err)
	}
	defer rows.Close()
	industries := []string{}
	for rows.Next() {
		var industry string
		if err := rows.Scan(&industry); err != nil {
			return nil, fmt.Errorf("прочитать отрасли: %w", err)
		}
		industries = append(industries, industry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("прочитать отрасли: %w", err)
	}
	return industries, nil
}
