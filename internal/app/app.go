// Package app orchestrates stats collection, rendering, and README updates.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/vihuvac/read-waka-stats/internal/chart"
	"github.com/vihuvac/read-waka-stats/internal/commits"
	"github.com/vihuvac/read-waka-stats/internal/config"
	"github.com/vihuvac/read-waka-stats/internal/githubx"
	"github.com/vihuvac/read-waka-stats/internal/gitops"
	"github.com/vihuvac/read-waka-stats/internal/httpx"
	"github.com/vihuvac/read-waka-stats/internal/i18n"
	"github.com/vihuvac/read-waka-stats/internal/logging"
	"github.com/vihuvac/read-waka-stats/internal/readme"
	"github.com/vihuvac/read-waka-stats/internal/render"
	"github.com/vihuvac/read-waka-stats/internal/wakatime"
)

// App is the action runtime.
type App struct {
	Cfg  *config.Config
	Log  *logging.Logger
	HTTP *httpx.Client
	Now  time.Time
}

// New constructs the application.
func New(cfg *config.Config, log *logging.Logger) *App {
	return &App{
		Cfg:  cfg,
		Log:  log,
		HTTP: httpx.New(60 * time.Second),
		Now:  time.Now().UTC(),
	}
}

// Run collects stats and updates the profile README.
func (a *App) Run(ctx context.Context) error {
	bundle, err := i18n.Load(a.Cfg.Locale)
	if err != nil {
		return err
	}
	gh := &githubx.Client{HTTP: a.HTTP, Token: a.Cfg.GHToken, Log: a.Log}
	user, err := gh.FetchUser(ctx, a.Cfg.GHUser)
	if err != nil {
		return fmt.Errorf("github user: %w", err)
	}
	a.Log.Info("Current user: %s", user.Login)

	waka := &wakatime.Client{
		HTTP:    a.HTTP,
		APIURL:  a.Cfg.WakaTimeAPIURL,
		APIKey:  a.Cfg.WakaTimeAPIKey,
		Mock:    a.Cfg.MockWakaTime,
		MockDir: a.Cfg.MockDataDir,
		Log:     a.Log,
	}

	rnd := render.Renderer{Cfg: a.Cfg, T: bundle, Now: a.Now}
	var stats strings.Builder

	var repos []githubx.Repository
	if a.Cfg.NeedsRepos() {
		a.Log.Info("Fetching repositories")
		repos, err = gh.FetchRepositories(ctx, user.Login, a.Cfg.MaxRepos)
		if err != nil {
			return fmt.Errorf("repositories: %w", err)
		}
		a.Log.Info("Fetched %d repositories", len(repos))
	}

	var commitResult commits.Result
	if a.Cfg.NeedsCommitData() {
		calc := &commits.Calculator{
			GH:       gh,
			AuthorID: user.NodeID,
			Ignored:  commits.IgnoreSet(a.Cfg.IgnoredRepos),
			Log:      a.Log,
			Sleep:    400 * time.Millisecond,
		}
		if a.Cfg.DebugRun {
			calc.Sleep = 0
		}
		commitResult, err = calc.Calculate(ctx, repos)
		if err != nil {
			a.Log.Warn("commit aggregation failed: %v", err)
		}
	}

	if a.Cfg.ShowTotalCodeTime || a.Cfg.ShowAICodeTime {
		all, err := waka.FetchAllTime(ctx)
		if err != nil {
			a.Log.Warn("WakaTime all-time unavailable: %v", err)
		} else if all != nil {
			if a.Cfg.ShowTotalCodeTime && all.Data.HumanReadableTotal != "" {
				stats.WriteString(rnd.CodeTimeBadge(all.Data.HumanReadableTotal))
			}
			if a.Cfg.ShowAICodeTime {
				if cat := wakatime.FindCategory(all.Data.Categories, "AI Coding"); cat != nil {
					stats.WriteString(rnd.AICodeTimeBadge(cat.Text))
				} else {
					a.Log.Warn("No all-time AI coding data, skipping badge")
				}
			}
		}
	}

	var gitRepo *gitops.Repository
	readmePath := "README.md"
	defaultBranch := "main"
	if !a.Cfg.DebugRun {
		meta, err := fetchRepoMeta(ctx, a.HTTP, a.Cfg.GHToken, user.Login)
		if err != nil {
			a.Log.Warn("repo metadata: %v", err)
		} else {
			readmePath = meta.Readme
			defaultBranch = meta.DefaultBranch
		}
		authorName, authorEmail := a.commitIdentity(user.Login, user.Email)
		gitRepo, err = gitops.Clone(gitops.Options{
			Owner:         user.Login,
			Repo:          user.Login,
			Token:         a.Cfg.PushToken,
			WorkDir:       "repo",
			PullBranch:    a.Cfg.PullBranchName,
			PushBranch:    a.Cfg.PushBranchName,
			DefaultBranch: defaultBranch,
			CommitSingle:  a.Cfg.CommitSingle,
			CommitMessage: a.Cfg.CommitMessage,
			AuthorName:    authorName,
			AuthorEmail:   authorEmail,
			Log:           a.Log,
		})
		if err != nil {
			return fmt.Errorf("clone profile repo: %w", err)
		}
	}

	if a.Cfg.ShowProfileViews {
		if a.Cfg.DebugRun {
			a.Log.Warn("Profile views skipped in debug mode")
		} else {
			count, err := gh.FetchProfileViews(ctx, user.Login+"/"+user.Login)
			if err != nil {
				a.Log.Warn("Profile views unavailable: %v", err)
				count = 0
			}
			stats.WriteString(rnd.ProfileViewsBadge(count))
		}
	}

	if a.Cfg.ShowLinesOfCode {
		stats.WriteString(rnd.LinesOfCodeBadge(commits.TotalAdditions(commitResult.Yearly)))
	}

	if a.Cfg.ShowShortInfo {
		year, total := 0, 0
		years, err := gh.FetchContributions(ctx, user.Login)
		if err != nil {
			a.Log.Warn("GitHub contributions unavailable: %v", err)
		} else if len(years) > 0 {
			year, total = years[0].Year, years[0].Total
		}
		stats.WriteString(rnd.ShortGitHubInfo(user, year, total))
	}

	weekly, err := waka.FetchWeekly(ctx)
	if err != nil {
		a.Log.Warn("WakaTime weekly unavailable: %v", err)
	} else if weekly != nil {
		tz := weekly.Data.Timezone
		if tz == "" {
			tz = "UTC"
		}
		if a.Cfg.ShowCommit || a.Cfg.ShowDaysOfWeek {
			day, week := commits.CountDayParts(commitResult.Dates, tz)
			stats.WriteString(rnd.CommitDayTime(day, week))
		}
		stats.WriteString(rnd.WeeklyWaka(weekly.Data))
		if a.Cfg.ShowAICoding {
			stats.WriteString(rnd.AICodingStats(weekly.Data))
		}
	}

	if a.Cfg.ShowLanguageRepo {
		stats.WriteString(rnd.LanguagePerRepo(repos))
	}

	if a.Cfg.ShowLOCChart {
		chartRel := chart.Path
		chartDest := chartRel
		if gitRepo != nil {
			chartDest = gitRepo.Path(chartRel)
		}
		colors, err := gh.FetchLinguistColors(ctx)
		if err != nil {
			a.Log.Warn("linguist colors unavailable: %v", err)
			colors = githubx.LinguistColors{}
		}
		if err := chart.Draw(commitResult.Yearly, colors, chartDest); err != nil {
			a.Log.Warn("chart: %v", err)
		} else if gitRepo != nil {
			if err := gitRepo.Add(chartRel); err != nil {
				a.Log.Warn("staging chart: %v", err)
			}
			branch := a.Cfg.PushBranchName
			if branch == "" {
				branch = defaultBranch
			}
			imageURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", user.Login, user.Login, branch, chartRel)
			stats.WriteString(rnd.TimelineImage(imageURL))
		} else {
			stats.WriteString(rnd.TimelineImage(chartRel))
		}
	}

	stats.WriteString(rnd.UpdatedDate())
	content := stats.String()

	if a.Cfg.DebugRun {
		return writeOutput(content)
	}

	fullReadme := gitRepo.Path(readmePath)
	if err := readme.UpdateFile(fullReadme, a.Cfg.SectionName, content); err != nil {
		return err
	}
	if err := gitRepo.Add(readmePath); err != nil {
		return err
	}
	return gitRepo.CommitAndPush()
}

type repoMeta struct {
	DefaultBranch string
	Readme        string
}

func fetchRepoMeta(ctx context.Context, httpc *httpx.Client, token, login string) (repoMeta, error) {
	meta := repoMeta{DefaultBranch: "main", Readme: "README.md"}
	headers := map[string]string{
		"Authorization": "Bearer " + token,
		"Accept":        "application/vnd.github+json",
		"User-Agent":    "read-waka-stats",
	}
	body, status, err := httpc.GetJSON(ctx, fmt.Sprintf("https://api.github.com/repos/%s/%s", login, login), headers)
	if err != nil {
		return meta, err
	}
	if status != 200 {
		return meta, fmt.Errorf("HTTP %d", status)
	}
	var repo struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal(body, &repo); err == nil && repo.DefaultBranch != "" {
		meta.DefaultBranch = repo.DefaultBranch
	}
	body, status, err = httpc.GetJSON(ctx, fmt.Sprintf("https://api.github.com/repos/%s/%s/readme", login, login), headers)
	if err == nil && status == 200 {
		var file struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(body, &file); err == nil && file.Path != "" {
			meta.Readme = file.Path
		}
	}
	return meta, nil
}

func (a *App) commitIdentity(login, email string) (string, string) {
	if a.Cfg.CommitByMe {
		name := a.Cfg.CommitUsername
		if name == "" {
			name = login
		}
		mail := a.Cfg.CommitEmail
		if mail == "" {
			mail = email
		}
		if mail == "" {
			mail = "41898282+github-actions[bot]@users.noreply.github.com"
		}
		return name, mail
	}
	name := a.Cfg.CommitUsername
	if name == "" {
		name = "readme-bot"
	}
	mail := a.Cfg.CommitEmail
	if mail == "" {
		mail = "41898282+github-actions[bot]@users.noreply.github.com"
	}
	return name, mail
}

func writeOutput(stats string) error {
	path := os.Getenv("GITHUB_OUTPUT")
	if path == "" {
		fmt.Println(stats)
		return nil
	}
	eol := randomToken(10)
	block := fmt.Sprintf("README_CONTENT<<%s\nREADME stats current output:\n\n%s\n%s\n", eol, stats, eol)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(block)
	return err
}

func randomToken(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
