// Package index builds and exposes an in-memory index of OKF markdown docs.
package index

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/nrkno/plattform-okf-mcp/internal/parser"
	"github.com/nrkno/plattform-okf-mcp/internal/scanner"
)

// Index holds the set of parsed OKF docs for a directory tree.
// It is safe for concurrent use: mcp-go dispatches handlers in goroutines.
type Index struct {
	dir  string // absolute path to scan root
	mu   sync.Mutex
	docs []parser.Doc
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
// slice atomically.
//
// Per-file parse errors are logged to stderr and skipped — they do not cause
// Rebuild to return an error.
// If no conformant docs are found the result is logged as a warning to stderr
// and Rebuild returns nil (invariant I-7: zero docs is not an error).
func (idx *Index) Rebuild() error {
	paths, err := scanner.Scan(idx.dir)
	if err != nil {
		return fmt.Errorf("index: scan %s: %w", idx.dir, err)
	}

	var docs []parser.Doc
	for _, absPath := range paths {
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
			// Should never happen for paths returned by scanner.Scan(idx.dir).
			fmt.Fprintf(os.Stderr, "okf-mcp: WARN: could not relativize %s: %v\n", absPath, relErr)
			continue
		}
		doc.FilePath = rel
		docs = append(docs, doc)
	}

	if len(docs) == 0 {
		fmt.Fprintf(os.Stderr, "okf-mcp: WARN: no conformant OKF docs found in %s\n", idx.dir)
	}

	idx.mu.Lock()
	idx.docs = docs
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
