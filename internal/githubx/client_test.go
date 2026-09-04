package githubx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vihuvac/read-waka-stats/internal/githubx"
	"github.com/vihuvac/read-waka-stats/internal/httpx"
)

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

	c := &githubx.Client{HTTP: httpx.New(5 * time.Second), Token: "t", APIBase: srv.URL}
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

func TestFetchProfileViews(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"count": 9})
	}))
	defer srv.Close()
	c := &githubx.Client{HTTP: httpx.New(5 * time.Second), Token: "t", APIBase: srv.URL}
	n, err := c.FetchProfileViews(context.Background(), "alice/alice")
	if err != nil {
		t.Fatal(err)
	}
	if n != 9 {
		t.Fatalf("count=%d", n)
	}
}

func TestYearContribUnmarshalStringAndIntYears(t *testing.T) {
	payload := `{"years":[{"year":"2026","total":10},{"year":2025,"total":20}]}`
	var raw struct {
		Years []githubx.YearContrib `json:"years"`
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
	var y githubx.YearContrib
	if err := json.Unmarshal([]byte(`{"year":"not-a-year","total":1}`), &y); err == nil {
		t.Fatal("expected error for non-numeric year string")
	}
}

func TestYearContribUnmarshalResetsYearOnAbsentOrNull(t *testing.T) {
	var y githubx.YearContrib
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
