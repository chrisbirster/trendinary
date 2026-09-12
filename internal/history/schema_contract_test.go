package history

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestApplicationSchemaOwnershipMatchesGooseMigrations(t *testing.T) {
	entries, err := os.ReadDir("../../migrations")
	if err != nil {
		t.Fatal(err)
	}

	tablePattern := regexp.MustCompile(`(?mi)^\s*CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	fromMigrations := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		contents, err := os.ReadFile(filepath.Join("../../migrations", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range tablePattern.FindAllSubmatch(contents, -1) {
			fromMigrations = append(fromMigrations, string(match[1]))
		}
	}

	fromRuntime := append([]string{}, applicationSchemaTables...)
	slices.Sort(fromMigrations)
	slices.Sort(fromRuntime)

	if !slices.Equal(fromMigrations, fromRuntime) {
		t.Fatalf("Goose migration tables and runtime ownership boundary differ\nMigrations: %v\nRuntime:    %v", fromMigrations, fromRuntime)
	}
}
