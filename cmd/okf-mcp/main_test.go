package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/mark3labs/mcp-go/server"

	"github.com/nrkno/plattform-okf-mcp/internal/index"
)

// ---------------------------------------------------------------------------
// Fixture helpers
// ---------------------------------------------------------------------------

// frontmatter returns a YAML frontmatter block for the given fields.
func frontmatter(typ, title, description string, tags []string) string {
	tagLines := ""
	for _, t := range tags {
		tagLines += "  - " + t + "\n"
	}
	return "---\ntype: " + typ + "\ntitle: " + title +
		"\ndescription: " + description + "\ntags:\n" + tagLines + "---\n"
}

// setupFixtureDir creates the standard fixture directory used by most tests:
//
//	guide.md          — type:guide,     tags:[api,setup]
//	reference.md      — type:reference, tags:[api]
//	.hidden/secret.md — must NOT appear (I-5)
//	index.md          — must NOT appear (I-4)
//	log.md            — must NOT appear (I-4)
//	notype.md         — no type field   — must NOT appear (I-3)
//	nofront.md        — no frontmatter  — must NOT appear
//
// Returns the temp directory path.
func setupFixtureDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	write("guide.md", frontmatter("guide", "User Guide", "A guide", []string{"api", "setup"}))
	write("reference.md", frontmatter("reference", "API Reference", "API docs", []string{"api"}))

	// I-4: OKF-reserved filenames.
	write("index.md", frontmatter("index", "Index", "Index file", []string{"index"}))
	write("log.md", frontmatter("log", "Log", "Log file", []string{"log"}))

	// I-3: type field absent — parser returns (Doc{}, false, nil).
	write("notype.md", "---\ntitle: No Type\ndescription: missing type\n---\n")

	// No frontmatter at all.
	write("nofront.md", "# Just markdown, no frontmatter\n")

	// I-5: hidden directory — scanner skips entirely.
	hiddenDir := filepath.Join(dir, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		t.Fatalf("mkdir .hidden: %v", err)
	}
	secretContent := frontmatter("secret", "Secret", "A secret", []string{"secret"})
	if err := os.WriteFile(filepath.Join(hiddenDir, "secret.md"), []byte(secretContent), 0o644); err != nil {
		t.Fatalf("write .hidden/secret.md: %v", err)
	}

	return dir
}

// newFixtureServer must not be called from parallel tests.
// It mutates the package-level idx variable and restores it via t.Cleanup.
//
// newFixtureServer sets idx to a new index rooted at dir, registers a
// t.Cleanup to restore it, then starts an mcptest.Server with all three
// production tools. The caller must defer srv.Close().
func newFixtureServer(t *testing.T, dir string) *mcptest.Server {
	t.Helper()

	origIdx := idx
	idx = index.New(dir)
	t.Cleanup(func() { idx = origIdx })

	srv, err := mcptest.NewServer(t,
		server.ServerTool{Tool: listTagsTool, Handler: listTagsHandler},
		server.ServerTool{Tool: listDocsTool, Handler: listDocsHandler},
		server.ServerTool{Tool: getDocTool, Handler: getDocHandler},
	)
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

// ---------------------------------------------------------------------------
// Content extraction helper (Constraint 3)
// ---------------------------------------------------------------------------

// getTextContent extracts the text string from the first content item in result.
// All test code MUST go through this helper — no direct .Text field access.
func getTextContent(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] is not TextContent: %T", result.Content[0])
	}
	return tc.Text
}

// ---------------------------------------------------------------------------
// Tool call helper
// ---------------------------------------------------------------------------

func callTool(t *testing.T, srv *mcptest.Server, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := srv.Client().CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	})
	if err != nil {
		t.Fatalf("CallTool %q: %v", name, err)
	}
	return result
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestListTags verifies list_tags returns the sorted, deduplicated tag union.
// Only guide.md (api, setup) and reference.md (api) are indexed — want ["api","setup"].
func TestListTags(t *testing.T) {
	dir := setupFixtureDir(t)
	srv := newFixtureServer(t, dir)
	defer srv.Close()

	result := callTool(t, srv, "list_tags", nil)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}

	var tags []string
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &tags); err != nil {
		t.Fatalf("unmarshal tags: %v", err)
	}

	want := []string{"api", "setup"}
	if len(tags) != len(want) {
		t.Fatalf("got tags %v, want %v", tags, want)
	}
	for i, tag := range tags {
		if tag != want[i] {
			t.Errorf("tags[%d]: got %q, want %q", i, tag, want[i])
		}
	}
}

// TestListDocs_Count verifies list_docs returns exactly 2 entries and that
// excluded files (I-3, I-4, I-5) are absent from the result.
func TestListDocs_Count(t *testing.T) {
	dir := setupFixtureDir(t)
	srv := newFixtureServer(t, dir)
	defer srv.Close()

	result := callTool(t, srv, "list_docs", nil)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}

	var docs []map[string]any
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &docs); err != nil {
		t.Fatalf("unmarshal docs: %v", err)
	}

	if len(docs) != 2 {
		t.Fatalf("got %d docs, want 2: %v", len(docs), docs)
	}

	excluded := []string{"index.md", "log.md", "notype.md", "nofront.md", "secret.md"}
	for _, doc := range docs {
		fp, _ := doc["file_path"].(string)
		base := filepath.Base(fp)
		for _, ex := range excluded {
			if base == ex {
				t.Errorf("excluded file %q appears in list_docs result", ex)
			}
		}
		// I-5: hidden directory must not appear.
		if strings.Contains(fp, ".hidden") {
			t.Errorf("hidden-dir file %q appears in list_docs result", fp)
		}
	}
}

// TestGetDoc_ByTopic verifies get_doc retrieves guide.md for topic="guide",
// that file_path is relative (I-1), and that content is the markdown body
// only — no leading frontmatter block.
func TestGetDoc_ByTopic(t *testing.T) {
	dir := setupFixtureDir(t)
	srv := newFixtureServer(t, dir)
	defer srv.Close()

	result := callTool(t, srv, "get_doc", map[string]any{"topic": "guide"})
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &payload); err != nil {
		t.Fatalf("unmarshal get_doc: %v", err)
	}

	// I-1: file_path must be relative.
	fp, _ := payload["file_path"].(string)
	if strings.HasPrefix(fp, "/") {
		t.Errorf("I-1 violation: file_path is absolute: %q", fp)
	}
	if fp == "" {
		t.Errorf("file_path is empty")
	}

	// content must not start with "---" (frontmatter must be stripped).
	got, _ := payload["content"].(string)
	if strings.HasPrefix(got, "---") {
		t.Errorf("content starts with frontmatter delimiter: %q", got[:min(len(got), 40)])
	}
}

// TestGetDoc_AndFilter verifies that match="and" with tags=["setup"] returns
// only guide.md (the sole doc with tag "setup").
func TestGetDoc_AndFilter(t *testing.T) {
	dir := setupFixtureDir(t)
	srv := newFixtureServer(t, dir)
	defer srv.Close()

	result := callTool(t, srv, "get_doc", map[string]any{
		"topic": "api",
		"tags":  []any{"setup"},
		"match": "and",
	})
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &payload); err != nil {
		t.Fatalf("unmarshal get_doc: %v", err)
	}

	fp, _ := payload["file_path"].(string)
	if filepath.Base(fp) != "guide.md" {
		t.Errorf("and-filter tags=[setup]: expected guide.md, got %q", fp)
	}
}

// TestGetDoc_OrFilter verifies that match="or" with tags=["setup","x"] returns
// a doc that has at least one of those tags (guide.md has "setup").
func TestGetDoc_OrFilter(t *testing.T) {
	dir := setupFixtureDir(t)
	srv := newFixtureServer(t, dir)
	defer srv.Close()

	result := callTool(t, srv, "get_doc", map[string]any{
		"topic": "api",
		"tags":  []any{"setup", "x"},
		"match": "or",
	})
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &payload); err != nil {
		t.Fatalf("unmarshal get_doc: %v", err)
	}

	fp, _ := payload["file_path"].(string)
	if fp == "" {
		t.Errorf("or-filter returned empty file_path")
	}
}

// TestGetDoc_NoMatch verifies get_doc returns IsError=true with prefix
// "no document matched" when the topic has no match.
func TestGetDoc_NoMatch(t *testing.T) {
	dir := setupFixtureDir(t)
	srv := newFixtureServer(t, dir)
	defer srv.Close()

	result := callTool(t, srv, "get_doc", map[string]any{"topic": "xyz"})
	if !result.IsError {
		t.Fatalf("expected error result, got success: %s", getTextContent(t, result))
	}

	text := getTextContent(t, result)
	if !strings.HasPrefix(text, "no document matched") {
		t.Errorf("expected prefix %q, got %q", "no document matched", text)
	}
}

// TestGetDoc_EmptyIndex verifies that get_doc returns IsError=true with
// prefix "index is empty" when no conformant docs exist (I-7).
//
// Constraint 4: owns its idx and its mcptest.NewServer. NOT t.Parallel().
func TestGetDoc_EmptyIndex(t *testing.T) {
	origIdx := idx
	idx = index.New(t.TempDir()) // empty dir — zero .md files
	t.Cleanup(func() { idx = origIdx })

	// Own server that closes over the locally-set idx.
	srv, err := mcptest.NewServer(t,
		server.ServerTool{Tool: listTagsTool, Handler: listTagsHandler},
		server.ServerTool{Tool: listDocsTool, Handler: listDocsHandler},
		server.ServerTool{Tool: getDocTool, Handler: getDocHandler},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	result := callTool(t, srv, "get_doc", map[string]any{"topic": "x"})
	if !result.IsError {
		t.Fatalf("expected error result, got success: %s", getTextContent(t, result))
	}

	text := getTextContent(t, result)
	if !strings.HasPrefix(text, "index is empty") {
		t.Errorf("expected prefix %q, got %q", "index is empty", text)
	}
}

// TestGetDoc_InvalidMatch verifies that an unrecognised match value returns
// IsError=true and the error text contains the invalid value and valid hints.
func TestGetDoc_InvalidMatch(t *testing.T) {
	dir := setupFixtureDir(t)
	srv := newFixtureServer(t, dir)
	defer srv.Close()

	result := callTool(t, srv, "get_doc", map[string]any{
		"topic": "guide",
		"match": "xor",
	})
	if !result.IsError {
		t.Fatalf("expected error result for match=xor, got success: %s", getTextContent(t, result))
	}

	text := getTextContent(t, result)
	if !strings.Contains(text, "xor") {
		t.Errorf("error text should contain invalid value %q, got: %q", "xor", text)
	}
	if !strings.Contains(text, "and") && !strings.Contains(text, "or") {
		t.Errorf("error text should hint at valid values, got: %q", text)
	}
}

// TestGetDoc_TagsAsString verifies that sending "tags" as a scalar string
// (instead of an array) returns IsError=true with an error mentioning "tags".
// This exercises the explicit guard added for M-2: silent filter drop.
//
// Must NOT call t.Parallel() — uses the package-level idx variable.
func TestGetDoc_TagsAsString(t *testing.T) {
	dir := setupFixtureDir(t)
	origIdx := idx
	idx = index.New(dir)
	t.Cleanup(func() { idx = origIdx })

	srv, err := mcptest.NewServer(t,
		server.ServerTool{Tool: getDocTool, Handler: getDocHandler},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	result, err := srv.Client().CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "get_doc",
			Arguments: map[string]any{
				"topic": "guide",
				"tags":  "api", // scalar string — must be rejected
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true when tags is a scalar string")
	}
	text := getTextContent(t, result)
	if !strings.Contains(text, "tags") {
		t.Fatalf("expected error text to mention 'tags', got: %q", text)
	}
}

// TestGetDoc_FilePathRelative is a dedicated I-1 assertion: file_path in the
// get_doc response must never start with '/'.
func TestGetDoc_FilePathRelative(t *testing.T) {
	dir := setupFixtureDir(t)
	srv := newFixtureServer(t, dir)
	defer srv.Close()

	result := callTool(t, srv, "get_doc", map[string]any{"topic": "reference"})
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &payload); err != nil {
		t.Fatalf("unmarshal get_doc: %v", err)
	}

	fp, _ := payload["file_path"].(string)
	if strings.HasPrefix(fp, "/") {
		t.Errorf("I-1 violation: file_path is absolute: %q", fp)
	}
	if fp == "" {
		t.Errorf("I-1: file_path is empty")
	}
}

// TestGetDoc_LiveRead is a dedicated I-2 assertion: the content field must
// reflect a live disk read — verified by mutating the body after the fixture
// is written, then confirming the mutated body appears in the response.
//
// Also verifies the frontmatter is stripped: content must not start with "---".
func TestGetDoc_LiveRead(t *testing.T) {
	dir := setupFixtureDir(t)

	// Rewrite guide.md with a real body so we have something concrete to assert.
	const wantBody = "# User Guide\n\nThis is the guide body.\n"
	guidePath := filepath.Join(dir, "guide.md")
	full := frontmatter("guide", "User Guide", "A guide", []string{"api", "setup"}) + wantBody
	if err := os.WriteFile(guidePath, []byte(full), 0o644); err != nil {
		t.Fatalf("rewrite guide.md: %v", err)
	}

	srv := newFixtureServer(t, dir)
	defer srv.Close()

	result := callTool(t, srv, "get_doc", map[string]any{"topic": "guide"})
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &payload); err != nil {
		t.Fatalf("unmarshal get_doc: %v", err)
	}

	fp, _ := payload["file_path"].(string)
	if fp == "" {
		t.Fatal("file_path is empty")
	}

	got, _ := payload["content"].(string)

	// Frontmatter must be stripped — content must not begin with "---".
	if strings.HasPrefix(got, "---") {
		t.Errorf("I-2/frontmatter: content starts with frontmatter delimiter: %q", got[:min(len(got), 40)])
	}

	// I-2 (body): content must equal the body portion only (what follows the closing "---\n").
	if got != wantBody {
		t.Errorf("I-2 violation: content = %q, want body %q", got, wantBody)
	}
}
