package main

import (
	"os"
	"strings"
	"testing"
)

func TestFlyConfigNeverContainsStartupDatabaseReset(t *testing.T) {
	contents, err := os.ReadFile("../../fly.toml")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "TRENDINARY_RESET_DATABASE_ID") {
		t.Fatal("fly.toml must never trigger destructive database reset during application startup")
	}
}
