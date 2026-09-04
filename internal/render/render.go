// Package render builds README markdown from collected statistics.
package render

import (
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vihuvac/read-waka-stats/internal/config"
	"github.com/vihuvac/read-waka-stats/internal/githubx"
	"github.com/vihuvac/read-waka-stats/internal/i18n"
	"github.com/vihuvac/read-waka-stats/internal/wakatime"
)

var (
	dayTimeEmoji = []string{"🌞", "🌆", "🌃", "🌙"}
	dayTimeNames = []string{"Morning", "Daytime", "Evening", "Night"}
	weekDayNames = []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
)

var symbolPairs = map[int][2]string{
	1: {"█", "░"},
	2: {"⣿", "⣀"},
	3: {"⬛", "⬜"},
}

// Item is a progress-bar row.
type Item struct {
	Name    string
	Text    string
	Percent float64
}

// Renderer formats stats using config and translations.
type Renderer struct {
	Cfg *config.Config
	T   *i18n.Bundle
	Now time.Time
}

// FromWaka converts WakaTime stat items to render items.
func FromWaka(items []wakatime.StatItem) []Item {
	out := make([]Item, 0, len(items))
	for _, it := range items {
		out = append(out, Item{Name: it.Name, Text: it.Text, Percent: it.Percent})
	}
	return out
}

// SortThenTruncate sorts by percent descending, then keeps at most limit items.
func SortThenTruncate(items []Item, limit int) []Item {
	cp := make([]Item, len(items))
	copy(cp, items)
	sort.SliceStable(cp, func(i, j int) bool {
		if cp[i].Percent != cp[j].Percent {
			return cp[i].Percent > cp[j].Percent
		}
		return cp[i].Name < cp[j].Name
	})
	if limit > 0 && len(cp) > limit {
		cp = cp[:limit]
	}
	return cp
}

// MakeGraph returns a 25-character progress bar for percent.
func MakeGraph(percent float64, version int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	pair, ok := symbolPairs[version]
	if !ok {
		pair = symbolPairs[1]
	}
	filled := int(math.Round(percent / 4))
	if filled < 0 {
		filled = 0
	}
	if filled > 25 {
		filled = 25
	}
	return strings.Repeat(pair[0], filled) + strings.Repeat(pair[1], 25-filled)
}

// pad right-pads s with spaces to the given rune width.
func pad(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

// truncateRunes returns at most max runes from s.
func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}

// MakeList formats progress rows. When sortItems is true, the full set is sorted before truncation.
func MakeList(items []Item, topNum int, sortItems bool, symbolVersion int) string {
	rows := items
	if sortItems {
		rows = SortThenTruncate(items, topNum)
	} else if topNum > 0 && len(rows) > topNum {
		rows = rows[:topNum]
	}
	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		name := truncateRunes(r.Name, 25)
		text := r.Text
		lines = append(lines, fmt.Sprintf("%s%s%s   %05.2f %% ", pad(name, 25), pad(text, 20), MakeGraph(r.Percent, symbolVersion), r.Percent))
	}
	return strings.Join(lines, "\n")
}

// badge returns a shields.io markdown image for label/message.
func badge(label, message, style string) string {
	return fmt.Sprintf("![%s](https://img.shields.io/badge/%s-%s-blue?style=%s)\n\n",
		label,
		url.PathEscape(label),
		url.PathEscape(message),
		url.QueryEscape(style),
	)
}

// t looks up a translation key via the renderer's bundle.
func (r *Renderer) t(key string) string {
	return r.T.T(key)
}

// sprintf formats a translated string that contains printf verbs.
func sprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

// intcomma formats n with thousands separators.
func intcomma(n int) string {
	s := strconv.Itoa(n)
	if n < 0 {
		s = strconv.Itoa(-n)
	}
	var b strings.Builder
	if n < 0 {
		b.WriteByte('-')
	}
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// intword formats large integers as thousand/million/billion phrases.
func intword(n int64) string {
	abs := n
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs < 1000:
		return strconv.FormatInt(n, 10)
	case abs < 1_000_000:
		return fmt.Sprintf("%.2f thousand", float64(n)/1000)
	case abs < 1_000_000_000:
		return fmt.Sprintf("%.2f million", float64(n)/1_000_000)
	default:
		return fmt.Sprintf("%.2f billion", float64(n)/1_000_000_000)
	}
}

// naturalsize formats a byte count as Bytes/kB/MB/GB.
func naturalsize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d Bytes", bytes)
	}
	kb := float64(bytes) / 1024
	if kb < 1024 {
		return fmt.Sprintf("%.1f kB", kb)
	}
	mb := kb / 1024
	if mb < 1024 {
		return fmt.Sprintf("%.1f MB", mb)
	}
	return fmt.Sprintf("%.1f GB", mb/1024)
}

// CodeTimeBadge renders the all-time coding time shield.
func (r *Renderer) CodeTimeBadge(total string) string {
	return badge("Code Time", total, r.Cfg.BadgeStyle)
}

// AICodeTimeBadge renders the all-time AI coding time shield.
func (r *Renderer) AICodeTimeBadge(text string) string {
	return badge("AI Code Time", text, r.Cfg.BadgeStyle)
}

// ProfileViewsBadge renders profile view count.
func (r *Renderer) ProfileViewsBadge(count int) string {
	return badge(r.t("Profile Views"), strconv.Itoa(count), r.Cfg.BadgeStyle)
}

// LinesOfCodeBadge renders lifetime additions.
func (r *Renderer) LinesOfCodeBadge(total int64) string {
	msg := intword(total) + " " + r.t("Lines of code")
	return badge(r.t("From Hello World I have written"), msg, r.Cfg.BadgeStyle)
}

// ShortGitHubInfo renders the GitHub facts block.
func (r *Renderer) ShortGitHubInfo(user githubx.User, contribYear int, contribTotal int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("**🐱 %s** \n\n", r.t("My GitHub Data")))

	size := "?"
	if user.DiskUsage >= 0 {
		size = naturalsize(user.DiskUsage)
	}
	b.WriteString(fmt.Sprintf("> 📦 %s \n > \n", sprintf(r.t("Used in GitHub's Storage"), size)))

	if contribYear > 0 {
		b.WriteString(fmt.Sprintf("> 🏆 %s\n > \n", sprintf(r.t("Contributions in the year"), intcomma(contribTotal), strconv.Itoa(contribYear))))
	}

	if user.Hireable {
		b.WriteString(fmt.Sprintf("> 💼 %s\n > \n", r.t("Opted to Hire")))
	} else {
		b.WriteString(fmt.Sprintf("> 🚫 %s\n > \n", r.t("Not Opted to Hire")))
	}

	if user.PublicRepos == 1 {
		b.WriteString(fmt.Sprintf("> 📜 %s \n > \n", sprintf(r.t("public repository"), user.PublicRepos)))
	} else {
		b.WriteString(fmt.Sprintf("> 📜 %s \n > \n", sprintf(r.t("public repositories"), user.PublicRepos)))
	}

	priv := user.OwnedPrivateRepos
	if user.PublicRepos == 1 {
		b.WriteString(fmt.Sprintf("> 🔑 %s \n > \n", sprintf(r.t("private repository"), priv)))
	} else {
		b.WriteString(fmt.Sprintf("> 🔑 %s \n > \n", sprintf(r.t("private repositories"), priv)))
	}
	return b.String()
}

// listOrEmpty formats WakaTime items as a progress list, or a localized empty message.
func (r *Renderer) listOrEmpty(items []wakatime.StatItem, limit int) string {
	if len(items) == 0 {
		return r.t("No Activity Tracked This Week")
	}
	return MakeList(FromWaka(items), limit, true, r.Cfg.SymbolVersion)
}

// WeeklyWaka renders timezone, languages, editors, projects, and OS.
func (r *Renderer) WeeklyWaka(data wakatime.Data) string {
	cfg := r.Cfg
	if !(cfg.ShowTimezone || cfg.ShowLanguage || cfg.ShowEditors || cfg.ShowProjects || cfg.ShowOS) {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("📊 **%s** \n\n```text\n", r.t("This Week I Spend My Time On")))
	if cfg.ShowTimezone {
		tz := data.Timezone
		b.WriteString(fmt.Sprintf("🕑︎ %s: %s\n\n", r.t("Timezone"), tz))
	}
	if cfg.ShowLanguage {
		b.WriteString(fmt.Sprintf("💬 %s: \n%s\n\n", r.t("Languages"), r.listOrEmpty(data.Languages, cfg.ShowLanguageCount)))
	}
	if cfg.ShowEditors {
		b.WriteString(fmt.Sprintf("🔥 %s: \n%s\n\n", r.t("Editors"), r.listOrEmpty(data.Editors, 5)))
	}
	if cfg.ShowProjects {
		b.WriteString(fmt.Sprintf("🐱‍💻 %s: \n%s\n\n", r.t("Projects"), r.listOrEmpty(data.Projects, 5)))
	}
	if cfg.ShowOS {
		b.WriteString(fmt.Sprintf("💻 %s: \n%s\n\n", r.t("operating system"), r.listOrEmpty(data.OperatingSystems, 5)))
	}
	s := strings.TrimSuffix(b.String(), "\n")
	return s + "```\n\n"
}

// AICodingStats renders the weekly AI coding section.
func (r *Renderer) AICodingStats(data wakatime.Data) string {
	ai := wakatime.FindCategory(data.Categories, "AI Coding")
	var b strings.Builder
	b.WriteString(fmt.Sprintf("🤖 **%s** \n\n```text\n", r.t("AI Coding This Week")))
	if ai == nil || data.AISessions == 0 {
		b.WriteString(r.t("No AI Coding Activity Tracked This Week") + "\n```\n\n")
		return b.String()
	}

	totalAdd := data.AIAdditions + data.HumanAdditions
	aiWritten := 0.0
	if totalAdd > 0 {
		aiWritten = float64(data.AIAdditions) / float64(totalAdd) * 100
	}
	totalChanges := data.AIAdditions + data.AIDeletions + data.HumanAdditions + data.HumanDeletions
	manual := 0.0
	if totalChanges > 0 {
		manual = float64(data.HumanAdditions+data.HumanDeletions) / float64(totalChanges) * 100
	}

	b.WriteString(fmt.Sprintf("⏱ %s: %s (%v%%)\n\n", r.t("AI Coding Time"), ai.Text, ai.Percent))
	b.WriteString(fmt.Sprintf("✍️ %s\n\n", sprintf(r.t("AI vs Human Lines"), intcomma(data.AIAdditions), intcomma(data.HumanAdditions), fmt.Sprintf("%.2f", aiWritten))))
	b.WriteString(fmt.Sprintf("🔤 %s\n\n", sprintf(r.t("AI Token Usage"), intcomma(data.AIInputTokens), intcomma(data.AIOutputTokens))))
	b.WriteString(fmt.Sprintf("💵 %s\n\n", sprintf(r.t("Estimated AI Cost"), fmt.Sprintf("%.2f", data.AIModelTotalCost))))
	b.WriteString(fmt.Sprintf("🧠 %s\n\n", sprintf(r.t("AI Sessions and Prompts"), strconv.Itoa(data.AISessions), strconv.Itoa(data.AIPromptEventsTotal))))

	if len(data.AIModelBreakdown) > 0 {
		totalLines := 0
		for _, m := range data.AIModelBreakdown {
			totalLines += m.Lines
		}
		if totalLines == 0 {
			totalLines = 1
		}
		items := make([]Item, 0, len(data.AIModelBreakdown))
		for _, m := range data.AIModelBreakdown {
			items = append(items, Item{
				Name:    m.Name,
				Text:    fmt.Sprintf("%s lines", intcomma(m.Lines)),
				Percent: math.Round(float64(m.Lines)/float64(totalLines)*10000) / 100,
			})
		}
		b.WriteString(MakeList(items, 5, true, r.Cfg.SymbolVersion) + "\n\n")
	}

	b.WriteString(r.aiInsights(aiWritten, data.AIPromptLengthAvg, data.AIPromptEventsAvgPerSession, manual) + "\n")
	s := strings.TrimSuffix(b.String(), "\n")
	return s + "```\n\n"
}

// aiInsights builds localized reliance/prompt/session insight lines from AI metrics.
func (r *Renderer) aiInsights(aiWritten, promptLen, promptsPerSession, manual float64) string {
	reliance := r.t("AI Reliance: Hands-On")
	if aiWritten >= 66 {
		reliance = r.t("AI Reliance: AI-Driven")
	} else if aiWritten >= 33 {
		reliance = r.t("AI Reliance: Balanced")
	}
	promptStyle := r.t("Prompt Style: Concise")
	if promptLen > 1500 {
		promptStyle = r.t("Prompt Style: Verbose")
	} else if promptLen >= 500 {
		promptStyle = r.t("Prompt Style: Detailed")
	}
	session := r.t("Session Style: One-Shot")
	if promptsPerSession > 1.5 {
		session = r.t("Session Style: Iterative")
	}
	review := r.t("Review Style: High AI Trust")
	if manual >= 50 {
		review = r.t("Review Style: Hands-On Reviewer")
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("🔎 %s:\n", r.t("AI Coding Insights")))
	b.WriteString(sprintf(r.t("AI Reliance Detail"), reliance, fmt.Sprintf("%.2f", aiWritten)) + "\n")
	b.WriteString(sprintf(r.t("Prompt Style Detail"), promptStyle, intcomma(int(math.Round(promptLen)))) + "\n")
	b.WriteString(sprintf(r.t("Session Style Detail"), session, fmt.Sprintf("%.1f", promptsPerSession)) + "\n")
	b.WriteString(sprintf(r.t("Review Style Detail"), review, fmt.Sprintf("%.2f", manual)) + "\n")
	return b.String()
}

// CommitDayTime renders early/night owl and weekday productivity lists.
func (r *Renderer) CommitDayTime(dayTimes [4]int, weekDays [7]int) string {
	var b strings.Builder
	sumDay := 0
	for _, v := range dayTimes {
		sumDay += v
	}
	sumWeek := 0
	for _, v := range weekDays {
		sumWeek += v
	}

	// Rotate so Morning is first: original [0-6,6-12,12-18,18-24] -> [6-12,12-18,18-24,0-6]
	rotated := [4]int{dayTimes[1], dayTimes[2], dayTimes[3], dayTimes[0]}

	if r.Cfg.ShowCommit {
		items := make([]Item, 4)
		for i := 0; i < 4; i++ {
			pct := 0.0
			if sumDay > 0 {
				pct = math.Round(float64(rotated[i])/float64(sumDay)*10000) / 100
			}
			items[i] = Item{
				Name:    dayTimeEmoji[i] + " " + r.t(dayTimeNames[i]),
				Text:    fmt.Sprintf("%d commits", rotated[i]),
				Percent: pct,
			}
		}
		title := r.t("I am a Night")
		if rotated[0]+rotated[1] >= rotated[2]+rotated[3] {
			title = r.t("I am an Early")
		}
		b.WriteString(fmt.Sprintf("**%s** \n\n```text\n%s\n```\n", title, MakeList(items, 7, false, r.Cfg.SymbolVersion)))
	}

	if r.Cfg.ShowDaysOfWeek {
		items := make([]Item, 7)
		maxIdx := 0
		maxPct := -1.0
		for i := 0; i < 7; i++ {
			pct := 0.0
			if sumWeek > 0 {
				pct = math.Round(float64(weekDays[i])/float64(sumWeek)*10000) / 100
			}
			items[i] = Item{
				Name:    r.t(weekDayNames[i]),
				Text:    fmt.Sprintf("%d commits", weekDays[i]),
				Percent: pct,
			}
			if pct > maxPct {
				maxPct = pct
				maxIdx = i
			}
		}
		title := sprintf(r.t("I am Most Productive on"), r.t(weekDayNames[maxIdx]))
		b.WriteString(fmt.Sprintf("📅 **%s** \n\n```text\n%s\n```\n", title, MakeList(items, 7, false, r.Cfg.SymbolVersion)))
	}
	s := b.String()
	if s != "" {
		s += "\n"
	}
	return s
}

// LanguagePerRepo renders repository language distribution.
func (r *Renderer) LanguagePerRepo(repos []githubx.Repository) string {
	ignored := map[string]struct{}{}
	for _, name := range r.Cfg.IgnoredRepos {
		ignored[name] = struct{}{}
	}
	counts := map[string]int{}
	withLang := 0
	for _, repo := range repos {
		if repo.PrimaryLanguage == "" {
			continue
		}
		if _, skip := ignored[repo.Name]; skip {
			continue
		}
		withLang++
		counts[repo.PrimaryLanguage]++
	}
	if withLang == 0 {
		return ""
	}
	items := make([]Item, 0, len(counts))
	topLang := ""
	topCount := 0
	for lang, n := range counts {
		label := "repos"
		if n == 1 {
			label = "repo"
		}
		items = append(items, Item{
			Name:    lang,
			Text:    fmt.Sprintf("%d %s", n, label),
			Percent: math.Round(float64(n)/float64(withLang)*10000) / 100,
		})
		if n > topCount {
			topCount = n
			topLang = lang
		}
	}
	title := sprintf(r.t("I Mostly Code in"), topLang)
	list := MakeList(items, 5, true, r.Cfg.SymbolVersion)
	return fmt.Sprintf("**%s** \n\n```text\n%s\n```\n\n", title, list)
}

// TimelineImage returns markdown for the committed PNG chart.
func (r *Renderer) TimelineImage(imageURL string) string {
	return fmt.Sprintf("**%s**\n\n![Lines of Code chart](%s)\n\n", r.t("Timeline"), imageURL)
}

// UpdatedDate appends the last-updated footer.
func (r *Renderer) UpdatedDate() string {
	if !r.Cfg.ShowUpdatedDate {
		return ""
	}
	return fmt.Sprintf("\n Last Updated on %s UTC", r.Now.UTC().Format(r.Cfg.UpdatedDateFormat))
}
