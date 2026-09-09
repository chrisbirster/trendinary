package sqlscript

import (
	"reflect"
	"testing"
)

func TestSplitPreservesQuotedSemicolonsAndComments(t *testing.T) {
	script := `
-- comment ; does not split
CREATE TABLE example (value TEXT DEFAULT ';');
/* block ; comment */
INSERT INTO example(value) VALUES ('it''s;fine');
CREATE INDEX "semi;colon" ON example(value);
`
	got := Split(script)
	want := []string{
		"-- comment ; does not split\nCREATE TABLE example (value TEXT DEFAULT ';')",
		"/* block ; comment */\nINSERT INTO example(value) VALUES ('it''s;fine')",
		"CREATE INDEX \"semi;colon\" ON example(value)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Split() = %#v, want %#v", got, want)
	}
}

func TestSplitIgnoresEmptyStatements(t *testing.T) {
	got := Split(" ; CREATE TABLE x (id INTEGER); ; ")
	want := []string{"CREATE TABLE x (id INTEGER)"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Split() = %#v, want %#v", got, want)
	}
}
