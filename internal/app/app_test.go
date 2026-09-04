package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/vihuvac/read-waka-stats/internal/config"
	"github.com/vihuvac/read-waka-stats/internal/githubx"
	"github.com/vihuvac/read-waka-stats/internal/httpx"
	"github.com/vihuvac/read-waka-stats/internal/logging"
)

func TestMain(m *testing.M) {
	logging.Output = io.Discard
	os.Exit(m.Run())
}

func TestNew(t *testing.T) {
	a := New(&config.Config{GHToken: "t"}, logging.New(false))
	if a.HTTP == nil || a.Now.IsZero() {
		t.Fatal("expected initialized app")
	}
}

func TestWriteOutputStdout(t *testing.T) {
	t.Setenv("GITHUB_OUTPUT", "")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	err = writeOutput("printed")
	_ = w.Close()
	os.Stdout = orig
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "printed") {
		t.Fatalf("%q", data)
	}
}

func TestWriteOutputFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	t.Setenv("GITHUB_OUTPUT", path)
	if err := writeOutput("hello stats"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "hello stats") || !strings.Contains(s, "README_CONTENT<<") {
		t.Fatalf("output: %s", s)
	}
}

func TestWriteOutputBadPath(t *testing.T) {
	t.Setenv("GITHUB_OUTPUT", filepath.Join(t.TempDir(), "no", "such", "dir", "out"))
	if err := writeOutput("x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestStaleHeadRetryErrorWraps(t *testing.T) {
	err := fmt.Errorf("publish failed after %d attempts: %w", maxPublishAttempts, githubx.ErrStaleHead)
	if !errors.Is(err, githubx.ErrStaleHead) {
		t.Fatalf("expected ErrStaleHead, got %v", err)
	}
}

func TestFetchRepoMeta(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/readme"):
			_ = json.NewEncoder(w).Encode(map[string]string{"path": "profile/README.md"})
		case strings.Contains(r.URL.Path, "/repos/"):
			_ = json.NewEncoder(w).Encode(map[string]string{"default_branch": "develop"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	meta, err := fetchRepoMeta(context.Background(), httpx.New(5*time.Second), "tok", "alice", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.DefaultBranch != "develop" || meta.Readme != "profile/README.md" {
		t.Fatalf("%+v", meta)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer bad.Close()
	if _, err := fetchRepoMeta(context.Background(), httpx.New(5*time.Second), "tok", "alice", bad.URL); err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchRepoMetaPartialReadmeFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/readme") {
			w.WriteHeader(404)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"default_branch": "trunk"})
	}))
	defer srv.Close()
	meta, err := fetchRepoMeta(context.Background(), httpx.New(2*time.Second), "t", "alice", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.DefaultBranch != "trunk" || meta.Readme != "README.md" {
		t.Fatalf("%+v", meta)
	}
}

func TestFetchRepoMetaNetworkAndBadJSON(t *testing.T) {
	c := &httpx.Client{HTTP: &http.Client{Timeout: 50 * time.Millisecond}, Retries: 1}
	if _, err := fetchRepoMeta(context.Background(), c, "t", "alice", "http://127.0.0.1:1"); err == nil {
		t.Fatal("expected network error")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{"))
	}))
	defer srv.Close()
	meta, err := fetchRepoMeta(context.Background(), httpx.New(2*time.Second), "t", "alice", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.DefaultBranch != "main" {
		t.Fatalf("%+v", meta)
	}
}

func TestRunFetchUserError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	a := New(&config.Config{GHToken: "t", Locale: "en", DebugRun: true, MockWakaTime: true, MockDataDir: filepath.Join("..", "testdata")}, logging.New(false))
	a.GitHubAPIBase = srv.URL
	a.HTTP = httpx.New(2 * time.Second)
	if err := a.Run(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunReposError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/graphql" {
			w.WriteHeader(400)
			_, _ = w.Write([]byte("bad"))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"login": "alice", "node_id": "U"})
	}))
	defer srv.Close()
	a := New(&config.Config{
		GHToken: "t", Locale: "en", DebugRun: true, MockWakaTime: true,
		MockDataDir: filepath.Join("..", "testdata"), ShowLanguageRepo: true,
	}, logging.New(false))
	a.GitHubAPIBase = srv.URL
	a.HTTP = httpx.New(2 * time.Second)
	if err := a.Run(context.Background()); err == nil {
		t.Fatal("expected repos error")
	}
}

func TestRunWakaAndWeeklyFailures(t *testing.T) {
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/graphql" {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"user": map[string]any{
				"repositories":              map[string]any{"nodes": []any{}, "pageInfo": map[string]any{"hasNextPage": false}},
				"repositoriesContributedTo": map[string]any{"nodes": []any{}, "pageInfo": map[string]any{"hasNextPage": false}},
			}}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"login": "alice", "node_id": "U"})
	}))
	defer gh.Close()

	dir := t.TempDir()
	cfg := &config.Config{
		GHToken: "t", Locale: "en", DebugRun: true, MockWakaTime: true, MockDataDir: dir,
		ShowTotalCodeTime: true, ShowAICodeTime: true, ShowLanguageRepo: false,
		ShowLOCChart: false, ShowCommit: false, ShowDaysOfWeek: false, ShowLinesOfCode: false,
		ShowProfileViews: false, ShowShortInfo: false, ShowAICoding: false,
		ShowLanguage: false, ShowEditors: false, ShowProjects: false, ShowOS: false, ShowTimezone: false,
		ShowUpdatedDate: false, BadgeStyle: "flat",
	}
	a := New(cfg, logging.New(false))
	a.GitHubAPIBase = gh.URL
	a.HTTP = httpx.New(2 * time.Second)
	t.Setenv("GITHUB_OUTPUT", filepath.Join(t.TempDir(), "o.txt"))
	if err := a.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRunDebugFull(t *testing.T) {
	mockDir := filepath.Join("..", "testdata")
	outPath := filepath.Join(t.TempDir(), "out.txt")
	t.Setenv("GITHUB_OUTPUT", outPath)

	gh := mockGitHubMux(t)

	cfg := &config.Config{
		GHToken:           "t",
		PushToken:         "t",
		GHUser:            "alice",
		SectionName:       "waka",
		Locale:            "en",
		BadgeStyle:        "flat",
		ShowTotalCodeTime: true,
		ShowAICodeTime:    true,
		ShowProfileViews:  true,
		ShowLinesOfCode:   true,
		ShowShortInfo:     true,
		ShowCommit:        true,
		ShowDaysOfWeek:    true,
		ShowLanguage:      true,
		ShowEditors:       true,
		ShowProjects:      true,
		ShowOS:            true,
		ShowTimezone:      true,
		ShowAICoding:      true,
		ShowLanguageRepo:  true,
		ShowLOCChart:      true,
		ShowUpdatedDate:   true,
		UpdatedDateFormat: "02/01/2006",
		ShowLanguageCount: 5,
		SymbolVersion:     1,
		MaxRepos:          5,
		DebugRun:          true,
		DebugLogging:      true,
		MockWakaTime:      true,
		MockDataDir:       mockDir,
		CommitMessage:     "test",
		IgnoredRepos:      []string{"ignored"},
	}
	a := New(cfg, logging.New(true))
	a.GitHubAPIBase = gh.URL
	a.HTTP = &httpx.Client{
		HTTP: &http.Client{
			Timeout: 10 * time.Second,
			Transport: hostRewriteTransport(map[string]string{
				"api.github.com":                  gh.URL,
				"github-contributions.vercel.app": gh.URL,
				"cdn.jsdelivr.net":                gh.URL,
			}),
		},
		Retries: 1,
	}
	a.Now = time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)

	if err := a.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "README_CONTENT<<") {
		t.Fatalf("%s", data)
	}
}

func TestRunWarnPathsAndNonDebugPublish(t *testing.T) {
	bare := filepath.Join(t.TempDir(), "bare.git")
	initBareWithREADME(t, bare, t.TempDir())

	mux := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" || strings.HasPrefix(r.URL.Path, "/users/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"login": "alice", "node_id": "U_1", "public_repos": 1})
		case strings.HasSuffix(r.URL.Path, "/readme"):
			_ = json.NewEncoder(w).Encode(map[string]string{"path": "README.md"})
		case strings.Contains(r.URL.Path, "/repos/alice/alice") && !strings.Contains(r.URL.Path, "traffic"):
			_ = json.NewEncoder(w).Encode(map[string]string{"default_branch": "master"})
		case strings.Contains(r.URL.Path, "/traffic/views"):
			w.WriteHeader(403)
		case strings.HasPrefix(r.URL.Path, "/api/v1/"):
			w.WriteHeader(500)
		case strings.Contains(r.URL.Path, "linguist"):
			w.WriteHeader(500)
		case r.URL.Path == "/graphql":
			body, _ := io.ReadAll(r.Body)
			q := string(body)
			if strings.Contains(q, "createCommitOnBranch") {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{"createCommitOnBranch": map[string]any{"commit": map[string]any{"oid": "x"}}},
				})
				return
			}
			if strings.Contains(q, "repositoriesContributedTo") {
				_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"user": map[string]any{
					"repositoriesContributedTo": map[string]any{"nodes": []any{}, "pageInfo": map[string]any{"hasNextPage": false}},
				}}})
				return
			}
			if strings.Contains(q, "affiliations") || strings.Contains(q, "repositories(") {
				_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"user": map[string]any{
					"repositories": map[string]any{"nodes": []any{}, "pageInfo": map[string]any{"hasNextPage": false}},
				}}})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer mux.Close()

	dir := t.TempDir()
	weekly := `{"data":{"timezone":"","languages":[],"editors":[],"operating_systems":[],"projects":[],"categories":[]}}`
	allTime := `{"data":{"human_readable_total":"1 hr","categories":[{"name":"Coding","text":"1 hr"}]}}`
	if err := os.WriteFile(filepath.Join(dir, "wakatime_stats.json"), []byte(weekly), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "wakatime_all_time.json"), []byte(allTime), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		GHToken: "t", PushToken: "t", GHUser: "alice", Locale: "en", SectionName: "waka",
		BadgeStyle: "flat", ShowTotalCodeTime: true, ShowAICodeTime: true, ShowProfileViews: true,
		ShowShortInfo: true, ShowLOCChart: true, ShowLanguageRepo: true, ShowUpdatedDate: true,
		UpdatedDateFormat: "02/01/2006", ShowLanguageCount: 5, SymbolVersion: 1, CommitMessage: "msg",
		MockWakaTime: true, MockDataDir: dir, DebugRun: false, PushBranchName: "master",
		ShowCommit: false, ShowDaysOfWeek: false, ShowLinesOfCode: false, ShowAICoding: false,
		ShowLanguage: true, ShowEditors: true, ShowProjects: true, ShowOS: true, ShowTimezone: true,
	}
	a := New(cfg, logging.New(true))
	a.GitHubAPIBase = mux.URL
	a.CloneURL = bare
	a.HTTP = &httpx.Client{
		HTTP: &http.Client{Timeout: 10 * time.Second, Transport: hostRewriteTransport(map[string]string{
			"api.github.com": mux.URL, "github-contributions.vercel.app": mux.URL, "cdn.jsdelivr.net": mux.URL,
		})},
		Retries: 1,
	}
	a.Now = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := a.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRunMetaWarn(t *testing.T) {
	bare := filepath.Join(t.TempDir(), "bare.git")
	initBareWithREADME(t, bare, t.TempDir())

	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" || r.URL.Path == "/users/alice":
			_ = json.NewEncoder(w).Encode(map[string]any{"login": "alice", "node_id": "U"})
		case r.URL.Path == "/repos/alice/alice":
			w.WriteHeader(500)
		case r.URL.Path == "/graphql":
			body := make([]byte, 1)
			_, _ = r.Body.Read(body)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"createCommitOnBranch": map[string]any{"commit": map[string]any{"oid": "1"}}},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{})
		}
	}))
	defer gh.Close()

	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "wakatime_stats.json"), []byte(`{"data":{}}`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "wakatime_all_time.json"), []byte(`{"data":{}}`), 0o644)

	a := New(&config.Config{
		GHToken: "t", PushToken: "t", GHUser: "alice", Locale: "en", SectionName: "waka",
		MockWakaTime: true, MockDataDir: dir, DebugRun: false, CommitMessage: "m",
		ShowLOCChart: false, ShowLanguageRepo: false, ShowCommit: false, ShowDaysOfWeek: false,
		ShowLinesOfCode: false, ShowProfileViews: false, ShowShortInfo: false,
		ShowTotalCodeTime: false, ShowAICodeTime: false, ShowAICoding: false,
		ShowLanguage: false, ShowEditors: false, ShowProjects: false, ShowOS: false, ShowTimezone: false,
		ShowUpdatedDate: false, BadgeStyle: "flat", PushBranchName: "master",
	}, logging.New(false))
	a.GitHubAPIBase = gh.URL
	a.CloneURL = bare
	a.HTTP = httpx.New(2 * time.Second)
	if err := a.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRunCommitAggregationCanceled(t *testing.T) {
	gh := mockGitHubMux(t)
	t.Cleanup(gh.Close)

	a := New(&config.Config{
		GHToken: "t", Locale: "en", DebugRun: false, MockWakaTime: true,
		MockDataDir: filepath.Join("..", "testdata"), ShowCommit: true, ShowDaysOfWeek: false,
		ShowLOCChart: false, ShowLanguageRepo: false, ShowLinesOfCode: false,
		ShowProfileViews: false, ShowShortInfo: false, ShowTotalCodeTime: false, ShowAICodeTime: false,
		ShowAICoding: false, ShowLanguage: false, ShowEditors: false, ShowProjects: false, ShowOS: false,
		ShowTimezone: false, ShowUpdatedDate: false, BadgeStyle: "flat", SectionName: "waka",
		PushToken: "t", CommitMessage: "m", PushBranchName: "master", MaxRepos: 1,
	}, logging.New(false))
	a.GitHubAPIBase = gh.URL
	a.HTTP = &httpx.Client{
		HTTP: &http.Client{Timeout: 5 * time.Second, Transport: hostRewriteTransport(map[string]string{
			"api.github.com": gh.URL, "github-contributions.vercel.app": gh.URL, "cdn.jsdelivr.net": gh.URL,
		})},
		Retries: 1,
	}
	a.CloneURL = filepath.Join(t.TempDir(), "missing.git")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(80 * time.Millisecond)
		cancel()
	}()
	_ = a.Run(ctx)
}

func TestPublishVerified(t *testing.T) {
	bare := filepath.Join(t.TempDir(), "bare.git")
	seed := t.TempDir()
	initBareWithREADME(t, bare, seed)

	var commits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" {
			http.NotFound(w, r)
			return
		}
		atomic.AddInt32(&commits, 1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"createCommitOnBranch": map[string]any{
					"commit": map[string]any{"oid": "newoid"},
				},
			},
		})
	}))
	defer srv.Close()

	a := New(&config.Config{
		PushToken:     "t",
		SectionName:   "waka",
		CommitMessage: "Updated with Dev Metrics",
	}, logging.New(false))
	a.GitHubAPIBase = srv.URL
	a.CloneURL = bare
	a.HTTP = httpx.New(5 * time.Second)

	pushGH := &githubx.Client{HTTP: a.HTTP, Token: a.Cfg.PushToken, Log: a.Log, APIBase: a.GitHubAPIBase}
	err := a.publishVerified(context.Background(), pushGH, "alice", "master", "master", "README.md", "fresh stats", []byte("png-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&commits) < 1 {
		t.Fatal("expected createCommit call")
	}
}

func TestPublishVerifiedCloneError(t *testing.T) {
	a := New(&config.Config{PushToken: "t", SectionName: "waka"}, logging.New(false))
	a.CloneURL = filepath.Join(t.TempDir(), "missing.git")
	pushGH := &githubx.Client{HTTP: httpx.New(time.Second), Token: "t"}
	err := a.publishVerified(context.Background(), pushGH, "alice", "main", "main", "README.md", "x", nil)
	if err == nil {
		t.Fatal("expected clone error")
	}
}

func TestPublishVerifiedReadmeError(t *testing.T) {
	bare := filepath.Join(t.TempDir(), "bare.git")
	initBareWithREADME(t, bare, t.TempDir())
	a := New(&config.Config{PushToken: "t", SectionName: "missing", CommitMessage: "m"}, logging.New(false))
	a.CloneURL = bare
	a.HTTP = httpx.New(2 * time.Second)
	pushGH := &githubx.Client{HTTP: a.HTTP, Token: "t", APIBase: "http://127.0.0.1:1"}
	if err := a.publishVerified(context.Background(), pushGH, "alice", "master", "master", "README.md", "x", nil); err == nil {
		t.Fatal("expected readme marker error")
	}
}

func TestPublishVerifiedNonStaleError(t *testing.T) {
	bare := filepath.Join(t.TempDir(), "bare.git")
	initBareWithREADME(t, bare, t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"message": "permission denied"}},
		})
	}))
	defer srv.Close()
	a := New(&config.Config{PushToken: "t", SectionName: "waka", CommitMessage: "m"}, logging.New(false))
	a.CloneURL = bare
	a.GitHubAPIBase = srv.URL
	a.HTTP = httpx.New(2 * time.Second)
	pushGH := &githubx.Client{HTTP: a.HTTP, Token: "t", APIBase: srv.URL}
	if err := a.publishVerified(context.Background(), pushGH, "alice", "master", "master", "README.md", "changed", nil); err == nil {
		t.Fatal("expected publish error")
	}
}

func TestPublishVerifiedStaleRetryThenFail(t *testing.T) {
	bare := filepath.Join(t.TempDir(), "bare.git")
	initBareWithREADME(t, bare, t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"message": "Expected head oid does not match"}},
		})
	}))
	defer srv.Close()

	a := New(&config.Config{PushToken: "t", SectionName: "waka", CommitMessage: "m"}, logging.New(false))
	a.CloneURL = bare
	a.GitHubAPIBase = srv.URL
	a.HTTP = httpx.New(5 * time.Second)
	pushGH := &githubx.Client{HTTP: a.HTTP, Token: "t", APIBase: srv.URL}
	err := a.publishVerified(context.Background(), pushGH, "alice", "master", "master", "README.md", "new", nil)
	if err == nil || !strings.Contains(err.Error(), "publish failed after") {
		t.Fatalf("got %v", err)
	}
}

func mockGitHubMux(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" || strings.HasPrefix(r.URL.Path, "/users/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"login": "alice", "node_id": "U_1", "public_repos": 2,
				"disk_usage": 10, "hireable": true, "owned_private_repos": 1,
			})
		case strings.Contains(r.URL.Path, "/traffic/views"):
			_ = json.NewEncoder(w).Encode(map[string]any{"count": 7})
		case strings.HasPrefix(r.URL.Path, "/api/v1/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"years": []map[string]any{{"year": 2026, "total": 11}},
			})
		case strings.Contains(r.URL.Path, "linguist"):
			_, _ = w.Write([]byte("Go:\n  color: \"#00ADD8\"\n"))
		case r.URL.Path == "/graphql":
			body, _ := io.ReadAll(r.Body)
			q := string(body)
			switch {
			case strings.Contains(q, "repositoriesContributedTo"):
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{"user": map[string]any{
						"repositoriesContributedTo": map[string]any{
							"nodes":    []any{},
							"pageInfo": map[string]any{"hasNextPage": false},
						},
					}},
				})
			case strings.Contains(q, "repositories(") || strings.Contains(q, "affiliations"):
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{"user": map[string]any{
						"repositories": map[string]any{
							"nodes": []map[string]any{
								{"name": "app", "isPrivate": false, "owner": map[string]string{"login": "alice"}, "primaryLanguage": map[string]string{"name": "Go"}},
							},
							"pageInfo": map[string]any{"hasNextPage": false},
						},
					}},
				})
			case strings.Contains(q, "refPrefix"):
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{"repository": map[string]any{
						"refs": map[string]any{
							"nodes":    []map[string]string{{"name": "main"}},
							"pageInfo": map[string]any{"hasNextPage": false},
						},
					}},
				})
			case strings.Contains(q, "history"):
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{"repository": map[string]any{
						"ref": map[string]any{"target": map[string]any{
							"history": map[string]any{
								"nodes": []map[string]any{
									{"additions": 4, "deletions": 1, "committedDate": "2026-02-01T10:00:00Z", "oid": "c1"},
								},
								"pageInfo": map[string]any{"hasNextPage": false},
							},
						}},
					}},
				})
			default:
				_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
			}
		default:
			http.NotFound(w, r)
		}
	}))
}

func hostRewriteTransport(hosts map[string]string) http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if base, ok := hosts[req.URL.Host]; ok {
			target, err := url.Parse(base)
			if err != nil {
				return nil, err
			}
			req = req.Clone(req.Context())
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		}
		return http.DefaultTransport.RoundTrip(req)
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func initBareWithREADME(t *testing.T, barePath, seedDir string) {
	t.Helper()
	r, err := git.PlainInit(seedDir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	readme := "# hi\n<!--START_SECTION:waka-->\nold\n<!--END_SECTION:waka-->\n"
	if err := os.WriteFile(filepath.Join(seedDir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("README.md"); err != nil {
		t.Fatal(err)
	}
	sig := &object.Signature{Name: "t", Email: "t@example.com", When: time.Now()}
	if _, err := wt.Commit("init", &git.CommitOptions{Author: sig, Committer: sig}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(barePath), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err = git.PlainClone(barePath, true, &git.CloneOptions{URL: seedDir})
	if err != nil {
		t.Fatal(err)
	}
}
