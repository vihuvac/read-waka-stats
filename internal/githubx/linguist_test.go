package githubx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vihuvac/read-waka-stats/internal/githubx"
	"github.com/vihuvac/read-waka-stats/internal/httpx"
)

func TestParseLinguistColors(t *testing.T) {
	yml := `
Go:
  type: programming
  color: "#00ADD8"
Python:
  color: '#3572A5'
`
	got := githubx.ParseLinguistColors(yml)
	if got["Go"] != "#00ADD8" {
		t.Fatalf("Go=%q", got["Go"])
	}
	if got["Python"] != "#3572A5" {
		t.Fatalf("Python=%q", got["Python"])
	}
}

func TestFetchLinguistColors(t *testing.T) {
	yml := "Go:\n  color: \"#00ADD8\"\nPython:\n  color: '#3572A5'\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(yml))
	}))
	defer srv.Close()
	c := &githubx.Client{
		HTTP: &httpx.Client{
			HTTP: &http.Client{Transport: rewriteHost(srv.URL, "cdn.jsdelivr.net")},
		},
	}
	colors, err := c.FetchLinguistColors(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if colors["Go"] != "#00ADD8" || colors["Python"] != "#3572A5" {
		t.Fatalf("%v", colors)
	}
}

func TestFetchLinguistHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	c := &githubx.Client{
		HTTP: &httpx.Client{HTTP: &http.Client{Transport: rewriteHost(srv.URL, "cdn.jsdelivr.net")}},
	}
	if _, err := c.FetchLinguistColors(context.Background()); err == nil {
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
