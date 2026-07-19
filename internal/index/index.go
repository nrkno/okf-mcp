// Package index builds and exposes an in-memory index of OKF markdown docs.
package index

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/nrkno/plattform-okf-mcp/internal/parser"
	"github.com/nrkno/plattform-okf-mcp/internal/scanner"
	"gopkg.in/yaml.v3"
)

// ReservedFile holds metadata for a file the OKF standard reserves for its own
// use (e.g. index.md, log.md). These files are never returned by Docs().
type ReservedFile struct {
	FilePath       string // relative path (e.g. "index.md", "docs/log.md")
	HasFrontmatter bool
	Type           string // from frontmatter if present, empty otherwise
}

// TreeNode represents a node in the bundle tree.
type TreeNode struct {
	Name     string     `json:"name"`               // filename or directory name
	Path     string     `json:"path"`               // relative path from scan root
	Type     string     `json:"type"`               // "file", "directory", "reserved"
	DocType  string     `json:"doc_type,omitempty"` // from frontmatter (e.g. "Architecture")
	Title    string     `json:"title,omitempty"`    // from frontmatter
	Children []TreeNode `json:"children,omitempty"` // non-nil for directories
}

// Index holds the set of parsed OKF docs for a directory tree.
// It is safe for concurrent use: mcp-go dispatches handlers in goroutines.
type Index struct {
	dir      string // absolute path to scan root
	mu       sync.Mutex
	docs     []parser.Doc
	reserved []ReservedFile
}

// New returns an empty Index rooted at dir.
// Call Rebuild to populate it.
func New(dir string) *Index {
	return &Index{dir: dir}
}

// Dir returns the absolute path to the scan root for this Index.
// Used by handlers to resolve relative doc paths for os.ReadFile.
func (idx *Index) Dir() string {
	return idx.dir
}

// Rebuild scans idx.dir, parses every .md file, and replaces the internal doc
// and reserved slices atomically.
//
// Per-file parse errors are logged to stderr and skipped — they do not cause
// Rebuild to return an error.
// If no conformant docs are found the result is logged as a warning to stderr
// and Rebuild returns nil (invariant I-7: zero docs is not an error).
func (idx *Index) Rebuild() error {
	result, err := scanner.ScanAll(idx.dir)
	if err != nil {
		return fmt.Errorf("index: scan %s: %w", idx.dir, err)
	}

	var docs []parser.Doc
	for _, absPath := range result.Docs {
		doc, ok, parseErr := parser.Parse(absPath)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "okf-mcp: WARN: skipping %s: %v\n", absPath, parseErr)
			continue
		}
		if !ok {
			// File had no valid frontmatter or missing type — skip silently;
			// parser already handles I-3 semantics.
			continue
		}

		// Invariant I-1: store paths relative to the scan root so tool handlers
		// always emit paths relative to cwd.
		rel, relErr := filepath.Rel(idx.dir, absPath)
		if relErr != nil {
			fmt.Fprintf(os.Stderr, "okf-mcp: WARN: could not relativize %s: %v\n", absPath, relErr)
			continue
		}
		doc.FilePath = rel
		docs = append(docs, doc)
	}

	var reserved []ReservedFile
	for _, absPath := range result.Reserved {
		rel, relErr := filepath.Rel(idx.dir, absPath)
		if relErr != nil {
			fmt.Fprintf(os.Stderr, "okf-mcp: WARN: could not relativize %s: %v\n", absPath, relErr)
			continue
		}
		rf := ReservedFile{FilePath: rel}
		rf.HasFrontmatter, rf.Type = detectReservedFrontmatter(absPath)
		reserved = append(reserved, rf)
	}

	if len(docs) == 0 {
		fmt.Fprintf(os.Stderr, "okf-mcp: WARN: no conformant OKF docs found in %s\n", idx.dir)
	}

	idx.mu.Lock()
	idx.docs = docs
	idx.reserved = reserved
	idx.mu.Unlock()

	return nil
}

// Docs returns a copy of the indexed doc slice.
// The caller is free to mutate the returned slice without affecting the index.
func (idx *Index) Docs() []parser.Doc {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if len(idx.docs) == 0 {
		return nil
	}
	out := make([]parser.Doc, len(idx.docs))
	copy(out, idx.docs)
	return out
}

// Tags returns a sorted, deduplicated list of all tags across all indexed docs.
func (idx *Index) Tags() []string {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	seen := make(map[string]struct{})
	for _, doc := range idx.docs {
		for _, tag := range doc.Tags {
			seen[tag] = struct{}{}
		}
	}

	tags := make([]string, 0, len(seen))
	for tag := range seen {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

// Reserved returns a copy of the reserved file metadata from the last Rebuild.
// Reserved files (index.md, log.md) never appear in Docs() (invariant I-4).
func (idx *Index) Reserved() []ReservedFile {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if len(idx.reserved) == 0 {
		return nil
	}
	out := make([]ReservedFile, len(idx.reserved))
	copy(out, idx.reserved)
	return out
}

// Tree returns the bundle tree for the current index state.
// Root node represents the scan root. Children are directories and files.
// Reserved files appear with type "reserved". Content files with type "file".
// Empty index returns a root node with no children.
func (idx *Index) Tree() TreeNode {
	idx.mu.Lock()
	docs := idx.docs
	reserved := idx.reserved
	idx.mu.Unlock()

	root := TreeNode{
		Name:     ".",
		Path:     "",
		Type:     "directory",
		Children: []TreeNode{},
	}

	// Insert all docs as file nodes.
	for _, doc := range docs {
		insertNode(&root, doc.FilePath, TreeNode{
			Name:    filepath.Base(doc.FilePath),
			Path:    doc.FilePath,
			Type:    "file",
			DocType: doc.Type,
			Title:   doc.Title,
		})
	}

	// Insert reserved files.
	for _, rf := range reserved {
		insertNode(&root, rf.FilePath, TreeNode{
			Name:    filepath.Base(rf.FilePath),
			Path:    rf.FilePath,
			Type:    "reserved",
			DocType: rf.Type,
		})
	}

	// Ensure root has no children if both slices are empty.
	if len(docs) == 0 && len(reserved) == 0 {
		root.Children = nil
	}

	return root
}

// insertNode places a leaf node into the tree at the path given by segments.
// Intermediate directory nodes are created as needed.
func insertNode(root *TreeNode, filePath string, node TreeNode) {
	segments := strings.Split(filePath, string(filepath.Separator))
	current := root
	for _, seg := range segments[:len(segments)-1] {
		child := findChild(current, seg)
		if child == nil {
			child = &TreeNode{
				Name:     seg,
				Path:     seg,
				Type:     "directory",
				Children: []TreeNode{},
			}
			// Build the cumulative path.
			if current.Path == "" {
				child.Path = seg
			} else {
				child.Path = current.Path + string(filepath.Separator) + seg
			}
			current.Children = append(current.Children, *child)
			child = &current.Children[len(current.Children)-1]
		}
		current = child
	}
	current.Children = append(current.Children, node)
}

// findChild returns a pointer to the child of dir with the given name, or nil.
func findChild(dir *TreeNode, name string) *TreeNode {
	for i := range dir.Children {
		if dir.Children[i].Name == name {
			return &dir.Children[i]
		}
	}
	return nil
}

// reservedFrontmatter is used to extract just the type field from reserved
// file frontmatter without needing the full parser.Doc schema.
type reservedFrontmatter struct {
	Type string `yaml:"type"`
}

// detectReservedFrontmatter reads a reserved file and returns whether it has
// frontmatter and, if so, the value of its "type" field.
func detectReservedFrontmatter(absPath string) (hasFrontmatter bool, typ string) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return false, ""
	}
	fmInfo := parser.DetectFrontmatter(string(data))
	if !fmInfo.HasFrontmatter {
		return false, ""
	}
	var fm reservedFrontmatter
	if err := yaml.Unmarshal([]byte(fmInfo.YAMLBlock), &fm); err != nil {
		return true, ""
	}
	return true, fm.Type
}
