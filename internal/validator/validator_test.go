package validator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nrkno/plattform-okf-mcp/internal/index"
	"github.com/nrkno/plattform-okf-mcp/internal/parser"
)

// writeFile is a test helper that writes content to a file in dir.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	return path
}

// hasCode checks if findings contain a specific code.
func hasCode(findings []Finding, code string) bool {
	for _, f := range findings {
		if f.Code == code {
			return true
		}
	}
	return false
}


func TestValidateDoc_ValidDoc(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "guide.md", "---\ntype: guide\ntitle: My Guide\ndescription: A guide\ntags:\n  - go\n  - testing\n---\n# Body\n")
	findings, err := ValidateDoc(filepath.Join(dir, "guide.md"), []string{"guide", "reference"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestValidateDoc_MissingFrontmatter_E1(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "plain.md", "# Just a heading\n\nNo frontmatter.\n")
	findings, err := ValidateDoc(filepath.Join(dir, "plain.md"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasCode(findings, "E1") {
		t.Errorf("expected E1, got findings: %v", findings)
	}
	if len(findings) != 1 {
		t.Errorf("expected exactly 1 finding (E1), got %d", len(findings))
	}
}

func TestValidateDoc_EmptyType_E2(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "notype.md", "---\ntitle: Some Title\ndescription: Some desc\n---\n")
	findings, err := ValidateDoc(filepath.Join(dir, "notype.md"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasCode(findings, "E2") {
		t.Errorf("expected E2, got findings: %v", findings)
	}
	// E2 only — no further checks when type is empty.
	if len(findings) != 1 {
		t.Errorf("expected exactly 1 finding (E2), got %d: %v", len(findings), findings)
	}
}

func TestValidateDoc_MissingTitle_W1(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "notitle.md", "---\ntype: guide\ndescription: Has desc but no title\ntags:\n  - a\n---\n")
	findings, err := ValidateDoc(filepath.Join(dir, "notitle.md"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasCode(findings, "W1") {
		t.Errorf("expected W1, got findings: %v", findings)
	}
}

func TestValidateDoc_MissingDescription_W2(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "nodesc.md", "---\ntype: guide\ntitle: Has Title\ntags:\n  - a\n---\n")
	findings, err := ValidateDoc(filepath.Join(dir, "nodesc.md"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasCode(findings, "W2") {
		t.Errorf("expected W2, got findings: %v", findings)
	}
}

func TestValidateDoc_UnknownType_W3(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "badtype.md", "---\ntype: nonexistent\ntitle: Title\ndescription: Desc\ntags:\n  - a\n---\n")
	findings, err := ValidateDoc(filepath.Join(dir, "badtype.md"), []string{"guide", "reference"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasCode(findings, "W3") {
		t.Errorf("expected W3, got findings: %v", findings)
	}
}

func TestValidateDoc_NoW3WhenKnownTypesNil(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "anything.md", "---\ntype: whatever\ntitle: Title\ndescription: Desc\ntags:\n  - a\n---\n")
	findings, err := ValidateDoc(filepath.Join(dir, "anything.md"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasCode(findings, "W3") {
		t.Errorf("expected no W3 when knownTypes is nil, got findings: %v", findings)
	}
}

func TestValidateDoc_MissingTags_W4(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "notags.md", "---\ntype: guide\ntitle: Title\ndescription: Desc\n---\n")
	findings, err := ValidateDoc(filepath.Join(dir, "notags.md"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasCode(findings, "W4") {
		t.Errorf("expected W4, got findings: %v", findings)
	}
}

func TestValidateDoc_SingleTag_N1(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "onetag.md", "---\ntype: guide\ntitle: Title\ndescription: Desc\ntags:\n  - only-one\n---\n")
	findings, err := ValidateDoc(filepath.Join(dir, "onetag.md"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasCode(findings, "N1") {
		t.Errorf("expected N1, got findings: %v", findings)
	}
}

func TestValidateDoc_NoN1WithMultipleTags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "twotags.md", "---\ntype: guide\ntitle: Title\ndescription: Desc\ntags:\n  - a\n  - b\n---\n")
	findings, err := ValidateDoc(filepath.Join(dir, "twotags.md"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasCode(findings, "N1") {
		t.Errorf("expected no N1 with multiple tags, got findings: %v", findings)
	}
}

func TestValidateDoc_E0_NonExistentFile(t *testing.T) {
	t.Parallel()
	findings, err := ValidateDoc("/nonexistent/path/doc.md", nil)
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
	if !hasCode(findings, "E0") {
		t.Errorf("expected E0, got findings: %v", findings)
	}
}

func TestValidateDoc_AllWarnings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "everything.md", "---\ntype: obscure-type\n---\n")
	findings, err := ValidateDoc(filepath.Join(dir, "everything.md"), []string{"guide"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, code := range []string{"W1", "W2", "W3", "W4"} {
		if !hasCode(findings, code) {
			t.Errorf("expected %s, got findings: %v", code, findings)
		}
	}
}

// --- ValidateReserved tests ---

func TestValidateReserved_IndexNoFrontmatter_Pass(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "# Welcome\n\nThis is the index.\n")
	findings, err := ValidateReserved(filepath.Join(dir, "index.md"), "index.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings for valid index.md, got %v", findings)
	}
}

func TestValidateReserved_IndexWithFrontmatter_E3(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "---\ntitle: Index\n---\n# Index\n")
	findings, err := ValidateReserved(filepath.Join(dir, "index.md"), "index.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasCode(findings, "E3") {
		t.Errorf("expected E3 for index.md with frontmatter, got %v", findings)
	}
}

func TestValidateReserved_LogWithTypeLog_Pass(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "log.md", "---\ntype: Log\n---\n## 2025-01-01\n**Creation**: `doc.md` — initial\n")
	findings, err := ValidateReserved(filepath.Join(dir, "log.md"), "log.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasCode(findings, "E3") {
		t.Errorf("expected no E3 for valid log.md, got %v", findings)
	}
}

func TestValidateReserved_LogWithoutType_E3(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "log.md", "---\ntitle: Log\n---\n")
	findings, err := ValidateReserved(filepath.Join(dir, "log.md"), "log.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasCode(findings, "E3") {
		t.Errorf("expected E3 for log.md with wrong type, got %v", findings)
	}
}

func TestValidateReserved_LogNoFrontmatter_E3(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "log.md", "# Just a log\n")
	findings, err := ValidateReserved(filepath.Join(dir, "log.md"), "log.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasCode(findings, "E3") {
		t.Errorf("expected E3 for log.md without frontmatter, got %v", findings)
	}
}

func TestValidateReserved_LogN2_NoCrossLinks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "log.md", "---\ntype: Log\n---\n## 2025-01-01\n**Creation**: `doc.md` — initial\n")
	findings, err := ValidateReserved(filepath.Join(dir, "log.md"), "log.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasCode(findings, "N2") {
		t.Errorf("expected N2 (no cross-links), got findings: %v", findings)
	}
}

func TestValidateReserved_LogN2_WithCrossLinks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "log.md", "---\ntype: Log\n---\n## 2025-01-01\n**Creation**: `doc.md` — see [guide](/docs/guide.md)\n")
	findings, err := ValidateReserved(filepath.Join(dir, "log.md"), "log.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasCode(findings, "N2") {
		t.Errorf("expected no N2 when cross-links present, got findings: %v", findings)
	}
}

func TestValidateReserved_LogN3_ReverseChronological_Pass(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	body := "---\ntype: Log\n---\n## 2025-06-01\n**Update**: `a.md`\n## 2025-01-01\n**Creation**: `b.md`\n"
	writeFile(t, dir, "log.md", body)
	findings, err := ValidateReserved(filepath.Join(dir, "log.md"), "log.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasCode(findings, "N3") {
		t.Errorf("expected no N3 for reverse-chronological log, got findings: %v", findings)
	}
}

func TestValidateReserved_LogN3_WrongOrder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	body := "---\ntype: Log\n---\n## 2025-01-01\n**Creation**: `b.md`\n## 2025-06-01\n**Update**: `a.md`\n"
	writeFile(t, dir, "log.md", body)
	findings, err := ValidateReserved(filepath.Join(dir, "log.md"), "log.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasCode(findings, "N3") {
		t.Errorf("expected N3 for non-chronological log, got findings: %v", findings)
	}
}

func TestValidateReserved_UnknownBasename_Pass(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "other.md", "---\ntitle: X\n---\n")
	findings, err := ValidateReserved(filepath.Join(dir, "other.md"), "other.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-reserved file, got %v", findings)
	}
}

func TestValidateReserved_IndexWithSubdir_Pass(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "# Welcome\n\nSee [docs](/docs/).\n")
	findings, err := ValidateReserved(filepath.Join(dir, "index.md"), "subdir/index.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %v", findings)
	}
}

func TestValidateReserved_E0_NonExistentFile(t *testing.T) {
	t.Parallel()
	findings, err := ValidateReserved("/nonexistent/index.md", "index.md")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
	if !hasCode(findings, "E0") {
		t.Errorf("expected E0, got findings: %v", findings)
	}
}

// --- ValidateBundle tests ---

func TestValidateBundle_IndexMDNeverTriggersE1(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "# Welcome\n\nThis is the index.\n")
	writeFile(t, dir, "guide.md", "---\ntype: guide\ntitle: Guide\ndescription: A guide\ntags:\n  - a\n---\nBody\n")
	idx := index.New(dir)
	result := ValidateBundle(idx)
	if hasCode(result.Findings, "E1") {
		t.Errorf("index.md must never trigger E1 in ValidateBundle, got findings: %v", result.Findings)
	}
}

func TestValidateBundle_FullBundleValidation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "# Index\n")
	writeFile(t, dir, "guide.md", "---\ntype: guide\ntitle: Guide\ndescription: A guide\ntags:\n  - go\n---\nBody\n")
	writeFile(t, dir, "log.md", "---\ntype: Log\n---\n## 2025-01-01\n**Creation**: `guide.md` — see [guide](/docs/guide.md)\n")
	idx := index.New(dir)
	result := ValidateBundle(idx)
	if result.Summary.Files != 3 {
		t.Errorf("expected 3 files, got %d", result.Summary.Files)
	}
	if result.Summary.Errors != 0 {
		t.Errorf("expected 0 errors for valid bundle, got %d: %v", result.Summary.Errors, result.Findings)
	}
}

func TestValidateBundle_InvalidDocInBundle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "# Index\n")
	// Doc with frontmatter but no title/description/tags — goes through the
	// index (parser.Parse keeps it) but produces warnings from ValidateDoc.
	writeFile(t, dir, "minimal.md", "---\ntype: guide\n---\n")
	idx := index.New(dir)
	result := ValidateBundle(idx)
	if !hasCode(result.Findings, "W1") {
		t.Errorf("expected W1 for doc missing title, got findings: %v", result.Findings)
	}
	if !hasCode(result.Findings, "W2") {
		t.Errorf("expected W2 for doc missing description, got findings: %v", result.Findings)
	}
	if !hasCode(result.Findings, "W4") {
		t.Errorf("expected W4 for doc missing tags, got findings: %v", result.Findings)
	}
}

func TestValidateBundle_ConvertsRelativePathsToAbsolute(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "# Index\n")
	writeFile(t, dir, "doc.md", "no frontmatter\n")
	idx := index.New(dir)
	result := ValidateBundle(idx)
	for _, f := range result.Findings {
		if f.Code == "E1" {
			if len(f.File) == 0 || f.File[0] != '/' {
				t.Errorf("expected absolute path in finding, got %q", f.File)
			}
			if filepath.Base(f.File) != "doc.md" {
				t.Errorf("expected doc.md in path, got %q", f.File)
			}
		}
	}
}

// --- ValidatePaths tests ---

func TestValidatePaths_SingleDocFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeFile(t, dir, "guide.md", "---\ntype: guide\ntitle: Guide\ndescription: Desc\ntags:\n  - go\n---\nBody\n")
	result := ValidatePaths([]string{path}, []string{"guide"})
	if result.Summary.Files != 1 {
		t.Errorf("expected 1 file, got %d", result.Summary.Files)
	}
	if result.Summary.Errors != 0 {
		t.Errorf("expected 0 errors, got %d: %v", result.Summary.Errors, result.Findings)
	}
}

func TestValidatePaths_ReservedIndexFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeFile(t, dir, "index.md", "# Welcome\n")
	result := ValidatePaths([]string{path}, nil)
	if result.Summary.Files != 1 {
		t.Errorf("expected 1 file, got %d", result.Summary.Files)
	}
	if result.Summary.Errors != 0 {
		t.Errorf("expected 0 errors for valid index.md, got %d: %v", result.Summary.Errors, result.Findings)
	}
}

func TestValidatePaths_ReservedLogInvalid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeFile(t, dir, "log.md", "no frontmatter\n")
	result := ValidatePaths([]string{path}, nil)
	if !hasCode(result.Findings, "E3") {
		t.Errorf("expected E3 for log.md without frontmatter, got findings: %v", result.Findings)
	}
}

func TestValidatePaths_MixedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	goodDoc := writeFile(t, dir, "guide.md", "---\ntype: guide\ntitle: Guide\ndescription: Desc\ntags:\n  - go\n---\nBody\n")
	badDoc := writeFile(t, dir, "bad.md", "no frontmatter\n")
	result := ValidatePaths([]string{goodDoc, badDoc}, []string{"guide"})
	if result.Summary.Files != 2 {
		t.Errorf("expected 2 files, got %d", result.Summary.Files)
	}
	if result.Summary.Errors != 1 {
		t.Errorf("expected 1 error, got %d", result.Summary.Errors)
	}
	if !hasCode(result.Findings, "E1") {
		t.Errorf("expected E1, got findings: %v", result.Findings)
	}
}

func TestValidateDoc_SummaryCounts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "bad.md", "no frontmatter\n")
	findings, _ := ValidateDoc(filepath.Join(dir, "bad.md"), nil)
	result := buildResult(findings, 1)
	if result.Summary.Errors != 1 {
		t.Errorf("expected 1 error in summary, got %d", result.Summary.Errors)
	}
	if result.Summary.Files != 1 {
		t.Errorf("expected 1 file in summary, got %d", result.Summary.Files)
	}
}

func TestValidateDoc_SummaryWarningsAndNotifications(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "many-issues.md", "---\ntype: guide\n---\n")
	findings, _ := ValidateDoc(filepath.Join(dir, "many-issues.md"), nil)
	result := buildResult(findings, 1)
	if result.Summary.Warnings < 3 {
		t.Errorf("expected at least 3 warnings (W1, W2, W4), got %d", result.Summary.Warnings)
	}
}

// --- Unique types helper ---

func TestUniqueTypes(t *testing.T) {
	t.Parallel()
	docs := []parser.Doc{
		{Type: "guide"},
		{Type: "reference"},
		{Type: "guide"},
		{Type: ""},
	}
	ut := uniqueTypes(docs)
	if len(ut) != 2 {
		t.Errorf("expected 2 unique types, got %d: %v", len(ut), ut)
	}
	if !containsType(ut, "guide") || !containsType(ut, "reference") {
		t.Errorf("expected guide and reference, got %v", ut)
	}
}

// --- logEntriesChronological helper ---

func TestLogEntriesChronological_Empty(t *testing.T) {
	t.Parallel()
	if !logEntriesChronological("") {
		t.Error("expected true for empty body")
	}
}

func TestLogEntriesChronological_SingleDate(t *testing.T) {
	t.Parallel()
	if !logEntriesChronological("## 2025-01-01\n**Creation**: `a.md`\n") {
		t.Error("expected true for single date heading")
	}
}

func TestLogEntriesChronological_CorrectOrder(t *testing.T) {
	t.Parallel()
	body := "## 2025-06-01\n**Update**: `a.md`\n## 2025-01-01\n**Creation**: `b.md`\n"
	if !logEntriesChronological(body) {
		t.Error("expected true for reverse-chronological dates")
	}
}

func TestLogEntriesChronological_WrongOrder(t *testing.T) {
	t.Parallel()
	body := "## 2025-01-01\n**Creation**: `b.md`\n## 2025-06-01\n**Update**: `a.md`\n"
	if logEntriesChronological(body) {
		t.Error("expected false for chronological (wrong) order")
	}
}
