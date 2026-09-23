package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Store struct {
	DB *sql.DB
}

func Open(path, schema string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("открыть базу %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("применить схему: %w", err)
	}
	for _, column := range []struct{ name, definition string }{
		{"reward_type", `TEXT NOT NULL DEFAULT ''`},
		{"reward", `TEXT NOT NULL DEFAULT ''`},
	} {
		rows, err := db.Query(`PRAGMA table_info(tasks)`)
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("проверить столбцы задач: %w", err)
		}
		found := false
		for rows.Next() {
			var cid, notnull, pk int
			var name, typ string
			var defaultValue any
			if err := rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
				rows.Close()
				db.Close()
				return nil, fmt.Errorf("прочитать столбцы задач: %w", err)
			}
			if name == column.name {
				found = true
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			db.Close()
			return nil, fmt.Errorf("прочитать столбцы задач: %w", err)
		}
		rows.Close()
		if !found {
			if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN ` + column.name + ` ` + column.definition); err != nil {
				db.Close()
				return nil, fmt.Errorf("добавить столбец %s: %w", column.name, err)
			}
		}
	}
	return &Store{DB: db}, nil
}

func (s *Store) Close() error {
	return s.DB.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.DB.PingContext(ctx)
}
