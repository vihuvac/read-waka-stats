package httpx_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vihuvac/read-waka-stats/internal/httpx"
)

type errBody struct{}

func (errBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (errBody) Close() error             { return nil }

type staticRT struct {
	resp *http.Response
	err  error
}

func (s staticRT) RoundTrip(*http.Request) (*http.Response, error) {
	return s.resp, s.err
}

func TestNewDefaultTimeout(t *testing.T) {
	c := httpx.New(0)
	if c.HTTP.Timeout <= 0 {
		t.Fatal("expected default timeout")
	}
}

func TestGetJSONSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c := httpx.New(5 * time.Second)
	body, status, err := c.GetJSON(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if status != 200 || string(body) != `{"ok":true}` {
		t.Fatalf("status=%d body=%s", status, body)
	}
}

func TestGetJSONHeadersAndNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test") != "1" {
			t.Fatalf("header=%q", r.Header.Get("X-Test"))
		}
		w.WriteHeader(404)
		_, _ = w.Write([]byte("missing"))
	}))
	defer srv.Close()
	c := httpx.New(5 * time.Second)
	body, status, err := c.GetJSON(context.Background(), srv.URL, map[string]string{"X-Test": "1"})
	if err != nil {
		t.Fatal(err)
	}
	if status != 404 || string(body) != "missing" {
		t.Fatalf("%d %s", status, body)
	}
}

func TestGetJSONInvalidURL(t *testing.T) {
	c := httpx.New(time.Second)
	if _, _, err := c.GetJSON(context.Background(), "://bad", nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestGetJSONDoAndReadErrors(t *testing.T) {
	c := &httpx.Client{HTTP: &http.Client{Transport: staticRT{err: io.ErrUnexpectedEOF}}, Retries: 1}
	if _, _, err := c.GetJSON(context.Background(), "http://example.com", nil); err == nil {
		t.Fatal("expected do error")
	}

	c2 := &httpx.Client{HTTP: &http.Client{Transport: staticRT{resp: &http.Response{
		StatusCode: 200,
		Body:       errBody{},
		Header:     make(http.Header),
	}}}, Retries: 1}
	if _, _, err := c2.GetJSON(context.Background(), "http://example.com", nil); err == nil {
		t.Fatal("expected read error")
	}
}

func TestRetriesThenSuccess(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if n < 2 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	c := httpx.New(5 * time.Second)
	c.Retries = 3
	body, status, err := c.GetJSON(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if status != 200 || string(body) != "ok" {
		t.Fatalf("status=%d body=%s", status, body)
	}
}

func TestDoRetriesExhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()
	c := &httpx.Client{HTTP: &http.Client{Timeout: 2 * time.Second}, Retries: 1}
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Do(context.Background(), req); err == nil {
		t.Fatal("expected error")
	}
}

func TestDoRetriesZeroMeansOne(t *testing.T) {
	c := &httpx.Client{HTTP: &http.Client{Timeout: 50 * time.Millisecond}, Retries: 0}
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Do(context.Background(), req); err == nil {
		t.Fatal("expected error")
	}
}
