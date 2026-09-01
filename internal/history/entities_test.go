package history_test

import (
	"context"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/history"
)

func TestResolveEntityReusesAliasAndTerms(t *testing.T) {
	store, err := history.Open(":memory:")
	if err != nil { t.Fatal(err) }
	defer store.Close()
	ctx := context.Background()
	firstAt := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)

	first, err := store.ResolveEntity(ctx, "AT Protocol", "at-protocol-social-apps", []string{"atproto", "social", "protocol"}, firstAt)
	if err != nil { t.Fatal(err) }
	second, err := store.ResolveEntity(ctx, "ATProto", "atproto-app-launches", []string{"atproto", "social", "apps"}, firstAt.Add(time.Hour))
	if err != nil { t.Fatal(err) }

	if first.ID != second.ID {
		t.Fatalf("entity split across aliases: %s != %s", first.ID, second.ID)
	}
	if first.Slug != second.Slug {
		t.Fatalf("stable slug changed: %q != %q", first.Slug, second.Slug)
	}
	resolved, err := store.ResolveTrendKey(ctx, second.Slug)
	if err != nil { t.Fatal(err) }
	if resolved != first.ID {
		t.Fatalf("slug resolved to %q, want %q", resolved, first.ID)
	}
}

func TestResolveEntityDoesNotMergeWeakSingleTermOverlap(t *testing.T) {
	store, err := history.Open(":memory:")
	if err != nil { t.Fatal(err) }
	defer store.Close()
	ctx := context.Background()
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)

	left, err := store.ResolveEntity(ctx, "SolidJS native renderer", "solidjs-native-renderer", []string{"solidjs", "native", "renderer"}, now)
	if err != nil { t.Fatal(err) }
	right, err := store.ResolveEntity(ctx, "React Native release", "react-native-release", []string{"native", "react", "release"}, now)
	if err != nil { t.Fatal(err) }
	if left.ID == right.ID {
		t.Fatalf("weak single-term overlap incorrectly merged unrelated trends: %s", left.ID)
	}
}
