package config_test

import (
	"testing"
	"time"

	"github.com/vihuvac/read-waka-stats/internal/config"
)

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
