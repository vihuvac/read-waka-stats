package wakatime_test

import (
	"context"
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
	"github.com/vihuvac/read-waka-stats/internal/wakatime"
)

func TestMain(m *testing.M) {
	logging.Output = io.Discard
	os.Exit(m.Run())
}

func TestFetchWeeklyHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "secret" {
			t.Fatal("missing api key")
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data":{"timezone":"UTC","languages":[{"name":"Go","text":"1 hr","percent":100}]}}`))
	}))
	defer srv.Close()

	c := &wakatime.Client{
		HTTP:   httpx.New(5 * time.Second),
		APIURL: srv.URL + "/",
		APIKey: "secret",
	}
	stats, err := c.FetchWeekly(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Data.Timezone != "UTC" || stats.Data.Languages[0].Name != "Go" {
		t.Fatalf("%+v", stats.Data)
	}
}

func TestFetchHTTPAcceptedRetriesThenSucceeds(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			w.WriteHeader(202)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data":{"timezone":"UTC","is_up_to_date":true,"languages":[{"name":"Go","text":"1 hr","percent":100}]}}`))
	}))
	defer srv.Close()

	c := &wakatime.Client{
		HTTP:        httpx.New(5 * time.Second),
		APIURL:      srv.URL + "/",
		APIKey:      "k",
		Log:         logging.New(true),
		MaxAttempts: 5,
		Sleep:       func(time.Duration) {},
	}
	stats, err := c.FetchWeekly(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats == nil || stats.Data.Timezone != "UTC" {
		t.Fatalf("got %+v", stats)
	}
	if calls.Load() != 3 {
		t.Fatalf("calls=%d want 3", calls.Load())
	}
}

func TestFetchHTTPAcceptedExhaustsRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(202)
	}))
	defer srv.Close()
	c := &wakatime.Client{
		HTTP:        httpx.New(5 * time.Second),
		APIURL:      srv.URL + "/",
		APIKey:      "k",
		Log:         logging.New(true),
		MaxAttempts: 3,
		Sleep:       func(time.Duration) {},
	}
	stats, err := c.FetchAllTime(context.Background())
	if err == nil || stats != nil {
		t.Fatalf("stats=%v err=%v", stats, err)
	}
	if !strings.Contains(err.Error(), "still calculating") {
		t.Fatalf("got %v", err)
	}
}

func TestFetchHTTP201ExhaustsRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
	}))
	defer srv.Close()
	c := &wakatime.Client{
		HTTP:        httpx.New(5 * time.Second),
		APIURL:      srv.URL + "/",
		APIKey:      "k",
		Log:         logging.New(true),
		MaxAttempts: 2,
		Sleep:       func(time.Duration) {},
	}
	stats, err := c.FetchAllTime(context.Background())
	if err == nil || stats != nil {
		t.Fatalf("stats=%v err=%v", stats, err)
	}
}

func TestFetchRetriesStaleStats(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			_, _ = w.Write([]byte(`{"data":{"timezone":"UTC","is_up_to_date":false,"languages":[]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"timezone":"UTC","is_up_to_date":true,"languages":[{"name":"Go","text":"1 hr","percent":100}]}}`))
	}))
	defer srv.Close()
	c := &wakatime.Client{
		HTTP:        httpx.New(5 * time.Second),
		APIURL:      srv.URL + "/",
		APIKey:      "k",
		MaxAttempts: 3,
		Sleep:       func(time.Duration) {},
	}
	stats, err := c.FetchWeekly(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.Data.Languages) != 1 || calls.Load() != 2 {
		t.Fatalf("stats=%+v calls=%d", stats.Data, calls.Load())
	}
}

func TestFetchHTTPErrorAndTruncate(t *testing.T) {
	long := strings.Repeat("e", 400)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(long))
	}))
	defer srv.Close()
	c := &wakatime.Client{HTTP: httpx.New(5 * time.Second), APIURL: srv.URL + "/", APIKey: "k", Log: logging.New(false)}
	_, err := c.FetchWeekly(context.Background())
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("got %v", err)
	}
}

func TestTruncateShortBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("short"))
	}))
	defer srv.Close()
	c := &wakatime.Client{HTTP: httpx.New(2 * time.Second), APIURL: srv.URL + "/", APIKey: "k"}
	_, err := c.FetchWeekly(context.Background())
	if err == nil || !strings.Contains(err.Error(), "short") {
		t.Fatalf("got %v", err)
	}
}

func TestFetchBadURLAndBadJSON(t *testing.T) {
	c := &wakatime.Client{HTTP: httpx.New(time.Second), APIURL: "://bad", APIKey: "k"}
	if _, err := c.FetchWeekly(context.Background()); err == nil {
		t.Fatal("expected parse error")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{"))
	}))
	defer srv.Close()
	c2 := &wakatime.Client{HTTP: httpx.New(2 * time.Second), APIURL: srv.URL + "/", APIKey: "k"}
	if _, err := c2.FetchWeekly(context.Background()); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("got %v", err)
	}
}

func TestFetchNetworkError(t *testing.T) {
	c := &wakatime.Client{
		HTTP:   &httpx.Client{HTTP: &http.Client{Timeout: 50 * time.Millisecond}, Retries: 1},
		APIURL: "http://127.0.0.1:1/",
		APIKey: "k",
	}
	if _, err := c.FetchWeekly(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
