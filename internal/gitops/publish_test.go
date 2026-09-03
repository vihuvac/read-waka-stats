package gitops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/vihuvac/read-waka-stats/internal/githubx"
)

type fakePublisher struct {
	calls []githubx.CreateCommitInput
	oid   string
	err   error
}

func (f *fakePublisher) CreateCommitOnBranch(_ context.Context, in githubx.CreateCommitInput) (string, error) {
	f.calls = append(f.calls, in)
	if f.err != nil {
		return "", f.err
	}
	if f.oid == "" {
		return "newoid", nil
	}
	return f.oid, nil
}

func initTestRepo(t *testing.T, dir string, files map[string]string) *Repository {
	t.Helper()
	r, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	for path, content := range files {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := wt.Add(path); err != nil {
			t.Fatal(err)
		}
	}
	sig := &object.Signature{Name: "t", Email: "t@example.com", When: time.Now()}
	if _, err := wt.Commit("init", &git.CommitOptions{Author: sig, Committer: sig}); err != nil {
		t.Fatal(err)
	}
	return &Repository{
		opts: Options{Owner: "alice", Repo: "alice", WorkDir: dir, Branch: "master", CommitMessage: "Updated with Dev Metrics"},
		repo: r,
		wt:   wt,
	}
}

func TestPublishNoChanges(t *testing.T) {
	dir := t.TempDir()
	repo := initTestRepo(t, dir, map[string]string{"README.md": "# hi\n"})
	pub := &fakePublisher{}
	if err := repo.Publish(context.Background(), pub, []string{"README.md"}); err != nil {
		t.Fatal(err)
	}
	if len(pub.calls) != 0 {
		t.Fatalf("expected no publish calls, got %d", len(pub.calls))
	}
}

func TestPublishChangedFiles(t *testing.T) {
	dir := t.TempDir()
	repo := initTestRepo(t, dir, map[string]string{
		"README.md":          "# hi\n",
		"assets/bar_graph.png": "png-v1",
	})
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# updated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets/bar_graph.png"), []byte("png-v2"), 0o644); err != nil {
		t.Fatal(err)
	}

	pub := &fakePublisher{oid: "commit1"}
	if err := repo.Publish(context.Background(), pub, []string{"README.md", "assets/bar_graph.png"}); err != nil {
		t.Fatal(err)
	}
	if len(pub.calls) != 1 {
		t.Fatalf("calls=%d", len(pub.calls))
	}
	in := pub.calls[0]
	if in.Owner != "alice" || in.Repo != "alice" || in.BranchName != "master" {
		t.Fatalf("input identity: %+v", in)
	}
	if in.Message != "Updated with Dev Metrics" {
		t.Fatalf("message=%q", in.Message)
	}
	if in.ExpectedHeadOid == "" {
		t.Fatal("missing expectedHeadOid")
	}
	if len(in.FileAdditions) != 2 {
		t.Fatalf("additions=%d", len(in.FileAdditions))
	}
}

func TestPublishStaleHead(t *testing.T) {
	dir := t.TempDir()
	repo := initTestRepo(t, dir, map[string]string{"README.md": "# hi\n"})
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# next\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pub := &fakePublisher{err: githubx.ErrStaleHead}
	err := repo.Publish(context.Background(), pub, []string{"README.md"})
	if !errors.Is(err, githubx.ErrStaleHead) {
		t.Fatalf("got %v", err)
	}
}

func TestPathJoinsWorkDir(t *testing.T) {
	r := &Repository{opts: Options{WorkDir: "repo"}}
	got := r.Path("README.md")
	want := filepath.Join("repo", "README.md")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if r.Root() != "repo" {
		t.Fatalf("root %q", r.Root())
	}
}
