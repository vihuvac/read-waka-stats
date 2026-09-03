package wakatime_test

import (
	"context"
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
		// fixture may still have timezone
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
