package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/rating"
	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/store"
)

type businessFixture struct {
	st             *store.Store
	mux            *http.ServeMux
	taskID, teamID int64
}

func newBusinessFixture(t *testing.T) businessFixture {
	t.Helper()
	st := openStore(t)
	if err := st.Seed(context.Background(), os.DirFS("../data")); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	RegisterFrontend(mux, st)
	RegisterProposals(mux, st)
	RegisterBusiness(mux, st)
	tasks, err := st.ListCatalog(context.Background(), store.CatalogFilter{})
	if err != nil || len(tasks) == 0 {
		t.Fatalf("tasks: %v", err)
	}
	teams, err := st.ListTeams(context.Background())
	if err != nil || len(teams) == 0 {
		t.Fatalf("teams: %v", err)
	}
	return businessFixture{st, mux, tasks[0].ID, teams[0].ID}
}
func (f businessFixture) call(t *testing.T, method, path, body string, status int) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	f.mux.ServeHTTP(w, r)
	if w.Code != status {
		t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, status, w.Body.String())
	}
	return w
}
func (f businessFixture) pending(t *testing.T) store.Proposal {
	t.Helper()
	p, err := f.st.CreateProposal(context.Background(), store.ProposalInput{TaskID: f.taskID, TeamID: f.teamID,
		Idea: "Идея решения", Plan: "План работы", Deadline: "2026-12-31", PrototypeURL: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func proposalResponse(t *testing.T, w *httptest.ResponseRecorder) proposalJSON {
	t.Helper()
	var p proposalJSON
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}
func requireBusinessError(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != want {
		t.Fatalf("error = %q want %q", body["error"], want)
	}
}

func TestBusinessDecisionsAndStages(t *testing.T) {
	f := newBusinessFixture(t)
	ctx := context.Background()
	first, second, rejected, pending := f.pending(t), f.pending(t), f.pending(t), f.pending(t)
	decision := func(id int64) string { return fmt.Sprintf("/api/proposals/%d/decision", id) }
	stage := func(id int64) string { return fmt.Sprintf("/api/proposals/%d/stage", id) }
	for _, p := range []store.Proposal{first, second} {
		got := proposalResponse(t, f.call(t, "POST", decision(p.ID), `{"status":"accepted"}`, 200))
		if got.ID != p.ID || got.Status != "accepted" || got.DecidedAt == "" || got.TeamName == "" {
			t.Fatalf("accepted: %+v", got)
		}
	}
	items, err := f.st.ListProposalsByTask(ctx, f.taskID)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{first.ID, second.ID} {
		found := false
		for _, p := range items {
			if p.ID == id {
				found = p.Status == store.ProposalAccepted
			}
		}
		if !found {
			t.Fatalf("proposal %d not accepted", id)
		}
	}
	requireBusinessError(t, f.call(t, "POST", decision(first.ID), `{"status":"rejected"}`, 409), "Решение по этому предложению уже принято")
	got := proposalResponse(t, f.call(t, "POST", decision(rejected.ID), `{"status":"rejected"}`, 200))
	if got.Status != "rejected" {
		t.Fatalf("rejected: %+v", got)
	}
	requireBusinessError(t, f.call(t, "POST", decision(pending.ID), `{"status":"maybe"}`, 400), "Выберите: принять или отклонить")
	f.call(t, "POST", decision(999999), `{"status":"accepted"}`, 404)
	f.call(t, "POST", stage(999999), `{"points":10}`, 404)
	before, err := f.st.GetTeam(ctx, f.teamID)
	if err != nil {
		t.Fatal(err)
	}
	got = proposalResponse(t, f.call(t, "POST", stage(first.ID), `{"points":10}`, 200))
	if got.Status != "accepted" || got.PointsAwarded != 10 || got.StageConfirmedAt == "" {
		t.Fatalf("stage: %+v", got)
	}
	requireBusinessError(t, f.call(t, "POST", stage(first.ID), `{"points":10}`, 409), "Этап можно подтвердить только для принятого предложения, один раз")
	for _, p := range []store.Proposal{pending, rejected} {
		requireBusinessError(t, f.call(t, "POST", stage(p.ID), `{"points":10}`, 409), "Этап можно подтвердить только для принятого предложения, один раз")
	}
	for _, points := range []int{0, 101} {
		requireBusinessError(t, f.call(t, "POST", stage(second.ID), fmt.Sprintf(`{"points":%d}`, points), 400), "Баллы за этап: от 1 до 100")
	}
	var teams []teamView
	if err := json.Unmarshal(f.call(t, "GET", "/api/teams", "", 200).Body.Bytes(), &teams); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, team := range teams {
		if team.ID == f.teamID {
			found = true
			if team.Points != before.Points+10 {
				t.Fatalf("points %d want %d", team.Points, before.Points+10)
			}
		}
	}
	if !found {
		t.Fatal("team absent")
	}
	var full []fullProposalJSON
	if err := json.Unmarshal(f.call(t, "GET", fmt.Sprintf("/api/tasks/%d/proposals/full", f.taskID), "", 200).Body.Bytes(), &full); err != nil {
		t.Fatal(err)
	}
	for _, p := range full {
		if p.ID == first.ID {
			if p.Team.Skills != before.Skills || p.Team.Points != before.Points+10 || p.Team.Tech != before.Tech || !reflect.DeepEqual(p.Team.Interests, before.Interests) {
				t.Fatalf("team view: %+v", p.Team)
			}
			return
		}
	}
	t.Fatal("proposal absent from full list")
}

func TestBusinessInvalidBodiesDoNotMutate(t *testing.T) {
	f := newBusinessFixture(t)
	p := f.pending(t)
	for _, tc := range []struct{ suffix, body string }{
		{"decision", ""}, {"decision", `{}`}, {"decision", `null`}, {"decision", `{"status":"accepted","extra":1}`},
		{"decision", `[{"status":"accepted"}]`}, {"decision", `{"status":"accepted"} {}`},
		{"stage", ""}, {"stage", `{}`}, {"stage", `{"points":10,"status":"accepted"}`},
		{"stage", `{"points":1.5}`}, {"stage", `{"points":"10"}`},
	} {
		f.call(t, "POST", fmt.Sprintf("/api/proposals/%d/%s", p.ID, tc.suffix), tc.body, 400)
	}
	got, err := readBusinessProposal(context.Background(), f.st, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, p) {
		t.Fatalf("invalid bodies changed proposal: %+v", got)
	}
}

func TestBusinessTasksAndCompanies(t *testing.T) {
	f := newBusinessFixture(t)
	var tasks []businessTaskJSON
	if err := json.Unmarshal(f.call(t, "GET", "/api/business/tasks", "", 200).Body.Bytes(), &tasks); err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 10 {
		t.Fatalf("tasks: %d", len(tasks))
	}
	var total, pending, accepted, rejected int
	for _, task := range tasks {
		total += task.Proposals
		pending += task.Pending
		accepted += task.Accepted
		rejected += task.Rejected
		if task.Level != rating.LevelFor(task.Score) || task.LevelLabel != rating.LevelLabel(task.Level) {
			t.Fatalf("level: %+v", task)
		}
	}
	if total != 5 || pending != 3 || accepted != 1 || rejected != 1 {
		t.Fatalf("counts %d/%d/%d/%d", total, pending, accepted, rejected)
	}
	company := tasks[0].Company
	var filtered []businessTaskJSON
	if err := json.Unmarshal(f.call(t, "GET", "/api/business/tasks?company="+url.QueryEscape(" "+company+" "), "", 200).Body.Bytes(), &filtered); err != nil {
		t.Fatal(err)
	}
	want := []businessTaskJSON{}
	for _, task := range tasks {
		if task.Company == company {
			want = append(want, task)
		}
	}
	if !reflect.DeepEqual(filtered, want) {
		t.Fatalf("filtered: %+v", filtered)
	}
	if got := strings.TrimSpace(f.call(t, "GET", "/api/business/tasks?company=unknown", "", 200).Body.String()); got != "[]" {
		t.Fatalf("unknown company: %s", got)
	}
	var companies []string
	if err := json.Unmarshal(f.call(t, "GET", "/api/business/companies", "", 200).Body.Bytes(), &companies); err != nil {
		t.Fatal(err)
	}
	if len(companies) == 0 || !sort.StringsAreSorted(companies) {
		t.Fatalf("companies: %v", companies)
	}
	seen := map[string]bool{}
	for _, company := range companies {
		if company == "" || seen[company] {
			t.Fatalf("invalid company %q", company)
		}
		seen[company] = true
	}
	f.call(t, "GET", "/api/tasks/999999/proposals/full", "", 404)
}

func TestBusinessGETsNeverMutateProposals(t *testing.T) {
	f := newBusinessFixture(t)
	ctx := context.Background()
	tasks, err := f.st.ListBusinessTasks(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := func() []store.Proposal {
		result := []store.Proposal{}
		for _, task := range tasks {
			p, err := f.st.ListProposalsByTask(ctx, task.ID)
			if err != nil {
				t.Fatal(err)
			}
			result = append(result, p...)
		}
		return result
	}
	before := snapshot()
	for _, path := range []string{"/api/business/companies", "/api/business/tasks", "/api/categories", "/api/teams", "/api/tasks"} {
		f.call(t, "GET", path, "", 200)
	}
	for _, task := range tasks {
		for _, suffix := range []string{"", "/proposals", "/proposals/full"} {
			f.call(t, "GET", fmt.Sprintf("/api/tasks/%d%s", task.ID, suffix), "", 200)
		}
		f.call(t, "GET", "/api/business/tasks?company="+url.QueryEscape(task.Company), "", 200)
	}
	for _, p := range before {
		f.call(t, "GET", fmt.Sprintf("/api/proposals/%d/decision?status=accepted", p.ID), "", 405)
		f.call(t, "GET", fmt.Sprintf("/api/proposals/%d/stage?points=10", p.ID), "", 405)
	}
	if after := snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("GET changed proposals, decisions or awarded points")
	}
}

func TestBusinessStoreFailure(t *testing.T) {
	f := newBusinessFixture(t)
	if err := f.st.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/business/tasks", "/api/business/companies", "/api/tasks/6/proposals/full"} {
		f.call(t, "GET", path, "", 500)
	}
	f.call(t, "POST", "/api/proposals/1/decision", `{"status":"accepted"}`, 500)
	f.call(t, "POST", "/api/proposals/1/stage", `{"points":10}`, 500)
}
