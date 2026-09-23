package store

import (
	"context"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestListBusinessTasksSeeded(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	if err := s.Seed(ctx, os.DirFS("../data")); err != nil {
		t.Fatal(err)
	}
	tasks, err := s.ListBusinessTasks(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 10 {
		t.Fatalf("tasks = %d, want 10", len(tasks))
	}
	var total, pending, accepted, rejected, published int
	for i, task := range tasks {
		if task.Status == StatusPublished {
			published++
		}
		if i > 0 {
			previous := tasks[i-1]
			if previous.Status == StatusDraft && task.Status == StatusPublished {
				t.Fatal("published task after a draft")
			}
			if previous.Status == task.Status && (previous.Score < task.Score || (previous.Score == task.Score && previous.ID < task.ID)) {
				t.Fatalf("wrong order: %+v then %+v", previous, task)
			}
		}
		if task.Proposals != task.Pending+task.Accepted+task.Rejected {
			t.Errorf("counts disagree for task %d", task.ID)
		}
		total += task.Proposals
		pending += task.Pending
		accepted += task.Accepted
		rejected += task.Rejected
		original, err := s.GetTask(ctx, task.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(task.Task, original) {
			t.Errorf("task fields lost for %d", task.ID)
		}
		proposals, err := s.ListProposalsByTask(ctx, task.ID)
		if err != nil {
			t.Fatal(err)
		}
		counts := map[string]int{}
		for _, p := range proposals {
			counts[p.Status]++
		}
		if task.Proposals != len(proposals) || task.Pending != counts[ProposalPending] || task.Accepted != counts[ProposalAccepted] || task.Rejected != counts[ProposalRejected] {
			t.Errorf("wrong per-task counts: %+v", task)
		}
	}
	if published != 5 || total != 5 || pending != 3 || accepted != 1 || rejected != 1 {
		t.Fatalf("published=%d counts=%d/%d/%d/%d", published, total, pending, accepted, rejected)
	}
	company := tasks[0].Company
	filtered, err := s.ListBusinessTasks(ctx, " \t"+company+"\n ")
	if err != nil {
		t.Fatal(err)
	}
	want := []TaskSummary{}
	for _, task := range tasks {
		if task.Company == company {
			want = append(want, task)
		}
	}
	if !reflect.DeepEqual(filtered, want) {
		t.Fatalf("company filter: got %+v want %+v", filtered, want)
	}
	for _, filter := range []string{"unknown company", "' OR 1=1 --", company + " suffix"} {
		got, err := s.ListBusinessTasks(ctx, filter)
		if err != nil || got == nil || len(got) != 0 {
			t.Errorf("filter %q: tasks=%v err=%v", filter, got, err)
		}
	}
	all, err := s.ListBusinessTasks(ctx, " \t\n")
	if err != nil || !reflect.DeepEqual(all, tasks) {
		t.Fatalf("blank filter: %v", err)
	}
}

func TestBusinessTaskOrderAndZeroCounts(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	var ids []int64
	for _, status := range []string{StatusPublished, StatusDraft, StatusPublished} {
		task := Task{Title: "Task", DraftText: "Draft", Company: "Same"}
		if err := s.CreateTask(ctx, &task); err != nil {
			t.Fatal(err)
		}
		if status == StatusPublished {
			if err := s.PublishTask(ctx, task.ID); err != nil {
				t.Fatal(err)
			}
		}
		ids = append(ids, task.ID)
	}
	// A high-scoring draft must still follow all published tasks; tied IDs descend.
	if _, err := s.DB.Exec(`UPDATE tasks SET score=100 WHERE id=?`, ids[1]); err != nil {
		t.Fatal(err)
	}
	tasks, err := s.ListBusinessTasks(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	var got []int64
	for _, task := range tasks {
		got = append(got, task.ID)
		if task.Proposals != 0 || task.Pending != 0 || task.Accepted != 0 || task.Rejected != 0 {
			t.Fatalf("unexpected counts: %+v", task)
		}
	}
	if want := []int64{ids[2], ids[0], ids[1]}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order %v want %v", got, want)
	}
}

func TestListCompanies(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	companies, err := s.ListCompanies(ctx)
	if err != nil || companies == nil || len(companies) != 0 {
		t.Fatalf("empty companies: %v %v", companies, err)
	}
	tasks, err := s.ListBusinessTasks(ctx, "")
	if err != nil || tasks == nil || len(tasks) != 0 {
		t.Fatalf("empty tasks: %v %v", tasks, err)
	}
	if err := s.Seed(ctx, os.DirFS("../data")); err != nil {
		t.Fatal(err)
	}
	all, err := s.ListBusinessTasks(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	wantSet := map[string]bool{}
	for _, task := range all {
		if strings.TrimSpace(task.Company) != "" {
			wantSet[task.Company] = true
		}
	}
	for _, company := range []string{"", " \t\n", "Draft-only company", "Draft-only company"} {
		task := Task{DraftText: "Draft", Company: company}
		if err := s.CreateTask(ctx, &task); err != nil {
			t.Fatal(err)
		}
	}
	wantSet["Draft-only company"] = true
	want := []string{}
	for company := range wantSet {
		want = append(want, company)
	}
	sort.Strings(want)
	companies, err = s.ListCompanies(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(companies, want) {
		t.Fatalf("companies %v want %v", companies, want)
	}
}

func TestBusinessReadErrors(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task := Task{DraftText: "Draft"}
	if err := s.CreateTask(ctx, &task); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`UPDATE tasks SET qa='invalid json' WHERE id=?`, task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListBusinessTasks(ctx, ""); err == nil {
		t.Fatal("expected task decoding error")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListBusinessTasks(ctx, ""); err == nil {
		t.Fatal("expected closed database error for tasks")
	}
	if _, err := s.ListCompanies(ctx); err == nil {
		t.Fatal("expected closed database error for companies")
	}
}
