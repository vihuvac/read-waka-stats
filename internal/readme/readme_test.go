package readme_test

import (
	"strings"
	"testing"

	"github.com/vihuvac/read-waka-stats/internal/readme"
)

func TestReplaceSection(t *testing.T) {
	in := "# Title\n<!--START_SECTION:waka-->\nold\n<!--END_SECTION:waka-->\nfooter\n"
	out, ok, err := readme.Replace(in, "waka", "new stats")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected replacement")
	}
	if !strings.Contains(out, "new stats") {
		t.Fatalf("missing stats: %s", out)
	}
	if strings.Contains(out, "old") {
		t.Fatal("old content remained")
	}
	if !strings.Contains(out, "footer") {
		t.Fatal("footer lost")
	}
}

func TestReplaceMissingMarkers(t *testing.T) {
	_, _, err := readme.Replace("# no markers", "waka", "x")
	if err == nil {
		t.Fatal("expected error")
	}
}
