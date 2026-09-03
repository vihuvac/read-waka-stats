// Package config loads and validates GitHub Action inputs from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/vihuvac/read-waka-stats/internal/logging"
)

// Config holds all runtime settings for the action.
type Config struct {
	GHToken           string
	PushToken         string
	GHUser            string
	WakaTimeAPIKey    string
	WakaTimeAPIURL    string
	SectionName       string
	PushBranchName    string
	ShowOS            bool
	ShowProjects      bool
	ShowEditors       bool
	ShowTimezone      bool
	ShowCommit        bool
	ShowLanguage      bool
	ShowLinesOfCode   bool
	ShowLanguageRepo  bool
	ShowLOCChart      bool
	ShowDaysOfWeek    bool
	ShowProfileViews  bool
	ShowShortInfo     bool
	ShowUpdatedDate   bool
	ShowTotalCodeTime bool
	ShowAICodeTime    bool
	ShowAICoding      bool
	CommitMessage     string
	Locale            string
	UpdatedDateFormat string
	IgnoredRepos      []string
	MaxRepos          int
	ShowLanguageCount int
	SymbolVersion     int
	BadgeStyle        string
	DebugLogging      bool
	DebugRun          bool
	MockWakaTime      bool
	MockDataDir       string
}

// Load reads INPUT_* environment variables and validates required fields.
func Load() (*Config, error) {
	cfg := &Config{
		GHToken:           strings.TrimSpace(os.Getenv("INPUT_GH_TOKEN")),
		PushToken:         strings.TrimSpace(os.Getenv("INPUT_PUSH_TOKEN")),
		GHUser:            strings.TrimSpace(os.Getenv("INPUT_GH_USER")),
		WakaTimeAPIKey:    os.Getenv("INPUT_WAKATIME_API_KEY"),
		WakaTimeAPIURL:    getenvDefault("INPUT_WAKATIME_API_URL", "https://wakatime.com/api/v1/"),
		SectionName:       getenvDefault("INPUT_SECTION_NAME", "waka"),
		PushBranchName:    os.Getenv("INPUT_PUSH_BRANCH_NAME"),
		ShowOS:            truthyDefault("INPUT_SHOW_OS", true),
		ShowProjects:      truthyDefault("INPUT_SHOW_PROJECTS", true),
		ShowEditors:       truthyDefault("INPUT_SHOW_EDITORS", true),
		ShowTimezone:      truthyDefault("INPUT_SHOW_TIMEZONE", true),
		ShowCommit:        truthyDefault("INPUT_SHOW_COMMIT", true),
		ShowLanguage:      truthyDefault("INPUT_SHOW_LANGUAGE", true),
		ShowLinesOfCode:   truthyDefault("INPUT_SHOW_LINES_OF_CODE", false),
		ShowLanguageRepo:  truthyDefault("INPUT_SHOW_LANGUAGE_PER_REPO", true),
		ShowLOCChart:      truthyDefault("INPUT_SHOW_LOC_CHART", true),
		ShowDaysOfWeek:    truthyDefault("INPUT_SHOW_DAYS_OF_WEEK", true),
		ShowProfileViews:  truthyDefault("INPUT_SHOW_PROFILE_VIEWS", true),
		ShowShortInfo:     truthyDefault("INPUT_SHOW_SHORT_INFO", true),
		ShowUpdatedDate:   truthyDefault("INPUT_SHOW_UPDATED_DATE", true),
		ShowTotalCodeTime: truthyDefault("INPUT_SHOW_TOTAL_CODE_TIME", true),
		ShowAICodeTime:    truthyDefault("INPUT_SHOW_AI_CODE_TIME", true),
		ShowAICoding:      truthyDefault("INPUT_SHOW_AI_CODING", true),
		CommitMessage:     getenvDefault("INPUT_COMMIT_MESSAGE", "Updated with Dev Metrics"),
		Locale:            getenvDefault("INPUT_LOCALE", "en"),
		UpdatedDateFormat: pythonToGoTime(getenvDefault("INPUT_UPDATED_DATE_FORMAT", "%d/%m/%Y %H:%M:%S")),
		IgnoredRepos:      parseList(os.Getenv("INPUT_IGNORED_REPOS")),
		MaxRepos:          parseIntDefault("INPUT_MAX_REPOS", 0),
		ShowLanguageCount: parseIntDefault("INPUT_SHOW_LANGUAGE_COUNT", 5),
		SymbolVersion:     parseIntDefault("INPUT_SYMBOL_VERSION", 1),
		BadgeStyle:        firstNonEmpty(os.Getenv("INPUT_BADGE_STYLE"), os.Getenv("BADGE_STYLE"), "flat"),
		DebugLogging:      truthyDefault("INPUT_DEBUG_LOGGING", false),
		DebugRun:          logging.ParseBoolTruth(os.Getenv("DEBUG_RUN")),
		MockWakaTime:      logging.ParseBoolTruth(os.Getenv("MOCK_WAKATIME")),
		MockDataDir:       getenvDefault("MOCK_DATA_DIR", "internal/testdata"),
	}

	if !strings.HasSuffix(cfg.WakaTimeAPIURL, "/") {
		cfg.WakaTimeAPIURL += "/"
	}
	if cfg.ShowLanguageCount < 1 {
		cfg.ShowLanguageCount = 1
	}
	if cfg.MaxRepos < 0 {
		cfg.MaxRepos = 0
	}
	if cfg.SymbolVersion < 1 || cfg.SymbolVersion > 3 {
		cfg.SymbolVersion = 1
	}

	if cfg.GHToken == "" {
		return nil, fmt.Errorf("missing required input: GH_TOKEN")
	}
	if !cfg.MockWakaTime && cfg.WakaTimeAPIKey == "" {
		return nil, fmt.Errorf("missing required input: WAKATIME_API_KEY")
	}
	if cfg.PushToken == "" {
		cfg.PushToken = cfg.GHToken
	}

	return cfg, nil
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func truthyDefault(key string, defaultVal bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return defaultVal
	}
	return logging.ParseBoolTruth(v)
}

func parseIntDefault(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func parseList(raw string) []string {
	raw = strings.ReplaceAll(raw, " ", "")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// pythonToGoTime converts common strftime tokens to Go reference layout.
func pythonToGoTime(layout string) string {
	replacer := strings.NewReplacer(
		"%Y", "2006",
		"%m", "01",
		"%d", "02",
		"%H", "15",
		"%M", "04",
		"%S", "05",
		"%y", "06",
	)
	return replacer.Replace(layout)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// NeedsCommitData reports whether commit history must be fetched.
func (c *Config) NeedsCommitData() bool {
	return c.ShowLinesOfCode || c.ShowLOCChart || c.ShowCommit || c.ShowDaysOfWeek
}

// NeedsRepos reports whether repository lists are required.
func (c *Config) NeedsRepos() bool {
	return c.NeedsCommitData() || c.ShowLanguageRepo || c.ShowCommit || c.ShowDaysOfWeek
}
