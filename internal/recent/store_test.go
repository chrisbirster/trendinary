package recent_test

import (
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
	"github.com/chrisbirster/trendinary/internal/recent"
)

func signal(id string) model.Signal {
	return model.Signal{ID: id, Text: "signal " + id}
}

func TestRecentWindowAndDelete(t *testing.T) {
	store := recent.New(10, time.Hour)
	base := time.Now().UTC()
	store.Upsert(signal("old"), base.Add(-30*time.Minute))
	store.Upsert(signal("new"), base.Add(-5*time.Minute))

	values := store.Recent(base.Add(-10 * time.Minute))
	if len(values) != 1 || values[0].ID != "new" {
		t.Fatalf("unexpected recent values: %+v", values)
	}
	store.Delete("new")
	if got := store.Len(); got != 1 {
		t.Fatalf("len = %d, want 1", got)
	}
}

func TestStoreEvictsOldestAtCapacity(t *testing.T) {
	store := recent.New(2, time.Hour)
	base := time.Now().UTC()
	store.Upsert(signal("one"), base.Add(-3*time.Minute))
	store.Upsert(signal("two"), base.Add(-2*time.Minute))
	store.Upsert(signal("three"), base.Add(-time.Minute))

	values := store.Recent(base.Add(-time.Hour))
	if len(values) != 2 {
		t.Fatalf("len = %d, want 2", len(values))
	}
	if values[0].ID != "two" || values[1].ID != "three" {
		t.Fatalf("unexpected eviction order: %+v", values)
	}
}

func TestUpsertReplacesSignal(t *testing.T) {
	store := recent.New(10, time.Hour)
	base := time.Now().UTC()
	first := signal("same")
	first.Text = "first"
	second := signal("same")
	second.Text = "updated"
	store.Upsert(first, base.Add(-time.Minute))
	store.Upsert(second, base)

	values := store.Recent(base.Add(-time.Hour))
	if len(values) != 1 || values[0].Text != "updated" {
		t.Fatalf("unexpected upsert: %+v", values)
	}
}
