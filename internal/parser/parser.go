package parser

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Doc holds the indexed metadata for a single markdown file.
type Doc struct {
	Title       string
	Description string
	Type        string
	Tags        []string
	FilePath    string // set from the path argument, not from frontmatter
	// BodyOffset is the byte offset of the first character after the closing
	// "---\n" delimiter in the raw file bytes — i.e. where the markdown body
	// starts. Slicing fileContent[doc.BodyOffset:] yields the body only.
	// If the file has no body (frontmatter only), BodyOffset equals len(fileContent)
	// so the slice returns "".
	BodyOffset int
}

// frontmatter is the YAML structure we unmarshal into.
type frontmatter struct {
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Type        string   `yaml:"type"`
	Tags        []string `yaml:"tags"`
}

// Parse reads the file at path, extracts YAML frontmatter, and returns a Doc.
//
// Returns (Doc{}, false, nil) if the file has no "---" frontmatter or is missing
// the "type" field (invariant I-3).
// Returns (Doc{}, false, err) on file read or YAML parse error.
// Warns to stderr if title or description is missing, but still returns (doc, true, nil).
// doc.FilePath is always set to the path argument.
func Parse(path string) (Doc, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Doc{}, false, err
	}

	content := string(data)

	// File must begin with "---\n"
	if !strings.HasPrefix(content, "---\n") {
		return Doc{}, false, nil
	}

	// Find the closing "---" delimiter (a line that is exactly "---")
	rest := content[len("---\n"):]
	end := findClosingDelimiter(rest)
	if end < 0 {
		// No closing delimiter — treat entire remainder as body, no frontmatter
		return Doc{}, false, nil
	}

	yamlBlock := rest[:end]

	var fm frontmatter
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		return Doc{}, false, err
	}

	// type is required (invariant I-3)
	if fm.Type == "" {
		return Doc{}, false, nil
	}

	// BodyOffset: opening "---\n" (4 bytes) + yaml block + closing "---\n" (4 bytes).
	// Clamp to len(content) so that a file with no newline after the closing "---"
	// (or no body at all) slices to "" rather than panicking.
	bodyOffset := 4 + end + 4
	if bodyOffset > len(content) {
		bodyOffset = len(content)
	}

	doc := Doc{
		Title:       fm.Title,
		Description: fm.Description,
		Type:        fm.Type,
		Tags:        fm.Tags,
		FilePath:    path,
		BodyOffset:  bodyOffset,
	}

	if fm.Title == "" {
		fmt.Fprintf(os.Stderr, "okf-mcp: WARN: %s: missing title\n", path)
	}
	if fm.Description == "" {
		fmt.Fprintf(os.Stderr, "okf-mcp: WARN: %s: missing description\n", path)
	}

	return doc, true, nil
}

// findClosingDelimiter returns the byte offset of the start of the closing "---"
// line within s (where s is everything after the opening "---\n").
// It returns -1 if no such line exists.
func findClosingDelimiter(s string) int {
	offset := 0
	for {
		// Find next newline
		idx := strings.Index(s[offset:], "\n")
		var line string
		if idx < 0 {
			// Last line, no trailing newline
			line = s[offset:]
		} else {
			line = s[offset : offset+idx]
		}

		if line == "---" {
			return offset
		}

		if idx < 0 {
			break
		}
		offset += idx + 1
	}
	return -1
}
