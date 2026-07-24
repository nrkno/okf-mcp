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
	Bundle      string // set by index package during Rebuild, not by the parser
}

// FrontmatterInfo holds the result of frontmatter detection.
type FrontmatterInfo struct {
	HasFrontmatter bool
	YAMLBlock      string // raw YAML between delimiters (empty if !HasFrontmatter)
	BodyOffset     int    // byte offset of markdown body start
}

// frontmatter is the YAML structure we unmarshal into.
type frontmatter struct {
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Type        string   `yaml:"type"`
	Tags        []string `yaml:"tags"`
}

// DetectFrontmatter checks if content starts with "---\n" and has a closing
// "---" delimiter. Single source of truth for frontmatter detection.
func DetectFrontmatter(content string) FrontmatterInfo {
	if !strings.HasPrefix(content, "---\n") {
		return FrontmatterInfo{}
	}
	rest := content[len("---\n"):]
	end := findClosingDelimiter(rest)
	if end < 0 {
		return FrontmatterInfo{}
	}
	bodyOffset := 4 + end + 4
	if bodyOffset > len(content) {
		bodyOffset = len(content)
	}
	return FrontmatterInfo{
		HasFrontmatter: true,
		YAMLBlock:      rest[:end],
		BodyOffset:     bodyOffset,
	}
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

	fmInfo := DetectFrontmatter(content)
	if !fmInfo.HasFrontmatter {
		return Doc{}, false, nil
	}

	var fm frontmatter
	if err := yaml.Unmarshal([]byte(fmInfo.YAMLBlock), &fm); err != nil {
		return Doc{}, false, err
	}

	// type is required (invariant I-3)
	if fm.Type == "" {
		return Doc{}, false, nil
	}

	doc := Doc{
		Title:       fm.Title,
		Description: fm.Description,
		Type:        fm.Type,
		Tags:        fm.Tags,
		FilePath:    path,
		BodyOffset:  fmInfo.BodyOffset,
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
