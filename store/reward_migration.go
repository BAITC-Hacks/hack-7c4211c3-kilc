package store

import (
	"database/sql"
	"fmt"
)

// Existing databases predate rewards. Add columns without replacing user data.
func migrateRewards(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(tasks)`)
	if err != nil {
		return err
	}
	columns := map[string]bool{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, kind string
		var def any
		if err := rows.Scan(&cid, &name, &kind, &notnull, &def, &pk); err != nil {
			rows.Close()
			return err
		}
		columns[name] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, column := range []struct{ name, definition string }{
		{"reward", "TEXT NOT NULL DEFAULT ''"},
		{"reward_type", "TEXT NOT NULL DEFAULT ''"},
		{"bonus", "INTEGER NOT NULL DEFAULT 0"},
	} {
		if !columns[column.name] {
			if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN ` + column.name + ` ` + column.definition); err != nil {
				return fmt.Errorf("миграция вознаграждений: %w", err)
			}
		}
	}
	return nil
}
