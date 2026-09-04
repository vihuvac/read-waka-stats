package i18n

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnglishAndFallback(t *testing.T) {
	en, err := Load("en")
	if err != nil {
		t.Fatal(err)
	}
	if en.T("Languages") != "Programming Languages" {
		t.Fatalf("Languages = %q", en.T("Languages"))
	}
	if en.T("missing-key") != "missing-key" {
		t.Fatalf("missing key should echo, got %q", en.T("missing-key"))
	}

	unknown, err := Load("zz")
	if err != nil {
		t.Fatal(err)
	}
	if unknown.Locale() != "en" {
		t.Fatalf("fallback locale = %q", unknown.Locale())
	}

	es, err := Load("es")
	if err != nil {
		t.Fatal(err)
	}
	if es.Locale() != "es" {
		t.Fatalf("es locale = %q", es.Locale())
	}
}

func TestNilBundle(t *testing.T) {
	var b *Bundle
	if b.T("x") != "x" {
		t.Fatal(b.T("x"))
	}
	if b.Locale() != "en" {
		t.Fatal(b.Locale())
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.json")
	raw := `{"en":{"Hi":"Hello"},"fr":{"Hi":"Bonjour"}}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	fr, err := LoadFromFile(path, "fr")
	if err != nil {
		t.Fatal(err)
	}
	if fr.T("Hi") != "Bonjour" || fr.Locale() != "fr" {
		t.Fatalf("%s %s", fr.T("Hi"), fr.Locale())
	}
	fallback, err := LoadFromFile(path, "de")
	if err != nil {
		t.Fatal(err)
	}
	if fallback.Locale() != "en" || fallback.T("Hi") != "Hello" {
		t.Fatalf("%s %s", fallback.Locale(), fallback.T("Hi"))
	}
	if _, err := LoadFromFile(filepath.Join(dir, "missing.json"), "en"); err == nil {
		t.Fatal("expected error")
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFromFile(bad, "en"); err == nil {
		t.Fatal("expected json error")
	}
}

func TestLoadParseAndMissingEnglish(t *testing.T) {
	orig := translationJSON
	t.Cleanup(func() { translationJSON = orig })

	translationJSON = []byte(`{`)
	if _, err := Load("en"); err == nil {
		t.Fatal("expected parse error")
	}

	translationJSON = []byte(`{"fr":{"Hi":"Bonjour"}}`)
	if _, err := Load("en"); err == nil {
		t.Fatal("expected missing english")
	}
	if _, err := Load("zz"); err == nil {
		t.Fatal("expected missing english fallback")
	}
	fr, err := Load("fr")
	if err != nil {
		t.Fatal(err)
	}
	if fr.T("Hi") != "Bonjour" {
		t.Fatalf("%q", fr.T("Hi"))
	}
}
