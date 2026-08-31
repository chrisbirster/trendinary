package engine_test

import (
	"testing"

	"github.com/chrisbirster/trendinary/internal/engine"
)

func TestLifecycle(t *testing.T) {
	cases := []struct {
		name string
		in   engine.LifecycleInput
		want string
	}{
		{"breaking", engine.LifecycleInput{Score: 90, PreviousScore: 70, Velocity: 0.9, SourceBreadth: 0.8}, "BREAKING"},
		{"rising", engine.LifecycleInput{Score: 68, PreviousScore: 55, Velocity: 0.7, SourceBreadth: 0.4}, "RISING"},
		{"peaking", engine.LifecycleInput{Score: 81, PreviousScore: 80, Velocity: 0.4, SourceBreadth: 0.8}, "PEAKING"},
		{"cooling", engine.LifecycleInput{Score: 62, PreviousScore: 78, Velocity: 0.3, SourceBreadth: 0.5}, "COOLING"},
		{"resurfacing", engine.LifecycleInput{Score: 70, PreviousScore: 20, Velocity: 0.8, SourceBreadth: 0.5, PreviouslyCold: true}, "RESURFACING"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := engine.Lifecycle(tc.in); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
