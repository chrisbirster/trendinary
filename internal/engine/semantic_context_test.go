package engine

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/chrisbirster/trendinary/internal/model"
)

func TestClusterSignalsV2ContextHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ClusterSignalsV2Context(ctx, []model.Signal{{ID: "one", Title: "One"}}, 0.42)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestClusterSignalsV2ContextMatchesClusterSignalsV2(t *testing.T) {
	input := []model.Signal{
		{ID: "a", Title: "OpenAI releases new model", PublishedAt: "2026-09-15T01:00:00Z"},
		{ID: "b", Title: "OpenAI launches its new model", PublishedAt: "2026-09-15T01:05:00Z"},
		{ID: "c", Title: "NASA studies asteroid trajectory", PublishedAt: "2026-09-15T01:10:00Z"},
		{ID: "d", Title: "Asteroid trajectory studied by NASA", PublishedAt: "2026-09-15T01:15:00Z"},
		{ID: "e", Title: "TypeScript compiler update ships", PublishedAt: "2026-09-15T01:20:00Z"},
	}

	want := ClusterSignalsV2(input, 0.42)
	got, err := ClusterSignalsV2Context(context.Background(), input, 0.42)
	if err != nil {
		t.Fatalf("ClusterSignalsV2Context() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("clusters differ\n got: %#v\nwant: %#v", got, want)
	}
}
