package engine

import (
	"context"
	"errors"
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
