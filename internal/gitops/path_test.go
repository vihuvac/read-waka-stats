package gitops

import (
	"path/filepath"
	"testing"
)

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
