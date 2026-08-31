package graph

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/nrkno/plattform-okf-mcp/internal/parser"
	"github.com/nrkno/plattform-okf-mcp/internal/profile"
)

// writeDoc writes a markdown file with frontmatter and body to dir and returns
// its relative path from dir.
func writeDoc(t *testing.T, dir string, relPath string, frontmatter string, body string) string {
	t.Helper()
	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "---\n" + frontmatter + "---\n" + body
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return relPath
}

// parseFixture parses a file written by writeDoc and returns its Doc.
func parseFixture(t *testing.T, dir string, relPath string) parser.Doc {
	t.Helper()
	doc, ok, err := parser.Parse(filepath.Join(dir, relPath))
	if err != nil {
		t.Fatalf("parse %s: %v", relPath, err)
	}
	if !ok {
		t.Fatalf("parse %s: not indexed", relPath)
	}
	doc.FilePath = relPath
	return doc
}

// loadProfile writes yaml to dir/.okf-profile.yaml and loads it.
func loadProfile(t *testing.T, dir string, yaml string) *profile.Profile {
	t.Helper()
	path := filepath.Join(dir, ".okf-profile.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	p, err := profile.Load(path)
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	return p
}

func TestBuild_EmptyDocs(t *testing.T) {
	g := Build(nil, t.TempDir(), profile.Default())
	if g == nil {
		t.Fatal("Build returned nil")
	}
	if len(g.Nodes) != 0 || len(g.OutEdges) != 0 || len(g.InEdges) != 0 || len(g.Dangling) != 0 {
		t.Fatalf("expected empty graph, got nodes=%d out=%d in=%d dangling=%d",
			len(g.Nodes), len(g.OutEdges), len(g.InEdges), len(g.Dangling))
	}
}

func TestBuild_SingleDocNoLinks(t *testing.T) {
	dir := t.TempDir()
	rel := writeDoc(t, dir, "a.md", "type: A\ntitle: Alpha\n", "# Alpha\nNo links here.\n")
	doc := parseFixture(t, dir, rel)

	g := Build([]parser.Doc{doc}, dir, profile.Default())
	if len(g.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(g.Nodes))
	}
	n, ok := g.Nodes["a.md"]
	if !ok {
		t.Fatal("node a.md not found")
	}
	if n.FilePath != "a.md" {
		t.Errorf("FilePath = %q, want relative %q", n.FilePath, "a.md")
	}
	if len(g.OutEdges["a.md"]) != 0 || len(g.InEdges["a.md"]) != 0 {
		t.Fatalf("expected zero edges")
	}
}

func TestBuild_TwoDocsWithLink(t *testing.T) {
	dir := t.TempDir()
	a := writeDoc(t, dir, "a.md", "type: A\ntitle: Alpha\n", "# Alpha\nSee [Beta](b.md).\n")
	b := writeDoc(t, dir, "b.md", "type: B\ntitle: Beta\n", "# Beta\nNo links.\n")
	docs := []parser.Doc{parseFixture(t, dir, a), parseFixture(t, dir, b)}

	g := Build(docs, dir, profile.Default())
	out := g.OutEdges["a.md"]
	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing edge from a, got %d", len(out))
	}
	if out[0].Source != "a.md" || out[0].Target != "b.md" || out[0].Type != "untyped" {
		t.Errorf("unexpected edge: %+v", out[0])
	}
	in := g.InEdges["b.md"]
	if len(in) != 1 {
		t.Fatalf("expected 1 incoming edge to b, got %d", len(in))
	}
	if in[0].Source != "a.md" || in[0].Target != "b.md" || in[0].Type != "referenced_by" || in[0].InverseOf != "untyped" {
		t.Errorf("unexpected inverse edge: %+v", in[0])
	}
}

func TestBuild_BodyIsFrontmatterStripped(t *testing.T) {
	dir := t.TempDir()
	// The body contains a line that looks like frontmatter; it must be parsed
	// as body, not as frontmatter, so the link inside it is extracted.
	front := "type: A\ntitle: Alpha\n"
	body := "# Alpha\n---\n[link](b.md)\n"
	a := writeDoc(t, dir, "a.md", front, body)
	b := writeDoc(t, dir, "b.md", "type: B\n", "# B\n")
	docs := []parser.Doc{parseFixture(t, dir, a), parseFixture(t, dir, b)}

	g := Build(docs, dir, profile.Default())
	if len(g.OutEdges["a.md"]) != 1 {
		t.Fatalf("expected link inside pseudo-frontmatter body to be extracted, got %d edges", len(g.OutEdges["a.md"]))
	}
}

func TestBuild_HeadingClassification(t *testing.T) {
	dir := t.TempDir()
	prof := loadProfile(t, dir, `version: "1.0"
relationships:
  - name: "depends_on"
    heading_aliases: ["Depends on", "Dependencies"]
    inverse: "depended_on_by"
`)
	a := writeDoc(t, dir, "a.md", "type: A\n", "# A\n## Dependencies\nSee [b](b.md).\n## Notes\nSee [c](c.md).\n")
	b := writeDoc(t, dir, "b.md", "type: B\n", "# B\n")
	c := writeDoc(t, dir, "c.md", "type: C\n", "# C\n")
	docs := []parser.Doc{parseFixture(t, dir, a), parseFixture(t, dir, b), parseFixture(t, dir, c)}

	g := Build(docs, dir, prof)
	out := g.OutEdges["a.md"]
	if len(out) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(out))
	}
	if out[0].Target != "b.md" || out[0].Type != "depends_on" {
		t.Errorf("expected typed edge to b, got %+v", out[0])
	}
	if out[1].Target != "c.md" || out[1].Type != "untyped" {
		t.Errorf("expected untyped edge to c, got %+v", out[1])
	}

	// No profile: all untyped.
	g2 := Build(docs, dir, profile.Default())
	for _, e := range g2.OutEdges["a.md"] {
		if e.Type != "untyped" {
			t.Errorf("expected untyped without profile, got %q", e.Type)
		}
	}
}

func TestBuild_InverseEdges(t *testing.T) {
	dir := t.TempDir()
	prof := loadProfile(t, dir, `version: "1.0"
relationships:
  - name: "implements"
    heading_aliases: ["Implements"]
    inverse: "implemented_by"
`)
	a := writeDoc(t, dir, "impl.md", "type: Implementation\n", "# Impl\n## Implements\nSee [req](req.md).\n")
	b := writeDoc(t, dir, "req.md", "type: Requirement\n", "# Req\n")
	docs := []parser.Doc{parseFixture(t, dir, a), parseFixture(t, dir, b)}

	g := Build(docs, dir, prof)
	out := g.OutEdges["impl.md"]
	if len(out) != 1 || out[0].Type != "implements" {
		t.Fatalf("expected implements edge, got %+v", out)
	}
	in := g.InEdges["req.md"]
	if len(in) != 1 {
		t.Fatalf("expected 1 incoming edge to req, got %d", len(in))
	}
	if in[0].Source != "impl.md" || in[0].Target != "req.md" || in[0].Type != "implemented_by" || in[0].InverseOf != "implements" {
		t.Errorf("unexpected inverse edge: %+v", in[0])
	}
}

func TestBuild_DanglingReference(t *testing.T) {
	dir := t.TempDir()
	a := writeDoc(t, dir, "a.md", "type: A\n", "# A\n## Dependencies\nSee [missing](/no/such.md).\n")
	docs := []parser.Doc{parseFixture(t, dir, a)}

	g := Build(docs, dir, profile.Default())
	if len(g.Dangling) != 1 {
		t.Fatalf("expected 1 dangling ref, got %d", len(g.Dangling))
	}
	d := g.Dangling[0]
	if d.Source != "a.md" || d.Target != "/no/such.md" || d.Heading != "Dependencies" || d.Line == 0 {
		t.Errorf("unexpected dangling ref: %+v", d)
	}
	if len(g.OutEdges["a.md"]) != 0 {
		t.Errorf("dangling target must not appear in OutEdges")
	}
}

func TestBuild_NoPhantomNodes(t *testing.T) {
	dir := t.TempDir()
	// a.md links to b.md which has no frontmatter/type.
	a := writeDoc(t, dir, "a.md", "type: A\n", "# A\nSee [b](b.md).\n")
	if err := os.WriteFile(filepath.Join(dir, "b.md"), []byte("# B\n"), 0o644); err != nil {
		t.Fatalf("write b.md: %v", err)
	}
	docs := []parser.Doc{parseFixture(t, dir, a)}

	g := Build(docs, dir, profile.Default())
	if _, ok := g.Nodes["b.md"]; ok {
		t.Error("b.md should not be a node")
	}
	if len(g.Dangling) != 1 || g.Dangling[0].Target != "b.md" {
		t.Errorf("b.md should be dangling, got %+v", g.Dangling)
	}
}

func TestBuild_I30_EscapingLinkDropped(t *testing.T) {
	dir := t.TempDir()
	// Place doc deep so a relative traversal can escape the corpus root.
	writeDoc(t, dir, "sub/deep/a.md", "type: A\n", "# A\n[escape](../../../../etc/passwd)\n")
	docs := []parser.Doc{parseFixture(t, dir, "sub/deep/a.md")}

	g := Build(docs, dir, profile.Default())
	if len(g.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(g.Nodes))
	}
	if len(g.OutEdges["sub/deep/a.md"]) != 0 {
		t.Errorf("escaping link must not produce an edge")
	}
	if len(g.Dangling) != 0 {
		t.Errorf("escaping link must not produce a dangling ref, got %+v", g.Dangling)
	}
}

func TestBuild_Determinism(t *testing.T) {
	dir := t.TempDir()
	for _, rel := range []string{"z.md", "a.md", "m.md"} {
		writeDoc(t, dir, rel, "type: T\n", "# T\n[root](root.md)\n")
	}
	writeDoc(t, dir, "root.md", "type: T\n", "# Root\n")

	var docs []parser.Doc
	for _, rel := range []string{"z.md", "a.md", "m.md", "root.md"} {
		docs = append(docs, parseFixture(t, dir, rel))
	}

	g1 := Build(docs, dir, profile.Default())
	// Shuffle order.
	shuffled := []parser.Doc{docs[3], docs[0], docs[2], docs[1]}
	g2 := Build(shuffled, dir, profile.Default())

	if !reflect.DeepEqual(g1, g2) {
		t.Error("shuffled build produced different graph")
	}
}

func TestConcept(t *testing.T) {
	dir := t.TempDir()
	rel := writeDoc(t, dir, "a.md", "type: A\ntitle: Alpha\ndescription: desc\ntags: [x, y]\n", "# A\n")
	doc := parseFixture(t, dir, rel)
	g := Build([]parser.Doc{doc}, dir, profile.Default())

	n, ok := g.Concept("a.md")
	if !ok || n.Title != "Alpha" || n.Type != "A" || n.Description != "desc" {
		t.Errorf("unexpected concept: %+v", n)
	}
	if !reflect.DeepEqual(n.Tags, []string{"x", "y"}) {
		t.Errorf("tags = %v", n.Tags)
	}
	if _, ok := g.Concept("missing.md"); ok {
		t.Error("expected missing concept")
	}
}

func TestSearch(t *testing.T) {
	dir := t.TempDir()
	a := writeDoc(t, dir, "a.md", "type: A\ntitle: Alpha One\ntags: [x]\n", "# A\n")
	b := writeDoc(t, dir, "b.md", "type: B\ntitle: Beta Two\ntags: [y]\n", "# B\n")
	docs := []parser.Doc{parseFixture(t, dir, a), parseFixture(t, dir, b)}
	g := Build(docs, dir, profile.Default())

	// Empty query returns all nodes with score 0, sorted by path.
	res := g.Search("", "", nil)
	if len(res) != 2 || res[0].Node.FilePath != "a.md" || res[0].Score != 0 {
		t.Errorf("empty query result: %+v", res)
	}

	// Text query: matches are sorted by score desc, then path. Non-matching
	// nodes still score 0 and are included (only -1 sentinel is excluded).
	res = g.Search("alpha", "", nil)
	if len(res) != 2 || res[0].Node.FilePath != "a.md" || res[0].Score != 3 || res[1].Score != 0 {
		t.Errorf("alpha query result: %+v", res)
	}

	// Tag filter excludes non-matching.
	res = g.Search("", "", []string{"y"})
	if len(res) != 1 || res[0].Node.FilePath != "b.md" {
		t.Errorf("tag filter result: %+v", res)
	}

	// Type filter.
	res = g.Search("", "B", nil)
	if len(res) != 1 || res[0].Node.FilePath != "b.md" {
		t.Errorf("type filter result: %+v", res)
	}
}

func TestOutgoingIncoming(t *testing.T) {
	dir := t.TempDir()
	prof := loadProfile(t, dir, `version: "1.0"
relationships:
  - name: "depends_on"
    heading_aliases: ["Depends on"]
    inverse: "depended_on_by"
  - name: "implements"
    heading_aliases: ["Implements"]
    inverse: "implemented_by"
`)
	a := writeDoc(t, dir, "a.md", "type: A\n", "# A\n## Depends on\n[b](b.md)\n## Implements\n[c](c.md)\n")
	b := writeDoc(t, dir, "b.md", "type: B\n", "# B\n")
	c := writeDoc(t, dir, "c.md", "type: C\n", "# C\n")
	docs := []parser.Doc{parseFixture(t, dir, a), parseFixture(t, dir, b), parseFixture(t, dir, c)}
	g := Build(docs, dir, prof)

	outAll := g.Outgoing("a.md", "")
	if len(outAll) != 2 {
		t.Fatalf("expected 2 outgoing edges, got %d", len(outAll))
	}
	outDep := g.Outgoing("a.md", "depends_on")
	if len(outDep) != 1 || outDep[0].Target != "b.md" {
		t.Errorf("unexpected depends_on filter: %+v", outDep)
	}
	inToC := g.Incoming("c.md", "")
	if len(inToC) != 1 || inToC[0].Type != "implemented_by" {
		t.Errorf("unexpected incoming to c: %+v", inToC)
	}
}

func TestNeighbors(t *testing.T) {
	dir := t.TempDir()
	a := writeDoc(t, dir, "a.md", "type: A\n", "# A\n[b](b.md)\n")
	b := writeDoc(t, dir, "b.md", "type: B\n", "# B\n[a](a.md)\n")
	docs := []parser.Doc{parseFixture(t, dir, a), parseFixture(t, dir, b)}
	g := Build(docs, dir, profile.Default())

	out, in := g.Neighbors("a.md")
	if len(out) != 1 || len(in) != 1 {
		t.Fatalf("expected 1 out and 1 in, got %d/%d", len(out), len(in))
	}
}

func TestTrace(t *testing.T) {
	dir := t.TempDir()
	a := writeDoc(t, dir, "a.md", "type: A\n", "# A\n[b](b.md)\n")
	b := writeDoc(t, dir, "b.md", "type: B\n", "# B\n[c](c.md)\n")
	c := writeDoc(t, dir, "c.md", "type: C\n", "# C\n")
	docs := []parser.Doc{parseFixture(t, dir, a), parseFixture(t, dir, b), parseFixture(t, dir, c)}
	g := Build(docs, dir, profile.Default())

	steps := g.Trace("a.md", "downstream", "", 2)
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if steps[0].Node != "b.md" || steps[0].Depth != 1 {
		t.Errorf("first step: %+v", steps[0])
	}
	if steps[1].Node != "c.md" || steps[1].Depth != 2 {
		t.Errorf("second step: %+v", steps[1])
	}
	if len(steps[1].Path) != 2 {
		t.Errorf("expected path length 2, got %d", len(steps[1].Path))
	}

	// Cycle termination.
	writeDoc(t, dir, "c.md", "type: C\n", "# C\n[a](a.md)\n") // c -> a cycle
	docs2 := []parser.Doc{parseFixture(t, dir, a), parseFixture(t, dir, b), parseFixture(t, dir, "c.md")}
	g2 := Build(docs2, dir, profile.Default())
	steps2 := g2.Trace("a.md", "downstream", "", 10)
	if len(steps2) != 2 {
		t.Fatalf("cycle should terminate with 2 steps, got %d", len(steps2))
	}
}

func TestIntegrity(t *testing.T) {
	dir := t.TempDir()
	prof := loadProfile(t, dir, `version: "1.0"
relationships:
  - name: "implements"
    heading_aliases: ["Implements"]
    inverse: "implemented_by"
    allowed_source_types: ["Implementation"]
    allowed_target_types: ["Requirement"]
  - name: "supersedes"
    heading_aliases: ["Supersedes"]
    inverse: "superseded_by"
invariants:
  - id: "P1"
    source_type: "Requirement"
    relationship: "implements"
    direction: "incoming"
    min: 1
    max: -1
    severity: "warning"
  - id: "P2"
    source_type: "*"
    relationship: "supersedes"
    direction: "outgoing"
    min: 0
    max: 0
    severity: "error"
`)
	req := writeDoc(t, dir, "req.md", "type: Requirement\n", "# Req\n")
	impl := writeDoc(t, dir, "impl.md", "type: Implementation\n", "# Impl\n## Implements\n[req](req.md)\n")
	old := writeDoc(t, dir, "old.md", "type: Requirement\n", "# Old\n")
	bad := writeDoc(t, dir, "bad.md", "type: Requirement\n", "# Bad\n## Implements\n[impl](impl.md)\n") // wrong direction
	dangling := writeDoc(t, dir, "dangling.md", "type: Requirement\n", "# Dangling\n## Implements\n[missing](missing.md)\n")
	orphan := writeDoc(t, dir, "orphan.md", "type: Requirement\n", "# Orphan\n")
	super := writeDoc(t, dir, "super.md", "type: Requirement\n", "# Super\n## Supersedes\n[old](old.md)\n")
	depends := writeDoc(t, dir, "depends.md", "type: Requirement\n", "# Depends\n## Implements\n[req](req.md)\n## Depends on\n[old](old.md)\n") // old is superseded

	docs := []parser.Doc{
		parseFixture(t, dir, req),
		parseFixture(t, dir, impl),
		parseFixture(t, dir, old),
		parseFixture(t, dir, bad),
		parseFixture(t, dir, dangling),
		parseFixture(t, dir, orphan),
		parseFixture(t, dir, super),
		parseFixture(t, dir, depends),
	}
	g := Build(docs, dir, prof)
	res := g.Integrity()

	if !res.ProfileLoaded {
		t.Error("expected ProfileLoaded true")
	}

	// Helper to locate at least one finding matching check and source.
	find := func(check, source string) (IntegrityFinding, bool) {
		for _, f := range res.Findings {
			if f.Check == check && f.Source == source {
				return f, true
			}
		}
		return IntegrityFinding{}, false
	}

	if _, ok := find("dangling", "dangling.md"); !ok {
		t.Errorf("expected dangling finding for dangling.md, got %+v", res.Findings)
	}
	if _, ok := find("orphans", "orphan.md"); !ok {
		t.Errorf("expected orphan finding for orphan.md, got %+v", res.Findings)
	}
	if _, ok := find("profile_violations", "bad.md"); !ok {
		t.Errorf("expected profile violation for bad.md, got %+v", res.Findings)
	}
	if _, ok := find("superseded_deps", "depends.md"); !ok {
		t.Errorf("expected superseded dependency finding for depends.md, got %+v", res.Findings)
	}

	// Cardinality: at minimum super.md violates P2 (outgoing supersedes forbidden).
	if _, ok := find("cardinality_violation", "super.md"); !ok {
		t.Errorf("expected cardinality violation for super.md, got %+v", res.Findings)
	}
}

func TestIntegrity_NoProfile(t *testing.T) {
	dir := t.TempDir()
	a := writeDoc(t, dir, "a.md", "type: A\n", "# A\n[b](b.md)\n")
	b := writeDoc(t, dir, "b.md", "type: B\n", "# B\n")
	docs := []parser.Doc{parseFixture(t, dir, a), parseFixture(t, dir, b)}
	g := Build(docs, dir, profile.Default())
	res := g.Integrity()
	if res.ProfileLoaded {
		t.Error("expected ProfileLoaded false with default profile")
	}
	for _, f := range res.Findings {
		if f.Check == "profile_violations" || f.Check == "cardinality_violation" {
			t.Errorf("unexpected profile finding with default profile: %+v", f)
		}
	}
}

func TestIntegrity_SupersededRenamedRelationship(t *testing.T) {
	dir := t.TempDir()
	prof := loadProfile(t, dir, `version: "1.0"
relationships:
  - name: "replaces"
    heading_aliases: ["Replaces"]
    inverse: "replaced_by"
invariants:
  - id: "P1"
    source_type: "*"
    relationship: "replaces"
    direction: "outgoing"
    min: 0
    max: 0
    severity: "error"
`)
	old := writeDoc(t, dir, "old.md", "type: Requirement\n", "# Old\n")
	replacement := writeDoc(t, dir, "replacement.md", "type: Requirement\n", "# Replacement\n## Replaces\n[old](old.md)\n")
	depends := writeDoc(t, dir, "depends.md", "type: Requirement\n", "# Depends\n## Depends on\n[old](old.md)\n")
	docs := []parser.Doc{
		parseFixture(t, dir, old),
		parseFixture(t, dir, replacement),
		parseFixture(t, dir, depends),
	}

	g := Build(docs, dir, prof)
	res := g.Integrity()

	found := false
	for _, f := range res.Findings {
		if f.Check == "superseded_deps" && f.Source == "depends.md" && f.Target == "old.md" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected superseded_deps finding for depends.md -> old.md, got %+v", res.Findings)
	}
}

func TestIntegrity_NoSupersededInvariantSkipped(t *testing.T) {
	dir := t.TempDir()
	// Profile defines a replaces relationship but no outgoing-max-0 invariant,
	// so the superseded-deps check must be skipped.
	prof := loadProfile(t, dir, `version: "1.0"
relationships:
  - name: "replaces"
    heading_aliases: ["Replaces"]
    inverse: "replaced_by"
`)
	old := writeDoc(t, dir, "old.md", "type: Requirement\n", "# Old\n")
	replacement := writeDoc(t, dir, "replacement.md", "type: Requirement\n", "# Replacement\n## Replaces\n[old](old.md)\n")
	depends := writeDoc(t, dir, "depends.md", "type: Requirement\n", "# Depends\n## Depends on\n[old](old.md)\n")
	docs := []parser.Doc{
		parseFixture(t, dir, old),
		parseFixture(t, dir, replacement),
		parseFixture(t, dir, depends),
	}

	g := Build(docs, dir, prof)
	res := g.Integrity()

	for _, f := range res.Findings {
		if f.Check == "superseded_deps" {
			t.Errorf("expected no superseded_deps finding when no invariant defines supersession, got %+v", f)
		}
	}
}

func TestCoverage(t *testing.T) {
	dir := t.TempDir()
	prof := loadProfile(t, dir, `version: "1.0"
relationships:
  - name: "implements"
    heading_aliases: ["Implements"]
    inverse: "implemented_by"
`)
	req1 := writeDoc(t, dir, "req1.md", "type: Requirement\n", "# R1\n")
	req2 := writeDoc(t, dir, "req2.md", "type: Requirement\n", "# R2\n")
	impl1 := writeDoc(t, dir, "impl1.md", "type: Implementation\n", "# I1\n## Implements\n[req1](req1.md)\n")
	impl2 := writeDoc(t, dir, "impl2.md", "type: Implementation\n", "# I2\n## Implements\n[req2](req2.md)\n")

	docs := []parser.Doc{parseFixture(t, dir, req1), parseFixture(t, dir, req2), parseFixture(t, dir, impl1), parseFixture(t, dir, impl2)}
	g := Build(docs, dir, prof)

	// From Requirement, can we reach Implementation via implements?
	// Edge is impl -> req, so coverage must follow inverse from req.
	res := g.Coverage("Requirement", "Implementation", "implements")
	if res.TotalSources != 2 || res.Covered != 2 || res.Uncovered != 0 {
		t.Errorf("coverage = %+v", res)
	}

	// No implementation for req2.
	writeDoc(t, dir, "impl2.md", "type: Implementation\n", "# I2\nNo links.\n")
	docs2 := []parser.Doc{parseFixture(t, dir, req1), parseFixture(t, dir, req2), parseFixture(t, dir, impl1), parseFixture(t, dir, "impl2.md")}
	g2 := Build(docs2, dir, prof)
	res2 := g2.Coverage("Requirement", "Implementation", "implements")
	if res2.Covered != 1 || res2.Uncovered != 1 {
		t.Errorf("coverage2 = %+v", res2)
	}
}

func TestContext(t *testing.T) {
	dir := t.TempDir()
	center := writeDoc(t, dir, "center.md", "type: C\n", "# Center\n")
	up1 := writeDoc(t, dir, "up1.md", "type: U\n", "# U1\n[center](center.md)\n")
	up2 := writeDoc(t, dir, "up2.md", "type: U\n", "# U2\n[center](center.md)\n")
	down1 := writeDoc(t, dir, "down1.md", "type: D\n", "# D1\n")
	down2 := writeDoc(t, dir, "down2.md", "type: D\n", "# D2\n")
	centerBody := "# Center\n[down1](down1.md)\n[down2](down2.md)\n"
	writeDoc(t, dir, "center.md", "type: C\n", centerBody)

	docs := []parser.Doc{
		parseFixture(t, dir, center),
		parseFixture(t, dir, up1),
		parseFixture(t, dir, up2),
		parseFixture(t, dir, down1),
		parseFixture(t, dir, down2),
	}
	g := Build(docs, dir, profile.Default())

	ctx := g.Context("center.md", 1, 100)
	if ctx.Center == nil || ctx.Center.FilePath != "center.md" {
		t.Fatalf("unexpected center: %+v", ctx.Center)
	}
	if len(ctx.Neighbors.Upstream) != 2 || len(ctx.Neighbors.Downstream) != 2 {
		t.Errorf("upstream=%d downstream=%d", len(ctx.Neighbors.Upstream), len(ctx.Neighbors.Downstream))
	}

	// Upstream first, sorted by path.
	paths := []string{}
	for _, n := range ctx.Neighbors.Upstream {
		paths = append(paths, n.FilePath)
	}
	if !sort.StringsAreSorted(paths) {
		t.Errorf("upstream not sorted: %v", paths)
	}

	// Truncation.
	ctx2 := g.Context("center.md", 1, 2)
	if !ctx2.Truncated || ctx2.TotalNeighbors != 2 {
		t.Errorf("truncated=%v total=%d", ctx2.Truncated, ctx2.TotalNeighbors)
	}

	// Depth 2.
	writeDoc(t, dir, "down1.md", "type: D\n", "# D1\n[leaf](leaf.md)\n")
	leaf := writeDoc(t, dir, "leaf.md", "type: L\n", "# Leaf\n")
	docs2 := []parser.Doc{
		parseFixture(t, dir, center),
		parseFixture(t, dir, up1),
		parseFixture(t, dir, up2),
		parseFixture(t, dir, "down1.md"),
		parseFixture(t, dir, down2),
		parseFixture(t, dir, leaf),
	}
	g2 := Build(docs2, dir, profile.Default())
	ctx3 := g2.Context("center.md", 2, 100)
	foundLeaf := false
	for _, n := range ctx3.Neighbors.Downstream {
		if n.FilePath == "leaf.md" {
			foundLeaf = true
			break
		}
	}
	if !foundLeaf {
		t.Error("depth-2 neighbor leaf.md not found")
	}
}

func TestContext_BudgetDepthFirst(t *testing.T) {
	dir := t.TempDir()
	center := writeDoc(t, dir, "center.md", "type: C\n", "# Center\n[down1](down1.md)\n")
	down1 := writeDoc(t, dir, "down1.md", "type: D\n", "# D1\n[down2](down2.md)\n")
	down2 := writeDoc(t, dir, "down2.md", "type: D\n", "# D2\n")
	docs := []parser.Doc{parseFixture(t, dir, center), parseFixture(t, dir, down1), parseFixture(t, dir, down2)}
	g := Build(docs, dir, profile.Default())

	// Budget 1: depth-1 neighbor down1 should be included, depth-2 down2 truncated.
	ctx := g.Context("center.md", 2, 1)
	if !ctx.Truncated {
		t.Error("expected truncated")
	}
	if len(ctx.Neighbors.Downstream) != 1 || ctx.Neighbors.Downstream[0].FilePath != "down1.md" {
		t.Errorf("expected only down1.md, got %+v", ctx.Neighbors.Downstream)
	}
}
