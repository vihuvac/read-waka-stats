package commits_test

import (
	"testing"

	"github.com/vihuvac/read-waka-stats/internal/commits"
)

func TestTotalAdditions(t *testing.T) {
	y := commits.YearlyData{
		2024: {1: {"Go": {Add: 10, Del: 2}, "Python": {Add: 5}}},
		2025: {2: {"Go": {Add: 7}}},
	}
	if got := commits.TotalAdditions(y); got != 22 {
		t.Fatalf("got %d", got)
	}
}

func TestCountDayPartsTimezone(t *testing.T) {
	dates := commits.DateData{
		"repo": {
			"main": {
				"a": "2026-01-05T15:00:00Z", // Monday 15:00 UTC -> afternoon
				"b": "2026-01-06T03:00:00Z", // Tuesday 03:00 UTC -> night
			},
		},
	}
	day, week := commits.CountDayParts(dates, "UTC")
	if day[2] != 1 { // 12-18
		t.Fatalf("afternoon=%v", day)
	}
	if day[0] != 1 { // 00-06
		t.Fatalf("night=%v", day)
	}
	if week[0] != 1 || week[1] != 1 { // Mon, Tue
		t.Fatalf("week=%v", week)
	}
}

func TestIgnoreSet(t *testing.T) {
	m := commits.IgnoreSet([]string{" a ", "", "b"})
	if _, ok := m["a"]; !ok {
		t.Fatal("missing a")
	}
	if len(m) != 2 {
		t.Fatalf("len=%d", len(m))
	}
}
