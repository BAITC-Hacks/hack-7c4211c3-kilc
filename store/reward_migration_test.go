package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewardMigrationPreservesExistingData(t *testing.T) {
	schema := loadSchema(t)
	var old []string
	for _, line := range strings.Split(schema, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "reward ") || strings.HasPrefix(trim, "reward_type ") || strings.HasPrefix(trim, "bonus ") {
			continue
		}
		old = append(old, line)
	}
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(strings.Join(old, "\n")); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO tasks(title,draft_text,created_at) VALUES('Existing','Original','2026-09-23T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	for i := 0; i < 2; i++ {
		st, err := Open(path, schema)
		if err != nil {
			t.Fatal(err)
		}
		task, err := st.GetTask(context.Background(), 1)
		if err != nil {
			t.Fatal(err)
		}
		if task.Title != "Existing" || task.Reward != "" || task.Bonus != 0 {
			t.Fatalf("data changed: %+v", task)
		}
		st.Close()
	}
}

func TestRewardCatalogOrderingAndConfirmation(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "rewards.db"), loadSchema(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	reward := Task{Title: "Reward", DraftText: "Draft", RewardType: "money", Reward: "Грант 100000 тенге"}
	if err := st.CreateTask(ctx, &reward); err != nil {
		t.Fatal(err)
	}
	if reward.Bonus != 0 {
		t.Fatal("unconfirmed reward earned bonus")
	}
	if err := st.PublishTask(ctx, reward.ID); err == nil {
		t.Fatal("unconfirmed reward published")
	}
	reward.Confirmed = []string{"reward"}
	if err := st.UpdateTask(ctx, &reward); err != nil {
		t.Fatal(err)
	}
	if err := st.PublishTask(ctx, reward.ID); err != nil {
		t.Fatal(err)
	}
	plain := Task{Title: "Plain", DraftText: "Draft"}
	if err := st.CreateTask(ctx, &plain); err != nil {
		t.Fatal(err)
	}
	if err := st.PublishTask(ctx, plain.ID); err != nil {
		t.Fatal(err)
	}
	tasks, err := st.ListCatalog(ctx, CatalogFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || tasks[0].ID != reward.ID || tasks[0].Bonus != 10 || tasks[0].Score != 0 {
		t.Fatalf("ordering or score changed: %+v", tasks)
	}
}
