package chart_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vihuvac/read-waka-stats/internal/chart"
	"github.com/vihuvac/read-waka-stats/internal/commits"
	"github.com/vihuvac/read-waka-stats/internal/githubx"
)

func TestDrawWritesPNG(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "bar_graph.png")
	yearly := commits.YearlyData{
		2025: {
			1: {"Go": {Add: 100, Del: 10}, "Python": {Add: 40, Del: 5}},
			2: {"Go": {Add: 80, Del: 20}},
		},
	}
	colors := githubx.LinguistColors{"Go": "#00ADD8", "Python": "#3572A5"}
	if err := chart.Draw(yearly, colors, dest); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() < 100 {
		t.Fatalf("png too small: %d", info.Size())
	}
}

func TestDrawUnwritableDest(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "nope", "x.png")
	// make parent a file so MkdirAll fails
	parent := filepath.Dir(dest)
	if err := os.WriteFile(parent, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := chart.Draw(commits.YearlyData{}, githubx.LinguistColors{}, dest); err == nil {
		t.Fatal("expected error")
	}
}

func TestDrawEmpty(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "empty.png")
	if err := chart.Draw(commits.YearlyData{}, githubx.LinguistColors{}, dest); err != nil {
		t.Fatal(err)
	}
}

func TestDrawManyLanguagesAndBadColor(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "many.png")
	yearly := commits.YearlyData{
		2024: {
			1: {
				"Go":     {Add: 100, Del: 1},
				"Python": {Add: 90, Del: 1},
				"Rust":   {Add: 80, Del: 1},
				"JS":     {Add: 70, Del: 1},
				"TS":     {Add: 60, Del: 1},
				"C":      {Add: 50, Del: 1},
				"C++":    {Add: 40, Del: 1},
			},
		},
	}
	colors := githubx.LinguistColors{"Go": "bad", "Python": "#3572A5"}
	if err := chart.Draw(yearly, colors, dest); err != nil {
		t.Fatal(err)
	}
}
