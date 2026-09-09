package history

import (
	"os"
	"regexp"
	"slices"
	"testing"
)

func TestApplicationSchemaOwnershipMatchesAtlasDesiredSchema(t *testing.T) {
	contents, err := os.ReadFile("../../schema/trendinary.sql")
	if err != nil {
		t.Fatal(err)
	}

	matches := regexp.MustCompile(`(?mi)^\s*CREATE\s+TABLE\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`).FindAllSubmatch(contents, -1)
	fromAtlas := make([]string, 0, len(matches))
	for _, match := range matches {
		fromAtlas = append(fromAtlas, string(match[1]))
	}
	fromRuntime := append([]string{}, applicationSchemaTables...)
	slices.Sort(fromAtlas)
	slices.Sort(fromRuntime)

	if !slices.Equal(fromAtlas, fromRuntime) {
		t.Fatalf("Atlas desired tables and runtime ownership boundary differ\nAtlas:   %v\nRuntime: %v", fromAtlas, fromRuntime)
	}
}
