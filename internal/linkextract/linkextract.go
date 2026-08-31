// Package linkextract extracts Markdown links from document body content using
// goldmark AST parsing. It is a pure package: no I/O, no profile dependency.
package linkextract

import (
	"bytes"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// ExtractedLink describes a single Markdown link discovered in a document body.
type ExtractedLink struct {
	// Target is the resolved relative path, cleaned and with no leading slash.
	Target string
	// RawTarget is the original link target from the Markdown source.
	RawTarget string
	// Heading is the nearest preceding H1–H6 heading text ("" if none).
	Heading string
	// HeadingNormal is the normalized heading for profile matching:
	// lowercased, inline markdown formatting stripped, whitespace collapsed.
	HeadingNormal string
	// Line is the 1-based line number in the body where the link appears.
	Line int
}

// Extract parses body and returns all local file links with heading context.
// docDir is the document's directory and corpusRoot is the scan root; both are
// expected to be relative to the same base (usually the current working
// directory). Links whose resolved path escapes corpusRoot are silently dropped.
func Extract(body string, docDir string, corpusRoot string) []ExtractedLink {
	src := []byte(body)
	parser := goldmark.DefaultParser()
	doc := parser.Parse(text.NewReader(src))

	var links []ExtractedLink
	var headingStack []heading
	var currentHeading string
	var currentHeadingNormal string

	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch node := n.(type) {
		case *ast.Heading:
			h := heading{
				text:  headingText(node, src),
				depth: node.Level,
			}
			// Maintain a heading stack by depth. Markdown headings do not nest
			// subsequent content as AST children, so we pop shallower-or-equal
			// headings when a new heading is encountered.
			for len(headingStack) > 0 && headingStack[len(headingStack)-1].depth >= h.depth {
				headingStack = headingStack[:len(headingStack)-1]
			}
			headingStack = append(headingStack, h)
			currentHeading = h.text
			currentHeadingNormal = normalizeHeading(h.text)
			return ast.WalkContinue, nil

		case *ast.Link:
			raw := string(node.Destination)
			target, ok := resolveTarget(raw, docDir, corpusRoot)
			if !ok {
				return ast.WalkSkipChildren, nil
			}
			links = append(links, ExtractedLink{
				Target:        target,
				RawTarget:     raw,
				Heading:       currentHeading,
				HeadingNormal: currentHeadingNormal,
				Line:          lineForOffset(src, n.Pos()),
			})
			return ast.WalkSkipChildren, nil

		case *ast.FencedCodeBlock, *ast.CodeBlock, *ast.CodeSpan, *ast.HTMLBlock:
			return ast.WalkSkipChildren, nil
		}

		return ast.WalkContinue, nil
	})

	return links
}

type heading struct {
	text  string
	depth int
}

func headingText(h *ast.Heading, src []byte) string {
	var b strings.Builder
	for child := h.FirstChild(); child != nil; child = child.NextSibling() {
		collectText(child, src, &b)
	}
	return strings.TrimSpace(b.String())
}

func collectText(n ast.Node, src []byte, b *strings.Builder) {
	if textNode, ok := n.(*ast.Text); ok {
		b.Write(textNode.Value(src))
		return
	}
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		collectText(child, src, b)
	}
}

func normalizeHeading(s string) string {
	// Lowercase, then collapse any run of whitespace to a single space.
	var b strings.Builder
	inSpace := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsSpace(r) {
			inSpace = true
			continue
		}
		if inSpace && b.Len() > 0 {
			b.WriteByte(' ')
		}
		inSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func resolveTarget(raw string, docDir string, corpusRoot string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if isNonFileLink(raw) {
		return "", false
	}

	var resolved string
	if strings.HasPrefix(raw, "/") {
		resolved = filepath.Clean(filepath.Join(corpusRoot, strings.TrimPrefix(raw, "/")))
	} else {
		resolved = filepath.Clean(filepath.Join(docDir, raw))
	}

	rel, err := filepath.Rel(corpusRoot, resolved)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}

	// Return the path relative to corpusRoot. The check above guarantees it
	// does not escape corpusRoot.
	return rel, true
}

func isNonFileLink(raw string) bool {
	if strings.HasPrefix(raw, "http://") ||
		strings.HasPrefix(raw, "https://") ||
		strings.HasPrefix(raw, "mailto:") ||
		strings.HasPrefix(raw, "#") {
		return true
	}
	// autolinks and other URI schemes are also skipped.
	if strings.Contains(raw, "://") {
		return true
	}
	return false
}

func lineForOffset(src []byte, offset int) int {
	if offset < 0 || offset > len(src) {
		return 0
	}
	return bytes.Count(src[:offset], []byte("\n")) + 1
}
