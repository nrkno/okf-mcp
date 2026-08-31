// Package graph builds and queries a directed graph projection over OKF
// documents. The graph is derived from Markdown links, profile-defined
// relationship types, and inverse edges. It is read-only: no mutation API is
// exported.
package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nrkno/plattform-okf-mcp/internal/linkextract"
	"github.com/nrkno/plattform-okf-mcp/internal/matcher"
	"github.com/nrkno/plattform-okf-mcp/internal/parser"
	"github.com/nrkno/plattform-okf-mcp/internal/profile"
)

// Node is an indexed OKF document in the graph.
type Node struct {
	FilePath    string   // relative path from scan root (I-1, I-27)
	Type        string   // from frontmatter
	Title       string   // from frontmatter
	Description string   // from frontmatter
	Tags        []string // from frontmatter
	Bundle      string   // from index bundle resolution (I-17)
}

// Edge is a directed relationship between two nodes.
type Edge struct {
	Source    string // relative file path of source document
	Target    string // relative file path of target document
	Type      string // relationship type (from profile classification or "untyped")
	Heading   string // section heading the link appeared under
	InverseOf string // non-empty only for derived inverse edges
	Line      int    // line number in source document body
}

// DanglingRef records a link whose target did not resolve to an indexed doc.
type DanglingRef struct {
	Source  string // relative file path of document containing the link
	Target  string // raw link target string (unresolved)
	Heading string // section heading context
	Line    int    // line number in source document body
}

// Graph is the derived read-only projection.
type Graph struct {
	Nodes          map[string]*Node
	OutEdges       map[string][]Edge
	InEdges        map[string][]Edge
	Dangling       []DanglingRef
	Profile        *profile.Profile
	supersededRels map[string]struct{} // relationship names whose inverse incoming edges mark a node as superseded
}

// SearchResult is a scored concept from Graph.Search.
type SearchResult struct {
	Node  Node
	Score float64
}

// TraceStep is one reachable node from Graph.Trace.
type TraceStep struct {
	Node    string
	Depth   int
	ViaType string
	Path    []string
}

// IntegrityFinding is one structural problem reported by Graph.Integrity.
type IntegrityFinding struct {
	Check    string
	Severity string
	Source   string
	Target   string
	Heading  string
	Message  string
}

// IntegritySummary aggregates the counts from Integrity.
type IntegritySummary struct {
	DanglingRefs      int
	OrphanConcepts    int
	ProfileViolations int
	SupersededDeps    int
	TotalFindings     int
}

// IntegrityResult is the full output of Graph.Integrity.
type IntegrityResult struct {
	Summary       IntegritySummary
	Findings      []IntegrityFinding
	ProfileLoaded bool
}

// CoverageEntry is the per-source result from Graph.Coverage.
type CoverageEntry struct {
	Source         string
	Covered        bool
	NearestTargets []string
}

// CoverageResult is the full output of Graph.Coverage.
type CoverageResult struct {
	SourceType     string
	TargetType     string
	Relationship   string
	TotalSources   int
	Covered        int
	Uncovered      int
	CoverageRatio  float64
	UncoveredItems []UncoveredItem
}

// UncoveredItem describes one source that failed coverage.
type UncoveredItem struct {
	FilePath       string
	Title          string
	NearestTargets []string
}

// ContextNeighbor is one neighbor in a ContextSlice.
type ContextNeighbor struct {
	FilePath string
	Type     string
	Via      string
}

// ContextNeighbors groups upstream and downstream neighbors.
type ContextNeighbors struct {
	Upstream   []ContextNeighbor
	Downstream []ContextNeighbor
}

// ContextSlice is the full output of Graph.Context.
type ContextSlice struct {
	Center         *Node
	Neighbors      ContextNeighbors
	Depth          int
	TotalNeighbors int
	MaxResults     int
	Truncated      bool
}

// Build derives a Graph from docs using dir as the scan root and prof as the
// relationship profile. It reads each document body from disk (I-2), strips
// frontmatter, extracts links, classifies them, resolves targets, and derives
// inverse edges.
func Build(docs []parser.Doc, dir string, prof *profile.Profile) *Graph {
	if prof == nil {
		prof = profile.Default()
	}

	g := &Graph{
		Nodes:          make(map[string]*Node, len(docs)),
		OutEdges:       make(map[string][]Edge),
		InEdges:        make(map[string][]Edge),
		Dangling:       []DanglingRef{},
		Profile:        prof,
		supersededRels: buildSupersededRels(prof),
	}

	// Sort docs by path so output order is independent of input order (I-21).
	sorted := make([]parser.Doc, len(docs))
	copy(sorted, docs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].FilePath < sorted[j].FilePath
	})

	for i := range sorted {
		doc := &sorted[i]
		g.Nodes[doc.FilePath] = &Node{
			FilePath:    doc.FilePath,
			Type:        doc.Type,
			Title:       doc.Title,
			Description: doc.Description,
			Tags:        copyStrings(doc.Tags),
			Bundle:      doc.Bundle,
		}
	}

	for i := range sorted {
		doc := sorted[i]
		absPath := filepath.Join(dir, doc.FilePath)
		body, ok := readBody(absPath, doc.BodyOffset)
		if !ok {
			// Body read failed; the node exists but has no extractable links.
			continue
		}

		docDir := filepath.Dir(absPath)
		links := linkextract.Extract(body, docDir, dir)

		for _, link := range links {
			relType, _ := prof.ClassifyHeading(link.Heading)
			if relType == "" {
				relType = "untyped"
			}

			if targetNode, exists := g.Nodes[link.Target]; exists {
				out := Edge{
					Source:  doc.FilePath,
					Target:  targetNode.FilePath,
					Type:    relType,
					Heading: link.Heading,
					Line:    link.Line,
				}
			in := Edge{
				Source:    doc.FilePath,
				Target:    targetNode.FilePath,
				Type:      prof.InverseName(relType),
				Heading:   link.Heading,
				InverseOf: relType,
				Line:      link.Line,
			}
			// Out edge is stored keyed by its actual source; in edge by actual target.
			g.OutEdges[doc.FilePath] = append(g.OutEdges[doc.FilePath], out)
			g.InEdges[targetNode.FilePath] = append(g.InEdges[targetNode.FilePath], in)
			} else {
				g.Dangling = append(g.Dangling, DanglingRef{
					Source:  doc.FilePath,
					Target:  link.RawTarget,
					Heading: link.Heading,
					Line:    link.Line,
				})
			}
		}
	}

	return g
}

// Concept returns the node for filePath, if present.
func (g *Graph) Concept(filePath string) (*Node, bool) {
	if g == nil {
		return nil, false
	}
	n, ok := g.Nodes[filePath]
	if !ok {
		return nil, false
	}
	// Return a copy so callers cannot mutate the graph's node.
	cp := *n
	cp.Tags = copyStrings(n.Tags)
	return &cp, true
}

// Search delegates text scoring to matcher.Score and returns results sorted by
// score descending, then file_path ascending (I-6).
func (g *Graph) Search(query string, typeFilter string, tagFilter []string) []SearchResult {
	if g == nil {
		return nil
	}

	paths := make([]string, 0, len(g.Nodes))
	for p := range g.Nodes {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	results := make([]SearchResult, 0, len(paths))
	for _, p := range paths {
		n := g.Nodes[p]
		if typeFilter != "" && n.Type != typeFilter {
			continue
		}
		doc := parser.Doc{
			Title:       n.Title,
			Description: n.Description,
			Type:        n.Type,
			Tags:        n.Tags,
			FilePath:    n.FilePath,
		}
		score := matcher.Score(query, tagFilter, "or", doc)
		if score < 0 {
			continue
		}
		cp := *n
		cp.Tags = copyStrings(n.Tags)
		results = append(results, SearchResult{Node: cp, Score: score})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Node.FilePath < results[j].Node.FilePath
	})
	return results
}

// Outgoing returns outgoing edges for filePath, optionally filtered by type.
func (g *Graph) Outgoing(filePath string, typeFilter string) []Edge {
	if g == nil {
		return nil
	}
	return filterEdges(g.OutEdges[filePath], typeFilter)
}

// Incoming returns incoming edges for filePath, optionally filtered by type.
func (g *Graph) Incoming(filePath string, typeFilter string) []Edge {
	if g == nil {
		return nil
	}
	return filterEdges(g.InEdges[filePath], typeFilter)
}

// Neighbors returns outgoing and incoming edges for filePath.
func (g *Graph) Neighbors(filePath string) ([]Edge, []Edge) {
	if g == nil {
		return nil, nil
	}
	return copyEdges(g.OutEdges[filePath]), copyEdges(g.InEdges[filePath])
}

// Trace performs a BFS from filePath following direction ("upstream" or
// "downstream"), optionally filtering edges by typeFilter. maxDepth is clamped
// to the range [1, 20]. Cycles are terminated via a visited set.
func (g *Graph) Trace(filePath string, direction string, typeFilter string, maxDepth int) []TraceStep {
	if g == nil {
		return nil
	}
	if maxDepth < 1 {
		maxDepth = 1
	}
	if maxDepth > 20 {
		maxDepth = 20
	}

	type queueItem struct {
		node  string
		depth int
		path  []string
	}

	visited := map[string]struct{}{filePath: {}}
	queue := []queueItem{{node: filePath, depth: 0, path: []string{}}}
	var results []TraceStep

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if item.depth >= maxDepth {
			continue
		}

		var edges []Edge
		switch direction {
		case "upstream":
			edges = g.InEdges[item.node]
		default: // downstream
			edges = g.OutEdges[item.node]
		}

		for _, edge := range edges {
			if typeFilter != "" && edge.Type != typeFilter {
				continue
			}
			next := edge.Target
			if direction == "upstream" {
				next = edge.Source
			}
			if _, seen := visited[next]; seen {
				continue
			}
			visited[next] = struct{}{}

			newPath := append([]string{}, item.path...)
			newPath = append(newPath, fmt.Sprintf("%s → %s", edge.Source, edge.Target))
			results = append(results, TraceStep{
				Node:    next,
				Depth:   item.depth + 1,
				ViaType: edge.Type,
				Path:    newPath,
			})
			queue = append(queue, queueItem{node: next, depth: item.depth + 1, path: newPath})
		}
	}

	return results
}

// Integrity reports structural problems in the graph.
func (g *Graph) Integrity() IntegrityResult {
	if g == nil {
		return IntegrityResult{Summary: IntegritySummary{}, ProfileLoaded: false}
	}

	res := IntegrityResult{
		Findings:      []IntegrityFinding{},
		ProfileLoaded: g.Profile != nil && g.Profile != profile.Default() && (len(g.Profile.Relationships) > 0 || len(g.Profile.Invariants) > 0),
	}

	// Dangling references.
	for _, d := range g.Dangling {
		res.Findings = append(res.Findings, IntegrityFinding{
			Check:    "dangling",
			Severity: "warning",
			Source:   d.Source,
			Target:   d.Target,
			Heading:  d.Heading,
			Message:  "Link target does not resolve to an indexed document",
		})
	}

	paths := g.sortedNodePaths()

	// Orphan concepts and superseded dependencies.
	for _, p := range paths {
		out := g.OutEdges[p]
		in := g.InEdges[p]
		if len(out) == 0 && len(in) == 0 {
			res.Findings = append(res.Findings, IntegrityFinding{
				Check:    "orphans",
				Severity: "notification",
				Source:   p,
				Message:  "Concept has no incoming or outgoing relationships",
			})
			continue
		}

		for _, edge := range out {
			if g.isSuperseded(edge.Target) {
				res.Findings = append(res.Findings, IntegrityFinding{
					Check:    "superseded_deps",
					Severity: "error",
					Source:   edge.Source,
					Target:   edge.Target,
					Heading:  edge.Heading,
					Message:  "Target has been superseded by another concept",
				})
			}
		}
	}

	// Profile-dependent checks.
	if g.Profile != nil {
		res.Findings = append(res.Findings, g.profileFindings(paths)...)
	}

	res.Summary = summarizeFindings(res.Findings)
	return res
}

// Coverage performs a bidirectional BFS from every sourceType node, following
// relType edges and their inverses, and reports how many reach a targetType
// node.
func (g *Graph) Coverage(sourceType string, targetType string, relType string) CoverageResult {
	if g == nil {
		return CoverageResult{SourceType: sourceType, TargetType: targetType, Relationship: relType}
	}

	sources := g.sortedNodePaths()
	var entries []CoverageEntry
	for _, p := range sources {
		if g.Nodes[p].Type != sourceType {
			continue
		}
		nearest := g.coverageBFS(p, targetType, relType)
		entries = append(entries, CoverageEntry{
			Source:         p,
			Covered:        len(nearest) > 0,
			NearestTargets: nearest,
		})
	}

	res := CoverageResult{
		SourceType:   sourceType,
		TargetType:   targetType,
		Relationship: relType,
		TotalSources: len(entries),
	}
	for _, e := range entries {
		if e.Covered {
			res.Covered++
		} else {
			res.Uncovered++
			res.UncoveredItems = append(res.UncoveredItems, UncoveredItem{
				FilePath:       e.Source,
				Title:          g.Nodes[e.Source].Title,
				NearestTargets: e.NearestTargets,
			})
		}
	}
	if res.TotalSources > 0 {
		res.CoverageRatio = float64(res.Covered) / float64(res.TotalSources)
	}
	return res
}

// Context returns a bounded neighborhood around filePath.
func (g *Graph) Context(filePath string, depth int, maxResults int) ContextSlice {
	if g == nil {
		return ContextSlice{Depth: depth, MaxResults: maxResults}
	}

	if depth < 1 {
		depth = 1
	}
	if maxResults < 1 {
		maxResults = 1
	}

	center, ok := g.Nodes[filePath]
	if !ok {
		return ContextSlice{Depth: depth, MaxResults: maxResults}
	}

	visited := map[string]struct{}{filePath: {}}
	ctx := ContextSlice{
		Center:     center,
		Depth:      depth,
		MaxResults: maxResults,
	}

	// Helper that adds neighbors from a set of edges up to the budget.
	addNeighbors := func(edges []Edge, isUpstream bool, budget int) (added []ContextNeighbor, remaining int, stopped bool) {
		remaining = budget
		// Sort by neighbor path for determinism.
		sorted := make([]Edge, len(edges))
		copy(sorted, edges)
		sort.Slice(sorted, func(i, j int) bool {
			if isUpstream {
				return sorted[i].Source < sorted[j].Source
			}
			return sorted[i].Target < sorted[j].Target
		})

		for _, edge := range sorted {
			nbrPath := edge.Target
			if isUpstream {
				nbrPath = edge.Source
			}
			if _, seen := visited[nbrPath]; seen {
				continue
			}
			if remaining <= 0 {
				return added, 0, true
			}
			visited[nbrPath] = struct{}{}
			nbr := g.Nodes[nbrPath]
			added = append(added, ContextNeighbor{
				FilePath: nbrPath,
				Type:     nbr.Type,
				Via:      edge.Type,
			})
			remaining--
		}
		return added, remaining, false
	}

	// Depth 1.
	upstream1, rem, truncated := addNeighbors(g.InEdges[filePath], true, maxResults)
	ctx.Neighbors.Upstream = append(ctx.Neighbors.Upstream, upstream1...)

	downstream1, rem, truncatedDown := addNeighbors(g.OutEdges[filePath], false, rem)
	ctx.Neighbors.Downstream = append(ctx.Neighbors.Downstream, downstream1...)
	truncated = truncated || truncatedDown

	if depth >= 2 {
		// Expand depth-1 neighbors in the order they were returned.
		for _, nbr := range ctx.Neighbors.Upstream {
			if rem <= 0 {
				truncated = true
				break
			}
			up2, r, t := addNeighbors(g.InEdges[nbr.FilePath], true, rem)
			ctx.Neighbors.Upstream = append(ctx.Neighbors.Upstream, up2...)
			rem = r
			truncated = truncated || t

			if rem <= 0 {
				truncated = true
				break
			}
			down2, r, t := addNeighbors(g.OutEdges[nbr.FilePath], false, rem)
			ctx.Neighbors.Downstream = append(ctx.Neighbors.Downstream, down2...)
			rem = r
			truncated = truncated || t
		}
		for _, nbr := range ctx.Neighbors.Downstream {
			if rem <= 0 {
				truncated = true
				break
			}
			up2, r, t := addNeighbors(g.InEdges[nbr.FilePath], true, rem)
			ctx.Neighbors.Upstream = append(ctx.Neighbors.Upstream, up2...)
			rem = r
			truncated = truncated || t

			if rem <= 0 {
				truncated = true
				break
			}
			down2, r, t := addNeighbors(g.OutEdges[nbr.FilePath], false, rem)
			ctx.Neighbors.Downstream = append(ctx.Neighbors.Downstream, down2...)
			rem = r
			truncated = truncated || t
		}
	}

	ctx.Truncated = truncated
	ctx.TotalNeighbors = len(ctx.Neighbors.Upstream) + len(ctx.Neighbors.Downstream)
	return ctx
}

// profileFindings evaluates profile type violations and cardinality invariants.
func (g *Graph) profileFindings(paths []string) []IntegrityFinding {
	var findings []IntegrityFinding

	for _, p := range paths {
		sourceType := g.Nodes[p].Type
		for _, edge := range g.OutEdges[p] {
			if edge.Type == "untyped" {
				continue
			}
			if !g.Profile.IsAllowedSource(edge.Type, sourceType) {
				findings = append(findings, IntegrityFinding{
					Check:    "profile_violations",
					Severity: "warning",
					Source:   edge.Source,
					Target:   edge.Target,
					Heading:  edge.Heading,
					Message:  fmt.Sprintf("Source type %q is not allowed for relationship %q", sourceType, edge.Type),
				})
			}
			if targetNode, ok := g.Nodes[edge.Target]; ok {
				if !g.Profile.IsAllowedTarget(edge.Type, targetNode.Type) {
					findings = append(findings, IntegrityFinding{
						Check:    "profile_violations",
						Severity: "warning",
						Source:   edge.Source,
						Target:   edge.Target,
						Heading:  edge.Heading,
						Message:  fmt.Sprintf("Target type %q is not allowed for relationship %q", targetNode.Type, edge.Type),
					})
				}
			}
		}
	}

	for _, inv := range g.Profile.Invariants {
		for _, p := range paths {
			nodeType := g.Nodes[p].Type
			if inv.SourceType != "*" && inv.SourceType != nodeType {
				continue
			}
			count := g.countEdges(p, inv.Relationship, inv.Direction)
			if count >= inv.Min && (inv.Max == -1 || count <= inv.Max) {
				continue
			}
			var msg string
			switch {
			case inv.Min > 0 && count < inv.Min:
				msg = fmt.Sprintf("%s: expected at least %d %s %q edges, found %d", inv.ID, inv.Min, inv.Direction, inv.Relationship, count)
			case inv.Max >= 0 && count > inv.Max:
				msg = fmt.Sprintf("%s: expected at most %d %s %q edges, found %d", inv.ID, inv.Max, inv.Direction, inv.Relationship, count)
			default:
				msg = fmt.Sprintf("%s: cardinality violation for %s %q edges (count=%d)", inv.ID, inv.Direction, inv.Relationship, count)
			}
			findings = append(findings, IntegrityFinding{
				Check:    "cardinality_violation",
				Severity: inv.Severity,
				Source:   p,
				Message:  msg,
			})
		}
	}

	return findings
}

// countEdges returns the number of edges of the given relationship and direction
// incident to filePath.
func (g *Graph) countEdges(filePath string, relationship string, direction string) int {
	var edges []Edge
	switch direction {
	case "incoming":
		edges = g.InEdges[filePath]
	default:
		edges = g.OutEdges[filePath]
	}
	count := 0
	for _, edge := range edges {
		switch direction {
		case "incoming":
			if edge.InverseOf == relationship {
				count++
			}
		default:
			if edge.Type == relationship {
				count++
			}
		}
	}
	return count
}

// buildSupersededRels returns the set of relationship names that mark a node as
// superseded when they appear as an incoming inverse edge. Supersession
// relationships are identified from profile invariants whose relationship name
// contains "supersede" (case-insensitive) and whose direction is outgoing.
// This captures the canonical "supersedes" relationship as well as variants
// such as "superseded_by", regardless of the invariant's cardinality value.
func buildSupersededRels(prof *profile.Profile) map[string]struct{} {
	rels := make(map[string]struct{})
	if prof == nil {
		return rels
	}
	for _, inv := range prof.Invariants {
		if inv.Direction == "outgoing" &&
			inv.Relationship != "" &&
			strings.Contains(strings.ToLower(inv.Relationship), "supersede") {
			rels[inv.Relationship] = struct{}{}
		}
	}
	return rels
}

// isSuperseded reports whether filePath has an incoming inverse edge from a
// relationship that the profile identifies as supersession. When no profile
// invariant defines such a relationship, the check is skipped and returns
// false to avoid false negatives.
func (g *Graph) isSuperseded(filePath string) bool {
	if len(g.supersededRels) == 0 {
		return false
	}
	for _, edge := range g.InEdges[filePath] {
		if _, ok := g.supersededRels[edge.InverseOf]; ok {
			return true
		}
	}
	return false
}

// coverageBFS returns the nearest targetType nodes reachable from source by
// following relType edges in either direction.
func (g *Graph) coverageBFS(source string, targetType string, relType string) []string {
	visited := map[string]struct{}{source: {}}
	queue := []string{source}
	var nearest []string

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, edge := range g.OutEdges[current] {
			if relType != "" && edge.Type != relType {
				continue
			}
			next := edge.Target
			if _, seen := visited[next]; seen {
				continue
			}
			visited[next] = struct{}{}
			queue = append(queue, next)
			if g.Nodes[next].Type == targetType {
				nearest = append(nearest, next)
			}
		}

		for _, edge := range g.InEdges[current] {
			if relType != "" && edge.InverseOf != relType {
				continue
			}
			next := edge.Source
			if _, seen := visited[next]; seen {
				continue
			}
			visited[next] = struct{}{}
			queue = append(queue, next)
			if g.Nodes[next].Type == targetType {
				nearest = append(nearest, next)
			}
		}
	}

	return nearest
}

// sortedNodePaths returns all node paths sorted ascending.
func (g *Graph) sortedNodePaths() []string {
	paths := make([]string, 0, len(g.Nodes))
	for p := range g.Nodes {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// summarizeFindings aggregates finding counts by check.
func summarizeFindings(findings []IntegrityFinding) IntegritySummary {
	s := IntegritySummary{TotalFindings: len(findings)}
	for _, f := range findings {
		switch f.Check {
		case "dangling":
			s.DanglingRefs++
		case "orphans":
			s.OrphanConcepts++
		case "profile_violations", "cardinality_violation":
			s.ProfileViolations++
		case "superseded_deps":
			s.SupersededDeps++
		}
	}
	return s
}

// readBody reads absPath and returns the substring from offset onward.
func readBody(absPath string, offset int) (string, bool) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "okf-mcp: WARN: could not read body %s: %v\n", absPath, err)
		return "", false
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(data) {
		offset = len(data)
	}
	return string(data[offset:]), true
}

// filterEdges returns a copy of edges filtered by type.
func filterEdges(edges []Edge, typeFilter string) []Edge {
	if typeFilter == "" {
		return copyEdges(edges)
	}
	out := make([]Edge, 0, len(edges))
	for _, e := range edges {
		if e.Type == typeFilter {
			out = append(out, e)
		}
	}
	return out
}

// copyEdges returns a shallow copy of edges.
func copyEdges(edges []Edge) []Edge {
	if edges == nil {
		return nil
	}
	out := make([]Edge, len(edges))
	copy(out, edges)
	return out
}

// copyStrings returns a copy of ss.
func copyStrings(ss []string) []string {
	if ss == nil {
		return nil
	}
	out := make([]string, len(ss))
	copy(out, ss)
	return out
}
