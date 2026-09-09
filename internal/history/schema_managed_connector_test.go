package history

import (
	"database/sql"
	"testing"

	"modernc.org/sqlite"
)

func TestSchemaDDLRecognizesLeadingComments(t *testing.T) {
	tests := []struct {
		name      string
		statement string
		want      bool
	}{
		{name: "create", statement: `CREATE TABLE x (id INTEGER)`, want: true},
		{name: "line comment", statement: "-- generated defensively\nCREATE INDEX idx_x ON x(id)", want: true},
		{name: "block comment", statement: `/* generated defensively */ ALTER TABLE x ADD COLUMN name TEXT`, want: true},
		{name: "stacked comments", statement: "/* one */\n-- two\nDROP TABLE x", want: true},
		{name: "reindex", statement: `REINDEX idx_x`, want: true},
		{name: "select", statement: `SELECT 'CREATE TABLE x'`, want: false},
		{name: "insert", statement: `INSERT INTO x(id) VALUES(1)`, want: false},
		{name: "comment only", statement: `-- nothing follows`, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := schemaDDL(tt.statement); got != tt.want {
				t.Fatalf("schemaDDL(%q) = %v, want %v", tt.statement, got, tt.want)
			}
		})
	}
}

func TestSchemaManagedConnectorDoesNotExecuteCommentedDDL(t *testing.T) {
	connector, err := sqlite.NewConnector(t.TempDir() + "/managed.db")
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(schemaManagedConnector{inner: connector})
	defer db.Close()

	if _, err := db.Exec("-- legacy defensive schema\nCREATE TABLE runtime_must_not_create (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='runtime_must_not_create'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("comment-prefixed runtime DDL escaped the Atlas schema boundary")
	}
}
