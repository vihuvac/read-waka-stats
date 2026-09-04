// Package readme replaces the marked section in a README file.
package readme

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Markers returns the HTML comment pair for a section name.
func Markers(section string) (start, end string) {
	return fmt.Sprintf("<!--START_SECTION:%s-->", section), fmt.Sprintf("<!--END_SECTION:%s-->", section)
}

// Replace injects stats between START/END comments. Returns the updated contents
// and whether a replacement occurred.
func Replace(contents, section, stats string) (string, bool, error) {
	start, end := Markers(section)
	pattern := regexp.QuoteMeta(start) + `[\s\S]*?` + regexp.QuoteMeta(end)
	re := regexp.MustCompile(pattern)
	if !re.MatchString(contents) {
		return contents, false, fmt.Errorf("section markers %s ... %s not found", start, end)
	}
	block := start + "\n" + strings.TrimSpace(stats) + "\n" + end
	return re.ReplaceAllString(contents, block), true, nil
}

// UpdateFile reads path, replaces the section, and writes it back.
func UpdateFile(path, section, stats string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated, _, err := Replace(string(data), section, stats)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(updated), 0o644)
}
