// Package validator checks markdown files for frontmatter conformance.
package validator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nrkno/plattform-okf-mcp/internal/index"
	"github.com/nrkno/plattform-okf-mcp/internal/parser"
	"gopkg.in/yaml.v3"
)

// Severity classifies a validation finding.
type Severity int

const (
	SeverityError       Severity = iota // Blocks conformity — E0, E1, E2, E3
	SeverityWarning                     // Quality issue — W1, W2, W3, W4
	SeverityNotification                // Informational — N1, N2, N3
)

// Finding is a single validation result.
type Finding struct {
	Code     string   // e.g. "E1", "E2", "W1", "N1"
	Severity Severity // Error, Warning, or Notification
	File     string   // path as received by the validator
	Line     int      // 0 if not line-specific
	Message  string   // human-readable description
}

// Result holds all findings for a validation run.
type Result struct {
	Findings []Finding
	Summary  Summary
}

// Summary counts files and findings by severity.
type Summary struct {
	Files         int
	Errors        int
	Warnings      int
	Notifications int
}

// ValidateDoc validates a regular (non-reserved) document file against OKF rules.
//
// It reads the file from disk, calls parser.DetectFrontmatter (I-15 single source
// of truth for frontmatter detection), and checks:
//
//	E0 — read failure (os.ReadFile error)
//	E1 — missing frontmatter
//	E2 — empty type field
//	W1 — missing title
//	W2 — missing description
//	W3 — unknown type (not in knownTypes list)
//	W4 — missing tags
//	N1 — single-tag collection
//
// Returns ([]Finding, error) per the design spec. The error is non-nil only
// on file read failure; callers should treat it as E0 equivalent.
func ValidateDoc(absPath string, knownTypes []string) ([]Finding, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return []Finding{{
			Code:     "E0",
			Severity: SeverityError,
			File:     absPath,
			Message:  fmt.Sprintf("read error: %v", err),
		}}, err
	}

	content := string(data)

	// I-15: parser.DetectFrontmatter is the single source of truth for
	// frontmatter detection. Never reimplement the "---\n" check.
	fmInfo := parser.DetectFrontmatter(content)
	if !fmInfo.HasFrontmatter {
		return []Finding{{
			Code:     "E1",
			Severity: SeverityError,
			File:     absPath,
			Message:  "missing YAML frontmatter (file must start with ---)",
		}}, nil
	}

	// Parse YAML to check E2 and warnings.
	var fm struct {
		Title       string   `yaml:"title"`
		Description string   `yaml:"description"`
		Type        string   `yaml:"type"`
		Tags        []string `yaml:"tags"`
	}
	if err := yaml.Unmarshal([]byte(fmInfo.YAMLBlock), &fm); err != nil {
		return []Finding{{
			Code:     "E2",
			Severity: SeverityError,
			File:     absPath,
			Message:  fmt.Sprintf("invalid YAML frontmatter: %v", err),
		}}, nil
	}

	var findings []Finding

	// E2: empty type field.
	if fm.Type == "" {
		findings = append(findings, Finding{
			Code:     "E2",
			Severity: SeverityError,
			File:     absPath,
			Message:  "empty or missing type field in frontmatter",
		})
	}

	// If type is empty, no further checks apply — type is the minimum gate.
	if fm.Type == "" {
		return findings, nil
	}

	// W1: missing title.
	if fm.Title == "" {
		findings = append(findings, Finding{
			Code:     "W1",
			Severity: SeverityWarning,
			File:     absPath,
			Message:  "missing title in frontmatter",
		})
	}

	// W2: missing description.
	if fm.Description == "" {
		findings = append(findings, Finding{
			Code:     "W2",
			Severity: SeverityWarning,
			File:     absPath,
			Message:  "missing description in frontmatter",
		})
	}

	// W3: unknown type (only checked when knownTypes is provided).
	if len(knownTypes) > 0 && !containsType(knownTypes, fm.Type) {
		findings = append(findings, Finding{
			Code:     "W3",
			Severity: SeverityWarning,
			File:     absPath,
			Message:  fmt.Sprintf("unknown type %q (not in known vocabulary)", fm.Type),
		})
	}

	// W4: missing tags.
	if len(fm.Tags) == 0 {
		findings = append(findings, Finding{
			Code:     "W4",
			Severity: SeverityWarning,
			File:     absPath,
			Message:  "no tags in frontmatter",
		})
	}

	// N1: single-tag collection.
	if len(fm.Tags) == 1 {
		findings = append(findings, Finding{
			Code:     "N1",
			Severity: SeverityNotification,
			File:     absPath,
			Message:  fmt.Sprintf("only one tag %q — collections typically have multiple tags", fm.Tags[0]),
		})
	}

	return findings, nil
}

// ValidateReserved validates a reserved file (index.md or log.md).
//
// Checks only E3 with per-filename logic — no E1/E2. Reserved status is
// determined by filepath.Base(relPath).
//
//	index.md — must NOT have frontmatter (E3 if it does)
//	log.md   — must have frontmatter with type: "Log" (E3 if not)
//
// For log.md, also checks N2 (cross-links) and N3 (reverse-chronological order).
func ValidateReserved(absPath string, relPath string) ([]Finding, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return []Finding{{
			Code:     "E0",
			Severity: SeverityError,
			File:     relPath,
			Message:  fmt.Sprintf("read error: %v", err),
		}}, err
	}

	content := string(data)
	fmInfo := parser.DetectFrontmatter(content)

	switch name := filepath.Base(relPath); name {
	case "index.md":
		return validateIndex(absPath, relPath, fmInfo)
	case "log.md":
		return validateLog(absPath, relPath, fmInfo, content)
	default:
		return nil, nil
	}
}

// validateIndex checks E3 for index.md: must NOT have frontmatter.
func validateIndex(absPath, relPath string, fmInfo parser.FrontmatterInfo) ([]Finding, error) {
	if fmInfo.HasFrontmatter {
		return []Finding{{
			Code:     "E3",
			Severity: SeverityError,
			File:     relPath,
			Message:  "index.md must not have frontmatter (OKF spec)",
		}}, nil
	}
	return nil, nil
}

// validateLog checks E3 for log.md: must have frontmatter with type: "Log".
// Also checks N2 (cross-links) and N3 (reverse-chronological order).
func validateLog(absPath, relPath string, fmInfo parser.FrontmatterInfo, content string) ([]Finding, error) {
	var findings []Finding

	if !fmInfo.HasFrontmatter {
		findings = append(findings, Finding{
			Code:     "E3",
			Severity: SeverityError,
			File:     relPath,
			Message:  "log.md must have frontmatter with type: Log",
		})
		return findings, nil
	}

	// Check type: Log in frontmatter.
	var fm struct {
		Type string `yaml:"type"`
	}
	if err := yaml.Unmarshal([]byte(fmInfo.YAMLBlock), &fm); err != nil {
		findings = append(findings, Finding{
			Code:     "E3",
			Severity: SeverityError,
			File:     relPath,
			Message:  "log.md has invalid YAML frontmatter",
		})
		return findings, nil
	}
	if fm.Type != "Log" {
		findings = append(findings, Finding{
			Code:     "E3",
			Severity: SeverityError,
			File:     relPath,
			Message:  fmt.Sprintf("log.md type must be \"Log\", got %q", fm.Type),
		})
		return findings, nil
	}

	// N2: check for cross-links in body.
	body := content[fmInfo.BodyOffset:]
	if !strings.Contains(body, "](/") {
		findings = append(findings, Finding{
			Code:     "N2",
			Severity: SeverityNotification,
			File:     relPath,
			Message:  "log.md has no bundle-relative cross-links",
		})
	}

	// N3: check log entries are reverse-chronological.
	if !logEntriesChronological(body) {
		findings = append(findings, Finding{
			Code:     "N3",
			Severity: SeverityNotification,
			File:     relPath,
			Message:  "log.md date headings are not in reverse-chronological order",
		})
	}

	return findings, nil
}

// logDatePattern matches ## YYYY-MM-DD heading lines in log.md body.
var logDatePattern = regexp.MustCompile(`^## (\d{4}-\d{2}-\d{2})$`)

// logEntriesChronological checks that date headings in the log body are in
// reverse-chronological order (newest first). Empty body or no date headings
// is not an error — returns true.
func logEntriesChronological(body string) bool {
	var dates []string
	for _, line := range strings.Split(body, "\n") {
		if m := logDatePattern.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			dates = append(dates, m[1])
		}
	}
	if len(dates) <= 1 {
		return true
	}
	for i := 1; i < len(dates); i++ {
		if dates[i] > dates[i-1] {
			return false // older date before newer date — wrong order
		}
	}
	return true
}

// ValidateBundle validates all files in the index.
//
// It rebuilds the index, then dispatches:
//   - ValidateDoc for each indexed doc (using filepath.Join(idx.Dir(), relPath))
//   - ValidateReserved for each reserved file
//
// F4 fix: doc.FilePath is relative; must convert to absolute via filepath.Join.
func ValidateBundle(idx *index.Index) Result {
	if err := idx.Rebuild(); err != nil {
		return Result{
			Summary: Summary{
				Errors: 1,
			},
			Findings: []Finding{
				{Code: "REBUILD_ERROR", Severity: SeverityError, Message: err.Error()},
			},
		}
	}
	knownTypes := uniqueTypes(idx.Docs())
	var findings []Finding

	for _, doc := range idx.Docs() {
		absPath := filepath.Join(idx.Dir(), doc.FilePath)
		fs, err := ValidateDoc(absPath, knownTypes)
		if err != nil {
			findings = append(findings, fs...)
			continue
		}
		findings = append(findings, fs...)
	}

	for _, r := range idx.Reserved() {
		absPath := filepath.Join(idx.Dir(), r.FilePath)
		fs, err := ValidateReserved(absPath, r.FilePath)
		if err != nil {
			findings = append(findings, fs...)
			continue
		}
		findings = append(findings, fs...)
	}

	return buildResult(findings, len(idx.Docs())+len(idx.Reserved()))
}

// ValidatePaths validates specified absolute file paths.
//
// It determines reserved status by basename matching (index.md, log.md)
// and dispatches to ValidateDoc or ValidateReserved accordingly.
func ValidatePaths(paths []string, knownTypes []string) Result {
	var findings []Finding

	for _, absPath := range paths {
		name := filepath.Base(absPath)
		if reservedBasename(name) {
			fs, err := ValidateReserved(absPath, name)
			if err != nil {
				findings = append(findings, fs...)
				continue
			}
			findings = append(findings, fs...)
		} else {
			fs, err := ValidateDoc(absPath, knownTypes)
			if err != nil {
				findings = append(findings, fs...)
				continue
			}
			findings = append(findings, fs...)
		}
	}

	return buildResult(findings, len(paths))
}

// reservedBasename determines if a file's basename marks it as a reserved file.
func reservedBasename(basename string) bool {
	return basename == "index.md" || basename == "log.md"
}

// containsType reports whether typ is in the knownTypes slice.
func containsType(knownTypes []string, typ string) bool {
	for _, kt := range knownTypes {
		if kt == typ {
			return true
		}
	}
	return false
}

// uniqueTypes extracts a deduplicated list of type values from the doc slice.
func uniqueTypes(docs []parser.Doc) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, doc := range docs {
		if doc.Type == "" {
			continue
		}
		if _, ok := seen[doc.Type]; ok {
			continue
		}
		seen[doc.Type] = struct{}{}
		out = append(out, doc.Type)
	}
	return out
}

// buildResult constructs a Result from a flat finding slice.
func buildResult(findings []Finding, files int) Result {
	var s Summary
	s.Files = files
	for _, f := range findings {
		switch f.Severity {
		case SeverityError:
			s.Errors++
		case SeverityWarning:
			s.Warnings++
		case SeverityNotification:
			s.Notifications++
		}
	}
	return Result{Findings: findings, Summary: s}
}
