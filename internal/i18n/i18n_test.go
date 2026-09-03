package i18n_test

import (
	"testing"

	"github.com/vihuvac/read-waka-stats/internal/i18n"
)

func TestLoadEnglishAndFallback(t *testing.T) {
	en, err := i18n.Load("en")
	if err != nil {
		t.Fatal(err)
	}
	if en.T("Languages") != "Programming Languages" {
		t.Fatalf("Languages = %q", en.T("Languages"))
	}
	if en.T("missing-key") != "missing-key" {
		t.Fatalf("missing key should echo, got %q", en.T("missing-key"))
	}

	unknown, err := i18n.Load("zz")
	if err != nil {
		t.Fatal(err)
	}
	if unknown.Locale() != "en" {
		t.Fatalf("fallback locale = %q", unknown.Locale())
	}

	es, err := i18n.Load("es")
	if err != nil {
		t.Fatal(err)
	}
	if es.Locale() != "es" {
		t.Fatalf("es locale = %q", es.Locale())
	}
}
