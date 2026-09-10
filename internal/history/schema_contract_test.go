package history

import (
	"os"
	"regexp"
	"slices"
	"testing"
)

func TestApplicationSchemaOwnershipMatchesGooseBaseline(t *testing.T) {
	contents, err := os.ReadFile("../../migrations/00001_baseline.sql")
	if err != nil {
		t.Fatal(err)
	}

	matches := regexp.MustCompile(`(?mi)^\s*CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*\(`).FindAllSubmatch(contents, -1)
	fromMigrations := make([]string, 0, len(matches))
	for _, match := range matches {
		fromMigrations = append(fromMigrations, string(match[1]))
	}
	fromRuntime := append([]string{}, applicationSchemaTables...)
	slices.Sort(fromMigrations)
	slices.Sort(fromRuntime)

	if !slices.Equal(fromMigrations, fromRuntime) {
		t.Fatalf("Goose baseline tables and runtime ownership boundary differ\nMigrations: %v\nRuntime:    %v", fromMigrations, fromRuntime)
	}
}
