// Package logparser parses log.md bodies into structured LogEntry slices.
package logparser

import (
	"regexp"
	"strings"
)

// LogEntry represents a single entry from log.md.
type LogEntry struct {
	Date   string `json:"date"`              // YYYY-MM-DD
	Action string `json:"action"`            // "Creation", "Update", etc.
	Target string `json:"target"`            // target file path
	Detail string `json:"detail,omitempty"`  // full description
}

// dateHeading matches ## YYYY-MM-DD lines.
var dateHeading = regexp.MustCompile(`^## (\d{4}-\d{2}-\d{2})$`)

// entryLine matches **Action**: `target` — detail lines.
var entryLine = regexp.MustCompile(`^\*\*(\w+)\*\*:\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*—\s*(.+)`)

// Parse scans the body of a log.md file (after frontmatter) and returns
// LogEntry slices in document order. Lines before the first date heading
// are skipped. Non-matching lines within a date section become Detail of
// the preceding entry. Empty or malformed input returns nil.
func Parse(body string) []LogEntry {
	if body == "" {
		return nil
	}

	var entries []LogEntry
	var currentDate string
	var currentIdx int // index of the entry we're appending Detail to (-1 = none)
	hasDate := false

	for _, line := range strings.Split(body, "\n") {
		// Check for date heading
		if m := dateHeading.FindStringSubmatch(line); m != nil {
			currentDate = m[1]
			hasDate = true
			currentIdx = -1 // reset: no entry yet under this date
			continue
		}

		// Skip everything before the first date heading
		if !hasDate {
			continue
		}

		// Check for entry line
		if m := entryLine.FindStringSubmatch(line); m != nil {
			entries = append(entries, LogEntry{
				Date:   currentDate,
				Action: m[1],
				Target: m[2],
				Detail: strings.TrimSpace(m[3]),
			})
			currentIdx = len(entries) - 1
			continue
		}

		// Non-matching line becomes Detail of the preceding entry
		if currentIdx >= 0 && strings.TrimSpace(line) != "" {
			trimmed := strings.TrimSpace(line)
			if entries[currentIdx].Detail == "" {
				entries[currentIdx].Detail = trimmed
			} else {
				entries[currentIdx].Detail += "\n" + trimmed
			}
		}
	}

	return entries
}
