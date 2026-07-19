package parser

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile is a test helper that writes content to a named file in dir.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	return path
}

func TestParse_ValidAllFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeFile(t, dir, "doc.md", "---\ntype: guide\ntitle: My Title\ndescription: My description\ntags:\n  - alpha\n  - beta\n---\n# Body content here\n")

	doc, ok, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true, got false")
	}
	if doc.Type != "guide" {
		t.Errorf("Type = %q, want %q", doc.Type, "guide")
	}
	if doc.Title != "My Title" {
		t.Errorf("Title = %q, want %q", doc.Title, "My Title")
	}
	if doc.Description != "My description" {
		t.Errorf("Description = %q, want %q", doc.Description, "My description")
	}
	if len(doc.Tags) != 2 || doc.Tags[0] != "alpha" || doc.Tags[1] != "beta" {
		t.Errorf("Tags = %v, want [alpha beta]", doc.Tags)
	}
	if doc.FilePath != path {
		t.Errorf("FilePath = %q, want %q", doc.FilePath, path)
	}
}

func TestParse_MissingTypeField(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeFile(t, dir, "doc.md", "---\ntitle: Some Title\ndescription: Some description\n---\n")

	_, ok, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false when type is missing, got true")
	}
}

func TestParse_EmptyTypeField(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeFile(t, dir, "doc.md", "---\ntype: \"\"\ntitle: Some Title\ndescription: Some description\n---\n")

	_, ok, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false when type is empty string, got true")
	}
}

func TestParse_NoFrontmatterDelimiter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeFile(t, dir, "doc.md", "# Just a heading\n\nNo frontmatter at all.\n")

	_, ok, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for file without frontmatter, got true")
	}
}

func TestParse_ContentButNoOpeningDelimiter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// File starts with content (not "---\n")
	path := writeFile(t, dir, "doc.md", "type: guide\ntitle: My Title\n---\nbody\n")

	_, ok, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for file not starting with ---, got true")
	}
}

func TestParse_MissingTitle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeFile(t, dir, "doc.md", "---\ntype: guide\ndescription: Has description but no title\n---\n")

	doc, ok, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true when title is missing (not a skip condition), got false")
	}
	if doc.Title != "" {
		t.Errorf("Title = %q, want empty string", doc.Title)
	}
	if doc.Type != "guide" {
		t.Errorf("Type = %q, want %q", doc.Type, "guide")
	}
}

func TestParse_MissingDescription(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeFile(t, dir, "doc.md", "---\ntype: guide\ntitle: Has Title\n---\n")

	doc, ok, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true when description is missing (not a skip condition), got false")
	}
	if doc.Description != "" {
		t.Errorf("Description = %q, want empty string", doc.Description)
	}
	if doc.Type != "guide" {
		t.Errorf("Type = %q, want %q", doc.Type, "guide")
	}
}

func TestParse_MissingTags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeFile(t, dir, "doc.md", "---\ntype: guide\ntitle: Some Title\ndescription: Some description\n---\n")

	doc, ok, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true when tags are missing, got false")
	}
	if len(doc.Tags) != 0 {
		t.Errorf("Tags = %v, want nil or empty", doc.Tags)
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Invalid YAML: mismatched bracket
	path := writeFile(t, dir, "doc.md", "---\ntype: guide\n  title: [invalid: yaml: {broken\n---\n")

	_, ok, err := Parse(path)
	if err == nil {
		t.Fatal("expected non-nil error for invalid YAML, got nil")
	}
	if ok {
		t.Fatal("expected ok=false for invalid YAML, got true")
	}
}

func TestParse_FilePathEqualsArgument(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeFile(t, dir, "myfile.md", "---\ntype: reference\ntitle: Title\ndescription: Desc\n---\n")

	doc, ok, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if doc.FilePath != path {
		t.Errorf("FilePath = %q, want %q", doc.FilePath, path)
	}
}

func TestParse_BodyAfterSecondDelimiterIgnored(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	body := "---\ntype: tutorial\ntitle: Tutorial Title\ndescription: Tutorial description\ntags:\n  - go\n  - testing\n---\n# Body\ntype: should-not-be-parsed\ntitle: wrong title\n"
	path := writeFile(t, dir, "doc.md", body)

	doc, ok, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if doc.Type != "tutorial" {
		t.Errorf("Type = %q, want %q", doc.Type, "tutorial")
	}
	if doc.Title != "Tutorial Title" {
		t.Errorf("Title = %q, want %q", doc.Title, "Tutorial Title")
	}
	if doc.Description != "Tutorial description" {
		t.Errorf("Description = %q, want %q", doc.Description, "Tutorial description")
	}
	if len(doc.Tags) != 2 {
		t.Errorf("Tags = %v, want [go testing]", doc.Tags)
	}
}

func TestParse_NonExistentFile(t *testing.T) {
	t.Parallel()
	_, ok, err := Parse("/nonexistent/path/that/does/not/exist.md")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
	if ok {
		t.Fatal("expected ok=false for non-existent file, got true")
	}
}

// ---------------------------------------------------------------------------
// BodyOffset tests
// ---------------------------------------------------------------------------

// TestParse_BodyOffset_WithBody verifies that BodyOffset points past the
// closing "---\n" delimiter so that fileContent[BodyOffset:] equals the body.
func TestParse_BodyOffset_WithBody(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const body = "# Heading\n\nSome content.\n"
	raw := "---\ntype: guide\ntitle: T\ndescription: D\n---\n" + body
	path := writeFile(t, dir, "doc.md", raw)

	doc, ok, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}

	if doc.BodyOffset > len(raw) {
		t.Fatalf("BodyOffset %d > file length %d", doc.BodyOffset, len(raw))
	}
	got := raw[doc.BodyOffset:]
	if got != body {
		t.Errorf("raw[BodyOffset:] = %q, want %q", got, body)
	}
}

// TestParse_BodyOffset_NoBody verifies that BodyOffset equals len(fileContent)
// when the file has no body after the closing "---\n".
func TestParse_BodyOffset_NoBody(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	raw := "---\ntype: guide\ntitle: T\ndescription: D\n---\n"
	path := writeFile(t, dir, "doc.md", raw)

	doc, ok, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}

	if doc.BodyOffset != len(raw) {
		t.Errorf("BodyOffset = %d, want %d (len of file with no body)", doc.BodyOffset, len(raw))
	}
	got := raw[doc.BodyOffset:]
	if got != "" {
		t.Errorf("raw[BodyOffset:] = %q, want empty string", got)
	}
}

// TestParse_BodyOffset_NoFrontmatter verifies that Parse returns ok=false for
// a file without frontmatter, and that BodyOffset is irrelevant (zero value).
func TestParse_BodyOffset_NoFrontmatter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeFile(t, dir, "doc.md", "# No frontmatter here\n")

	_, ok, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for file without frontmatter")
	}
	// BodyOffset is zero-valued and irrelevant when ok=false; no assertion needed.
}

func TestParse_NoPanicOnMalformedFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cases := []struct {
		name    string
		content string
	}{
		{"empty", ""},
		{"only_dashes", "---"},
		{"only_opening_delimiter", "---\n"},
		{"opening_no_close", "---\ntype: guide\ntitle: X\n"},
		{"binary_like", "---\n\x00\x01\x02\n---\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := writeFile(t, dir, tc.name+".md", tc.content)
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Parse panicked: %v", r)
					}
				}()
				Parse(path) //nolint:errcheck // panic-guard only
			}()
		})
	}
}

// ---------------------------------------------------------------------------
// DetectFrontmatter tests
// ---------------------------------------------------------------------------

func TestDetectFrontmatter_Valid(t *testing.T) {
	t.Parallel()
	content := "---\ntype: guide\ntitle: My Title\n---\n# Body\n"
	info := DetectFrontmatter(content)
	if !info.HasFrontmatter {
		t.Fatal("expected HasFrontmatter=true, got false")
	}
	wantYAML := "type: guide\ntitle: My Title\n"
	if info.YAMLBlock != wantYAML {
		t.Errorf("YAMLBlock = %q, want %q", info.YAMLBlock, wantYAML)
	}
	// opening "---\n" (4) + YAML (26) + closing "---\n" (4) = 34
	wantOffset := 4 + len(wantYAML) + 4
	if info.BodyOffset != wantOffset {
		t.Errorf("BodyOffset = %d, want %d", info.BodyOffset, wantOffset)
	}
	got := content[info.BodyOffset:]
	if got != "# Body\n" {
		t.Errorf("content[BodyOffset:] = %q, want %q", got, "# Body\n")
	}
}

func TestDetectFrontmatter_MissingClosingDelimiter(t *testing.T) {
	t.Parallel()
	content := "---\ntype: guide\ntitle: My Title\n"
	info := DetectFrontmatter(content)
	if info.HasFrontmatter {
		t.Fatal("expected HasFrontmatter=false for missing closing delimiter, got true")
	}
	if info.YAMLBlock != "" {
		t.Errorf("YAMLBlock = %q, want empty", info.YAMLBlock)
	}
	if info.BodyOffset != 0 {
		t.Errorf("BodyOffset = %d, want 0", info.BodyOffset)
	}
}

func TestDetectFrontmatter_NoFrontmatter(t *testing.T) {
	t.Parallel()
	content := "# Just a heading\n\nNo frontmatter.\n"
	info := DetectFrontmatter(content)
	if info.HasFrontmatter {
		t.Fatal("expected HasFrontmatter=false, got true")
	}
	if info.YAMLBlock != "" {
		t.Errorf("YAMLBlock = %q, want empty", info.YAMLBlock)
	}
	if info.BodyOffset != 0 {
		t.Errorf("BodyOffset = %d, want 0", info.BodyOffset)
	}
}

func TestDetectFrontmatter_EmptyContent(t *testing.T) {
	t.Parallel()
	info := DetectFrontmatter("")
	if info.HasFrontmatter {
		t.Fatal("expected HasFrontmatter=false for empty content, got true")
	}
	if info.YAMLBlock != "" {
		t.Errorf("YAMLBlock = %q, want empty", info.YAMLBlock)
	}
	if info.BodyOffset != 0 {
		t.Errorf("BodyOffset = %d, want 0", info.BodyOffset)
	}
}

func TestDetectFrontmatter_FrontmatterOnlyNoBody(t *testing.T) {
	t.Parallel()
	content := "---\ntype: guide\n---\n"
	info := DetectFrontmatter(content)
	if !info.HasFrontmatter {
		t.Fatal("expected HasFrontmatter=true, got false")
	}
	wantYAML := "type: guide\n"
	if info.YAMLBlock != wantYAML {
		t.Errorf("YAMLBlock = %q, want %q", info.YAMLBlock, wantYAML)
	}
	if info.BodyOffset != len(content) {
		t.Errorf("BodyOffset = %d, want %d (len of content with no body)", info.BodyOffset, len(content))
	}
	got := content[info.BodyOffset:]
	if got != "" {
		t.Errorf("content[BodyOffset:] = %q, want empty string", got)
	}
}
