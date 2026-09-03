package render_test

import (
	"strings"
	"testing"
	"time"

	"github.com/vihuvac/read-waka-stats/internal/config"
	"github.com/vihuvac/read-waka-stats/internal/githubx"
	"github.com/vihuvac/read-waka-stats/internal/i18n"
	"github.com/vihuvac/read-waka-stats/internal/render"
	"github.com/vihuvac/read-waka-stats/internal/wakatime"
)

func TestSortThenTruncateSortsFullSet(t *testing.T) {
	items := []render.Item{
		{Name: "A", Text: "1", Percent: 10},
		{Name: "B", Text: "2", Percent: 90},
		{Name: "C", Text: "3", Percent: 50},
		{Name: "D", Text: "4", Percent: 70},
	}
	got := render.SortThenTruncate(items, 2)
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].Name != "B" || got[1].Name != "D" {
		t.Fatalf("order = %s, %s (must sort before slice)", got[0].Name, got[1].Name)
	}
}

func TestMakeListSortsBeforeLimit(t *testing.T) {
	items := []render.Item{
		{Name: "Other", Text: "1 hrs", Percent: 5},
		{Name: "Go", Text: "10 hrs", Percent: 80},
		{Name: "Python", Text: "4 hrs", Percent: 15},
	}
	out := render.MakeList(items, 1, true, 1)
	if !strings.Contains(out, "Go") {
		t.Fatalf("expected Go first after full sort, got %q", out)
	}
	if strings.Contains(out, "Python") || strings.Contains(out, "Other") {
		t.Fatalf("limit failed: %q", out)
	}
}

func TestMakeGraphClamps(t *testing.T) {
	g := render.MakeGraph(200, 1)
	if strings.Contains(g, "░") {
		t.Fatalf("100%% should be full: %q", g)
	}
	g = render.MakeGraph(-10, 1)
	if strings.Contains(g, "█") {
		t.Fatalf("0%% should be empty: %q", g)
	}
}

func renderer(t *testing.T) render.Renderer {
	t.Helper()
	bundle, err := i18n.Load("en")
	if err != nil {
		t.Fatal(err)
	}
	return render.Renderer{
		Cfg: &config.Config{
			ShowTimezone:      true,
			ShowLanguage:      true,
			ShowEditors:       true,
			ShowProjects:      true,
			ShowOS:            true,
			ShowLanguageCount: 5,
			SymbolVersion:     1,
			BadgeStyle:        "flat",
			ShowUpdatedDate:   true,
			UpdatedDateFormat: "02/01/2006 15:04:05",
			ShowCommit:        true,
			ShowDaysOfWeek:    true,
		},
		T:   bundle,
		Now: time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC),
	}
}

func TestWeeklyAndAI(t *testing.T) {
	r := renderer(t)
	data := wakatime.Data{
		Timezone: "America/New_York",
		Languages: []wakatime.StatItem{
			{Name: "Go", Text: "10 hrs", Percent: 80},
			{Name: "Python", Text: "2 hrs", Percent: 20},
		},
		Editors:                     []wakatime.StatItem{{Name: "VS Code", Text: "12 hrs", Percent: 100}},
		OperatingSystems:            []wakatime.StatItem{{Name: "Mac", Text: "12 hrs", Percent: 100}},
		Projects:                    []wakatime.StatItem{{Name: "app", Text: "12 hrs", Percent: 100}},
		Categories:                  []wakatime.StatItem{{Name: "AI Coding", Text: "1 hr", Percent: 10}},
		AISessions:                  2,
		AIAdditions:                 80,
		HumanAdditions:              20,
		AIDeletions:                 5,
		HumanDeletions:              15,
		AIModelTotalCost:            1.5,
		AIInputTokens:               1000,
		AIOutputTokens:              200,
		AIPromptLengthAvg:           100,
		AIPromptEventsTotal:         4,
		AIPromptEventsAvgPerSession: 2,
		AIModelBreakdown:            []wakatime.ModelBreakdown{{Name: "gpt", Lines: 80}, {Name: "other", Lines: 20}},
	}
	weekly := r.WeeklyWaka(data)
	if !strings.Contains(weekly, "```text") || !strings.Contains(weekly, "Go") {
		t.Fatalf("weekly: %s", weekly)
	}
	ai := r.AICodingStats(data)
	if !strings.Contains(ai, "AI Coding This Week") {
		t.Fatalf("ai: %s", ai)
	}
	empty := r.AICodingStats(wakatime.Data{})
	if !strings.Contains(empty, "No AI Coding Activity Tracked This Week") {
		t.Fatalf("empty ai: %s", empty)
	}
}

func TestLanguagePerRepoIgnores(t *testing.T) {
	r := renderer(t)
	r.Cfg.IgnoredRepos = []string{"skip"}
	out := r.LanguagePerRepo([]githubx.Repository{
		{Name: "a", PrimaryLanguage: "Go"},
		{Name: "b", PrimaryLanguage: "Go"},
		{Name: "skip", PrimaryLanguage: "Python"},
		{Name: "c", PrimaryLanguage: "Rust"},
	})
	if !strings.Contains(out, "Go") {
		t.Fatal(out)
	}
	if strings.Contains(out, "Python") {
		t.Fatalf("ignored repo leaked: %s", out)
	}
}

func TestCommitDayTimeTitles(t *testing.T) {
	r := renderer(t)
	// night hours dominate after rotation: original [0-6, 6-12, 12-18, 18-24]
	out := r.CommitDayTime([4]int{10, 1, 1, 10}, [7]int{1, 5, 1, 1, 1, 1, 1})
	if !strings.Contains(out, "Night") && !strings.Contains(out, "Early") {
		t.Fatalf("missing title: %s", out)
	}
	if !strings.Contains(out, "Tuesday") {
		t.Fatalf("weekday: %s", out)
	}
}

func TestBadges(t *testing.T) {
	r := renderer(t)
	if !strings.Contains(r.CodeTimeBadge("10 hrs"), "img.shields.io") {
		t.Fatal("code time badge")
	}
	if !strings.Contains(r.UpdatedDate(), "03/09/2026") {
		t.Fatalf("date %s", r.UpdatedDate())
	}
}
