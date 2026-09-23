package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/rating"
)

var allFields = []string{
	rating.FieldContext, rating.FieldNeed, rating.FieldUsers, rating.FieldData,
	rating.FieldConstraints, rating.FieldExpectedResult, rating.FieldSuccessCriteria,
	rating.FieldContact, rating.FieldInteractionFormat,
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"), loadSchema(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func fillAll(t *Task) {
	t.Context = "Компания ведет продажи в нескольких регионах"
	t.Need = "Сотрудникам нужна общая система учета обращений"
	t.Users = "Менеджеры отдела продаж работают каждый день"
	t.Data = "Доступны выгрузки сделок и история обращений"
	t.Constraints = "Решение должно работать в существующей инфраструктуре"
	t.ExpectedResult = "Команда получает отчет по каждому клиенту"
	t.SuccessCriteria = "Доля обработанных заявок вырастет минимум на 20 процентов"
	t.Contact = "Свяжитесь с Айгерим по адресу aigerim@example.kz"
	t.InteractionFormat = "Команда обсуждает требования с бизнесом каждую неделю"
}

func TestCreateTaskDraftOnly(t *testing.T) {
	s := openTestStore(t)
	task := Task{DraftText: "Нужна CRM для отдела продаж"}
	if err := s.CreateTask(context.Background(), &task); err != nil {
		t.Fatalf("create: %v", err)
	}
	if task.ID <= 0 || task.Score != 0 || task.Status != StatusDraft || task.CreatedAt.IsZero() {
		t.Fatalf("got id=%d score=%d status=%q created=%v", task.ID, task.Score, task.Status, task.CreatedAt)
	}
	var qa, confirmed string
	if err := s.DB.QueryRow(`SELECT qa, confirmed FROM tasks WHERE id = ?`, task.ID).Scan(&qa, &confirmed); err != nil {
		t.Fatalf("select: %v", err)
	}
	if qa != "[]" || confirmed != "[]" {
		t.Errorf("empty lists stored as qa=%q confirmed=%q, want []", qa, confirmed)
	}
}

func TestCreateTaskValidation(t *testing.T) {
	s := openTestStore(t)
	cases := []struct {
		name  string
		task  Task
		field string
	}{
		{"empty draft", Task{DraftText: "  "}, "draft_text"},
		{"bad category", Task{DraftText: "x", Category: "xyz"}, "category"},
		{"bad confirmed", Task{DraftText: "x", Confirmed: []string{"title"}}, "confirmed"},
	}
	for _, c := range cases {
		err := s.CreateTask(context.Background(), &c.task)
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != c.field {
			t.Errorf("%s: got %v, want ValidationError on %q", c.name, err, c.field)
		}
	}
}

func TestUpdateTaskFullScore(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task := Task{DraftText: "Нужна CRM"}
	if err := s.CreateTask(ctx, &task); err != nil {
		t.Fatalf("create: %v", err)
	}
	fillAll(&task)
	task.Confirmed = append(append([]string{}, allFields...), rating.FieldContext)
	if err := s.UpdateTask(ctx, &task); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := s.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Score != 100 {
		t.Errorf("score = %d, want 100", got.Score)
	}
	if len(got.Confirmed) != 9 {
		t.Errorf("confirmed = %v, want 9 unique fields", got.Confirmed)
	}
	if got.Status != StatusDraft || !got.CreatedAt.Equal(task.CreatedAt) {
		t.Errorf("status=%q created=%v, want draft and %v", got.Status, got.CreatedAt, task.CreatedAt)
	}

	missing := Task{ID: 999, DraftText: "x"}
	if err := s.UpdateTask(ctx, &missing); !errors.Is(err, ErrNotFound) {
		t.Errorf("update missing: got %v, want ErrNotFound", err)
	}
}

func TestPublishTask(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task := Task{DraftText: "Нужна CRM", Title: "CRM для продаж", Context: "Компания ведет продажи в регионах"}
	if err := s.CreateTask(ctx, &task); err != nil {
		t.Fatalf("create: %v", err)
	}
	err := s.PublishTask(ctx, task.ID)
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "confirmed" {
		t.Fatalf("publish unconfirmed: got %v, want ValidationError on confirmed", err)
	}

	task.Confirmed = []string{rating.FieldContext}
	if err := s.UpdateTask(ctx, &task); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := s.PublishTask(ctx, task.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}
	got, err := s.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != StatusPublished || got.PublishedAt.IsZero() {
		t.Fatalf("status=%q published=%v", got.Status, got.PublishedAt)
	}
	if err := s.PublishTask(ctx, task.ID); err != nil {
		t.Fatalf("publish twice: %v", err)
	}
	again, err := s.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !again.PublishedAt.Equal(got.PublishedAt) {
		t.Errorf("second publish changed published_at %v -> %v", got.PublishedAt, again.PublishedAt)
	}

	untitled := Task{DraftText: "x"}
	if err := s.CreateTask(ctx, &untitled); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.PublishTask(ctx, untitled.ID); !errors.As(err, &ve) || ve.Field != "title" {
		t.Errorf("publish untitled: got %v, want ValidationError on title", err)
	}
	if err := s.PublishTask(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("publish missing: got %v, want ErrNotFound", err)
	}
}

func createScored(t *testing.T, s *Store, title, industry string, keys []string, publish bool) Task {
	t.Helper()
	ctx := context.Background()
	var full Task
	fillAll(&full)
	texts := full.fieldTexts()
	task := Task{DraftText: "Черновик " + title, Title: title, Industry: industry, Category: "crm", Confirmed: keys}
	for _, k := range keys {
		switch k {
		case rating.FieldContext:
			task.Context = texts[k]
		case rating.FieldNeed:
			task.Need = texts[k]
		case rating.FieldUsers:
			task.Users = texts[k]
		case rating.FieldData:
			task.Data = texts[k]
		case rating.FieldConstraints:
			task.Constraints = texts[k]
		case rating.FieldExpectedResult:
			task.ExpectedResult = texts[k]
		case rating.FieldSuccessCriteria:
			task.SuccessCriteria = texts[k]
		case rating.FieldContact:
			task.Contact = texts[k]
		case rating.FieldInteractionFormat:
			task.InteractionFormat = texts[k]
		}
	}
	if err := s.CreateTask(ctx, &task); err != nil {
		t.Fatalf("create %s: %v", title, err)
	}
	if publish {
		if err := s.PublishTask(ctx, task.ID); err != nil {
			t.Fatalf("publish %s: %v", title, err)
		}
	}
	return task
}

func TestListCatalog(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	low := createScored(t, s, "low", "Ритейл", []string{rating.FieldData}, true)
	high := createScored(t, s, "high", "Логистика", allFields[:8], true)
	mid := createScored(t, s, "mid", "Ритейл", []string{
		rating.FieldContext, rating.FieldNeed, rating.FieldData, rating.FieldUsers, rating.FieldConstraints,
	}, true)
	createScored(t, s, "draft", "Медицина", allFields, false)

	if low.Score != 20 || high.Score != 95 || mid.Score != 60 {
		t.Fatalf("scores low=%d high=%d mid=%d, want 20 95 60", low.Score, high.Score, mid.Score)
	}

	all, err := s.ListCatalog(ctx, CatalogFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	wantIDs := []int64{high.ID, mid.ID, low.ID}
	if len(all) != len(wantIDs) {
		t.Fatalf("got %d tasks, want %d", len(all), len(wantIDs))
	}
	for i, id := range wantIDs {
		if all[i].ID != id {
			t.Errorf("position %d: id %d (score %d), want id %d", i, all[i].ID, all[i].Score, id)
		}
	}

	working, err := s.ListCatalog(ctx, CatalogFilter{Level: rating.LevelWorking})
	if err != nil {
		t.Fatalf("list working: %v", err)
	}
	if len(working) != 1 || working[0].ID != mid.ID {
		t.Errorf("working level: got %v, want only mid", working)
	}

	retail, err := s.ListCatalog(ctx, CatalogFilter{Industry: "Ритейл", Category: "crm"})
	if err != nil {
		t.Fatalf("list retail: %v", err)
	}
	if len(retail) != 2 {
		t.Errorf("retail: got %d tasks, want 2", len(retail))
	}

	var ve *ValidationError
	if _, err := s.ListCatalog(ctx, CatalogFilter{Category: "xyz"}); !errors.As(err, &ve) {
		t.Errorf("unknown category: got %v, want ValidationError", err)
	}
	if _, err := s.ListCatalog(ctx, CatalogFilter{Level: "legendary"}); !errors.As(err, &ve) {
		t.Errorf("unknown level: got %v, want ValidationError", err)
	}

	industries, err := s.ListIndustries(ctx)
	if err != nil {
		t.Fatalf("industries: %v", err)
	}
	if len(industries) != 2 || industries[0] != "Логистика" || industries[1] != "Ритейл" {
		t.Errorf("industries = %v, want [Логистика Ритейл]", industries)
	}
}

func TestGetTaskNotFound(t *testing.T) {
	s := openTestStore(t)
	if _, err := s.GetTask(context.Background(), 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}
