// Package wakatime talks to the WakaTime REST API.
package wakatime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/vihuvac/read-waka-stats/internal/httpx"
	"github.com/vihuvac/read-waka-stats/internal/logging"
)

const (
	defaultMaxAttempts = 5
	defaultRetryWait   = 5 * time.Second
)

// StatItem is a named measure with duration and percentage.
type StatItem struct {
	Name         string  `json:"name"`
	TotalSeconds float64 `json:"total_seconds"`
	Text         string  `json:"text"`
	Percent      float64 `json:"percent"`
}

// Category is a WakaTime time category (e.g. AI Coding).
type Category = StatItem

// ModelBreakdown is per-model AI line counts.
type ModelBreakdown struct {
	Name  string `json:"name"`
	Lines int    `json:"lines"`
}

// Stats is a WakaTime stats payload (weekly or all-time).
type Stats struct {
	Data Data `json:"data"`
}

// Data is the inner WakaTime stats object.
type Data struct {
	Timezone                    string           `json:"timezone"`
	HumanReadableTotal          string           `json:"human_readable_total"`
	Languages                   []StatItem       `json:"languages"`
	Editors                     []StatItem       `json:"editors"`
	OperatingSystems            []StatItem       `json:"operating_systems"`
	Projects                    []StatItem       `json:"projects"`
	Categories                  []Category       `json:"categories"`
	AIAdditions                 int              `json:"ai_additions"`
	AIDeletions                 int              `json:"ai_deletions"`
	HumanAdditions              int              `json:"human_additions"`
	HumanDeletions              int              `json:"human_deletions"`
	AIModelTotalCost            float64          `json:"ai_model_total_cost"`
	AIInputTokens               int              `json:"ai_input_tokens"`
	AIOutputTokens              int              `json:"ai_output_tokens"`
	AIPromptLengthAvg           float64          `json:"ai_prompt_length_avg"`
	AIPromptEventsTotal         int              `json:"ai_prompt_events_total"`
	AIPromptEventsAvgPerSession float64          `json:"ai_prompt_events_avg_per_session"`
	AISessions                  int              `json:"ai_sessions"`
	AIModelBreakdown            []ModelBreakdown `json:"ai_model_breakdown"`
	// IsUpToDate is nil when the field is absent from the payload.
	IsUpToDate *bool `json:"is_up_to_date"`
}

// Client fetches WakaTime statistics.
type Client struct {
	HTTP    *httpx.Client
	APIURL  string
	APIKey  string
	Mock    bool
	MockDir string
	Log     *logging.Logger
	// MaxAttempts is how many times to poll when stats are still calculating (default 5).
	MaxAttempts int
	// RetryWait is the pause between calculating retries (default 5s).
	RetryWait time.Duration
	// Sleep overrides time.Sleep for tests.
	Sleep func(time.Duration)
}

// FetchWeekly returns last-7-days stats.
func (c *Client) FetchWeekly(ctx context.Context) (*Stats, error) {
	return c.fetch(ctx, "users/current/stats/last_7_days", "wakatime_stats.json")
}

// FetchAllTime returns all-time stats.
func (c *Client) FetchAllTime(ctx context.Context) (*Stats, error) {
	return c.fetch(ctx, "users/current/stats/all_time", "wakatime_all_time.json")
}

// fetch loads WakaTime stats from the API, or from a mock file when Mock is enabled.
// It retries briefly when WakaTime reports that stats are still calculating (HTTP 201/202
// or is_up_to_date=false). Exhausted retries return an error so the action fails loudly.
func (c *Client) fetch(ctx context.Context, path, mockFile string) (*Stats, error) {
	if c.Mock {
		return loadMock(c.MockDir, mockFile)
	}

	attempts, wait := c.retryPolicy()
	var lastStatus int
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		stats, status, retryable, err := c.fetchOnce(ctx, path)
		lastStatus = status
		if err != nil {
			return nil, err
		}
		if !retryable {
			return stats, nil
		}
		if c.Log != nil {
			c.Log.Warn("WakaTime stats not ready (HTTP %d), retry %d/%d", status, attempt, attempts)
		}
		if attempt == attempts {
			break
		}
		c.sleep(wait)
	}
	return nil, fmt.Errorf("WakaTime %s still calculating after %d attempts (last HTTP %d)", path, attempts, lastStatus)
}

func (c *Client) retryPolicy() (attempts int, wait time.Duration) {
	attempts = c.MaxAttempts
	if attempts < 1 {
		attempts = defaultMaxAttempts
	}
	wait = c.RetryWait
	if wait == 0 {
		wait = defaultRetryWait
	}
	return attempts, wait
}

func (c *Client) sleep(d time.Duration) {
	if c.Sleep != nil {
		c.Sleep(d)
		return
	}
	if d > 0 {
		time.Sleep(d)
	}
}

// fetchOnce performs a single stats request. retryable is true when WakaTime is still calculating.
func (c *Client) fetchOnce(ctx context.Context, path string) (*Stats, int, bool, error) {
	u, err := url.Parse(c.APIURL + path)
	if err != nil {
		return nil, 0, false, err
	}
	q := u.Query()
	q.Set("api_key", c.APIKey)
	u.RawQuery = q.Encode()

	body, status, err := c.HTTP.GetJSON(ctx, u.String(), nil)
	if err != nil {
		return nil, status, false, err
	}
	if status == 201 || status == 202 {
		return nil, status, true, nil
	}
	if status != 200 {
		return nil, status, false, fmt.Errorf("WakaTime %s returned HTTP %d: %s", path, status, truncate(body, 300))
	}
	var stats Stats
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, status, false, fmt.Errorf("decode WakaTime stats: %w", err)
	}
	if stats.Data.IsUpToDate != nil && !*stats.Data.IsUpToDate {
		return nil, status, true, nil
	}
	return &stats, status, false, nil
}

// loadMock reads and decodes a fixture stats JSON file from dir.
func loadMock(dir, file string) (*Stats, error) {
	data, err := os.ReadFile(filepath.Join(dir, file))
	if err != nil {
		return nil, err
	}
	var stats Stats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

// FindCategory returns the named category or nil.
func FindCategory(categories []Category, name string) *Category {
	for i := range categories {
		if categories[i].Name == name {
			return &categories[i]
		}
	}
	return nil
}

// truncate returns at most the first n bytes of b as a string.
func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) <= n {
		return s
	}
	return s[:n]
}
