package githubx_test

import (
	"testing"

	"github.com/vihuvac/read-waka-stats/internal/githubx"
)

func TestParseLinguistColors(t *testing.T) {
	yml := `
Go:
  type: programming
  color: "#00ADD8"
Python:
  color: '#3572A5'
`
	got := githubx.ParseLinguistColors(yml)
	if got["Go"] != "#00ADD8" {
		t.Fatalf("Go=%q", got["Go"])
	}
	if got["Python"] != "#3572A5" {
		t.Fatalf("Python=%q", got["Python"])
	}
}
