package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/nrkno/plattform-okf-mcp/internal/scanner"
)

// ---------------------------------------------------------------------------
// Graph fixture helpers
// ---------------------------------------------------------------------------

// graphProfileYAML is a domain profile used by graph integration fixtures.
// It defines Requirement, Implementation, and Architecture concept types,
// plus implements/depends_on/supersedes relationships with inverses and a
// cardinality invariant that every Requirement needs at least one incoming
// "implements" edge.
const graphProfileYAML = `name: integration-test
version: "1.0"
concept_types:
  - name: Requirement
  - name: Implementation
  - name: Architecture
relationships:
  - name: implements
    heading_aliases:
      - "Implements"
    inverse: implemented_by
    allowed_source_types:
      - Implementation
    allowed_target_types:
      - Requirement
  - name: depends_on
    heading_aliases:
      - "Depends on"
      - "Dependencies"
    inverse: depended_on_by
    allowed_source_types:
      - "*"
    allowed_target_types:
      - "*"
  - name: supersedes
    heading_aliases:
      - "Supersedes"
    inverse: superseded_by
    allowed_source_types:
      - "*"
    allowed_target_types:
      - "*"
invariants:
  - id: R1
    description: Every Requirement must have at least one Implementation
    source_type: Requirement
    relationship: implements
    direction: incoming
    min: 1
    max: -1
    severity: error
  - id: R2
    description: Supersession is an outgoing relationship
    source_type: "*"
    relationship: supersedes
    direction: outgoing
    min: 0
    max: 0
    severity: error
`

// writeProfile writes graphProfileYAML (or caller-supplied content) as
// .okf-profile.yaml in dir.
func writeProfile(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".okf-profile.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
}

// writeDoc writes a single OKF document with frontmatter + body to relPath
// under dir, creating parent directories as needed.
func writeDoc(t *testing.T, dir, relPath, typ, title, description string, tags []string, body string) {
	t.Helper()
	absPath := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(absPath), err)
	}
	content := frontmatter(typ, title, description, tags) + body
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
}

// setupGraphFixture creates a small typed corpus:
//   - req.md      (Requirement)
//   - impl.md     (Implementation) -> links req.md under "Implements"
//   - arch.md     (Architecture)  -> links impl.md under "Depends on"
func setupGraphFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeProfile(t, dir, graphProfileYAML)
	writeDoc(t, dir, "req.md", "Requirement", "Auth Requirement", "Must authenticate users", []string{"auth"},
		"# Auth Requirement\n\n- MFA required\n")
	writeDoc(t, dir, "impl.md", "Implementation", "Auth Implementation", "Implements auth", []string{"auth", "api"},
		"# Auth Implementation\n\n## Implements\n\n- [Requirement](req.md)\n")
	writeDoc(t, dir, "arch.md", "Architecture", "System Architecture", "Overall design", []string{"design"},
		"# System Architecture\n\n## Depends on\n\n- [Implementation](impl.md)\n")
	return dir
}

// setupNoProfileFixture creates two docs linked under an unrecognized heading.
// With no profile loaded, edges classify as "untyped".
func setupNoProfileFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeDoc(t, dir, "a.md", "Architecture", "A", "A doc", []string{},
		"# A\n\n## Links\n\n- [B](b.md)\n")
	writeDoc(t, dir, "b.md", "Architecture", "B", "B doc", []string{},
		"# B\n")
	return dir
}

// setupCoverageFixture creates a Requirement->Implementation coverage corpus:
//   - req1.md covered by impl1.md
//   - req2.md uncovered
func setupCoverageFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeProfile(t, dir, graphProfileYAML)
	writeDoc(t, dir, "req1.md", "Requirement", "Req 1", "First requirement", []string{}, "# Req 1\n")
	writeDoc(t, dir, "req2.md", "Requirement", "Req 2", "Second requirement", []string{}, "# Req 2\n")
	writeDoc(t, dir, "impl1.md", "Implementation", "Impl 1", "First implementation", []string{},
		"# Impl 1\n\n## Implements\n\n- [Req 1](req1.md)\n")
	writeDoc(t, dir, "impl2.md", "Implementation", "Impl 2", "Second implementation", []string{},
		"# Impl 2\n")
	return dir
}

// setupCircularFixture creates a 3-node cycle a->b->c->a via "Depends on".
func setupCircularFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeProfile(t, dir, graphProfileYAML)
	writeDoc(t, dir, "a.md", "Architecture", "A", "A", []string{},
		"# A\n\n## Depends on\n\n- [B](b.md)\n")
	writeDoc(t, dir, "b.md", "Architecture", "B", "B", []string{},
		"# B\n\n## Depends on\n\n- [C](c.md)\n")
	writeDoc(t, dir, "c.md", "Architecture", "C", "C", []string{},
		"# C\n\n## Depends on\n\n- [A](a.md)\n")
	return dir
}

// setupHighDegreeFixture creates a center.md with 5 outgoing and 5 incoming
// depends_on edges for context truncation tests.
func setupHighDegreeFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeProfile(t, dir, graphProfileYAML)

	centerBody := "# Center\n\n## Depends on\n"
	for i := 0; i < 5; i++ {
		centerBody += fmt.Sprintf("- [Out%d](out%d.md)\n", i, i)
	}
	writeDoc(t, dir, "center.md", "Architecture", "Center", "Center node", []string{}, centerBody)

	for i := 0; i < 5; i++ {
		writeDoc(t, dir, fmt.Sprintf("out%d.md", i), "Architecture", fmt.Sprintf("Out%d", i), "", []string{},
			fmt.Sprintf("# Out%d\n", i))
		writeDoc(t, dir, fmt.Sprintf("in%d.md", i), "Architecture", fmt.Sprintf("In%d", i), "", []string{},
			fmt.Sprintf("# In%d\n\n## Depends on\n\n- [Center](center.md)\n", i))
	}
	return dir
}

// setupPathTraversalFixture creates one safe doc and one doc with an escaping
// "../../../../" link that must be dropped per I-30.
func setupPathTraversalFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeProfile(t, dir, graphProfileYAML)
	writeDoc(t, dir, "safe.md", "Architecture", "Safe Doc", "Safe", []string{}, "# Safe\n")
	writeDoc(t, dir, "bad.md", "Architecture", "Bad Doc", "Bad", []string{},
		"# Bad\n\n## Depends on\n\n- [Escape](../../../../etc/passwd)\n- [Safe](safe.md)\n")
	return dir
}

// setupIntegrityFixture creates a corpus that triggers every integrity check:
//   - dangling.md   -> missing.md (dangling) plus a valid edge so it is not an orphan
//   - orphan.md     (no edges)    (orphan)
//   - arch.md       -> req_profile.md under Implements with wrong source type (profile violation)
//   - req_card.md   (Requirement, 0 incoming implements) (cardinality violation)
//   - new.md        -> old.md which is superseded (superseded dependency)
func setupIntegrityFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeProfile(t, dir, graphProfileYAML)
	writeDoc(t, dir, "req_card.md", "Requirement", "Req Card", "Requirement with no implementation", []string{},
		"# Req Card\n\n## Depends on\n\n- [Arch](arch.md)\n")
	writeDoc(t, dir, "req_profile.md", "Requirement", "Req Profile", "Requirement with wrong-source implementation", []string{}, "# Req Profile\n")
	writeDoc(t, dir, "arch.md", "Architecture", "Arch", "Architecture pretending to implement", []string{},
		"# Arch\n\n## Implements\n\n- [Req Profile](req_profile.md)\n")
	writeDoc(t, dir, "orphan.md", "Architecture", "Orphan", "No edges", []string{}, "# Orphan\n")
	writeDoc(t, dir, "old.md", "Architecture", "Old", "Old architecture", []string{}, "# Old\n")
	writeDoc(t, dir, "new.md", "Architecture", "New", "New architecture", []string{},
		"# New\n\n## Supersedes\n\n- [Old](old.md)\n")
	writeDoc(t, dir, "dangling.md", "Architecture", "Dangling", "Dangling link", []string{},
		"# Dangling\n\n## Depends on\n\n- [Missing](missing.md)\n- [Arch](arch.md)\n")
	return dir
}

// ---------------------------------------------------------------------------
// JSON parsing helpers
// ---------------------------------------------------------------------------

func unmarshalText(t *testing.T, result *mcp.CallToolResult, v any) {
	t.Helper()
	if result.IsError {
		t.Fatalf("expected success, got error: %s", getTextContent(t, result))
	}
	if err := json.Unmarshal([]byte(getTextContent(t, result)), v); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, getTextContent(t, result))
	}
}

func stringSlice(v any) []string {
	if v == nil {
		return nil
	}
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// anySlice returns v as a []any, treating JSON null as an empty slice.
func anySlice(v any) []any {
	if v == nil {
		return nil
	}
	s, _ := v.([]any)
	return s
}

func float64Int(v any) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}

func boolVal(v any) bool {
	b, _ := v.(bool)
	return b
}

// ---------------------------------------------------------------------------
// graph_concept tests
// ---------------------------------------------------------------------------

func TestGraphConcept_Found(t *testing.T) {
	dir := setupGraphFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var payload map[string]any
	unmarshalText(t, callTool(t, srv, "graph_concept", map[string]any{"file_path": "impl.md"}), &payload)

	if got, want := payload["file_path"], "impl.md"; got != want {
		t.Errorf("file_path: got %v, want %v", got, want)
	}
	if got, want := payload["type"], "Implementation"; got != want {
		t.Errorf("type: got %v, want %v", got, want)
	}
	if got, want := float64Int(payload["outgoing_count"]), 1; got != want {
		t.Errorf("outgoing_count: got %d, want %d", got, want)
	}
	if got, want := float64Int(payload["incoming_count"]), 1; got != want {
		t.Errorf("incoming_count: got %d, want %d", got, want)
	}
	outTypes := stringSlice(payload["outgoing_types"])
	if len(outTypes) != 1 || outTypes[0] != "implements" {
		t.Errorf("outgoing_types: got %v, want [implements]", outTypes)
	}
	inTypes := stringSlice(payload["incoming_types"])
	if len(inTypes) != 1 || inTypes[0] != "depended_on_by" {
		t.Errorf("incoming_types: got %v, want [depended_on_by]", inTypes)
	}
}

func TestGraphConcept_NotFound(t *testing.T) {
	dir := setupGraphFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	result := callTool(t, srv, "graph_concept", map[string]any{"file_path": "missing.md"})
	if !result.IsError {
		t.Fatalf("expected error, got success: %s", getTextContent(t, result))
	}
	text := getTextContent(t, result)
	if !strings.Contains(text, "concept not found") {
		t.Errorf("expected 'concept not found' error, got: %q", text)
	}
}

func TestGraphConcept_RequiresFilePath(t *testing.T) {
	dir := setupGraphFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	result := callTool(t, srv, "graph_concept", map[string]any{})
	if !result.IsError {
		t.Fatalf("expected error, got success: %s", getTextContent(t, result))
	}
	text := getTextContent(t, result)
	if !strings.Contains(text, "file_path is required") {
		t.Errorf("expected 'file_path is required' error, got: %q", text)
	}
}

// ---------------------------------------------------------------------------
// graph_relationships tests
// ---------------------------------------------------------------------------

func TestGraphRelationships_OutgoingIncomingBoth(t *testing.T) {
	dir := setupGraphFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var payload map[string]any
	unmarshalText(t, callTool(t, srv, "graph_relationships", map[string]any{
		"file_path": "impl.md",
		"direction": "outgoing",
	}), &payload)

	outgoing := anySlice(payload["outgoing"])
	if len(outgoing) != 1 {
		t.Fatalf("outgoing: got %d edges, want 1", len(outgoing))
	}
	edge := outgoing[0].(map[string]any)
	if got, want := edge["target"], "req.md"; got != want {
		t.Errorf("outgoing target: got %v, want %v", got, want)
	}
	if got, want := edge["type"], "implements"; got != want {
		t.Errorf("outgoing type: got %v, want %v", got, want)
	}

	unmarshalText(t, callTool(t, srv, "graph_relationships", map[string]any{
		"file_path": "req.md",
		"direction": "incoming",
	}), &payload)

	incoming := anySlice(payload["incoming"])
	if len(incoming) != 1 {
		t.Fatalf("incoming: got %d edges, want 1", len(incoming))
	}
	inedge := incoming[0].(map[string]any)
	if got, want := inedge["source"], "impl.md"; got != want {
		t.Errorf("incoming source: got %v, want %v", got, want)
	}
	if got, want := inedge["type"], "implemented_by"; got != want {
		t.Errorf("incoming type: got %v, want %v", got, want)
	}

	unmarshalText(t, callTool(t, srv, "graph_relationships", map[string]any{
		"file_path": "arch.md",
		"direction": "both",
	}), &payload)

	if len(anySlice(payload["outgoing"])) != 1 {
		t.Errorf("arch outgoing: got %d, want 1", len(anySlice(payload["outgoing"])))
	}
	if len(anySlice(payload["incoming"])) != 0 {
		t.Errorf("arch incoming: got %d, want 0", len(anySlice(payload["incoming"])))
	}
}

func TestGraphRelationships_TypeFilter(t *testing.T) {
	dir := setupGraphFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var payload map[string]any
	unmarshalText(t, callTool(t, srv, "graph_relationships", map[string]any{
		"file_path": "impl.md",
		"direction": "outgoing",
		"type":      "depends_on",
	}), &payload)

	if len(anySlice(payload["outgoing"])) != 0 {
		t.Errorf("outgoing depends_on filter: got %d edges, want 0", len(anySlice(payload["outgoing"])))
	}

	unmarshalText(t, callTool(t, srv, "graph_relationships", map[string]any{
		"file_path": "impl.md",
		"direction": "outgoing",
		"type":      "implements",
	}), &payload)

	if len(anySlice(payload["outgoing"])) != 1 {
		t.Errorf("outgoing implements filter: got %d edges, want 1", len(anySlice(payload["outgoing"])))
	}
}

func TestGraphRelationships_InvalidDirection(t *testing.T) {
	dir := setupGraphFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	result := callTool(t, srv, "graph_relationships", map[string]any{
		"file_path": "impl.md",
		"direction": "sideways",
	})
	if !result.IsError {
		t.Fatalf("expected error for invalid direction, got success")
	}
	text := getTextContent(t, result)
	if !strings.Contains(text, "sideways") {
		t.Errorf("error should mention invalid value, got: %q", text)
	}
}

// ---------------------------------------------------------------------------
// graph_trace tests
// ---------------------------------------------------------------------------

func TestGraphTrace_DownstreamDepthBound(t *testing.T) {
	dir := setupGraphFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var payload map[string]any
	unmarshalText(t, callTool(t, srv, "graph_trace", map[string]any{
		"file_path": "arch.md",
		"direction": "downstream",
		"max_depth": float64(2),
	}), &payload)

	if got, want := payload["start"], "arch.md"; got != want {
		t.Errorf("start: got %v, want %v", got, want)
	}
	steps := anySlice(payload["steps"])
	if len(steps) != 2 {
		t.Fatalf("steps: got %d, want 2", len(steps))
	}
	first := steps[0].(map[string]any)
	if got, want := first["node"], "impl.md"; got != want {
		t.Errorf("step[0].node: got %v, want %v", got, want)
	}
	if got, want := float64Int(first["depth"]), 1; got != want {
		t.Errorf("step[0].depth: got %d, want %d", got, want)
	}
	second := steps[1].(map[string]any)
	if got, want := second["node"], "req.md"; got != want {
		t.Errorf("step[1].node: got %v, want %v", got, want)
	}
	if got, want := float64Int(second["depth"]), 2; got != want {
		t.Errorf("step[1].depth: got %d, want %d", got, want)
	}
	if got, want := float64Int(payload["total_reachable"]), 2; got != want {
		t.Errorf("total_reachable: got %d, want %d", got, want)
	}
}

func TestGraphTrace_DepthLimitOne(t *testing.T) {
	dir := setupGraphFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var payload map[string]any
	unmarshalText(t, callTool(t, srv, "graph_trace", map[string]any{
		"file_path": "arch.md",
		"direction": "downstream",
		"max_depth": float64(1),
	}), &payload)

	steps := anySlice(payload["steps"])
	if len(steps) != 1 {
		t.Fatalf("steps: got %d, want 1", len(steps))
	}
}

func TestGraphTrace_CycleTermination(t *testing.T) {
	dir := setupCircularFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var payload map[string]any
	unmarshalText(t, callTool(t, srv, "graph_trace", map[string]any{
		"file_path": "a.md",
		"direction": "downstream",
		"max_depth": float64(5),
	}), &payload)

	steps := anySlice(payload["steps"])
	if len(steps) != 2 {
		t.Fatalf("cycle trace: got %d steps, want 2 (cycle must terminate)", len(steps))
	}
	visited := map[string]bool{}
	for _, s := range steps {
		node := s.(map[string]any)["node"].(string)
		if visited[node] {
			t.Errorf("node %q visited more than once (cycle not terminated)", node)
		}
		visited[node] = true
	}
}

func TestGraphTrace_InvalidDirection(t *testing.T) {
	dir := setupGraphFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	result := callTool(t, srv, "graph_trace", map[string]any{
		"file_path": "a.md",
		"direction": "sideways",
	})
	if !result.IsError {
		t.Fatalf("expected error for invalid direction, got success")
	}
}

// ---------------------------------------------------------------------------
// graph_search tests
// ---------------------------------------------------------------------------

func TestGraphSearch_TextQueryAndScore(t *testing.T) {
	dir := setupGraphFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var payload map[string]any
	unmarshalText(t, callTool(t, srv, "graph_search", map[string]any{
		"query": "requirement",
	}), &payload)

	concepts := anySlice(payload["concepts"])
	if len(concepts) == 0 {
		t.Fatalf("expected results for 'requirement', got 0")
	}
	first := concepts[0].(map[string]any)
	if got, want := first["file_path"], "req.md"; got != want {
		t.Errorf("top result: got %v, want %v", got, want)
	}
	if _, ok := first["score"]; !ok {
		t.Errorf("missing score field in search result")
	}
	if got, want := float64Int(payload["total"]), len(concepts); got != want {
		t.Errorf("total mismatch: got %d, want %d", got, want)
	}
}

func TestGraphSearch_TypeFilter(t *testing.T) {
	dir := setupGraphFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var payload map[string]any
	unmarshalText(t, callTool(t, srv, "graph_search", map[string]any{
		"query": "",
		"type":  "Implementation",
	}), &payload)

	concepts := anySlice(payload["concepts"])
	if len(concepts) != 1 {
		t.Fatalf("type filter: got %d results, want 1", len(concepts))
	}
	if got, want := concepts[0].(map[string]any)["file_path"], "impl.md"; got != want {
		t.Errorf("result: got %v, want %v", got, want)
	}
}

func TestGraphSearch_TagFilter(t *testing.T) {
	dir := setupGraphFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var payload map[string]any
	unmarshalText(t, callTool(t, srv, "graph_search", map[string]any{
		"query": "",
		"tags":  []any{"api"},
	}), &payload)

	concepts := anySlice(payload["concepts"])
	if len(concepts) != 1 {
		t.Fatalf("tag filter [api]: got %d results, want 1", len(concepts))
	}
	if got, want := concepts[0].(map[string]any)["file_path"], "impl.md"; got != want {
		t.Errorf("result: got %v, want %v", got, want)
	}
}

func TestGraphSearch_Limit(t *testing.T) {
	dir := setupGraphFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var payload map[string]any
	unmarshalText(t, callTool(t, srv, "graph_search", map[string]any{
		"query": "",
		"limit": float64(2),
	}), &payload)

	concepts := anySlice(payload["concepts"])
	if len(concepts) != 2 {
		t.Fatalf("limit 2: got %d results, want 2", len(concepts))
	}
	if got, want := float64Int(payload["total"]), 3; got != want {
		t.Errorf("total: got %d, want 3 (all matching nodes)", got)
	}
}

// ---------------------------------------------------------------------------
// graph_integrity tests
// ---------------------------------------------------------------------------

func TestGraphIntegrity_AllChecks(t *testing.T) {
	dir := setupIntegrityFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var payload map[string]any
	unmarshalText(t, callTool(t, srv, "graph_integrity", nil), &payload)

	if !boolVal(payload["profile_loaded"]) {
		t.Errorf("profile_loaded: got false, want true")
	}

	summary := payload["summary"].(map[string]any)
	if got, want := float64Int(summary["dangling_refs"]), 1; got != want {
		t.Errorf("dangling_refs: got %d, want %d", got, want)
	}
	if got, want := float64Int(summary["orphan_concepts"]), 1; got != want {
		t.Errorf("orphan_concepts: got %d, want %d", got, want)
	}
	if got, want := float64Int(summary["profile_violations"]), 3; got != want {
		t.Errorf("profile_violations: got %d, want %d (profile + 2 cardinality)", got, want)
	}
	if got, want := float64Int(summary["superseded_deps"]), 1; got != want {
		t.Errorf("superseded_deps: got %d, want %d", got, want)
	}
	if got, want := float64Int(summary["total_findings"]), 6; got != want {
		t.Errorf("total_findings: got %d, want %d", got, want)
	}

	findings := anySlice(payload["findings"])
	checks := map[string]int{}
	for _, f := range findings {
		check := f.(map[string]any)["check"].(string)
		checks[check]++
	}
	wantChecks := map[string]int{
		"dangling":             1,
		"orphans":              1,
		"profile_violations":   1,
		"cardinality_violation": 2,
		"superseded_deps":      1,
	}
	for check, want := range wantChecks {
		if got := checks[check]; got != want {
			t.Errorf("check %q: got %d findings, want %d", check, got, want)
		}
	}
}

func TestGraphIntegrity_ChecksSubset(t *testing.T) {
	dir := setupIntegrityFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var payload map[string]any
	unmarshalText(t, callTool(t, srv, "graph_integrity", map[string]any{
		"checks": []any{"dangling"},
	}), &payload)

	findings := anySlice(payload["findings"])
	if len(findings) != 1 {
		t.Fatalf("subset [dangling]: got %d findings, want 1", len(findings))
	}
	if got, want := findings[0].(map[string]any)["check"], "dangling"; got != want {
		t.Errorf("finding check: got %v, want %v", got, want)
	}
}

func TestGraphIntegrity_NoProfile(t *testing.T) {
	dir := setupNoProfileFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var payload map[string]any
	unmarshalText(t, callTool(t, srv, "graph_integrity", nil), &payload)

	if boolVal(payload["profile_loaded"]) {
		t.Errorf("profile_loaded: got true, want false")
	}
	summary := payload["summary"].(map[string]any)
	if got, want := float64Int(summary["profile_violations"]), 0; got != want {
		t.Errorf("profile_violations: got %d, want 0", got)
	}
}

func TestGraphIntegrity_InvalidCheck(t *testing.T) {
	dir := setupIntegrityFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	result := callTool(t, srv, "graph_integrity", map[string]any{
		"checks": []any{"bogus"},
	})
	if !result.IsError {
		t.Fatalf("expected error for invalid check, got success")
	}
	text := getTextContent(t, result)
	if !strings.Contains(text, "bogus") {
		t.Errorf("error should mention invalid check, got: %q", text)
	}
}

// ---------------------------------------------------------------------------
// graph_coverage tests
// ---------------------------------------------------------------------------

func TestGraphCoverage_CoveredUncovered(t *testing.T) {
	dir := setupCoverageFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var payload map[string]any
	unmarshalText(t, callTool(t, srv, "graph_coverage", map[string]any{
		"source_type":  "Requirement",
		"target_type":  "Implementation",
		"relationship": "implements",
	}), &payload)

	if got, want := payload["source_type"], "Requirement"; got != want {
		t.Errorf("source_type: got %v, want %v", got, want)
	}
	if got, want := payload["target_type"], "Implementation"; got != want {
		t.Errorf("target_type: got %v, want %v", got, want)
	}
	if got, want := float64Int(payload["total_sources"]), 2; got != want {
		t.Errorf("total_sources: got %d, want %d", got, want)
	}
	if got, want := float64Int(payload["covered"]), 1; got != want {
		t.Errorf("covered: got %d, want %d", got, want)
	}
	if got, want := float64Int(payload["uncovered"]), 1; got != want {
		t.Errorf("uncovered: got %d, want %d", got, want)
	}
	if payload["coverage_ratio"].(float64) != 0.5 {
		t.Errorf("coverage_ratio: got %v, want 0.5", payload["coverage_ratio"])
	}
	uncovered := anySlice(payload["uncovered_items"])
	if len(uncovered) != 1 {
		t.Fatalf("uncovered_items: got %d, want 1", len(uncovered))
	}
	if got, want := uncovered[0].(map[string]any)["file_path"], "req2.md"; got != want {
		t.Errorf("uncovered file_path: got %v, want %v", got, want)
	}
}

func TestGraphCoverage_RequiresTypes(t *testing.T) {
	dir := setupCoverageFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	result := callTool(t, srv, "graph_coverage", map[string]any{
		"target_type": "Implementation",
	})
	if !result.IsError {
		t.Fatalf("expected error for missing source_type, got success")
	}
}

// ---------------------------------------------------------------------------
// graph_context tests
// ---------------------------------------------------------------------------

func TestGraphContext_DepthOne(t *testing.T) {
	dir := setupGraphFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var payload map[string]any
	unmarshalText(t, callTool(t, srv, "graph_context", map[string]any{
		"file_path": "impl.md",
		"depth":     float64(1),
	}), &payload)

	center := payload["center"].(map[string]any)
	if got, want := center["file_path"], "impl.md"; got != want {
		t.Errorf("center file_path: got %v, want %v", got, want)
	}
	neighbors := payload["neighbors"].(map[string]any)
	upstream := anySlice(neighbors["upstream"])
	downstream := anySlice(neighbors["downstream"])
	if len(upstream) != 1 || upstream[0].(map[string]any)["file_path"] != "arch.md" {
		t.Errorf("upstream: got %v, want [arch.md]", upstream)
	}
	if len(downstream) != 1 || downstream[0].(map[string]any)["file_path"] != "req.md" {
		t.Errorf("downstream: got %v, want [req.md]", downstream)
	}
	if got, want := float64Int(payload["total_neighbors"]), 2; got != want {
		t.Errorf("total_neighbors: got %d, want %d", got, want)
	}
	if boolVal(payload["truncated"]) {
		t.Errorf("truncated: got true, want false")
	}
}

func TestGraphContext_DepthTwo(t *testing.T) {
	dir := setupGraphFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var payload map[string]any
	unmarshalText(t, callTool(t, srv, "graph_context", map[string]any{
		"file_path": "arch.md",
		"depth":     float64(2),
	}), &payload)

	neighbors := payload["neighbors"].(map[string]any)
	upstream := anySlice(neighbors["upstream"])
	downstream := anySlice(neighbors["downstream"])
	if got, want := float64Int(payload["depth"]), 2; got != want {
		t.Errorf("depth: got %d, want %d", got, want)
	}
	if len(downstream) != 2 {
		t.Errorf("depth-2 downstream: got %d neighbors, want 2", len(downstream))
	}
	if len(upstream) != 0 {
		t.Errorf("depth-2 upstream: got %d neighbors, want 0", len(upstream))
	}
}

func TestGraphContext_Truncated(t *testing.T) {
	dir := setupHighDegreeFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var payload map[string]any
	unmarshalText(t, callTool(t, srv, "graph_context", map[string]any{
		"file_path":  "center.md",
		"depth":      float64(1),
		"max_results": float64(2),
	}), &payload)

	total := float64Int(payload["total_neighbors"])
	if total != 2 {
		t.Errorf("total_neighbors: got %d, want 2", total)
	}
	if !boolVal(payload["truncated"]) {
		t.Errorf("truncated: got false, want true")
	}
}

func TestGraphContext_NotFound(t *testing.T) {
	dir := setupGraphFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	result := callTool(t, srv, "graph_context", map[string]any{
		"file_path": "missing.md",
	})
	if !result.IsError {
		t.Fatalf("expected error for missing concept, got success")
	}
	text := getTextContent(t, result)
	if !strings.Contains(text, "concept not found") {
		t.Errorf("expected 'concept not found' error, got: %q", text)
	}
}

// ---------------------------------------------------------------------------
// I-30 path traversal integration
// ---------------------------------------------------------------------------

func TestGraph_I30_PathTraversalDropped(t *testing.T) {
	dir := setupPathTraversalFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	// The escaping target must not appear as a node.
	result := callTool(t, srv, "graph_concept", map[string]any{
		"file_path": "../../../../etc/passwd",
	})
	if !result.IsError {
		t.Fatalf("escaping target must not be a concept")
	}

	// The escaping target must not appear as an outgoing edge from bad.md.
	var rels map[string]any
	unmarshalText(t, callTool(t, srv, "graph_relationships", map[string]any{
		"file_path": "bad.md",
		"direction": "outgoing",
	}), &rels)

	outgoing := anySlice(rels["outgoing"])
	if len(outgoing) != 1 {
		t.Fatalf("bad.md outgoing: got %d edges, want 1 (only safe.md)", len(outgoing))
	}
	if got, want := outgoing[0].(map[string]any)["target"], "safe.md"; got != want {
		t.Errorf("outgoing target: got %v, want %v", got, want)
	}

	// Trace downstream from bad.md must stop at safe.md.
	var trace map[string]any
	unmarshalText(t, callTool(t, srv, "graph_trace", map[string]any{
		"file_path": "bad.md",
		"direction": "downstream",
		"max_depth": float64(3),
	}), &trace)

	for _, s := range anySlice(trace["steps"]) {
		node := s.(map[string]any)["node"].(string)
		if strings.Contains(node, "..") {
			t.Errorf("trace includes escaping node %q", node)
		}
	}

	// Integrity must not report the escaping link as dangling.
	var integrity map[string]any
	unmarshalText(t, callTool(t, srv, "graph_integrity", nil), &integrity)

	findings := anySlice(integrity["findings"])
	for _, f := range findings {
		finding := f.(map[string]any)
		target := finding["target"].(string)
		if strings.Contains(target, "..") {
			t.Errorf("integrity includes escaping target %q", target)
		}
	}
}

// ---------------------------------------------------------------------------
// I-31 profile alias uniqueness integration
// ---------------------------------------------------------------------------

func TestCLI_ProfileDuplicateAliases_Exit2(t *testing.T) {
	dir := t.TempDir()
	badProfile := `name: bad
version: "1.0"
relationships:
  - name: rel1
    heading_aliases:
      - "Same"
  - name: rel2
    heading_aliases:
      - "Same"
`
	profilePath := filepath.Join(dir, "bad-profile.yaml")
	if err := os.WriteFile(profilePath, []byte(badProfile), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	bin := buildBinary(t)
	exit := runBinary(t, bin, dir, "--profile", profilePath)
	if exit != 2 {
		t.Errorf("exit code: got %d, want 2", exit)
	}
}

// ---------------------------------------------------------------------------
// I-28 zero-doc startup
// ---------------------------------------------------------------------------

func TestGraph_ZeroDocStartup(t *testing.T) {
	dir := t.TempDir()
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	// graph_concept
	result := callTool(t, srv, "graph_concept", map[string]any{"file_path": "x.md"})
	if !result.IsError {
		t.Errorf("graph_concept on empty index should error")
	}

	// graph_relationships
	var rels map[string]any
	unmarshalText(t, callTool(t, srv, "graph_relationships", map[string]any{
		"file_path": "x.md",
		"direction": "both",
	}), &rels)
	if len(anySlice(rels["outgoing"])) != 0 || len(anySlice(rels["incoming"])) != 0 {
		t.Errorf("graph_relationships on empty index should return empty arrays")
	}

	// graph_trace
	var trace map[string]any
	unmarshalText(t, callTool(t, srv, "graph_trace", map[string]any{
		"file_path": "x.md",
		"direction": "downstream",
	}), &trace)
	if len(anySlice(trace["steps"])) != 0 {
		t.Errorf("graph_trace on empty index should return empty steps")
	}

	// graph_search
	var search map[string]any
	unmarshalText(t, callTool(t, srv, "graph_search", map[string]any{"query": "x"}), &search)
	if len(anySlice(search["concepts"])) != 0 {
		t.Errorf("graph_search on empty index should return empty concepts")
	}

	// graph_integrity
	var integrity map[string]any
	unmarshalText(t, callTool(t, srv, "graph_integrity", nil), &integrity)
	if len(anySlice(integrity["findings"])) != 0 {
		t.Errorf("graph_integrity on empty index should return empty findings")
	}

	// graph_coverage
	var coverage map[string]any
	unmarshalText(t, callTool(t, srv, "graph_coverage", map[string]any{
		"source_type": "Requirement",
		"target_type": "Implementation",
	}), &coverage)
	if float64Int(coverage["total_sources"]) != 0 {
		t.Errorf("graph_coverage on empty index should return 0 total_sources")
	}

	// graph_context
	result = callTool(t, srv, "graph_context", map[string]any{"file_path": "x.md"})
	if !result.IsError {
		t.Errorf("graph_context on empty index should error")
	}
}

// ---------------------------------------------------------------------------
// I-20 rebuild consistency
// ---------------------------------------------------------------------------

func TestGraph_RebuildConsistency(t *testing.T) {
	dir := setupGraphFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var before map[string]any
	unmarshalText(t, callTool(t, srv, "graph_relationships", map[string]any{
		"file_path": "impl.md",
		"direction": "outgoing",
	}), &before)
	if len(anySlice(before["outgoing"])) != 1 {
		t.Fatalf("before mutation: expected 1 outgoing edge, got %d", len(anySlice(before["outgoing"])))
	}

	// Mutate impl.md to remove the link.
	newBody := frontmatter("Implementation", "Auth Implementation", "Implements auth", []string{"auth", "api"}) +
		"# Auth Implementation\n\nNo links now.\n"
	if err := os.WriteFile(filepath.Join(dir, "impl.md"), []byte(newBody), 0o644); err != nil {
		t.Fatalf("rewrite impl.md: %v", err)
	}

	var after map[string]any
	unmarshalText(t, callTool(t, srv, "graph_relationships", map[string]any{
		"file_path": "impl.md",
		"direction": "outgoing",
	}), &after)
	if len(anySlice(after["outgoing"])) != 0 {
		t.Errorf("after mutation: expected 0 outgoing edges, got %d", len(anySlice(after["outgoing"])))
	}
}

// ---------------------------------------------------------------------------
// I-26 no-profile fixture
// ---------------------------------------------------------------------------

func TestGraph_NoProfile_AllEdgesUntyped(t *testing.T) {
	dir := setupNoProfileFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var rels map[string]any
	unmarshalText(t, callTool(t, srv, "graph_relationships", map[string]any{
		"file_path": "a.md",
		"direction": "outgoing",
	}), &rels)

	outgoing := anySlice(rels["outgoing"])
	if len(outgoing) != 1 {
		t.Fatalf("outgoing: got %d edges, want 1", len(outgoing))
	}
	if got, want := outgoing[0].(map[string]any)["type"], "untyped"; got != want {
		t.Errorf("edge type without profile: got %v, want %v", got, want)
	}
}

func TestGraph_NoProfile_IntegrityNoViolations(t *testing.T) {
	dir := setupNoProfileFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var integrity map[string]any
	unmarshalText(t, callTool(t, srv, "graph_integrity", nil), &integrity)

	if boolVal(integrity["profile_loaded"]) {
		t.Errorf("profile_loaded: got true, want false")
	}
	summary := integrity["summary"].(map[string]any)
	if got, want := float64Int(summary["profile_violations"]), 0; got != want {
		t.Errorf("profile_violations: got %d, want 0", got)
	}
	if got, want := float64Int(summary["total_findings"]), 0; got != want {
		t.Errorf("total_findings: got %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// Relative path invariant (I-27) spot checks
// ---------------------------------------------------------------------------

func TestGraph_RelativePaths_I27(t *testing.T) {
	dir := setupGraphFixture(t)
	srv := newFixtureServer(t, dir, scanner.ScanOptions{})
	defer srv.Close()

	var concept map[string]any
	unmarshalText(t, callTool(t, srv, "graph_concept", map[string]any{"file_path": "impl.md"}), &concept)
	fp := concept["file_path"].(string)
	if strings.HasPrefix(fp, "/") {
		t.Errorf("I-27 violation: concept file_path is absolute: %q", fp)
	}

	var rels map[string]any
	unmarshalText(t, callTool(t, srv, "graph_relationships", map[string]any{
		"file_path": "impl.md",
		"direction": "outgoing",
	}), &rels)
	target := anySlice(rels["outgoing"])[0].(map[string]any)["target"].(string)
	if strings.HasPrefix(target, "/") {
		t.Errorf("I-27 violation: relationship target is absolute: %q", target)
	}
}
