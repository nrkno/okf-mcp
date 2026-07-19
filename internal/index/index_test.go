package index

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeReserved writes a reserved file (index.md or log.md) with optional frontmatter.
func writeReserved(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("writeReserved: mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("writeReserved: write %s: %v", full, err)
	}
}

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
	_ = append(original, original[0]) // exercise append; result unused — we test len via fresh call below

	// The index must be unaffected.
	fresh := idx.Docs()
	if fresh[0].Title == "MUTATED" {
		t.Error("mutating Docs() return value affected internal index state")
	}
	if len(fresh) != 1 {
		t.Errorf("Docs() len = %d after external append, want 1", len(fresh))
	}
}

// TestReserved_AppearsInReserved verifies that reserved files appear in Reserved()
// with correct relative paths.
func TestReserved_AppearsInReserved(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeReserved(t, dir, "index.md", "# Index\n")
	writeReserved(t, dir, "docs/log.md", "---\ntype: Log\n---\n# Log\n")
	writeDoc(t, dir, "docs/arch.md", "Arch", "Architecture", nil)

	idx := New(dir)
	if err := idx.Rebuild(); err != nil {
		t.Fatalf("Rebuild() error: %v", err)
	}

	reserved := idx.Reserved()
	if len(reserved) != 2 {
		t.Fatalf("Reserved() = %d files, want 2", len(reserved))
	}

	paths := make(map[string]bool)
	for _, rf := range reserved {
		paths[rf.FilePath] = true
		if filepath.IsAbs(rf.FilePath) {
			t.Errorf("Reserved file path is absolute: %q (invariant I-1)", rf.FilePath)
		}
	}
	if !paths["index.md"] {
		t.Error("index.md not found in Reserved()")
	}
	if !paths["docs/log.md"] {
		t.Error("docs/log.md not found in Reserved()")
	}
}

// TestReserved_NotInDocs verifies invariant I-4 + I-8:
// reserved files never appear in Docs().
func TestReserved_NotInDocs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeReserved(t, dir, "index.md", "# Index\n")
	writeReserved(t, dir, "docs/log.md", "---\ntype: Log\n---\n# Log\n")
	writeDoc(t, dir, "guide.md", "Guide", "guide", nil)

	idx := New(dir)
	if err := idx.Rebuild(); err != nil {
		t.Fatalf("Rebuild() error: %v", err)
	}

	docs := idx.Docs()
	for _, doc := range docs {
		if doc.FilePath == "index.md" || doc.FilePath == "docs/log.md" {
			t.Errorf("reserved file %q must not appear in Docs() (invariant I-4)", doc.FilePath)
		}
	}
}

// TestReserved_FrontmatterDetection verifies HasFrontmatter and Type fields.
func TestReserved_FrontmatterDetection(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeReserved(t, dir, "index.md", "# Index\n")
	writeReserved(t, dir, "docs/log.md", "---\ntitle: Log\ntype: Log\n---\n# Log\n")

	idx := New(dir)
	if err := idx.Rebuild(); err != nil {
		t.Fatalf("Rebuild() error: %v", err)
	}

	reserved := idx.Reserved()
	rfMap := make(map[string]ReservedFile)
	for _, rf := range reserved {
		rfMap[rf.FilePath] = rf
	}

	if rf, ok := rfMap["index.md"]; ok {
		if rf.HasFrontmatter {
			t.Error("index.md should not have frontmatter")
		}
	} else {
		t.Error("index.md not found in Reserved()")
	}

	if rf, ok := rfMap["docs/log.md"]; ok {
		if !rf.HasFrontmatter {
			t.Error("docs/log.md should have frontmatter")
		}
		if rf.Type != "Log" {
			t.Errorf("docs/log.md Type = %q, want %q", rf.Type, "Log")
		}
	} else {
		t.Error("docs/log.md not found in Reserved()")
	}
}

// TestTree_MultiLevel verifies Tree() returns the correct nested structure.
func TestTree_MultiLevel(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeReserved(t, dir, "index.md", "# Index\n")
	writeReserved(t, dir, "docs/log.md", "---\ntitle: Log\ntype: Log\n---\n# Log\n")
	writeDoc(t, dir, "docs/arch.md", "Architecture", "Architecture", []string{"design"})
	writeDoc(t, dir, "docs/tools.md", "Tools", "API Reference", nil)
	writeDoc(t, dir, "guide.md", "Guide", "guide", nil)

	idx := New(dir)
	if err := idx.Rebuild(); err != nil {
		t.Fatalf("Rebuild() error: %v", err)
	}

	tree := idx.Tree()

	if tree.Name != "." || tree.Path != "" || tree.Type != "directory" {
		t.Errorf("root = {Name:%q Path:%q Type:%q}, want{Name:. Path: Type:directory}", tree.Name, tree.Path, tree.Type)
	}
	if len(tree.Children) == 0 {
		t.Fatal("root has no children")
	}

	indexNode := findTreeNode(tree, "index.md")
	if indexNode == nil || indexNode.Type != "reserved" {
		t.Errorf("index.md Type = %q, want %q", indexNode.Type, "reserved")
	}

	guideNode := findTreeNode(tree, "guide.md")
	if guideNode == nil || guideNode.Type != "file" || guideNode.DocType != "guide" {
		t.Errorf("guide.md = {Type:%q DocType:%q}, want{Type:file DocType:guide}", guideNode.Type, guideNode.DocType)
	}

	docsDir := findChild(&tree, "docs")
	if docsDir == nil || docsDir.Type != "directory" {
		t.Fatal("docs/ directory not found in tree")
	}
	if len(docsDir.Children) != 3 {
		t.Fatalf("docs/ has %d children, want 3", len(docsDir.Children))
	}

	archNode := findTreeNode(*docsDir, "arch.md")
	if archNode == nil || archNode.DocType != "Architecture" {
		t.Errorf("docs/arch.md DocType = %q, want Architecture", archNode.DocType)
	}

	logNode := findTreeNode(*docsDir, "log.md")
	if logNode == nil || logNode.Type != "reserved" || logNode.DocType != "Log" {
		t.Errorf("docs/log.md = {Type:%q DocType:%q}, want{Type:reserved DocType:Log}", logNode.Type, logNode.DocType)
	}

	toolsNode := findTreeNode(*docsDir, "tools.md")
	if toolsNode == nil || toolsNode.Title != "Tools" {
		t.Errorf("docs/tools.md Title = %q, want Tools", toolsNode.Title)
	}
}

// TestTree_EmptyIndex verifies that an empty index returns a root with no children.
func TestTree_EmptyIndex(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeRaw(t, dir, "README.txt", "not markdown")

	idx := New(dir)
	if err := idx.Rebuild(); err != nil {
		t.Fatalf("Rebuild() error: %v", err)
	}

	tree := idx.Tree()
	if tree.Name != "." {
		t.Errorf("root Name = %q, want %q", tree.Name, ".")
	}
	if len(tree.Children) != 0 {
		t.Errorf("root has %d children, want 0", len(tree.Children))
	}
}

// TestTree_IncludesReservedAsReservedType verifies reserved files appear with type "reserved".
func TestTree_IncludesReservedAsReservedType(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeReserved(t, dir, "index.md", "# Index\n")
	writeReserved(t, dir, "docs/log.md", "---\ntype: Log\n---\n# Log\n")
	writeDoc(t, dir, "docs/arch.md", "Arch", "Architecture", nil)

	idx := New(dir)
	if err := idx.Rebuild(); err != nil {
		t.Fatalf("Rebuild() error: %v", err)
	}

	tree := idx.Tree()
	var reservedPaths, filePaths []string
	collectLeaves(tree, &reservedPaths, &filePaths)

	for _, p := range reservedPaths {
		if p != "index.md" && p != "docs/log.md" {
			t.Errorf("unexpected reserved file in tree: %s", p)
		}
	}
	for _, p := range filePaths {
		if p == "index.md" || p == "docs/log.md" {
			t.Errorf("reserved file %s has wrong type (should be reserved, got file)", p)
		}
	}
	if len(reservedPaths) != 2 {
		t.Errorf("found %d reserved nodes, want 2", len(reservedPaths))
	}
}

// findTreeNode searches for a node by name in the tree (depth-first).
func findTreeNode(node TreeNode, name string) *TreeNode {
	if node.Name == name {
		return &node
	}
	for i := range node.Children {
		if found := findTreeNode(node.Children[i], name); found != nil {
			return found
		}
	}
	return nil
}

// collectLeaves recursively collects file paths by type.
func collectLeaves(node TreeNode, reserved, files *[]string) {
	if len(node.Children) == 0 {
		switch node.Type {
		case "reserved":
			*reserved = append(*reserved, node.Path)
		case "file":
			*files = append(*files, node.Path)
		}
		return
	}
	for _, child := range node.Children {
		collectLeaves(child, reserved, files)
	}
}
