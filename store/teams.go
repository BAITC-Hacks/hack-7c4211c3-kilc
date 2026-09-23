package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type Team struct {
	ID        int64
	Name      string
	Interests []string
	Skills    string
	Tech      string
	Points    int
}

const teamSelect = `SELECT t.id, t.name, t.interests, t.skills, t.tech,
	COALESCE(SUM(p.points_awarded), 0)
	FROM teams t LEFT JOIN proposals p ON p.team_id = t.id`

func (s *Store) CreateTeam(ctx context.Context, t *Team) error {
	if strings.TrimSpace(t.Name) == "" {
		return &ValidationError{Field: "name", Message: "название команды не может быть пустым"}
	}
	if t.Interests == nil {
		t.Interests = []string{}
	}
	for _, code := range t.Interests {
		if code == "" || !ValidCategory(code) {
			return &ValidationError{Field: "interests", Message: "неизвестная категория «" + code + "»"}
		}
	}
	interests, err := json.Marshal(t.Interests)
	if err != nil {
		return fmt.Errorf("кодировать interests: %w", err)
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO teams (name, interests, skills, tech) VALUES (?, ?, ?, ?)`,
		t.Name, string(interests), t.Skills, t.Tech)
	if err != nil {
		return fmt.Errorf("создать команду: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("получить id команды: %w", err)
	}
	t.ID = id
	t.Points = 0
	return nil
}

func scanTeam(row rowScanner) (Team, error) {
	var t Team
	var interests string
	if err := row.Scan(&t.ID, &t.Name, &interests, &t.Skills, &t.Tech, &t.Points); err != nil {
		return Team{}, err
	}
	if err := json.Unmarshal([]byte(interests), &t.Interests); err != nil {
		return Team{}, fmt.Errorf("разобрать interests команды %d: %w", t.ID, err)
	}
	return t, nil
}

func (s *Store) GetTeam(ctx context.Context, id int64) (Team, error) {
	row := s.DB.QueryRowContext(ctx, teamSelect+` WHERE t.id = ? GROUP BY t.id`, id)
	t, err := scanTeam(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Team{}, ErrNotFound
	}
	if err != nil {
		return Team{}, fmt.Errorf("прочитать команду %d: %w", id, err)
	}
	return t, nil
}

func (s *Store) ListTeams(ctx context.Context) ([]Team, error) {
	rows, err := s.DB.QueryContext(ctx, teamSelect+` GROUP BY t.id ORDER BY t.name, t.id`)
	if err != nil {
		return nil, fmt.Errorf("прочитать команды: %w", err)
	}
	defer rows.Close()
	teams := []Team{}
	for rows.Next() {
		t, err := scanTeam(rows)
		if err != nil {
			return nil, fmt.Errorf("прочитать команды: %w", err)
		}
		teams = append(teams, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("прочитать команды: %w", err)
	}
	return teams, nil
}
