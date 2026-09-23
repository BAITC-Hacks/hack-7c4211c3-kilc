package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
)

// ErrSeedSkipped is returned by Seed when the database already has tasks.
var ErrSeedSkipped = errors.New("seed skipped: database not empty")

type seedTask struct {
	Key               string   `json:"key"`
	Company           string   `json:"company"`
	Title             string   `json:"title"`
	Industry          string   `json:"industry"`
	Category          string   `json:"category"`
	DraftText         string   `json:"draft_text"`
	QA                []QA     `json:"qa"`
	Context           string   `json:"context"`
	Need              string   `json:"need"`
	Users             string   `json:"users"`
	Data              string   `json:"data"`
	Constraints       string   `json:"constraints"`
	ExpectedResult    string   `json:"expected_result"`
	SuccessCriteria   string   `json:"success_criteria"`
	Contact           string   `json:"contact"`
	InteractionFormat string   `json:"interaction_format"`
	Confirmed         []string `json:"confirmed"`
	Publish           bool     `json:"publish"`
}

type seedTeam struct {
	Key       string   `json:"key"`
	Name      string   `json:"name"`
	Interests []string `json:"interests"`
	Skills    string   `json:"skills"`
	Tech      string   `json:"tech"`
}

type seedProposal struct {
	TaskKey      string `json:"task_key"`
	TeamKey      string `json:"team_key"`
	Idea         string `json:"idea"`
	Plan         string `json:"plan"`
	Deadline     string `json:"deadline"`
	PrototypeURL string `json:"prototype_url"`
	Decision     string `json:"decision"`
	StagePoints  *int   `json:"stage_points"`
}

func decodeFixture(fsys fs.FS, name string, v any) error {
	f, err := fsys.Open(name)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("%s: лишние данные после JSON-массива", name)
	}
	return nil
}

// Seed loads tasks.json, teams.json and proposals.json from fsys into an
// empty database through the regular store methods, so fixtures pass the
// same validation and scoring as user input. All inserts share one
// transaction: the store holds a single connection (SetMaxOpenConns(1)),
// so BEGIN/COMMIT on s.DB wrap every call made in between.
func (s *Store) Seed(ctx context.Context, fsys fs.FS) error {
	if n := s.DB.Stats().MaxOpenConnections; n != 1 {
		return fmt.Errorf("seed: нужна ровно одна открытая связь с базой, сейчас %d", n)
	}
	if _, err := s.DB.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("seed: начать транзакцию: %w", err)
	}
	if err := s.seed(ctx, fsys); err != nil {
		if _, rbErr := s.DB.ExecContext(context.WithoutCancel(ctx), `ROLLBACK`); rbErr != nil {
			return errors.Join(err, fmt.Errorf("seed: откатить транзакцию: %w", rbErr))
		}
		return err
	}
	if _, err := s.DB.ExecContext(ctx, `COMMIT`); err != nil {
		return fmt.Errorf("seed: завершить транзакцию: %w", err)
	}
	return nil
}

func (s *Store) seed(ctx context.Context, fsys fs.FS) error {
	var count int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks`).Scan(&count); err != nil {
		return fmt.Errorf("seed: посчитать задачи: %w", err)
	}
	if count > 0 {
		return ErrSeedSkipped
	}

	var tasks []seedTask
	var teams []seedTeam
	var proposals []seedProposal
	if err := decodeFixture(fsys, "tasks.json", &tasks); err != nil {
		return err
	}
	if err := decodeFixture(fsys, "teams.json", &teams); err != nil {
		return err
	}
	if err := decodeFixture(fsys, "proposals.json", &proposals); err != nil {
		return err
	}

	taskIDs := make(map[string]int64, len(tasks))
	for i, st := range tasks {
		where := fmt.Sprintf("tasks.json[%d] %q", i, st.Key)
		if st.Key == "" {
			return fmt.Errorf("%s: пустой key", where)
		}
		if _, dup := taskIDs[st.Key]; dup {
			return fmt.Errorf("%s: key повторяется", where)
		}
		t := Task{
			Company: st.Company, Title: st.Title, Industry: st.Industry, Category: st.Category,
			DraftText: st.DraftText, QA: st.QA,
			Context: st.Context, Need: st.Need, Users: st.Users, Data: st.Data,
			Constraints: st.Constraints, ExpectedResult: st.ExpectedResult,
			SuccessCriteria: st.SuccessCriteria, Contact: st.Contact,
			InteractionFormat: st.InteractionFormat, Confirmed: st.Confirmed,
		}
		if err := s.CreateTask(ctx, &t); err != nil {
			return fmt.Errorf("%s: %w", where, err)
		}
		if st.Publish {
			if err := s.PublishTask(ctx, t.ID); err != nil {
				return fmt.Errorf("%s: %w", where, err)
			}
		}
		taskIDs[st.Key] = t.ID
	}

	teamIDs := make(map[string]int64, len(teams))
	for i, st := range teams {
		where := fmt.Sprintf("teams.json[%d] %q", i, st.Key)
		if st.Key == "" {
			return fmt.Errorf("%s: пустой key", where)
		}
		if _, dup := teamIDs[st.Key]; dup {
			return fmt.Errorf("%s: key повторяется", where)
		}
		t := Team{Name: st.Name, Interests: st.Interests, Skills: st.Skills, Tech: st.Tech}
		if err := s.CreateTeam(ctx, &t); err != nil {
			return fmt.Errorf("%s: %w", where, err)
		}
		teamIDs[st.Key] = t.ID
	}

	for i, sp := range proposals {
		where := fmt.Sprintf("proposals.json[%d]", i)
		taskID, ok := taskIDs[sp.TaskKey]
		if !ok {
			return fmt.Errorf("%s: неизвестный task_key %q", where, sp.TaskKey)
		}
		teamID, ok := teamIDs[sp.TeamKey]
		if !ok {
			return fmt.Errorf("%s: неизвестный team_key %q", where, sp.TeamKey)
		}
		p, err := s.CreateProposal(ctx, ProposalInput{
			TaskID: taskID, TeamID: teamID, Idea: sp.Idea, Plan: sp.Plan,
			Deadline: sp.Deadline, PrototypeURL: sp.PrototypeURL,
		})
		if err != nil {
			return fmt.Errorf("%s: %w", where, err)
		}
		if sp.Decision != "" {
			if err := s.DecideProposal(ctx, p.ID, sp.Decision); err != nil {
				return fmt.Errorf("%s: decision: %w", where, err)
			}
		}
		if sp.StagePoints != nil {
			if err := s.ConfirmStage(ctx, p.ID, *sp.StagePoints); err != nil {
				return fmt.Errorf("%s: stage_points: %w", where, err)
			}
		}
	}
	return nil
}
