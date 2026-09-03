// Package i18n loads embedded UI translations.
package i18n

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
)

//go:embed translation.json
var translationJSON []byte

// Bundle maps localization keys to translated strings.
type Bundle struct {
	locale string
	table  map[string]string
}

// Load returns translations for locale, falling back to English.
func Load(locale string) (*Bundle, error) {
	var all map[string]map[string]string
	if err := json.Unmarshal(translationJSON, &all); err != nil {
		return nil, fmt.Errorf("parse translations: %w", err)
	}
	table, ok := all[locale]
	if !ok {
		table = all["en"]
		locale = "en"
	}
	if table == nil {
		return nil, fmt.Errorf("english translations missing")
	}
	return &Bundle{locale: locale, table: table}, nil
}

// T returns the translation for key, or the key itself if missing.
func (b *Bundle) T(key string) string {
	if b == nil {
		return key
	}
	if v, ok := b.table[key]; ok {
		return v
	}
	return key
}

// Locale returns the resolved locale.
func (b *Bundle) Locale() string {
	if b == nil {
		return "en"
	}
	return b.locale
}

// LoadFromFile is used in tests to load an alternate translation file.
func LoadFromFile(path, locale string) (*Bundle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var all map[string]map[string]string
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	table := all[locale]
	if table == nil {
		table = all["en"]
		locale = "en"
	}
	return &Bundle{locale: locale, table: table}, nil
}
