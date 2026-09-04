package commits_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/vihuvac/read-waka-stats/internal/commits"
	"github.com/vihuvac/read-waka-stats/internal/githubx"
	"github.com/vihuvac/read-waka-stats/internal/httpx"
	"github.com/vihuvac/read-waka-stats/internal/logging"
)

func TestMain(m *testing.M) {
	logging.Output = io.Discard
	os.Exit(m.Run())
}

func TestTotalAdditions(t *testing.T) {
	y := commits.YearlyData{
		2024: {1: {"Go": {Add: 10, Del: 2}, "Python": {Add: 5}}},
		2025: {2: {"Go": {Add: 7}}},
	}
	if got := commits.TotalAdditions(y); got != 22 {
		t.Fatalf("got %d", got)
	}
}

func TestCountDayPartsTimezone(t *testing.T) {
	dates := commits.DateData{
		"repo": {
			"main": {
				"a": "2026-01-05T15:00:00Z", // Monday 15:00 UTC -> afternoon
				"b": "2026-01-06T03:00:00Z", // Tuesday 03:00 UTC -> night
			},
		},
	}
	day, week := commits.CountDayParts(dates, "UTC")
	if day[2] != 1 { // 12-18
		t.Fatalf("afternoon=%v", day)
	}
	if day[0] != 1 { // 00-06
		t.Fatalf("night=%v", day)
	}
	if week[0] != 1 || week[1] != 1 { // Mon, Tue
		t.Fatalf("week=%v", week)
	}
}

func TestCountDayPartsInvalidTZAndBadDates(t *testing.T) {
	dates := commits.DateData{
		"repo": {"main": {
			"a": "2026-01-05T15:00:00Z",
			"b": "garbage",
		}},
	}
	day, week := commits.CountDayParts(dates, "Not/AZone")
	if day[2] != 1 {
		t.Fatalf("%v", day)
	}
	sum := 0
	for _, v := range week {
		sum += v
	}
	if sum != 1 {
		t.Fatalf("week=%v", week)
	}
}

func TestIgnoreSet(t *testing.T) {
	m := commits.IgnoreSet([]string{" a ", "", "b"})
	if _, ok := m["a"]; !ok {
		t.Fatal("missing a")
	}
	if len(m) != 2 {
		t.Fatalf("len=%d", len(m))
	}
}

func TestCalculateAggregates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		q := string(body)
		if strings.Contains(q, "refPrefix") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"repository": map[string]any{
						"refs": map[string]any{
							"nodes":    []map[string]string{{"name": "main"}, {"name": "bad"}},
							"pageInfo": map[string]any{"hasNextPage": false},
						},
					},
				},
			})
			return
		}
		if strings.Contains(q, `"bad"`) || strings.Contains(q, "refs/heads/bad") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errors": []map[string]string{{"message": "branch boom"}},
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
									{"additions": 10, "deletions": 2, "committedDate": "2026-04-15T12:00:00Z", "oid": "c1"},
									{"additions": 1, "deletions": 0, "committedDate": "not-a-date", "oid": "c2"},
									{"additions": 5, "deletions": 1, "committedDate": "2026-01-02T15:04:05Z", "oid": "c3"},
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

	gh := &githubx.Client{HTTP: httpx.New(5 * time.Second), Token: "t", APIBase: srv.URL}
	calc := &commits.Calculator{
		GH:       gh,
		AuthorID: "U_1",
		Ignored:  commits.IgnoreSet([]string{"skip-me"}),
		Log:      logging.New(true),
		Sleep:    0,
	}
	repos := []githubx.Repository{
		{Name: "skip-me", Owner: "alice", PrimaryLanguage: "Go"},
		{Name: "app", Owner: "alice", PrimaryLanguage: "Go", IsPrivate: true},
		{Name: "nolang", Owner: "alice", PrimaryLanguage: ""},
	}
	res, err := calc.Calculate(context.Background(), repos)
	if err != nil {
		t.Fatal(err)
	}
	if commits.TotalAdditions(res.Yearly) != 15 { // 10 + 5 from dated commits with language
		t.Fatalf("additions=%d yearly=%v", commits.TotalAdditions(res.Yearly), res.Yearly)
	}
	if res.Dates["app"]["main"]["c1"] == "" {
		t.Fatalf("dates=%v", res.Dates)
	}
	// nolang still records dates
	if res.Dates["nolang"] == nil {
		t.Fatal("expected nolang dates")
	}
}

func TestCalculateSkipsRepoOnBranchFetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"message": "denied"}},
		})
	}))
	defer srv.Close()
	gh := &githubx.Client{HTTP: httpx.New(5 * time.Second), Token: "t", APIBase: srv.URL}
	calc := &commits.Calculator{GH: gh, AuthorID: "U", Log: logging.New(false), Sleep: 0}
	res, err := calc.Calculate(context.Background(), []githubx.Repository{{Name: "x", Owner: "a", PrimaryLanguage: "Go"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Yearly) != 0 {
		t.Fatalf("%v", res.Yearly)
	}
}

func TestCalculateEmptyBranches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"repository": map[string]any{
					"refs": map[string]any{
						"nodes":    []any{},
						"pageInfo": map[string]any{"hasNextPage": false},
					},
				},
			},
		})
	}))
	defer srv.Close()
	gh := &githubx.Client{HTTP: httpx.New(5 * time.Second), Token: "t", APIBase: srv.URL}
	calc := &commits.Calculator{GH: gh, AuthorID: "U", Sleep: 0}
	res, err := calc.Calculate(context.Background(), []githubx.Repository{{Name: "x", Owner: "a"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Dates) != 0 {
		t.Fatalf("%v", res.Dates)
	}
}

func TestCalculateRespectsContextCancelDuringSleep(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "refPrefix") {
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
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"repository": map[string]any{
					"ref": map[string]any{
						"target": map[string]any{
							"history": map[string]any{
								"nodes":    []any{},
								"pageInfo": map[string]any{"hasNextPage": false},
							},
						},
					},
				},
			},
		})
	}))
	defer srv.Close()
	gh := &githubx.Client{HTTP: httpx.New(5 * time.Second), Token: "t", APIBase: srv.URL}
	calc := &commits.Calculator{GH: gh, AuthorID: "U", Sleep: 5 * time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	_, err := calc.Calculate(ctx, []githubx.Repository{{Name: "x", Owner: "a"}})
	if err == nil {
		t.Fatal("expected context error")
	}
}
