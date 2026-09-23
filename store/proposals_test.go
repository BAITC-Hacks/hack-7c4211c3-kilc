package store

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

type proposalFixture struct {
	store     *Store
	published Task
	draft     Task
	team      Team
}

func newProposalFixture(t *testing.T) proposalFixture {
	t.Helper()
	s := openTestStore(t)
	ctx := context.Background()
	published := Task{DraftText: "Нужна CRM", Title: "CRM для продаж", Confirmed: []string{FieldTitle}}
	if err := s.CreateTask(ctx, &published); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := s.PublishTask(ctx, published.ID); err != nil {
		t.Fatalf("publish task: %v", err)
	}
	draft := Task{DraftText: "Нужен бот", Title: "Бот"}
	if err := s.CreateTask(ctx, &draft); err != nil {
		t.Fatalf("create draft: %v", err)
	}
	team := Team{Name: "Альфа", Interests: []string{"crm", "analytics"}, Skills: "Go, SQL", Tech: "Go"}
	if err := s.CreateTeam(ctx, &team); err != nil {
		t.Fatalf("create team: %v", err)
	}
	return proposalFixture{store: s, published: published, draft: draft, team: team}
}

func (f proposalFixture) input() ProposalInput {
	return ProposalInput{
		TaskID:       f.published.ID,
		TeamID:       f.team.ID,
		Idea:         "Собрать CRM на готовом шаблоне",
		Plan:         "Неделя на прототип, неделя на пилот",
		Deadline:     "2026-12-31",
		PrototypeURL: "https://example.com/demo",
	}
}

func (f proposalFixture) create(t *testing.T) Proposal {
	t.Helper()
	p, err := f.store.CreateProposal(context.Background(), f.input())
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	return p
}

func TestCreateTeamAndList(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	var ve *ValidationError
	if err := s.CreateTeam(ctx, &Team{Name: " "}); !errors.As(err, &ve) || ve.Field != "name" {
		t.Errorf("empty name: got %v, want ValidationError on name", err)
	}
	for _, interests := range [][]string{{""}, {"xyz"}} {
		if err := s.CreateTeam(ctx, &Team{Name: "X", Interests: interests}); !errors.As(err, &ve) || ve.Field != "interests" {
			t.Errorf("interests %q: got %v, want ValidationError on interests", interests, err)
		}
	}
	for _, name := range []string{"Бета", "Альфа"} {
		if err := s.CreateTeam(ctx, &Team{Name: name}); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	teams, err := s.ListTeams(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(teams) != 2 || teams[0].Name != "Альфа" || teams[1].Name != "Бета" {
		t.Fatalf("teams = %+v, want Альфа, Бета", teams)
	}
	if teams[0].Interests == nil || teams[0].Points != 0 {
		t.Errorf("team = %+v, want empty interests and 0 points", teams[0])
	}
	if _, err := s.GetTeam(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("get missing: got %v, want ErrNotFound", err)
	}
}

func TestCreateProposalValidation(t *testing.T) {
	f := newProposalFixture(t)
	cases := []struct {
		name   string
		mutate func(*ProposalInput)
		field  string
	}{
		{"draft task", func(in *ProposalInput) { in.TaskID = f.draft.ID }, "task_id"},
		{"missing task", func(in *ProposalInput) { in.TaskID = 999 }, "task_id"},
		{"missing team", func(in *ProposalInput) { in.TeamID = 999 }, "team_id"},
		{"empty idea", func(in *ProposalInput) { in.Idea = "  " }, "idea"},
		{"empty plan", func(in *ProposalInput) { in.Plan = "" }, "plan"},
		{"dotted deadline", func(in *ProposalInput) { in.Deadline = "31.12.2026" }, "deadline"},
		{"ftp url", func(in *ProposalInput) { in.PrototypeURL = "ftp://x" }, "prototype_url"},
		{"no scheme", func(in *ProposalInput) { in.PrototypeURL = "example.com" }, "prototype_url"},
		{"no host", func(in *ProposalInput) { in.PrototypeURL = "https://" }, "prototype_url"},
	}
	for _, c := range cases {
		in := f.input()
		c.mutate(&in)
		_, err := f.store.CreateProposal(context.Background(), in)
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != c.field {
			t.Errorf("%s: got %v, want ValidationError on %q", c.name, err, c.field)
		}
	}
}

func TestCreateProposalPendingAndList(t *testing.T) {
	f := newProposalFixture(t)
	first := f.create(t)
	second := f.create(t)
	if first.Status != ProposalPending || first.ID <= 0 || first.TeamName != "Альфа" || first.CreatedAt.IsZero() {
		t.Fatalf("proposal = %+v, want pending with team name", first)
	}
	list, err := f.store.ListProposalsByTask(context.Background(), f.published.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 || list[0].ID != first.ID || list[1].ID != second.ID {
		t.Fatalf("list = %+v, want both proposals in creation order", list)
	}
}

func TestDecideProposal(t *testing.T) {
	f := newProposalFixture(t)
	ctx := context.Background()
	a, b, c := f.create(t), f.create(t), f.create(t)

	for _, p := range []Proposal{a, b} {
		if err := f.store.DecideProposal(ctx, p.ID, ProposalAccepted); err != nil {
			t.Fatalf("accept %d: %v", p.ID, err)
		}
	}
	if err := f.store.DecideProposal(ctx, c.ID, ProposalRejected); err != nil {
		t.Fatalf("reject: %v", err)
	}
	list, err := f.store.ListProposalsByTask(ctx, f.published.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []string{ProposalAccepted, ProposalAccepted, ProposalRejected}
	for i, p := range list {
		if p.Status != want[i] || p.DecidedAt.IsZero() {
			t.Errorf("proposal %d: status=%q decided=%v, want %q with decided_at", p.ID, p.Status, p.DecidedAt, want[i])
		}
	}

	if err := f.store.DecideProposal(ctx, c.ID, ProposalAccepted); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("decide rejected: got %v, want ErrInvalidTransition", err)
	}
	if err := f.store.DecideProposal(ctx, 999, ProposalAccepted); !errors.Is(err, ErrNotFound) {
		t.Errorf("decide missing: got %v, want ErrNotFound", err)
	}
	var ve *ValidationError
	if err := f.store.DecideProposal(ctx, f.create(t).ID, ProposalPending); !errors.As(err, &ve) || ve.Field != "status" {
		t.Errorf("decide pending: got %v, want ValidationError on status", err)
	}
}

func TestConfirmStage(t *testing.T) {
	f := newProposalFixture(t)
	ctx := context.Background()
	p := f.create(t)

	if err := f.store.ConfirmStage(ctx, p.ID, 10); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("confirm pending: got %v, want ErrInvalidTransition", err)
	}
	if err := f.store.DecideProposal(ctx, p.ID, ProposalAccepted); err != nil {
		t.Fatalf("accept: %v", err)
	}
	var ve *ValidationError
	for _, points := range []int{0, 101} {
		if err := f.store.ConfirmStage(ctx, p.ID, points); !errors.As(err, &ve) || ve.Field != "points" {
			t.Errorf("points %d: got %v, want ValidationError on points", points, err)
		}
	}
	if err := f.store.ConfirmStage(ctx, p.ID, 10); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	team, err := f.store.GetTeam(ctx, f.team.ID)
	if err != nil {
		t.Fatalf("get team: %v", err)
	}
	if team.Points != 10 {
		t.Errorf("team points = %d, want 10", team.Points)
	}
	if err := f.store.ConfirmStage(ctx, p.ID, 10); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("confirm twice: got %v, want ErrInvalidTransition", err)
	}
	if err := f.store.ConfirmStage(ctx, 999, 10); !errors.Is(err, ErrNotFound) {
		t.Errorf("confirm missing: got %v, want ErrNotFound", err)
	}
	list, err := f.store.ListProposalsByTask(ctx, f.published.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if list[0].PointsAwarded != 10 || list[0].StageConfirmedAt.IsZero() {
		t.Errorf("proposal = %+v, want 10 points and stage_confirmed_at", list[0])
	}
}

// TestOnlyDecideProposalSetsStatus parses the store sources and fails if any
// SQL other than DecideProposal's writes proposals.status, or if an INSERT
// into proposals uses a status other than the literal 'pending'.
func TestOnlyDecideProposalSetsStatus(t *testing.T) {
	updateSet := regexp.MustCompile(`(?is)UPDATE\s+proposals\s+SET\s+(.*?)(\bWHERE\b|$)`)
	statusAssign := regexp.MustCompile(`(?i)\bstatus\s*=`)
	insert := regexp.MustCompile(`(?i)INSERT\s+INTO\s+proposals`)

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			owner := ""
			if fn, ok := decl.(*ast.FuncDecl); ok {
				owner = fn.Name.Name
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				sqlText, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("unquote %s: %v", fset.Position(lit.Pos()), err)
				}
				if m := updateSet.FindStringSubmatch(sqlText); m != nil {
					checked++
					if statusAssign.MatchString(m[1]) && owner != "DecideProposal" {
						t.Errorf("%s: %s sets proposals.status; only DecideProposal may", fset.Position(lit.Pos()), owner)
					}
				}
				if insert.MatchString(sqlText) {
					checked++
					if owner != "CreateProposal" || !strings.Contains(sqlText, "'pending'") {
						t.Errorf("%s: %s inserts proposals without literal 'pending' status", fset.Position(lit.Pos()), owner)
					}
				}
				return true
			})
		}
	}
	if checked == 0 {
		t.Fatal("guard found no proposal SQL to check")
	}
}
