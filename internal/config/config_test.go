package config_test

import (
	"testing"
	"time"

	"github.com/vihuvac/read-waka-stats/internal/config"
)

func TestLoadRequiresTokens(t *testing.T) {
	t.Setenv("INPUT_GH_TOKEN", "")
	t.Setenv("INPUT_WAKATIME_API_KEY", "key")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected error for missing GH_TOKEN")
	}

	t.Setenv("INPUT_GH_TOKEN", "gh")
	t.Setenv("INPUT_WAKATIME_API_KEY", "")
	t.Setenv("MOCK_WAKATIME", "false")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected error for missing WAKATIME_API_KEY")
	}
}

func TestLoadDefaultsAndParsing(t *testing.T) {
	t.Setenv("INPUT_GH_TOKEN", "token")
	t.Setenv("INPUT_PUSH_TOKEN", "")
	t.Setenv("INPUT_WAKATIME_API_KEY", "waka")
	t.Setenv("INPUT_SHOW_OS", "yes")
	t.Setenv("INPUT_SHOW_LINES_OF_CODE", "0")
	t.Setenv("INPUT_IGNORED_REPOS", "a, b, c")
	t.Setenv("INPUT_SHOW_LANGUAGE_COUNT", "3")
	t.Setenv("INPUT_WAKATIME_API_URL", "https://wakatime.com/api/v1")
	t.Setenv("INPUT_MAX_REPOS", "10")
	t.Setenv("INPUT_SYMBOL_VERSION", "2")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.ShowOS {
		t.Fatal("ShowOS should be true")
	}
	if cfg.ShowLinesOfCode {
		t.Fatal("ShowLinesOfCode should be false")
	}
	if len(cfg.IgnoredRepos) != 3 {
		t.Fatalf("IgnoredRepos = %v", cfg.IgnoredRepos)
	}
	if cfg.ShowLanguageCount != 3 {
		t.Fatalf("ShowLanguageCount = %d", cfg.ShowLanguageCount)
	}
	if cfg.WakaTimeAPIURL != "https://wakatime.com/api/v1/" {
		t.Fatalf("API URL = %q", cfg.WakaTimeAPIURL)
	}
	if cfg.MaxRepos != 10 {
		t.Fatalf("MaxRepos = %d", cfg.MaxRepos)
	}
	if cfg.SymbolVersion != 2 {
		t.Fatalf("SymbolVersion = %d", cfg.SymbolVersion)
	}
	if cfg.PushToken != "token" {
		t.Fatalf("PushToken fallback = %q", cfg.PushToken)
	}
	if !cfg.NeedsCommitData() {
		t.Fatal("NeedsCommitData expected true with default ShowLOCChart")
	}
}

func TestLoadPushTokenPreferredOverGHToken(t *testing.T) {
	t.Setenv("INPUT_GH_TOKEN", "read-token")
	t.Setenv("INPUT_PUSH_TOKEN", "write-token")
	t.Setenv("INPUT_WAKATIME_API_KEY", "waka")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GHToken != "read-token" {
		t.Fatalf("GHToken = %q", cfg.GHToken)
	}
	if cfg.PushToken != "write-token" {
		t.Fatalf("PushToken = %q, want write-token", cfg.PushToken)
	}
}

func TestNeedsCommitData(t *testing.T) {
	cfg := &config.Config{}
	if cfg.NeedsCommitData() {
		t.Fatal("expected false")
	}
	cfg.ShowCommit = true
	if !cfg.NeedsCommitData() {
		t.Fatal("expected true")
	}
}

func TestUpdatedDateFormatConversion(t *testing.T) {
	t.Setenv("INPUT_GH_TOKEN", "token")
	t.Setenv("INPUT_WAKATIME_API_KEY", "waka")
	t.Setenv("INPUT_UPDATED_DATE_FORMAT", "%d/%m/%Y %H:%M:%S")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Date(2026, 9, 3, 14, 30, 5, 0, time.UTC)
	got := ts.Format(cfg.UpdatedDateFormat)
	want := "03/09/2026 14:30:05"
	if got != want {
		t.Fatalf("got %q want %q (layout %q)", got, want, cfg.UpdatedDateFormat)
	}
}

func TestNeedsRepos(t *testing.T) {
	cfg := &config.Config{}
	if cfg.NeedsRepos() {
		t.Fatal("expected false")
	}
	cfg.ShowLanguageRepo = true
	if !cfg.NeedsRepos() {
		t.Fatal("expected true")
	}
}

func TestLoadClampsAndBadgeStyle(t *testing.T) {
	t.Setenv("INPUT_GH_TOKEN", "t")
	t.Setenv("INPUT_WAKATIME_API_KEY", "k")
	t.Setenv("INPUT_SHOW_LANGUAGE_COUNT", "0")
	t.Setenv("INPUT_MAX_REPOS", "-5")
	t.Setenv("INPUT_SYMBOL_VERSION", "99")
	t.Setenv("INPUT_BADGE_STYLE", "")
	t.Setenv("BADGE_STYLE", "for-the-badge")
	t.Setenv("MOCK_WAKATIME", "true")
	t.Setenv("INPUT_WAKATIME_API_KEY", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ShowLanguageCount != 1 {
		t.Fatalf("count=%d", cfg.ShowLanguageCount)
	}
	if cfg.MaxRepos != 0 {
		t.Fatalf("max=%d", cfg.MaxRepos)
	}
	if cfg.SymbolVersion != 1 {
		t.Fatalf("symbol=%d", cfg.SymbolVersion)
	}
	if cfg.BadgeStyle != "for-the-badge" {
		t.Fatalf("badge=%q", cfg.BadgeStyle)
	}
}

func TestLoadInvalidIntFallsBack(t *testing.T) {
	t.Setenv("INPUT_GH_TOKEN", "t")
	t.Setenv("INPUT_WAKATIME_API_KEY", "k")
	t.Setenv("INPUT_MAX_REPOS", "nope")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxRepos != 0 {
		t.Fatalf("%d", cfg.MaxRepos)
	}
}

func TestLoadEmptyBadgeFallsThrough(t *testing.T) {
	t.Setenv("INPUT_GH_TOKEN", "t")
	t.Setenv("INPUT_WAKATIME_API_KEY", "k")
	t.Setenv("INPUT_BADGE_STYLE", "   ")
	t.Setenv("BADGE_STYLE", "")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BadgeStyle != "flat" {
		t.Fatalf("got %q", cfg.BadgeStyle)
	}
}
