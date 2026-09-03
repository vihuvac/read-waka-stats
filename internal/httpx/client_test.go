package httpx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vihuvac/read-waka-stats/internal/httpx"
)

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
