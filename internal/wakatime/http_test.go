package wakatime_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vihuvac/read-waka-stats/internal/httpx"
	"github.com/vihuvac/read-waka-stats/internal/wakatime"
)

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

func TestFetchHTTPAccepted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(202)
	}))
	defer srv.Close()
	c := &wakatime.Client{HTTP: httpx.New(5 * time.Second), APIURL: srv.URL + "/", APIKey: "k"}
	stats, err := c.FetchAllTime(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats != nil {
		t.Fatal("expected nil while calculating")
	}
}
