package history_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/history"
)

func TestResolveEntityStrictReusesStrongTermMatch(t *testing.T) {
	store, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

	first, err := store.ResolveEntityStrict(ctx, "AT Protocol", "at-protocol-social-apps", []string{"atproto", "social", "protocol"}, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.ResolveEntityStrict(ctx, "ATProto network", "atproto-network-apps", []string{"atproto", "social", "apps"}, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("strong 2/3 term match split stable entity: %s != %s", first.ID, second.ID)
	}
	if len(second.Terms) != 3 {
		t.Fatalf("strict entity terms grew instead of being replaced: %+v", second.Terms)
	}
	if len(second.Aliases) > 2 {
		t.Fatalf("strict entity aliases grew without bound: %+v", second.Aliases)
	}
}

func TestResolveEntityStrictQuarantinesPollutedLegacyEntity(t *testing.T) {
	store, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

	var polluted history.Entity
	for batch := 0; batch < 5; batch++ {
		terms := make([]string, 0, 8)
		for i := 0; i < 8; i++ {
			terms = append(terms, fmt.Sprintf("term-%d-%d", batch, i))
		}
		polluted, err = store.ResolveEntity(ctx, "Polluted Entity", "polluted-entity", terms, now.Add(time.Duration(batch)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(polluted.Terms) <= 24 {
		t.Fatalf("fixture did not create broad legacy entity: %d terms", len(polluted.Terms))
	}

	fresh, err := store.ResolveEntityStrict(ctx, "Unrelated candidate", "unrelated-candidate", []string{"term-0-0", "term-0-1", "term-0-2"}, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if fresh.ID == polluted.ID {
		t.Fatal("strict resolver fuzzy-matched an already polluted legacy entity")
	}
}

func TestResolveEntityStrictExactCanonicalNameCanCleanPollutedEntity(t *testing.T) {
	store, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

	var polluted history.Entity
	for batch := 0; batch < 5; batch++ {
		terms := make([]string, 0, 8)
		for i := 0; i < 8; i++ {
			terms = append(terms, fmt.Sprintf("legacy-%d-%d", batch, i))
		}
		polluted, err = store.ResolveEntity(ctx, "Stable Canonical", "stable-canonical", terms, now.Add(time.Duration(batch)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
	}

	cleaned, err := store.ResolveEntityStrict(ctx, "Stable Canonical", "stable-canonical", []string{"stable", "canonical", "event"}, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if cleaned.ID != polluted.ID {
		t.Fatalf("exact canonical name should salvage existing entity: %s != %s", cleaned.ID, polluted.ID)
	}
	if len(cleaned.Terms) != 3 {
		t.Fatalf("polluted terms were not replaced: %+v", cleaned.Terms)
	}
	if len(cleaned.Aliases) != 1 || cleaned.Aliases[0] != "Stable Canonical" {
		t.Fatalf("polluted aliases were not bounded: %+v", cleaned.Aliases)
	}
}
