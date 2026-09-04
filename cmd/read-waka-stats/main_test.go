package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vihuvac/read-waka-stats/internal/logging"
)

func TestMain(m *testing.M) {
	logging.Output = io.Discard
	os.Exit(m.Run())
}

func TestRunMissingConfig(t *testing.T) {
	t.Setenv("INPUT_GH_TOKEN", "")
	t.Setenv("INPUT_WAKATIME_API_KEY", "")
	t.Setenv("MOCK_WAKATIME", "false")
	if code := run(); code != 1 {
		t.Fatalf("code=%d", code)
	}
}

func TestMainCallsExit(t *testing.T) {
	orig := exitFunc
	t.Cleanup(func() { exitFunc = orig })
	var code int
	exitFunc = func(c int) {
		code = c
		panic("exited")
	}
	t.Setenv("INPUT_GH_TOKEN", "")
	t.Setenv("INPUT_WAKATIME_API_KEY", "")
	t.Setenv("MOCK_WAKATIME", "false")
	defer func() {
		_ = recover()
		if code != 1 {
			t.Fatalf("code=%d", code)
		}
	}()
	main()
}

func TestRunSuccess(t *testing.T) {
	mockDir := filepath.Join("..", "..", "internal", "testdata")
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" || strings.HasPrefix(r.URL.Path, "/users/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"login": "alice", "node_id": "U"})
		case r.URL.Path == "/graphql":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"user": map[string]any{
				"repositories":              map[string]any{"nodes": []any{}, "pageInfo": map[string]any{"hasNextPage": false}},
				"repositoriesContributedTo": map[string]any{"nodes": []any{}, "pageInfo": map[string]any{"hasNextPage": false}},
			}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer gh.Close()

	t.Setenv("INPUT_GH_TOKEN", "t")
	t.Setenv("INPUT_WAKATIME_API_KEY", "k")
	t.Setenv("GITHUB_API_BASE", gh.URL)
	t.Setenv("DEBUG_RUN", "true")
	t.Setenv("MOCK_WAKATIME", "true")
	t.Setenv("MOCK_DATA_DIR", mockDir)
	t.Setenv("INPUT_SHOW_LOC_CHART", "false")
	t.Setenv("INPUT_SHOW_LINES_OF_CODE", "false")
	t.Setenv("INPUT_SHOW_COMMIT", "false")
	t.Setenv("INPUT_SHOW_DAYS_OF_WEEK", "false")
	t.Setenv("INPUT_SHOW_LANGUAGE_PER_REPO", "false")
	t.Setenv("INPUT_SHOW_PROFILE_VIEWS", "false")
	t.Setenv("INPUT_SHOW_SHORT_INFO", "false")
	t.Setenv("INPUT_SHOW_TOTAL_CODE_TIME", "false")
	t.Setenv("INPUT_SHOW_AI_CODE_TIME", "false")
	t.Setenv("INPUT_SHOW_AI_CODING", "false")
	t.Setenv("INPUT_SHOW_LANGUAGE", "false")
	t.Setenv("INPUT_SHOW_EDITORS", "false")
	t.Setenv("INPUT_SHOW_PROJECTS", "false")
	t.Setenv("INPUT_SHOW_OS", "false")
	t.Setenv("INPUT_SHOW_TIMEZONE", "false")
	t.Setenv("GITHUB_OUTPUT", filepath.Join(t.TempDir(), "out.txt"))

	if code := run(); code != 0 {
		t.Fatalf("code=%d", code)
	}
}

func TestRunAppError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	t.Setenv("INPUT_GH_TOKEN", "t")
	t.Setenv("INPUT_WAKATIME_API_KEY", "k")
	t.Setenv("GITHUB_API_BASE", srv.URL)
	t.Setenv("DEBUG_RUN", "true")
	t.Setenv("MOCK_WAKATIME", "true")
	t.Setenv("MOCK_DATA_DIR", filepath.Join("..", "..", "internal", "testdata"))
	if code := run(); code != 1 {
		t.Fatalf("code=%d", code)
	}
}
