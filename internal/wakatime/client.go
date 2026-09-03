// Package wakatime talks to the WakaTime REST API.
package wakatime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/vihuvac/read-waka-stats/internal/httpx"
	"github.com/vihuvac/read-waka-stats/internal/logging"
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
}

// Client fetches WakaTime statistics.
type Client struct {
	HTTP    *httpx.Client
	APIURL  string
	APIKey  string
	Mock    bool
	MockDir string
	Log     *logging.Logger
}

// FetchWeekly returns last-7-days stats.
func (c *Client) FetchWeekly(ctx context.Context) (*Stats, error) {
	return c.fetch(ctx, "users/current/stats/last_7_days", "wakatime_stats.json")
}

// FetchAllTime returns all-time stats.
func (c *Client) FetchAllTime(ctx context.Context) (*Stats, error) {
	return c.fetch(ctx, "users/current/stats/all_time", "wakatime_all_time.json")
}

func (c *Client) fetch(ctx context.Context, path, mockFile string) (*Stats, error) {
	if c.Mock {
		return loadMock(c.MockDir, mockFile)
	}
	u, err := url.Parse(c.APIURL + path)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("api_key", c.APIKey)
	u.RawQuery = q.Encode()

	body, status, err := c.HTTP.GetJSON(ctx, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if status == 201 || status == 202 {
		if c.Log != nil {
			c.Log.Warn("WakaTime returned %d (stats still calculating)", status)
		}
		return nil, nil
	}
	if status != 200 {
		return nil, fmt.Errorf("WakaTime %s returned HTTP %d: %s", path, status, truncate(body, 300))
	}
	var stats Stats
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, fmt.Errorf("decode WakaTime stats: %w", err)
	}
	return &stats, nil
}

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

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) <= n {
		return s
	}
	return s[:n]
}
