package index

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeDoc creates a markdown file with valid OKF YAML frontmatter in dir.
// name is the file path relative to dir (e.g. "foo/doc.md").
func writeDoc(t *testing.T, dir, name, title, docType string, tags []string) {
	t.Helper()

	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("writeDoc: mkdir %s: %v", filepath.Dir(full), err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("title: " + title + "\n")
	sb.WriteString("description: A description.\n")
	sb.WriteString("type: " + docType + "\n")
	if len(tags) > 0 {
		sb.WriteString("tags:\n")
		for _, tag := range tags {
			sb.WriteString("  - " + tag + "\n")
		}
	}
	sb.WriteString("---\n")
	sb.WriteString("# Body\n")

	if err := os.WriteFile(full, []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("writeDoc: write %s: %v", full, err)
	}
}

// writeRaw writes raw bytes to a file relative to dir.
func writeRaw(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("writeRaw: mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("writeRaw: write %s: %v", full, err)
	}
}

// TestI7_ZeroConformantFiles covers invariant I-7:
// a directory with no conformant files must not be an error.
func TestI7_ZeroConformantFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeRaw(t, dir, "README.txt", "not markdown")

	idx := New(dir)
	if err := idx.Rebuild(); err != nil {
		t.Fatalf("Rebuild() unexpected error: %v", err)
	}
	if got := idx.Docs(); len(got) != 0 {
		t.Errorf("Docs() = %d docs, want 0", len(got))
	}
	if got := idx.Tags(); len(got) != 0 {
		t.Errorf("Tags() = %v, want empty", got)
	}
}

// TestTagsSortedAndDeduped verifies that Tags() returns a sorted, deduplicated list.
func TestTagsSortedAndDeduped(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeDoc(t, dir, "alpha.md", "Alpha", "guide", []string{"zebra", "apple"})
	writeDoc(t, dir, "beta.md", "Beta", "reference", []string{"apple", "mango"})

	idx := New(dir)
	if err := idx.Rebuild(); err != nil {
		t.Fatalf("Rebuild() unexpected error: %v", err)
	}

	got := idx.Tags()
	want := []string{"apple", "mango", "zebra"}

	if len(got) != len(want) {
		t.Fatalf("Tags() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Tags()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestI3_MissingTypeNotIndexed covers invariant I-3:
// a doc without a "type" field must not appear in the index.
func TestI3_MissingTypeNotIndexed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// No-type doc: raw frontmatter without type field.
	writeRaw(t, dir, "no-type.md", "---\ntitle: No Type\ndescription: Missing type.\n---\n# Body\n")
	// Conformant doc to confirm Rebuild works at all.
	writeDoc(t, dir, "ok.md", "OK Doc", "guide", nil)

	idx := New(dir)
	if err := idx.Rebuild(); err != nil {
		t.Fatalf("Rebuild() unexpected error: %v", err)
	}

	docs := idx.Docs()
	if len(docs) != 1 {
		t.Fatalf("Docs() = %d docs, want 1 (only the conformant doc)", len(docs))
	}
	for _, doc := range docs {
		if strings.HasSuffix(doc.FilePath, "no-type.md") {
			t.Error("no-type.md must not appear in index (invariant I-3)")
		}
	}
}

// TestI4_IndexMdNotIndexed covers invariant I-4:
// a file named "index.md" must not appear in the index even with valid frontmatter.
func TestI4_IndexMdNotIndexed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeRaw(t, dir, "index.md", "---\ntitle: Index\ndescription: Reserved.\ntype: guide\n---\n# Body\n")
	writeDoc(t, dir, "regular.md", "Regular", "guide", nil)

	idx := New(dir)
	if err := idx.Rebuild(); err != nil {
		t.Fatalf("Rebuild() unexpected error: %v", err)
	}

	docs := idx.Docs()
	for _, doc := range docs {
		if strings.HasSuffix(doc.FilePath, "index.md") {
			t.Error("index.md must not appear in index (invariant I-4)")
		}
	}
}

// TestI5_HiddenDirNotIndexed covers invariant I-5:
// files inside a hidden directory (name starts with '.') must not be indexed.
func TestI5_HiddenDirNotIndexed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeDoc(t, dir, ".hidden/doc.md", "Hidden Doc", "guide", nil)
	writeDoc(t, dir, "visible.md", "Visible Doc", "guide", nil)

	idx := New(dir)
	if err := idx.Rebuild(); err != nil {
		t.Fatalf("Rebuild() unexpected error: %v", err)
	}

	docs := idx.Docs()
	if len(docs) != 1 {
		t.Fatalf("Docs() = %d docs, want 1 (only visible.md)", len(docs))
	}
	for _, doc := range docs {
		if strings.Contains(doc.FilePath, ".hidden") {
			t.Errorf("file from hidden dir must not be indexed (invariant I-5): %s", doc.FilePath)
		}
	}
}

// TestDoubleRebuild ensures Rebuild replaces docs rather than accumulating them.
func TestDoubleRebuild(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeDoc(t, dir, "a.md", "A", "guide", nil)
	writeDoc(t, dir, "b.md", "B", "reference", nil)

	idx := New(dir)

	if err := idx.Rebuild(); err != nil {
		t.Fatalf("first Rebuild() error: %v", err)
	}
	countAfterFirst := len(idx.Docs())

	if err := idx.Rebuild(); err != nil {
		t.Fatalf("second Rebuild() error: %v", err)
	}
	countAfterSecond := len(idx.Docs())

	if countAfterFirst != countAfterSecond {
		t.Errorf("Docs() count changed across two Rebuilds: first=%d second=%d (must not accumulate)",
			countAfterFirst, countAfterSecond)
	}
}

// TestFilePathIsRelative ensures no doc.FilePath starts with '/' after Rebuild.
func TestFilePathIsRelative(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeDoc(t, dir, "sub/doc.md", "Sub Doc", "guide", nil)
	writeDoc(t, dir, "root.md", "Root Doc", "reference", nil)

	idx := New(dir)
	if err := idx.Rebuild(); err != nil {
		t.Fatalf("Rebuild() error: %v", err)
	}

	for _, doc := range idx.Docs() {
		if filepath.IsAbs(doc.FilePath) {
			t.Errorf("doc.FilePath is absolute: %q (invariant I-1)", doc.FilePath)
		}
	}
}

// TestDocsCopy ensures Docs() returns a copy: mutating the returned slice
// does not affect internal index state.
func TestDocsCopy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeDoc(t, dir, "one.md", "One", "guide", nil)

	idx := New(dir)
	if err := idx.Rebuild(); err != nil {
		t.Fatalf("Rebuild() error: %v", err)
	}

	original := idx.Docs()
	if len(original) == 0 {
		t.Fatal("expected at least one doc")
	}

	// Mutate the returned slice.
	original[0].Title = "MUTATED"
	original = append(original, original[0])

	// The index must be unaffected.
	fresh := idx.Docs()
	if fresh[0].Title == "MUTATED" {
		t.Error("mutating Docs() return value affected internal index state")
	}
	if len(fresh) != 1 {
		t.Errorf("Docs() len = %d after external append, want 1", len(fresh))
	}
}
