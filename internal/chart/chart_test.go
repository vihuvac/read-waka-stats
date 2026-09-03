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

func TestDrawEmpty(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "empty.png")
	if err := chart.Draw(commits.YearlyData{}, githubx.LinguistColors{}, dest); err != nil {
		t.Fatal(err)
	}
}
