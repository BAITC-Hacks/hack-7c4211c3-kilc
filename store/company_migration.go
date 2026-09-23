package store

import "database/sql"

func migrateSessionCompany(db *sql.DB) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('browser_sessions') WHERE name='company'`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		_, err := db.Exec(`ALTER TABLE browser_sessions ADD COLUMN company TEXT NOT NULL DEFAULT ''`)
		return err
	}
	return nil
}
