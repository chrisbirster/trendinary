package eval

import (
	"strings"
	"testing"

	"github.com/chrisbirster/trendinary/internal/model"
)

func TestLoadRejectsEmptyCorpus(t *testing.T) {
	if _, err := Load(strings.NewReader(`{"name":"empty","signals":[]}`)); err == nil {
		t.Fatal("expected empty corpus error")
	}
}

func TestEvaluateMeasuresKnownClusterPairs(t *testing.T) {
	corpus := Corpus{
		Name: "entity-alias-smoke",
		Signals: []LabeledSignal{
			{Signal: model.Signal{ID: "a", Title: "AT Protocol gains developer traction", URL: "https://example.com/a"}, Group: "atproto"},
			{Signal: model.Signal{ID: "b", Title: "ATProto gains developer momentum", URL: "https://different.example/b"}, Group: "atproto"},
			{Signal: model.Signal{ID: "c", Title: "Coffee machine review", URL: "https://coffee.example/review"}, Group: "coffee"},
		},
	}

	report := Evaluate(corpus, 0.42)
	if report.Signals != 3 || report.ExpectedPairs != 1 {
		t.Fatalf("report = %+v", report)
	}
	if report.PredictedPairs != 1 || report.TruePositivePairs != 1 {
		t.Fatalf("pair counts = %+v", report)
	}
	if report.Precision != 1 || report.Recall != 1 {
		t.Fatalf("precision/recall = %+v", report)
	}
}

func TestEvaluatePenalizesFalseMerge(t *testing.T) {
	corpus := Corpus{
		Name: "false-merge-smoke",
		Signals: []LabeledSignal{
			{Signal: model.Signal{ID: "a", Title: "SolidJS router release", URL: "https://one.example/solid"}, Group: "solid"},
			{Signal: model.Signal{ID: "b", Title: "SolidJS router release", URL: "https://two.example/solid"}, Group: "other"},
		},
	}

	report := Evaluate(corpus, 0.42)
	if report.PredictedPairs != 1 || report.TruePositivePairs != 0 || report.Precision != 0 {
		t.Fatalf("report = %+v", report)
	}
}
