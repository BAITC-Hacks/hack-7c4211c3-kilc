package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	ProposalPending  = "pending"
	ProposalAccepted = "accepted"
	ProposalRejected = "rejected"
)

type ProposalInput struct {
	TaskID       int64
	TeamID       int64
	Idea         string
	Plan         string
	Deadline     string
	PrototypeURL string
}

type Proposal struct {
	ID               int64
	TaskID           int64
	TeamID           int64
	TeamName         string
	Idea             string
	Plan             string
	Deadline         string
	PrototypeURL     string
	Status           string
	PointsAwarded    int
	CreatedAt        time.Time
	DecidedAt        time.Time
	StageConfirmedAt time.Time
}

func (s *Store) validateProposal(ctx context.Context, in *ProposalInput) error {
	var taskStatus string
	err := s.DB.QueryRowContext(ctx, `SELECT status FROM tasks WHERE id = ?`, in.TaskID).Scan(&taskStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return &ValidationError{Field: "task_id", Message: "задача не найдена"}
	}
	if err != nil {
		return fmt.Errorf("проверить задачу %d: %w", in.TaskID, err)
	}
	if taskStatus != StatusPublished {
		return &ValidationError{Field: "task_id", Message: "задача ещё не опубликована"}
	}
	if _, err := s.GetTeam(ctx, in.TeamID); errors.Is(err, ErrNotFound) {
		return &ValidationError{Field: "team_id", Message: "команда не найдена"}
	} else if err != nil {
		return err
	}
	in.Idea = strings.TrimSpace(in.Idea)
	if in.Idea == "" {
		return &ValidationError{Field: "idea", Message: "опишите идею решения"}
	}
	in.Plan = strings.TrimSpace(in.Plan)
	if in.Plan == "" {
		return &ValidationError{Field: "plan", Message: "опишите план работ"}
	}
	in.Deadline = strings.TrimSpace(in.Deadline)
	if _, err := time.Parse("2006-01-02", in.Deadline); err != nil {
		return &ValidationError{Field: "deadline", Message: "укажите срок в формате ГГГГ-ММ-ДД"}
	}
	in.PrototypeURL = strings.TrimSpace(in.PrototypeURL)
	u, err := url.Parse(in.PrototypeURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return &ValidationError{Field: "prototype_url", Message: "ссылка на прототип должна начинаться с http:// или https://"}
	}
	return nil
}

func (s *Store) CreateProposal(ctx context.Context, in ProposalInput) (Proposal, error) {
	if err := s.validateProposal(ctx, &in); err != nil {
		return Proposal{}, err
	}
	createdAt := time.Now().UTC().Truncate(time.Second)
	res, err := s.DB.ExecContext(ctx, `INSERT INTO proposals
		(task_id, team_id, idea, plan, deadline, prototype_url, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 'pending', ?)`,
		in.TaskID, in.TeamID, in.Idea, in.Plan, in.Deadline, in.PrototypeURL, formatTime(createdAt))
	if err != nil {
		return Proposal{}, fmt.Errorf("создать предложение: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Proposal{}, fmt.Errorf("получить id предложения: %w", err)
	}
	return s.getProposal(ctx, id)
}

const proposalSelect = `SELECT p.id, p.task_id, p.team_id, tm.name, p.idea, p.plan,
	p.deadline, p.prototype_url, p.status, p.points_awarded,
	p.created_at, p.decided_at, p.stage_confirmed_at
	FROM proposals p JOIN teams tm ON tm.id = p.team_id`

func parseNullTime(v sql.NullString) (time.Time, error) {
	if !v.Valid {
		return time.Time{}, nil
	}
	return parseTime(v.String)
}

func scanProposal(row rowScanner) (Proposal, error) {
	var p Proposal
	var createdAt string
	var decidedAt, stageConfirmedAt sql.NullString
	err := row.Scan(&p.ID, &p.TaskID, &p.TeamID, &p.TeamName, &p.Idea, &p.Plan,
		&p.Deadline, &p.PrototypeURL, &p.Status, &p.PointsAwarded,
		&createdAt, &decidedAt, &stageConfirmedAt)
	if err != nil {
		return Proposal{}, err
	}
	if p.CreatedAt, err = parseTime(createdAt); err != nil {
		return Proposal{}, err
	}
	if p.DecidedAt, err = parseNullTime(decidedAt); err != nil {
		return Proposal{}, err
	}
	if p.StageConfirmedAt, err = parseNullTime(stageConfirmedAt); err != nil {
		return Proposal{}, err
	}
	return p, nil
}

func (s *Store) getProposal(ctx context.Context, id int64) (Proposal, error) {
	p, err := scanProposal(s.DB.QueryRowContext(ctx, proposalSelect+` WHERE p.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Proposal{}, ErrNotFound
	}
	if err != nil {
		return Proposal{}, fmt.Errorf("прочитать предложение %d: %w", id, err)
	}
	return p, nil
}

func (s *Store) ListProposalsByTask(ctx context.Context, taskID int64) ([]Proposal, error) {
	rows, err := s.DB.QueryContext(ctx, proposalSelect+` WHERE p.task_id = ? ORDER BY p.created_at, p.id`, taskID)
	if err != nil {
		return nil, fmt.Errorf("прочитать предложения: %w", err)
	}
	defer rows.Close()
	proposals := []Proposal{}
	for rows.Next() {
		p, err := scanProposal(rows)
		if err != nil {
			return nil, fmt.Errorf("прочитать предложения: %w", err)
		}
		proposals = append(proposals, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("прочитать предложения: %w", err)
	}
	return proposals, nil
}

// transitionResult maps a conditional UPDATE that touched no rows to
// ErrNotFound or ErrInvalidTransition.
func (s *Store) transitionResult(ctx context.Context, res sql.Result, id int64) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("проверить обновление предложения %d: %w", id, err)
	}
	if n == 1 {
		return nil
	}
	var exists int
	err = s.DB.QueryRowContext(ctx, `SELECT 1 FROM proposals WHERE id = ?`, id).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("прочитать предложение %d: %w", id, err)
	}
	return ErrInvalidTransition
}

// DecideProposal is the only code path that moves a proposal out of
// pending. It must be called from an explicit business action.
func (s *Store) DecideProposal(ctx context.Context, id int64, status string) error {
	if status != ProposalAccepted && status != ProposalRejected {
		return &ValidationError{Field: "status", Message: "решение должно быть «accepted» или «rejected»"}
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE proposals SET status = ?, decided_at = ?
		WHERE id = ? AND status = 'pending'`, status, formatTime(time.Now()), id)
	if err != nil {
		return fmt.Errorf("принять решение по предложению %d: %w", id, err)
	}
	return s.transitionResult(ctx, res, id)
}

func (s *Store) ConfirmStage(ctx context.Context, id int64, points int) error {
	if points < 1 || points > 100 {
		return &ValidationError{Field: "points", Message: "баллы должны быть от 1 до 100"}
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE proposals SET stage_confirmed_at = ?, points_awarded = ?
		WHERE id = ? AND status = 'accepted' AND stage_confirmed_at IS NULL`,
		formatTime(time.Now()), points, id)
	if err != nil {
		return fmt.Errorf("подтвердить этап предложения %d: %w", id, err)
	}
	return s.transitionResult(ctx, res, id)
}
