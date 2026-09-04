package wakatime_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/vihuvac/read-waka-stats/internal/wakatime"
)

func TestFindCategory(t *testing.T) {
	cats := []wakatime.Category{{Name: "Coding"}, {Name: "AI Coding", Text: "1 hr"}}
	got := wakatime.FindCategory(cats, "AI Coding")
	if got == nil || got.Text != "1 hr" {
		t.Fatalf("got %+v", got)
	}
	if wakatime.FindCategory(cats, "nope") != nil {
		t.Fatal("expected nil")
	}
}

func TestLoadMock(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "testdata")
	c := &wakatime.Client{Mock: true, MockDir: dir}
	weekly, err := c.FetchWeekly(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if weekly == nil || len(weekly.Data.Languages) == 0 && weekly.Data.Timezone == "" {
		if weekly == nil {
			t.Fatal("nil weekly")
		}
	}
	all, err := c.FetchAllTime(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if all == nil {
		t.Fatal("nil all-time")
	}
}

func TestLoadMockErrors(t *testing.T) {
	c := &wakatime.Client{Mock: true, MockDir: t.TempDir()}
	if _, err := c.FetchWeekly(context.Background()); err == nil {
		t.Fatal("expected missing file error")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "wakatime_stats.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.MockDir = dir
	if _, err := c.FetchWeekly(context.Background()); err == nil {
		t.Fatal("expected json error")
	}
}
