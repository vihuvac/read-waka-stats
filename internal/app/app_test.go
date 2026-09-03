package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vihuvac/read-waka-stats/internal/githubx"
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

func TestStaleHeadRetryErrorWraps(t *testing.T) {
	err := fmt.Errorf("publish failed after %d attempts: %w", maxPublishAttempts, githubx.ErrStaleHead)
	if !errors.Is(err, githubx.ErrStaleHead) {
		t.Fatalf("expected ErrStaleHead, got %v", err)
	}
}
