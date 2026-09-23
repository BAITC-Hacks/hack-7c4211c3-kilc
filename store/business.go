package store

import (
	"context"
	"fmt"
	"strings"
)

// TaskSummary includes all task fields and proposal counts for the business view.
type TaskSummary struct {
	Task
	Proposals, Pending, Accepted, Rejected int
}

// summaryScanner lets scanTask decode its existing columns, followed by counts.
type summaryScanner struct {
	row     rowScanner
	summary *TaskSummary
}

func (s summaryScanner) Scan(dest ...any) error {
	return s.row.Scan(append(dest, &s.summary.Proposals, &s.summary.Pending,
		&s.summary.Accepted, &s.summary.Rejected)...)
}

// ListBusinessTasks returns published tasks first, then descending score and ID.
// Only the company filter is trimmed; stored company names are matched exactly.
func (s *Store) ListBusinessTasks(ctx context.Context, company string) ([]TaskSummary, error) {
	columns := strings.Split(taskColumns, ",")
	for i, column := range columns {
		columns[i] = "t." + strings.TrimSpace(column)
	}
	query := `SELECT ` + strings.Join(columns, ", ") + `,
		COUNT(p.id),
		SUM(CASE WHEN p.status = 'pending' THEN 1 ELSE 0 END),
		SUM(CASE WHEN p.status = 'accepted' THEN 1 ELSE 0 END),
		SUM(CASE WHEN p.status = 'rejected' THEN 1 ELSE 0 END)
		FROM tasks t LEFT JOIN proposals p ON p.task_id = t.id`
	var args []any
	if company = strings.TrimSpace(company); company != "" {
		query += ` WHERE t.company = ?`
		args = append(args, company)
	}
	query += ` GROUP BY t.id
		ORDER BY CASE WHEN t.status = 'published' THEN 0 ELSE 1 END, t.score DESC, t.id DESC`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("прочитать задачи бизнеса: %w", err)
	}
	defer rows.Close()
	result := []TaskSummary{}
	for rows.Next() {
		var summary TaskSummary
		task, err := scanTask(summaryScanner{row: rows, summary: &summary})
		if err != nil {
			return nil, fmt.Errorf("прочитать задачу бизнеса: %w", err)
		}
		summary.Task = task
		result = append(result, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("прочитать задачи бизнеса: %w", err)
	}
	return result, nil
}

// ListCompanies includes companies from both draft and published tasks.
func (s *Store) ListCompanies(ctx context.Context) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT DISTINCT company FROM tasks WHERE company <> '' ORDER BY company`)
	if err != nil {
		return nil, fmt.Errorf("прочитать компании: %w", err)
	}
	defer rows.Close()
	companies := []string{}
	for rows.Next() {
		var company string
		if err := rows.Scan(&company); err != nil {
			return nil, fmt.Errorf("прочитать компанию: %w", err)
		}
		if strings.TrimSpace(company) != "" {
			companies = append(companies, company)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("прочитать компании: %w", err)
	}
	return companies, nil
}
