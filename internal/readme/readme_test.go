package readme_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vihuvac/read-waka-stats/internal/readme"
)

func TestUpdateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "README.md")
	content := "# Hi\n<!--START_SECTION:waka-->\nold\n<!--END_SECTION:waka-->\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := readme.UpdateFile(path, "waka", "fresh"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "fresh") || strings.Contains(string(data), "old") {
		t.Fatalf("%s", data)
	}
}

func TestUpdateFileMissing(t *testing.T) {
	if err := readme.UpdateFile(filepath.Join(t.TempDir(), "nope.md"), "waka", "x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateFileMissingMarkers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	if err := os.WriteFile(path, []byte("# bare"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := readme.UpdateFile(path, "waka", "x"); err == nil {
		t.Fatal("expected marker error")
	}
}
