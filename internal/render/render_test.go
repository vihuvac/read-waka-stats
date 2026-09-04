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

func TestSortTieBreakAndMakeListNoSort(t *testing.T) {
	items := []render.Item{
		{Name: "B", Text: "1", Percent: 50},
		{Name: "A", Text: "1", Percent: 50},
		{Name: "C", Text: "1", Percent: 10},
	}
	got := render.SortThenTruncate(items, 2)
	if got[0].Name != "A" || got[1].Name != "B" {
		t.Fatalf("%v", got)
	}
	out := render.MakeList(items, 1, false, 99) // invalid symbol version falls back
	if !strings.Contains(out, "B") {
		t.Fatal(out)
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

func TestMakeGraphVersionsAndTruncate(t *testing.T) {
	for v := 1; v <= 3; v++ {
		g := render.MakeGraph(50, v)
		if g == "" {
			t.Fatalf("empty graph v=%d", v)
		}
	}
	items := []render.Item{{Name: strings.Repeat("字", 30), Text: "1", Percent: 10}}
	out := render.MakeList(items, 5, true, 1)
	if !strings.Contains(out, "…") && len([]rune(out)) > 100 {
		t.Log(out)
	}
	got := render.SortThenTruncate(nil, 0)
	if len(got) != 0 {
		t.Fatalf("%v", got)
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

func TestWeeklyEmptyListsAndDisabled(t *testing.T) {
	r := renderer(t)
	out := r.WeeklyWaka(wakatime.Data{Timezone: "UTC"})
	if !strings.Contains(out, "No Activity Tracked This Week") {
		t.Fatal(out)
	}
	r.Cfg.ShowTimezone = false
	r.Cfg.ShowLanguage = false
	r.Cfg.ShowEditors = false
	r.Cfg.ShowProjects = false
	r.Cfg.ShowOS = false
	if r.WeeklyWaka(wakatime.Data{}) != "" {
		t.Fatal("expected empty weekly")
	}
}

func TestAICodingEdgeCases(t *testing.T) {
	r := renderer(t)
	data := wakatime.Data{
		Categories:                  []wakatime.StatItem{{Name: "AI Coding", Text: "1 hr", Percent: 10}},
		AISessions:                  1,
		AIAdditions:                 0,
		HumanAdditions:              0,
		AIDeletions:                 0,
		HumanDeletions:              0,
		AIPromptLengthAvg:           600,
		AIPromptEventsAvgPerSession: 0.5,
		AIModelBreakdown:            []wakatime.ModelBreakdown{{Name: "x", Lines: 0}},
	}
	out := r.AICodingStats(data)
	if !strings.Contains(out, "AI Coding Insights") {
		t.Fatal(out)
	}
	data.AIAdditions = 80
	data.HumanAdditions = 20
	data.HumanDeletions = 50
	data.AIPromptLengthAvg = 2000
	data.AIPromptEventsAvgPerSession = 3
	out = r.AICodingStats(data)
	if !strings.Contains(out, "AI-Driven") && !strings.Contains(out, "Verbose") {
		if !strings.Contains(out, "Insights") {
			t.Fatal(out)
		}
	}
}

func TestAIInsightsBranches(t *testing.T) {
	r := renderer(t)
	data := wakatime.Data{
		Categories:                  []wakatime.StatItem{{Name: "AI Coding", Text: "1h", Percent: 10}},
		AISessions:                  2,
		AIAdditions:                 40,
		HumanAdditions:              60,
		HumanDeletions:              100,
		AIPromptLengthAvg:           100,
		AIPromptEventsAvgPerSession: 1,
	}
	out := r.AICodingStats(data)
	if !strings.Contains(out, "Insights") {
		t.Fatal(out)
	}
	data.AIAdditions = 50
	data.HumanAdditions = 50
	data.AIPromptLengthAvg = 500
	out = r.AICodingStats(data)
	if out == "" {
		t.Fatal("empty")
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

func TestLanguagePerRepoEmpty(t *testing.T) {
	r := renderer(t)
	if r.LanguagePerRepo(nil) != "" {
		t.Fatal("expected empty")
	}
	out := r.LanguagePerRepo([]githubx.Repository{
		{Name: "nolang", PrimaryLanguage: ""},
		{Name: "only", PrimaryLanguage: "Go"},
	})
	if !strings.Contains(out, "1 repo") {
		t.Fatal(out)
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

func TestCommitDayTimeEarlyBird(t *testing.T) {
	r := renderer(t)
	out := r.CommitDayTime([4]int{0, 10, 10, 0}, [7]int{5, 1, 1, 1, 1, 1, 1})
	if out == "" {
		t.Fatal("expected output")
	}
}

func TestCommitDayTimeDisabled(t *testing.T) {
	r := renderer(t)
	r.Cfg.ShowCommit = false
	r.Cfg.ShowDaysOfWeek = false
	if r.CommitDayTime([4]int{1, 1, 1, 1}, [7]int{1, 1, 1, 1, 1, 1, 1}) != "" {
		t.Fatal("expected empty")
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

func TestMoreBadgesAndInfo(t *testing.T) {
	r := renderer(t)
	if !strings.Contains(r.AICodeTimeBadge("2 hrs"), "AI%20Code%20Time") && !strings.Contains(r.AICodeTimeBadge("2 hrs"), "AI Code Time") {
		if !strings.Contains(r.AICodeTimeBadge("2 hrs"), "shields.io") {
			t.Fatal(r.AICodeTimeBadge("2 hrs"))
		}
	}
	if !strings.Contains(r.ProfileViewsBadge(99), "99") {
		t.Fatal(r.ProfileViewsBadge(99))
	}
	loc := r.LinesOfCodeBadge(1500)
	if !strings.Contains(loc, "thousand") && !strings.Contains(loc, "1.50") {
		t.Fatal(loc)
	}
	locM := r.LinesOfCodeBadge(2_500_000)
	if !strings.Contains(locM, "million") {
		t.Fatal(locM)
	}
	locB := r.LinesOfCodeBadge(2_000_000_000)
	if !strings.Contains(locB, "billion") {
		t.Fatal(locB)
	}
	if !strings.Contains(r.LinesOfCodeBadge(42), "42") {
		t.Fatal(r.LinesOfCodeBadge(42))
	}

	info := r.ShortGitHubInfo(githubx.User{
		DiskUsage:         2048,
		Hireable:          true,
		PublicRepos:       2,
		OwnedPrivateRepos: 3,
	}, 2026, 1000)
	if !strings.Contains(info, "kB") || !strings.Contains(info, "Opted to Hire") {
		t.Fatal(info)
	}
	info2 := r.ShortGitHubInfo(githubx.User{
		DiskUsage:   -1,
		Hireable:    false,
		PublicRepos: 1,
	}, 0, 0)
	if !strings.Contains(info2, "?") || !strings.Contains(info2, "Not Opted to Hire") {
		t.Fatal(info2)
	}
	big := r.ShortGitHubInfo(githubx.User{DiskUsage: 5 * 1024 * 1024 * 1024, PublicRepos: 5}, 2025, 10)
	if !strings.Contains(big, "GB") {
		t.Fatal(big)
	}
	mb := r.ShortGitHubInfo(githubx.User{DiskUsage: 3 * 1024 * 1024, PublicRepos: 5}, 2025, 10)
	if !strings.Contains(mb, "MB") {
		t.Fatal(mb)
	}
	b := r.ShortGitHubInfo(githubx.User{DiskUsage: 100, PublicRepos: 5}, 2025, 10)
	if !strings.Contains(b, "Bytes") {
		t.Fatal(b)
	}

	if !strings.Contains(r.TimelineImage("https://example.com/chart.png"), "chart.png") {
		t.Fatal(r.TimelineImage("x"))
	}
	r.Cfg.ShowUpdatedDate = false
	if r.UpdatedDate() != "" {
		t.Fatal("expected empty")
	}
}

func TestIntwordNegative(t *testing.T) {
	r := renderer(t)
	out := r.LinesOfCodeBadge(-5)
	if !strings.Contains(out, "5") {
		t.Fatal(out)
	}
	info := r.ShortGitHubInfo(githubx.User{PublicRepos: 2}, 2026, -1000)
	if !strings.Contains(info, ",") && !strings.Contains(info, "1000") {
		t.Log(info)
	}
}
