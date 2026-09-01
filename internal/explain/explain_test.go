package explain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/explain"
	"github.com/chrisbirster/trendinary/internal/model"
)

func TestBuildKeepsEvidenceCitableAndSourceDiverse(t *testing.T) {
	trend := model.Trend{
		Name: "AT Protocol", Score: 88, Change: "+240%", Status: "RISING",
		Reason: "AT Protocol is accelerating across developer networks.",
		Sources: []model.Source{{Name: "Bluesky", Domain: "bsky.app"}, {Name: "GitHub", Domain: "github.com"}},
		Propagation: []model.PropagationHop{
			{Source: model.Source{Name: "GitHub", Domain: "github.com"}},
			{Source: model.Source{Name: "Bluesky", Domain: "bsky.app"}},
		},
	}
	signals := []model.Signal{
		{ID: "g1", Source: model.Source{Name: "GitHub", Domain: "github.com"}, Title: "repo", URL: "https://github.com/example/repo", Engagement: model.Engagement{Score: 50}},
		{ID: "b1", Source: model.Source{Name: "Bluesky", Domain: "bsky.app"}, Text: "discussion", URL: "https://bsky.app/profile/a/post/1", Engagement: model.Engagement{Likes: 20}},
		{ID: "b2", Source: model.Source{Name: "Bluesky", Domain: "bsky.app"}, Text: "more discussion", URL: "https://bsky.app/profile/b/post/2", Engagement: model.Engagement{Likes: 200}},
	}

	result := explain.Build(trend, signals, time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC))
	if result.Mode != explain.Mode { t.Fatalf("mode = %q", result.Mode) }
	if len(result.Evidence) < 2 { t.Fatalf("evidence = %+v", result.Evidence) }
	seen := map[string]bool{}
	for _, item := range result.Evidence {
		if item.URL == "" { t.Fatal("evidence item missing URL") }
		seen[item.Source.Domain] = true
	}
	if !seen["github.com"] || !seen["bsky.app"] { t.Fatalf("evidence lacks source diversity: %+v", seen) }
	if !strings.Contains(result.WhatChanged, "GitHub") || !strings.Contains(result.WhatChanged, "Bluesky") {
		t.Fatalf("propagation missing from explanation: %q", result.WhatChanged)
	}
}

func TestAnswerQuestionUsesGroundedPerspective(t *testing.T) {
	trend := model.Trend{
		Name: "Election coverage",
		Perspective: model.PerspectiveMix{RatedSources: 2, TotalSources: 3, Left: 1, Right: 1, Unrated: 1, Note: "Source leaning is not a truth score."},
		Explanation: &model.Explanation{Summary: "Coverage is accelerating.", Mode: explain.Mode},
	}
	answer := explain.AnswerQuestion("What is the left/right perspective mix?", trend)
	if !strings.Contains(answer.Answer, "left 1") || !strings.Contains(answer.Answer, "right 1") || !strings.Contains(answer.Answer, "truth score") {
		t.Fatalf("unexpected perspective answer: %q", answer.Answer)
	}
}
