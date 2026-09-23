package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestCompanyMigrationKeepsSessions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TABLE browser_sessions(token TEXT PRIMARY KEY,role TEXT NOT NULL,team_id INTEGER,expires_at INTEGER NOT NULL);
 INSERT INTO browser_sessions VALUES('existing','business',NULL,9999999999)`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	for i := 0; i < 2; i++ {
		st, err := Open(path, loadSchema(t))
		if err != nil {
			t.Fatal(err)
		}
		var role, company string
		if err = st.DB.QueryRow(`SELECT role,company FROM browser_sessions WHERE token='existing'`).Scan(&role, &company); err != nil {
			t.Fatal(err)
		}
		if role != "business" || company != "" {
			t.Fatal("session changed")
		}
		st.Close()
	}
}
