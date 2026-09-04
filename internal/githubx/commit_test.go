package githubx_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vihuvac/read-waka-stats/internal/githubx"
	"github.com/vihuvac/read-waka-stats/internal/httpx"
)

func TestCreateCommitOnBranch(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" {
			http.NotFound(w, r)
			return
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			http.Error(w, err.Error(), 500)
			return
		}
		if err := json.Unmarshal(raw, &gotBody); err != nil {
			t.Errorf("unmarshal: %v", err)
			http.Error(w, err.Error(), 500)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"createCommitOnBranch": map[string]any{
					"commit": map[string]any{"oid": "abc123"},
				},
			},
		})
	}))
	defer srv.Close()

	c := &githubx.Client{HTTP: httpx.New(5 * time.Second), Token: "t", APIBase: srv.URL}
	oid, err := c.CreateCommitOnBranch(context.Background(), githubx.CreateCommitInput{
		Owner:           "alice",
		Repo:            "alice",
		BranchName:      "main",
		Message:         "Updated with Dev Metrics",
		ExpectedHeadOid: "deadbeef",
		FileAdditions: []githubx.FileAddition{
			{Path: "README.md", Contents: []byte("# hi\n")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if oid != "abc123" {
		t.Fatalf("oid=%s", oid)
	}

	vars, _ := gotBody["variables"].(map[string]any)
	input, _ := vars["input"].(map[string]any)
	if input["expectedHeadOid"] != "deadbeef" {
		t.Fatalf("expectedHeadOid=%v", input["expectedHeadOid"])
	}
	branch, _ := input["branch"].(map[string]any)
	if branch["repositoryNameWithOwner"] != "alice/alice" || branch["branchName"] != "main" {
		t.Fatalf("branch=%v", branch)
	}
	msg, _ := input["message"].(map[string]any)
	if msg["headline"] != "Updated with Dev Metrics" {
		t.Fatalf("message=%v", msg)
	}
	changes, _ := input["fileChanges"].(map[string]any)
	adds, _ := changes["additions"].([]any)
	if len(adds) != 1 {
		t.Fatalf("additions=%v", adds)
	}
	add, _ := adds[0].(map[string]any)
	if add["path"] != "README.md" {
		t.Fatalf("path=%v", add["path"])
	}
	wantB64 := base64.StdEncoding.EncodeToString([]byte("# hi\n"))
	if add["contents"] != wantB64 {
		t.Fatalf("contents=%v want %s", add["contents"], wantB64)
	}
}

func TestCreateCommitOnBranchStaleHead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{
				{"message": "Expected head oid to match but it does not"},
			},
		})
	}))
	defer srv.Close()

	c := &githubx.Client{HTTP: httpx.New(5 * time.Second), Token: "t", APIBase: srv.URL}
	_, err := c.CreateCommitOnBranch(context.Background(), githubx.CreateCommitInput{
		Owner:           "alice",
		Repo:            "alice",
		BranchName:      "main",
		Message:         "msg",
		ExpectedHeadOid: "old",
		FileAdditions:   []githubx.FileAddition{{Path: "README.md", Contents: []byte("x")}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, githubx.ErrStaleHead) {
		t.Fatalf("want ErrStaleHead, got %v", err)
	}
}

func TestCreateCommitOnBranchMoreValidation(t *testing.T) {
	c := &githubx.Client{HTTP: httpx.New(5 * time.Second), Token: "t"}
	cases := []githubx.CreateCommitInput{
		{Owner: "", Repo: "b"},
		{Owner: "a", Repo: ""},
		{Owner: "a", Repo: "b"},
		{Owner: "a", Repo: "b", BranchName: "main"},
		{Owner: "a", Repo: "b", BranchName: "main", ExpectedHeadOid: "x"},
		{Owner: "a", Repo: "b", BranchName: "main", ExpectedHeadOid: "x", FileAdditions: []githubx.FileAddition{{Path: ""}}},
	}
	for i, in := range cases {
		if _, err := c.CreateCommitOnBranch(context.Background(), in); err == nil {
			t.Fatalf("case %d: expected error", i)
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"createCommitOnBranch": map[string]any{
					"commit": map[string]any{"oid": ""},
				},
			},
		})
	}))
	defer srv.Close()
	c2 := &githubx.Client{HTTP: httpx.New(5 * time.Second), Token: "t", APIBase: srv.URL}
	_, err := c2.CreateCommitOnBranch(context.Background(), githubx.CreateCommitInput{
		Owner: "a", Repo: "b", BranchName: "main", ExpectedHeadOid: "h",
		FileAdditions: []githubx.FileAddition{{Path: "README.md", Contents: []byte("x")}},
	})
	if err == nil || !strings.Contains(err.Error(), "empty commit oid") {
		t.Fatalf("got %v", err)
	}

	srvBad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "nope"})
	}))
	defer srvBad.Close()
	cBad := &githubx.Client{HTTP: httpx.New(5 * time.Second), Token: "t", APIBase: srvBad.URL}
	if _, err := cBad.CreateCommitOnBranch(context.Background(), githubx.CreateCommitInput{
		Owner: "a", Repo: "b", BranchName: "main", ExpectedHeadOid: "h",
		FileAdditions: []githubx.FileAddition{{Path: "README.md", Contents: []byte("x")}},
	}); err == nil {
		t.Fatal("expected unmarshal error")
	}

	// empty message uses default headline
	var got string
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		got = string(raw)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"createCommitOnBranch": map[string]any{
					"commit": map[string]any{"oid": "z"},
				},
			},
		})
	}))
	defer srv2.Close()
	c3 := &githubx.Client{HTTP: httpx.New(5 * time.Second), Token: "t", APIBase: srv2.URL}
	oid, err := c3.CreateCommitOnBranch(context.Background(), githubx.CreateCommitInput{
		Owner: "a", Repo: "b", BranchName: "main", ExpectedHeadOid: "h", Message: "   ",
		FileAdditions: []githubx.FileAddition{{Path: "README.md", Contents: []byte("x")}},
	})
	if err != nil || oid != "z" {
		t.Fatalf("oid=%s err=%v", oid, err)
	}
	if !strings.Contains(got, "Updated with Dev Metrics") {
		t.Fatalf("body=%s", got)
	}
}

func TestCreateCommitNonStaleGraphQLError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"errors": []map[string]string{{"message": "permission denied"}}})
	}))
	defer srv.Close()
	c := &githubx.Client{HTTP: httpx.New(2 * time.Second), Token: "t", APIBase: srv.URL}
	_, err := c.CreateCommitOnBranch(context.Background(), githubx.CreateCommitInput{
		Owner: "a", Repo: "b", BranchName: "main", ExpectedHeadOid: "h",
		FileAdditions: []githubx.FileAddition{{Path: "README.md", Contents: []byte("x")}},
	})
	if err == nil || strings.Contains(err.Error(), "stale") {
		t.Fatalf("got %v", err)
	}
}

