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
