package githubx

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vihuvac/read-waka-stats/internal/httpx"
	"github.com/vihuvac/read-waka-stats/internal/logging"
)

func TestMain(m *testing.M) {
	logging.Output = io.Discard
	os.Exit(m.Run())
}

func TestFetchUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"login":               "alice",
			"node_id":             "U_1",
			"disk_usage":          10,
			"hireable":            true,
			"public_repos":        3,
			"owned_private_repos": 1,
		})
	}))
	defer srv.Close()

	c := &Client{HTTP: httpx.New(5 * time.Second), Token: "t", APIBase: srv.URL}
	u, err := c.FetchUser(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if u.Login != "alice" || u.PublicRepos != 3 || u.OwnedPrivateRepos != 1 || !u.Hireable {
		t.Fatalf("%+v", u)
	}
	if u.DiskUsage != 10*1024 {
		t.Fatalf("disk %d", u.DiskUsage)
	}
}

func TestFetchUserNamedAndErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/bob":
			_ = json.NewEncoder(w).Encode(map[string]any{"login": "bob", "node_id": "U_2", "public_repos": 1})
		case "/user":
			w.WriteHeader(401)
			_, _ = w.Write([]byte("nope"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &Client{HTTP: httpx.New(5 * time.Second), Token: "t", APIBase: srv.URL}
	u, err := c.FetchUser(context.Background(), "bob")
	if err != nil {
		t.Fatal(err)
	}
	if u.Login != "bob" || u.DiskUsage != -1 {
		t.Fatalf("%+v", u)
	}
	if _, err := c.FetchUser(context.Background(), ""); err == nil {
		t.Fatal("expected HTTP error")
	}
}

func TestFetchUserBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{"))
	}))
	defer srv.Close()
	c := &Client{HTTP: httpx.New(2 * time.Second), Token: "t", APIBase: srv.URL}
	if _, err := c.FetchUser(context.Background(), ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestApiBaseTrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"login": "x", "node_id": "1"})
	}))
	defer srv.Close()
	c := &Client{HTTP: httpx.New(2 * time.Second), Token: "t", APIBase: srv.URL + "/"}
	if _, err := c.FetchUser(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
}

func TestAPIBaseDefault(t *testing.T) {
	c := &Client{}
	if c.apiBase() != "https://api.github.com" {
		t.Fatalf("%q", c.apiBase())
	}
}

func TestFetchProfileViews(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"count": 9})
	}))
	defer srv.Close()
	c := &Client{HTTP: httpx.New(5 * time.Second), Token: "t", APIBase: srv.URL}
	n, err := c.FetchProfileViews(context.Background(), "alice/alice")
	if err != nil {
		t.Fatal(err)
	}
	if n != 9 {
		t.Fatalf("count=%d", n)
	}
}

func TestFetchProfileViewsErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer srv.Close()
	c := &Client{HTTP: httpx.New(5 * time.Second), Token: "t", APIBase: srv.URL}
	if _, err := c.FetchProfileViews(context.Background(), "a/a"); err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchProfileViewsBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{"))
	}))
	defer srv.Close()
	c := &Client{HTTP: httpx.New(2 * time.Second), Token: "t", APIBase: srv.URL}
	if _, err := c.FetchProfileViews(context.Background(), "a/a"); err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchContributions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/alice") {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"years": []map[string]any{{"year": 2026, "total": 42}},
		})
	}))
	defer srv.Close()

	c := &Client{
		HTTP: &httpx.Client{
			HTTP: &http.Client{Transport: rewriteHost(srv.URL, "github-contributions.vercel.app")},
		},
		Token: "t",
	}
	years, err := c.FetchContributions(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(years) != 1 || years[0].Total != 42 {
		t.Fatalf("%+v", years)
	}
}

func TestFetchContributionsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	c := &Client{
		HTTP: &httpx.Client{
			HTTP: &http.Client{Transport: rewriteHost(srv.URL, "github-contributions.vercel.app")},
		},
	}
	if _, err := c.FetchContributions(context.Background(), "alice"); err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchContributionsBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{"))
	}))
	defer srv.Close()
	c := &Client{
		HTTP: &httpx.Client{HTTP: &http.Client{Transport: rewriteHost(srv.URL, "github-contributions.vercel.app")}},
	}
	if _, err := c.FetchContributions(context.Background(), "a"); err == nil {
		t.Fatal("expected error")
	}
}

func TestYearContribUnmarshalStringAndIntYears(t *testing.T) {
	payload := `{"years":[{"year":"2026","total":10},{"year":2025,"total":20}]}`
	var raw struct {
		Years []YearContrib `json:"years"`
	}
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.Years) != 2 {
		t.Fatalf("len=%d", len(raw.Years))
	}
	if raw.Years[0].Year != 2026 || raw.Years[0].Total != 10 {
		t.Fatalf("string year: %+v", raw.Years[0])
	}
	if raw.Years[1].Year != 2025 || raw.Years[1].Total != 20 {
		t.Fatalf("int year: %+v", raw.Years[1])
	}
}

func TestYearContribUnmarshalInvalidYear(t *testing.T) {
	var y YearContrib
	if err := json.Unmarshal([]byte(`{"year":"not-a-year","total":1}`), &y); err == nil {
		t.Fatal("expected error for non-numeric year string")
	}
}

func TestYearContribUnmarshalResetsYearOnAbsentOrNull(t *testing.T) {
	var y YearContrib
	if err := json.Unmarshal([]byte(`{"year":2024,"total":5}`), &y); err != nil {
		t.Fatal(err)
	}
	if y.Year != 2024 || y.Total != 5 {
		t.Fatalf("first decode: %+v", y)
	}
	if err := json.Unmarshal([]byte(`{"total":9}`), &y); err != nil {
		t.Fatal(err)
	}
	if y.Year != 0 || y.Total != 9 {
		t.Fatalf("absent year: %+v", y)
	}
	if err := json.Unmarshal([]byte(`{"year":2023,"total":2}`), &y); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"year":null,"total":7}`), &y); err != nil {
		t.Fatal(err)
	}
	if y.Year != 0 || y.Total != 7 {
		t.Fatalf("null year: %+v", y)
	}
}

func TestYearContribInvalidYearType(t *testing.T) {
	var y YearContrib
	if err := json.Unmarshal([]byte(`{"year":true,"total":1}`), &y); err == nil {
		t.Fatal("expected error")
	}
}

func TestYearContribTopLevelUnmarshalError(t *testing.T) {
	var y YearContrib
	if err := json.Unmarshal([]byte(`[]`), &y); err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchRepositoriesAndPagination(t *testing.T) {
	var ownedPages, contribPages int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		payload := string(body)
		if strings.Contains(payload, "repositoriesContributedTo") {
			n := atomic.AddInt32(&contribPages, 1)
			hasNext := n == 1
			cursor := "c1"
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"user": map[string]any{
						"repositoriesContributedTo": map[string]any{
							"nodes": []map[string]any{
								{"name": "contrib1", "isPrivate": false, "isFork": false, "owner": map[string]string{"login": "org"}, "primaryLanguage": map[string]string{"name": "Go"}},
								{"name": "forked", "isPrivate": false, "isFork": true, "owner": map[string]string{"login": "org"}},
								{"name": "owned-dup", "isPrivate": false, "isFork": false, "owner": map[string]string{"login": "alice"}},
							},
							"pageInfo": map[string]any{"endCursor": cursor, "hasNextPage": hasNext},
						},
					},
				},
			})
			return
		}
		n := atomic.AddInt32(&ownedPages, 1)
		hasNext := n == 1
		nodes := []map[string]any{
			{"name": "owned-dup", "isPrivate": false, "owner": map[string]string{"login": "alice"}, "primaryLanguage": map[string]string{"name": "Go"}},
		}
		if n == 1 {
			nodes = append(nodes, map[string]any{
				"name": "owned2", "isPrivate": true, "owner": map[string]string{"login": "alice"},
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"user": map[string]any{
					"repositories": map[string]any{
						"nodes":    nodes,
						"pageInfo": map[string]any{"endCursor": "o1", "hasNextPage": hasNext},
					},
				},
			},
		})
	}))
	defer srv.Close()

	c := &Client{HTTP: httpx.New(5 * time.Second), Token: "t", APIBase: srv.URL, Log: logging.New(false)}
	repos, err := c.FetchRepositories(context.Background(), "alice", 10)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, r := range repos {
		names[r.Name] = true
	}
	if !names["owned-dup"] || !names["owned2"] || !names["contrib1"] {
		t.Fatalf("repos=%v", repos)
	}
	if names["forked"] {
		t.Fatal("fork should be skipped")
	}

	// maxRepos caps owned early
	repos, err = c.FetchRepositories(context.Background(), "alice", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 {
		t.Fatalf("len=%d", len(repos))
	}
}

func TestFetchReposMaxExactAndPaginationErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"user": map[string]any{
				"repositories": map[string]any{
					"nodes": []map[string]any{
						{"name": "r1", "owner": map[string]string{"login": "a"}, "primaryLanguage": map[string]string{"name": "Go"}},
						{"name": "r2", "owner": map[string]string{"login": "a"}},
					},
					"pageInfo": map[string]any{"hasNextPage": false},
				},
			}},
		})
	}))
	defer srv.Close()
	c := &Client{HTTP: httpx.New(2 * time.Second), Token: "t", APIBase: srv.URL}
	repos, err := c.FetchRepositories(context.Background(), "a", 2)
	if err != nil || len(repos) != 2 {
		t.Fatalf("%v %v", repos, err)
	}
}

func TestFetchReposCapsContrib(t *testing.T) {
	orig := graphqlAttemptLimit
	t.Cleanup(func() { graphqlAttemptLimit = orig })
	graphqlAttemptLimit = 1
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "repositoriesContributedTo") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"user": map[string]any{
					"repositoriesContributedTo": map[string]any{
						"nodes": []map[string]any{
							{"name": "c1", "isFork": false, "owner": map[string]string{"login": "o"}},
							{"name": "c2", "isFork": false, "owner": map[string]string{"login": "o"}},
							{"name": "c3", "isFork": false, "owner": map[string]string{"login": "o"}},
						},
						"pageInfo": map[string]any{"hasNextPage": false},
					},
				}},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"user": map[string]any{
				"repositories": map[string]any{
					"nodes":    []map[string]any{{"name": "owned", "owner": map[string]string{"login": "o"}}},
					"pageInfo": map[string]any{"hasNextPage": false},
				},
			}},
		})
	}))
	defer srv.Close()
	c := &Client{HTTP: httpx.New(2 * time.Second), Token: "t", APIBase: srv.URL}
	repos, err := c.FetchRepositories(context.Background(), "o", 2)
	if err != nil || len(repos) != 2 {
		t.Fatalf("%v %v", repos, err)
	}
}

func TestFetchReposErrorsAndCap(t *testing.T) {
	orig := graphqlAttemptLimit
	t.Cleanup(func() { graphqlAttemptLimit = orig })
	graphqlAttemptLimit = 1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "repositoriesContributedTo") {
			_ = json.NewEncoder(w).Encode(map[string]any{"errors": []map[string]string{{"message": "nope"}}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"user": map[string]any{
				"repositories": map[string]any{
					"nodes": []map[string]any{
						{"name": "a", "owner": map[string]string{"login": "o"}, "primaryLanguage": map[string]string{"name": "Go"}},
					},
					"pageInfo": map[string]any{"hasNextPage": false},
				},
			}},
		})
	}))
	defer srv.Close()
	c := &Client{HTTP: httpx.New(2 * time.Second), Token: "t", APIBase: srv.URL}
	if _, err := c.FetchRepositories(context.Background(), "o", 10); err == nil {
		t.Fatal("expected contrib error")
	}

	// owned page fails
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"errors": []map[string]string{{"message": "deny"}}})
	}))
	defer srv2.Close()
	c2 := &Client{HTTP: httpx.New(2 * time.Second), Token: "t", APIBase: srv2.URL}
	if _, err := c2.FetchRepositories(context.Background(), "o", 0); err == nil {
		t.Fatal("expected owned error")
	}
}

func TestFetchBranchesAndCommits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		q := string(body)
		if strings.Contains(q, "refPrefix") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"repository": map[string]any{
						"refs": map[string]any{
							"nodes":    []map[string]string{{"name": "main"}, {"name": "dev"}},
							"pageInfo": map[string]any{"hasNextPage": false},
						},
					},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"repository": map[string]any{
					"ref": map[string]any{
						"target": map[string]any{
							"history": map[string]any{
								"nodes": []map[string]any{
									{"additions": 3, "deletions": 1, "committedDate": "2026-01-02T15:04:05Z", "oid": "abc"},
								},
								"pageInfo": map[string]any{"hasNextPage": false},
							},
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	c := &Client{HTTP: httpx.New(5 * time.Second), Token: "t", APIBase: srv.URL}
	branches, err := c.FetchBranches(context.Background(), "alice", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 2 {
		t.Fatalf("%v", branches)
	}
	commits, err := c.FetchCommits(context.Background(), "alice", "repo", "main", "U_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 || commits[0].OID != "abc" {
		t.Fatalf("%v", commits)
	}

	// nil repository / ref paths
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "refPrefix") {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": nil}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"repository": map[string]any{"ref": nil}},
		})
	}))
	defer srv2.Close()
	c2 := &Client{HTTP: httpx.New(5 * time.Second), Token: "t", APIBase: srv2.URL}
	br, err := c2.FetchBranches(context.Background(), "a", "b")
	if err != nil || len(br) != 0 {
		t.Fatalf("branches=%v err=%v", br, err)
	}
	cm, err := c2.FetchCommits(context.Background(), "a", "b", "main", "id")
	if err != nil || len(cm) != 0 {
		t.Fatalf("commits=%v err=%v", cm, err)
	}
}

func TestBranchAndCommitPagination(t *testing.T) {
	var branchHits, commitHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		q := string(body)
		if strings.Contains(q, "refPrefix") {
			n := atomic.AddInt32(&branchHits, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"repository": map[string]any{
					"refs": map[string]any{
						"nodes":    []map[string]string{{"name": "b" + string(rune('0'+n))}},
						"pageInfo": map[string]any{"endCursor": "c", "hasNextPage": n == 1},
					},
				}},
			})
			return
		}
		n := atomic.AddInt32(&commitHits, 1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"repository": map[string]any{
				"ref": map[string]any{"target": map[string]any{
					"history": map[string]any{
						"nodes": []map[string]any{
							{"additions": 1, "deletions": 0, "committedDate": "2026-01-01T00:00:00Z", "oid": "o"},
						},
						"pageInfo": map[string]any{"endCursor": "c", "hasNextPage": n == 1},
					},
				}},
			}},
		})
	}))
	defer srv.Close()
	c := &Client{HTTP: httpx.New(2 * time.Second), Token: "t", APIBase: srv.URL}
	br, err := c.FetchBranches(context.Background(), "a", "b")
	if err != nil || len(br) != 2 {
		t.Fatalf("%v %v", br, err)
	}
	cm, err := c.FetchCommits(context.Background(), "a", "b", "refs/heads/main", "id")
	if err != nil || len(cm) != 2 {
		t.Fatalf("%v %v", cm, err)
	}
}

func TestGraphQLRetryAndTruncate(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n == 1 {
			w.WriteHeader(502)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"repository": map[string]any{
					"refs": map[string]any{
						"nodes":    []map[string]string{{"name": "main"}},
						"pageInfo": map[string]any{"hasNextPage": false},
					},
				},
			},
		})
	}))
	defer srv.Close()

	c := &Client{
		HTTP:    &httpx.Client{HTTP: &http.Client{Timeout: 5 * time.Second}, Retries: 1},
		Token:   "t",
		APIBase: srv.URL,
	}
	br, err := c.FetchBranches(context.Background(), "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if len(br) != 1 {
		t.Fatalf("%v", br)
	}

	// non-retryable error path exercises truncate
	long := strings.Repeat("x", 500)
	srvErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(long))
	}))
	defer srvErr.Close()
	cErr := &Client{HTTP: httpx.New(5 * time.Second), Token: "t", APIBase: srvErr.URL}
	_, err = cErr.FetchBranches(context.Background(), "a", "b")
	if err == nil || !strings.Contains(err.Error(), "graphql HTTP 400") {
		t.Fatalf("got %v", err)
	}
}

func TestTruncateShortAndLong(t *testing.T) {
	if truncate([]byte("hi"), 10) != "hi" {
		t.Fatal("short")
	}
	long := strings.Repeat("a", 50)
	if got := truncate([]byte(long), 10); got != long[:10] {
		t.Fatalf("%q", got)
	}
}

func TestGraphQLBadJSONAndExhaustedRetries(t *testing.T) {
	orig := graphqlAttemptLimit
	t.Cleanup(func() { graphqlAttemptLimit = orig })
	graphqlAttemptLimit = 0 // exercises graphqlAttempts min clamp
	if graphqlAttempts() != 1 {
		t.Fatalf("got %d", graphqlAttempts())
	}
	graphqlAttemptLimit = 2

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{"))
	}))
	defer srv.Close()
	c := &Client{HTTP: httpx.New(2 * time.Second), Token: "t", APIBase: srv.URL}
	if _, err := c.FetchBranches(context.Background(), "a", "b"); err == nil {
		t.Fatal("expected json error")
	}

	var hits int
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(503)
	}))
	defer srv2.Close()
	c2 := &Client{HTTP: &httpx.Client{HTTP: srv2.Client(), Retries: 1}, Token: "t", APIBase: srv2.URL}
	if _, err := c2.FetchBranches(context.Background(), "a", "b"); err == nil {
		t.Fatal("expected exhausted")
	}
	if hits < 2 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestPaginateAndBranchUnmarshalErrors(t *testing.T) {
	orig := graphqlAttemptLimit
	t.Cleanup(func() { graphqlAttemptLimit = orig })
	graphqlAttemptLimit = 1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "bad"})
	}))
	defer srv.Close()
	c := &Client{HTTP: httpx.New(2 * time.Second), Token: "t", APIBase: srv.URL}
	if _, err := c.FetchRepositories(context.Background(), "a", 0); err == nil {
		t.Fatal("expected unmarshal")
	}
	if _, err := c.FetchBranches(context.Background(), "a", "b"); err == nil {
		t.Fatal("expected unmarshal")
	}
	if _, err := c.FetchCommits(context.Background(), "a", "b", "main", "id"); err == nil {
		t.Fatal("expected unmarshal")
	}
}

func TestFetchNetworkErrors(t *testing.T) {
	c := &Client{HTTP: &httpx.Client{HTTP: &http.Client{Timeout: 50 * time.Millisecond}, Retries: 1}, Token: "t", APIBase: "http://127.0.0.1:1"}
	if _, err := c.FetchUser(context.Background(), ""); err == nil {
		t.Fatal("expected error")
	}
	if _, err := c.FetchProfileViews(context.Background(), "a/a"); err == nil {
		t.Fatal("expected error")
	}
	c2 := &Client{HTTP: &httpx.Client{HTTP: &http.Client{Timeout: 50 * time.Millisecond}, Retries: 1}}
	if _, err := c2.FetchContributions(context.Background(), "a"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := c2.FetchLinguistColors(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func rewriteHost(targetURL, host string) http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == host {
			u, err := http.NewRequest(req.Method, targetURL+req.URL.RequestURI(), req.Body)
			if err != nil {
				return nil, err
			}
			u.Header = req.Header
			req = u
		}
		return http.DefaultTransport.RoundTrip(req)
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
