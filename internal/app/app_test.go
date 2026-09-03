package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vihuvac/read-waka-stats/internal/config"
)

func TestWriteOutputFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	t.Setenv("GITHUB_OUTPUT", path)
	if err := writeOutput("hello stats"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "hello stats") || !strings.Contains(s, "README_CONTENT<<") {
		t.Fatalf("output: %s", s)
	}
}

func TestCommitIdentity(t *testing.T) {
	a := &App{Cfg: &config.Config{CommitByMe: false}}
	name, email := a.commitIdentity("alice", "a@example.com")
	if name != "readme-bot" {
		t.Fatalf("name=%s", name)
	}
	if !strings.Contains(email, "github-actions") {
		t.Fatalf("email=%s", email)
	}

	a.Cfg.CommitByMe = true
	name, email = a.commitIdentity("alice", "a@example.com")
	if name != "alice" || email != "a@example.com" {
		t.Fatalf("%s %s", name, email)
	}
}
