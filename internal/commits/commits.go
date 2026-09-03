// Package commits aggregates commit timestamps and lines of code by quarter.
package commits

import (
	"context"
	"strings"
	"time"

	"github.com/vihuvac/read-waka-stats/internal/githubx"
	"github.com/vihuvac/read-waka-stats/internal/logging"
)

// LangDelta is additions and deletions for a language.
type LangDelta struct {
	Add int
	Del int
}

// YearlyData is year -> quarter -> language -> delta.
type YearlyData map[int]map[int]map[string]LangDelta

// DateData is repo -> branch -> oid -> committedDate.
type DateData map[string]map[string]map[string]string

// Result is the aggregated commit dataset.
type Result struct {
	Yearly YearlyData
	Dates  DateData
}

// Calculator walks repositories and commits.
type Calculator struct {
	GH       *githubx.Client
	AuthorID string
	Ignored  map[string]struct{}
	Log      *logging.Logger
	Sleep    time.Duration
}

// Calculate inspects each repository's branches and commits.
func (c *Calculator) Calculate(ctx context.Context, repos []githubx.Repository) (Result, error) {
	res := Result{
		Yearly: YearlyData{},
		Dates:  DateData{},
	}
	for i, repo := range repos {
		if _, skip := c.Ignored[repo.Name]; skip {
			continue
		}
		if c.Log != nil {
			name := repo.Owner + "/" + repo.Name
			if repo.IsPrivate {
				name = "[private]"
			}
			c.Log.Info("%d/%d retrieving repo %s", i+1, len(repos), name)
		}
		if err := c.updateRepo(ctx, repo, res); err != nil {
			if c.Log != nil {
				c.Log.Warn("skipping repo %s/%s: %v", repo.Owner, repo.Name, err)
			}
			continue
		}
		if c.Sleep > 0 {
			select {
			case <-ctx.Done():
				return res, ctx.Err()
			case <-time.After(c.Sleep):
			}
		}
	}
	return res, nil
}

func (c *Calculator) updateRepo(ctx context.Context, repo githubx.Repository, res Result) error {
	branches, err := c.GH.FetchBranches(ctx, repo.Owner, repo.Name)
	if err != nil {
		return err
	}
	if len(branches) == 0 {
		return nil
	}
	for _, br := range branches {
		commits, err := c.GH.FetchCommits(ctx, repo.Owner, repo.Name, br.Name, c.AuthorID)
		if err != nil {
			if c.Log != nil {
				c.Log.Warn("skipping branch %s@%s: %v", repo.Name, br.Name, err)
			}
			continue
		}
		for _, cm := range commits {
			ts, err := time.Parse(time.RFC3339, cm.CommittedDate)
			if err != nil {
				ts, err = time.Parse("2006-01-02T15:04:05Z", cm.CommittedDate)
				if err != nil {
					continue
				}
			}
			year := ts.Year()
			quarter := (int(ts.Month())-1)/3 + 1

			if res.Dates[repo.Name] == nil {
				res.Dates[repo.Name] = map[string]map[string]string{}
			}
			if res.Dates[repo.Name][br.Name] == nil {
				res.Dates[repo.Name][br.Name] = map[string]string{}
			}
			res.Dates[repo.Name][br.Name][cm.OID] = cm.CommittedDate

			if repo.PrimaryLanguage == "" {
				continue
			}
			if res.Yearly[year] == nil {
				res.Yearly[year] = map[int]map[string]LangDelta{}
			}
			if res.Yearly[year][quarter] == nil {
				res.Yearly[year][quarter] = map[string]LangDelta{}
			}
			cur := res.Yearly[year][quarter][repo.PrimaryLanguage]
			cur.Add += cm.Additions
			cur.Del += cm.Deletions
			res.Yearly[year][quarter][repo.PrimaryLanguage] = cur
		}
	}
	return nil
}

// TotalAdditions sums all recorded additions.
func TotalAdditions(y YearlyData) int64 {
	var n int64
	for _, qs := range y {
		for _, langs := range qs {
			for _, d := range langs {
				n += int64(d.Add)
			}
		}
	}
	return n
}

// CountDayParts returns [night, morning, afternoon, evening] in the given IANA timezone.
// Index 0 is 00-06, 1 is 06-12, 2 is 12-18, 3 is 18-24.
func CountDayParts(dates DateData, tzName string) ([4]int, [7]int) {
	var dayTimes [4]int
	var weekDays [7]int
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		loc = time.UTC
	}
	for _, branches := range dates {
		for _, commits := range branches {
			for _, raw := range commits {
				ts, err := parseCommitTime(raw)
				if err != nil {
					continue
				}
				local := ts.In(loc)
				dayTimes[local.Hour()/6]++
				isoWeekday := (int(local.Weekday()) + 6) % 7 // Monday=0 ... Sunday=6
				weekDays[isoWeekday]++
			}
		}
	}
	return dayTimes, weekDays
}

func parseCommitTime(raw string) (time.Time, error) {
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts, nil
	}
	return time.Parse("2006-01-02T15:04:05Z", raw)
}

// IgnoreSet builds a lookup from a slice of names.
func IgnoreSet(names []string) map[string]struct{} {
	m := map[string]struct{}{}
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n != "" {
			m[n] = struct{}{}
		}
	}
	return m
}
