package buildinfo

import "testing"

func TestNormalizeDefaults(t *testing.T) {
	got := Normalize(Info{})
	if got.APIVersion != "v1" || got.Release != "dev" || got.Commit != "unknown" {
		t.Fatalf("unexpected defaults: %+v", got)
	}
}
