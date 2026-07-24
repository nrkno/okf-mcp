package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/mark3labs/mcp-go/server"

	"github.com/nrkno/plattform-okf-mcp/internal/index"
	"github.com/nrkno/plattform-okf-mcp/internal/logparser"
	"github.com/nrkno/plattform-okf-mcp/internal/scanner"
	"github.com/nrkno/plattform-okf-mcp/internal/validator"
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
// newFixtureServer sets idx to a new index rooted at dir with the given
// scan options (use scanner.ScanOptions{} for default behavior, or
// scanner.ScanOptions{EnableHidden: true} for tests that exercise hidden
// bundle directories), registers a t.Cleanup to restore idx, then starts an
// mcptest.Server with all six production tools. The caller must defer srv.Close().
func newFixtureServer(t *testing.T, dir string, opts scanner.ScanOptions) *mcptest.Server {
	t.Helper()

	origIdx := idx
	idx = index.New(dir, opts)
	t.Cleanup(func() { idx = origIdx })

	srv, err := mcptest.NewServer(t,
		server.ServerTool{Tool: listTagsTool, Handler: listTagsHandler},
		server.ServerTool{Tool: listDocsTool, Handler: listDocsHandler},
		server.ServerTool{Tool: getDocTool, Handler: getDocHandler},
		server.ServerTool{Tool: validateDocTool, Handler: validateDocHandler},
		server.ServerTool{Tool: getIndexTool, Handler: getIndexHandler},
		server.ServerTool{Tool: getLogTool, Handler: getLogHandler},
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
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
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
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
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


// setupMultiBundleFixture creates a fixture with two OKF bundles:
//
//	docs/index.md                     (reserved, conformant — no frontmatter)
//	docs/arch.md                      (doc, type: Architecture)
//	.opencode/architecture/index.md   (reserved, conformant — no frontmatter)
//	.opencode/architecture/design.md  (doc, type: Architecture)
//
// Tests using this fixture must pass scanner.ScanOptions{EnableHidden: true}
// to newFixtureServer to traverse the .opencode/ subdirectory.
func setupMultiBundleFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	mkBundle := func(relBundleDir string, archTitle, archDescription, archType string, archTags []string) {
		t.Helper()
		bundleDir := filepath.Join(dir, relBundleDir)
		if err := os.MkdirAll(bundleDir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", relBundleDir, err)
		}
		// Conformant index.md: no frontmatter.
		if err := os.WriteFile(filepath.Join(bundleDir, "index.md"), nil, 0o644); err != nil {
			t.Fatalf("write %s/index.md: %v", relBundleDir, err)
		}
		arch := relBundleDir + "/arch.md"
		if err := os.WriteFile(filepath.Join(dir, arch),
			[]byte(frontmatter(archType, archTitle, archDescription, archTags)), 0o644); err != nil {
			t.Fatalf("write %s: %v", arch, err)
		}
	}

	mkBundle("docs", "Architecture", "System design", "Architecture", []string{"design"})
	mkBundle(".opencode/architecture", "API Design", "API patterns", "Architecture", []string{"api"})

	return dir
}

// TestListDocs_BundleField verifies that each entry in list_docs carries a
// correct bundle field per the walk-up rule (I-17). Hidden-bundle docs are
// only visible with EnableHidden: true.

// TestListDocs_BundleField verifies that each entry in list_docs carries a
// correct bundle field per the walk-up rule (I-17). Hidden-bundle docs are
// only visible with EnableHidden: true.
func TestListDocs_BundleField(t *testing.T) {
	dir := setupMultiBundleFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{EnableHidden: true})
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

	bundleByPath := make(map[string]string, len(docs))
	for _, doc := range docs {
		fp, _ := doc["file_path"].(string)
		if fp == "" {
			t.Errorf("doc missing file_path: %v", doc)
		}
		bundle, _ := doc["bundle"].(string)
		if bundle == "" {
			t.Errorf("doc %q missing bundle field: %v", fp, doc)
		}
		bundleByPath[fp] = bundle
	}

	if got, want := bundleByPath["docs/arch.md"], "docs"; got != want {
		t.Errorf("docs/arch.md bundle: got %q, want %q", got, want)
	}
	if got, want := bundleByPath[".opencode/architecture/arch.md"], ".opencode/architecture"; got != want {
		t.Errorf(".opencode/architecture/arch.md bundle: got %q, want %q", got, want)
	}
}

// TestGetDoc_ByTopic verifies get_doc retrieves guide.md for topic="guide",
// that file_path is relative (I-1), and that content is the markdown body
// only — no leading frontmatter block.

// TestGetDoc_ByTopic verifies get_doc retrieves guide.md for topic="guide",
// that file_path is relative (I-1), and that content is the markdown body
// only — no leading frontmatter block.
func TestGetDoc_ByTopic(t *testing.T) {
	dir := setupFixtureDir(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
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
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
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
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
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
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
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
	idx = index.New(t.TempDir(), scanner.ScanOptions{}) // empty dir — zero .md files
	t.Cleanup(func() { idx = origIdx })

	// Own server that closes over the locally-set idx.
	srv, err := mcptest.NewServer(t,
		server.ServerTool{Tool: listTagsTool, Handler: listTagsHandler},
		server.ServerTool{Tool: listDocsTool, Handler: listDocsHandler},
		server.ServerTool{Tool: getDocTool, Handler: getDocHandler},
		server.ServerTool{Tool: validateDocTool, Handler: validateDocHandler},
		server.ServerTool{Tool: getIndexTool, Handler: getIndexHandler},
		server.ServerTool{Tool: getLogTool, Handler: getLogHandler},
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
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
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
	idx = index.New(dir, scanner.ScanOptions{})
	t.Cleanup(func() { idx = origIdx })

	srv, err := mcptest.NewServer(t,
		server.ServerTool{Tool: getDocTool, Handler: getDocHandler},
		server.ServerTool{Tool: validateDocTool, Handler: validateDocHandler},
		server.ServerTool{Tool: getIndexTool, Handler: getIndexHandler},
		server.ServerTool{Tool: getLogTool, Handler: getLogHandler},
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
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
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

	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
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


// TestGetDoc_BundleField verifies that get_doc response includes the bundle
// field per the walk-up rule (I-17), exercised against docs in both a visible
// bundle and a hidden bundle (the latter requires EnableHidden: true).
func TestGetDoc_BundleField(t *testing.T) {
	dir := setupMultiBundleFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{EnableHidden: true})
	defer srv.Close()

	// Doc in hidden bundle — the primary motivation.
	result := callTool(t, srv, "get_doc", map[string]any{"topic": "API Design"})
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &payload); err != nil {
		t.Fatalf("unmarshal get_doc: %v", err)
	}
	if got, want := payload["bundle"], ".opencode/architecture"; got != want {
		t.Errorf("hidden-bundle get_doc bundle: got %v, want %v", got, want)
	}
	if got, want := payload["file_path"], ".opencode/architecture/arch.md"; got != want {
		t.Errorf("hidden-bundle get_doc file_path: got %v, want %v", got, want)
	}

	// Doc in visible bundle — the regression guard.
	result = callTool(t, srv, "get_doc", map[string]any{"topic": "Architecture"})
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &payload); err != nil {
		t.Fatalf("unmarshal get_doc: %v", err)
	}
	if got, want := payload["bundle"], "docs"; got != want {
		t.Errorf("visible-bundle get_doc bundle: got %v, want %v", got, want)
	}
}

// ---------------------------------------------------------------------------
// Tool count
// ---------------------------------------------------------------------------

// TestNewFixtureServerToolsCount verifies newFixtureServer registers all 6 tools.
// ---------------------------------------------------------------------------
// Tool count
// ---------------------------------------------------------------------------

// TestNewFixtureServerToolsCount verifies newFixtureServer registers all 6 tools.
func TestNewFixtureServerToolsCount(t *testing.T) {
	dir := setupFixtureDir(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	tools, err := srv.Client().ListTools(context.Background(), mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	wantNames := map[string]bool{
		"list_tags": true, "list_docs": true, "get_doc": true,
		"validate_doc": true, "get_index": true, "get_log": true,
	}
	if len(tools.Tools) != len(wantNames) {
		t.Fatalf("got %d tools, want %d", len(tools.Tools), len(wantNames))
	}
	for _, tool := range tools.Tools {
		if !wantNames[tool.Name] {
			t.Errorf("unexpected tool: %s", tool.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// validate_doc tests
// ---------------------------------------------------------------------------

func TestValidateDoc_BundleValid(t *testing.T) {
	dir := setupFixtureDir(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	result := callTool(t, srv, "validate_doc", nil)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}

	var resp struct {
		Summary  validator.Summary `json:"summary"`
		Findings []struct{}        `json:"findings"`
	}
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Summary.Files == 0 {
		t.Errorf("expected Files > 0, got 0")
	}
}

func TestValidateDoc_BundleInvalid(t *testing.T) {
	dir := setupFixtureDir(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	result := callTool(t, srv, "validate_doc", nil)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}

	var resp struct {
		Summary validator.Summary `json:"summary"`
	}
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Fixture has notype.md (E2) and nofront.md (E1) — expect errors.
	if resp.Summary.Errors == 0 {
		t.Errorf("expected errors for invalid fixture files")
	}
}

func TestValidateDoc_SingleFileDoc(t *testing.T) {
	dir := setupFixtureDir(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	result := callTool(t, srv, "validate_doc", map[string]any{"file_path": "guide.md"})
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}

	var resp struct {
		Summary validator.Summary `json:"summary"`
	}
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Summary.Files != 1 {
		t.Errorf("expected 1 file, got %d", resp.Summary.Files)
	}
	if resp.Summary.Errors > 0 {
		t.Errorf("guide.md should be valid, got %d errors", resp.Summary.Errors)
	}
}

func TestValidateDoc_SingleFileReserved(t *testing.T) {
	dir := setupFixtureDir(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	result := callTool(t, srv, "validate_doc", map[string]any{"file_path": "index.md"})
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}

	var resp struct {
		Summary  validator.Summary `json:"summary"`
		Findings []struct {
			Code string `json:"code"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Summary.Files != 1 {
		t.Errorf("expected 1 file, got %d", resp.Summary.Files)
	}
	// index.md fixture has frontmatter → E3.
	found := false
	for _, f := range resp.Findings {
		if f.Code == "E3" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected E3 finding for index.md with frontmatter")
	}
}

// ---------------------------------------------------------------------------
// get_index tests
// ---------------------------------------------------------------------------

func TestGetIndex_FullTree(t *testing.T) {
	dir := setupFixtureDir(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	result := callTool(t, srv, "get_index", nil)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}

	var tree index.TreeNode
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &tree); err != nil {
		t.Fatalf("unmarshal tree: %v", err)
	}
	if tree.Type != "directory" {
		t.Errorf("root type: got %q, want directory", tree.Type)
	}
	if len(tree.Children) == 0 {
		t.Error("expected non-empty root children")
	}
}

func TestGetIndex_Subtree(t *testing.T) {
	dir := setupFixtureDir(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	result := callTool(t, srv, "get_index", nil)
	if result.IsError {
		t.Fatalf("get_index failed: %s", getTextContent(t, result))
	}
	// The fixture has no subdirectories — all files are at root.
	// Verify that requesting a non-existent subtree returns an error.
	result = callTool(t, srv, "get_index", map[string]any{"path": "nonexistent"})
	if !result.IsError {
		t.Fatalf("expected error for nonexistent path, got success")
	}
}

func TestGetIndex_RootPath(t *testing.T) {
	dir := setupFixtureDir(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	result := callTool(t, srv, "get_index", map[string]any{"path": "."})
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}

	var tree index.TreeNode
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &tree); err != nil {
		t.Fatalf("unmarshal tree: %v", err)
	}
	if tree.Type != "directory" {
		t.Errorf("root type: got %q, want directory", tree.Type)
	}
}


// TestGetIndex_BundleOnLeaves verifies that get_index leaf nodes carry the
// bundle field per the walk-up rule (I-17), while directory nodes do NOT.
// Also exercises the `path` parameter into a hidden bundle (critic m5).
func TestGetIndex_BundleOnLeaves(t *testing.T) {
	dir := setupMultiBundleFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{EnableHidden: true})
	defer srv.Close()

	// Full tree — every leaf must have a bundle; every directory must not.
	result := callTool(t, srv, "get_index", nil)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}

	var tree index.TreeNode
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &tree); err != nil {
		t.Fatalf("unmarshal tree: %v", err)
	}

	leafBundleByPath := make(map[string]string)
	var walk func(n *index.TreeNode)
	walk = func(n *index.TreeNode) {
		switch n.Type {
		case "file", "reserved":
			if n.Bundle == "" {
				t.Errorf("leaf %q (type=%s) missing bundle field", n.Path, n.Type)
			}
			leafBundleByPath[n.Path] = n.Bundle
		case "directory":
			if n.Bundle != "" {
				t.Errorf("directory %q unexpectedly has bundle=%q (leaves only)", n.Path, n.Bundle)
			}
			for i := range n.Children {
				walk(&n.Children[i])
			}
		}
	}
	walk(&tree)

	if got, want := leafBundleByPath["docs/arch.md"], "docs"; got != want {
		t.Errorf("docs/arch.md leaf bundle: got %q, want %q", got, want)
	}
	if got, want := leafBundleByPath[".opencode/architecture/arch.md"], ".opencode/architecture"; got != want {
		t.Errorf(".opencode/architecture/arch.md leaf bundle: got %q, want %q", got, want)
	}

	// path parameter into a hidden bundle (critic m5) — drills into
	// .opencode/architecture via findSubtree and confirms the returned subtree.
	result = callTool(t, srv, "get_index", map[string]any{"path": ".opencode/architecture"})
	if result.IsError {
		t.Fatalf("get_index with path into hidden bundle failed: %s", getTextContent(t, result))
	}
	var sub index.TreeNode
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &sub); err != nil {
		t.Fatalf("unmarshal subtree: %v", err)
	}
	if sub.Name != "architecture" {
		t.Errorf("subtree root name: got %q, want %q", sub.Name, "architecture")
	}
	if sub.Type != "directory" {
		t.Errorf("subtree root type: got %q, want directory", sub.Type)
	}
	// The hidden bundle's directory node must NOT carry bundle itself.
	if sub.Bundle != "" {
		t.Errorf("hidden-bundle directory node carries bundle=%q (leaves only)", sub.Bundle)
	}
	// Walk subtree — find the leaf arch.md and confirm its bundle.
	var archBundle string
	var find func(n *index.TreeNode)
	find = func(n *index.TreeNode) {
		if n.Type == "file" && filepath.Base(n.Path) == "arch.md" {
			archBundle = n.Bundle
			return
		}
		for i := range n.Children {
			find(&n.Children[i])
		}
	}
	find(&sub)
	if got, want := archBundle, ".opencode/architecture"; got != want {
		t.Errorf("hidden subtree arch.md leaf bundle: got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// get_log tests
// ---------------------------------------------------------------------------

// setupLogFixture creates a fixture with a log.md containing valid log entries.
// ---------------------------------------------------------------------------
// get_log tests
// ---------------------------------------------------------------------------

// setupLogFixture creates a fixture with a log.md containing valid log entries.
func setupLogFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	write("guide.md", frontmatter("guide", "User Guide", "A guide", []string{"api"}))

	logContent := "---\ntype: Log\ntitle: Log\ndescription: Change log\ntags:\n  - log\n---\n" +
		"## 2025-07-10\n\n**Update**: `[guide.md](/docs/guide.md)` — Revised section 2\n\n" +
		"## 2025-06-15\n\n**Creation**: `[guide.md](/docs/guide.md)` — Initial creation\n"
	write("log.md", logContent)

	return dir
}

func TestGetLog_ValidEntries(t *testing.T) {
	dir := setupLogFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	result := callTool(t, srv, "get_log", nil)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}

	var resp struct {
		Entries []logparser.LogEntry `json:"entries"`
		Source  string               `json:"source"`
	}
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(resp.Entries))
	}
	// Newest first: 2025-07-10 should be before 2025-06-15.
	if resp.Entries[0].Date != "2025-07-10" {
		t.Errorf("entries[0].Date: got %q, want 2025-07-10", resp.Entries[0].Date)
	}
	if resp.Source == "" {
		t.Error("expected non-empty source")
	}
}

func TestGetLog_MissingLog(t *testing.T) {
	// Fixture with no log.md.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "guide.md"),
		[]byte(frontmatter("guide", "Guide", "A guide", []string{"api"})), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	result := callTool(t, srv, "get_log", nil)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}

	var resp struct {
		Entries []logparser.LogEntry `json:"entries"`
		Note    string               `json:"note"`
	}
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(resp.Entries))
	}
	if resp.Note != "no log.md found" {
		t.Errorf("note: got %q, want %q", resp.Note, "no log.md found")
	}
}

func TestGetLog_Filtered(t *testing.T) {
	dir := setupLogFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	// Filter by action=Update.
	result := callTool(t, srv, "get_log", map[string]any{"action": "Update"})
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}

	var resp struct {
		Entries []logparser.LogEntry `json:"entries"`
	}
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(resp.Entries))
	}
	if resp.Entries[0].Action != "Update" {
		t.Errorf("action: got %q, want Update", resp.Entries[0].Action)
	}

	// Filter by since.
	result = callTool(t, srv, "get_log", map[string]any{"since": "2025-07-01"})
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(resp.Entries))
	}
	if resp.Entries[0].Date != "2025-07-10" {
		t.Errorf("date: got %q, want 2025-07-10", resp.Entries[0].Date)
	}

	// Filter by limit.
	result = callTool(t, srv, "get_log", map[string]any{"limit": float64(1)})
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}
	if err := json.Unmarshal([]byte(getTextContent(t, result)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(resp.Entries))
	}
}

// ---------------------------------------------------------------------------
// CLI --validate tests
// ---------------------------------------------------------------------------

func TestCLI_Validate_NoErrors(t *testing.T) {
	dir := t.TempDir()
	// Write a valid OKF doc.
	if err := os.WriteFile(filepath.Join(dir, "guide.md"),
		[]byte(frontmatter("guide", "User Guide", "A guide", []string{"api"})), 0o644); err != nil {
		t.Fatal(err)
	}

	bin := buildBinary(t)
	exit := runBinary(t, bin, dir, "--validate")
	if exit != 0 {
		t.Errorf("exit code: got %d, want 0", exit)
	}
}

func TestCLI_Validate_WithErrors(t *testing.T) {
	dir := t.TempDir()
	// Write an index.md WITH frontmatter → E3 (index.md must not have frontmatter).
	if err := os.WriteFile(filepath.Join(dir, "index.md"),
		[]byte("---\ntype: Index\ntitle: Index\ndescription: Index page\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	bin := buildBinary(t)
	exit := runBinary(t, bin, dir, "--validate")
	if exit != 1 {
		t.Errorf("exit code: got %d, want 1", exit)
	}
}

func TestCLI_Validate_WithPath(t *testing.T) {
	dir := t.TempDir()
	docsDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "guide.md"),
		[]byte(frontmatter("guide", "Guide", "A guide", []string{"api"})), 0o644); err != nil {
		t.Fatal(err)
	}

	bin := buildBinary(t)
	exit := runBinary(t, bin, dir, "--validate", "--path", "docs")
	if exit != 0 {
		t.Errorf("exit code: got %d, want 0", exit)
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "okf-mcp")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = filepath.Join(findModuleRoot(t), "cmd", "okf-mcp")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func runBinary(t *testing.T, bin, workDir string, args ...string) int {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = workDir
	_ = cmd.Run()
	return cmd.ProcessState.ExitCode()
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("could not find module root")
		}
		wd = parent
	}
}
