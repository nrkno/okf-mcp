package main

import (
	"testing"

	"github.com/nrkno/plattform-okf-mcp/internal/scanner"
)

func TestGraphContext_MixedDirectionDepthBound(t *testing.T) {
	for _, mirrored := range []bool{false, true} {
		name := "incoming then outgoing"
		if mirrored {
			name = "outgoing then incoming"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeProfile(t, dir, graphProfileYAML)
			bodies := map[string]string{
				"center.md": "# Center\n",
				"near.md":   "# Near\n\n## Depends on\n\n[Center](center.md)\n[Side](side.md)\n",
				"side.md":   "# Side\n\n## Depends on\n\n[Far](far.md)\n",
				"far.md":    "# Far\n",
			}
			if mirrored {
				bodies["center.md"] = "# Center\n\n## Depends on\n\n[Near](near.md)\n"
				bodies["near.md"] = "# Near\n"
				bodies["side.md"] = "# Side\n\n## Depends on\n\n[Near](near.md)\n"
				bodies["far.md"] = "# Far\n\n## Depends on\n\n[Side](side.md)\n"
			}
			for path, body := range bodies {
				writeDoc(t, dir, path, "Architecture", path, "Depth fixture", []string{"context"}, body)
			}
			srv := newFixtureServer(t, dir, scanner.ScanOptions{})
			defer srv.Close()
			for _, tc := range []struct {
				name      string
				depth     float64
				budget    float64
				wantCount int
				truncated bool
			}{
				{"one hop", 1, 10, 1, false},
				{"two hops", 2, 10, 2, false},
				{"budget", 2, 1, 1, true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					var payload map[string]any
					unmarshalText(t, callTool(t, srv, "graph_context", map[string]any{
						"file_path": "center.md", "depth": tc.depth, "max_results": tc.budget,
					}), &payload)
					neighbors := payload["neighbors"].(map[string]any)
					paths := map[string]bool{}
					for _, direction := range []string{"upstream", "downstream"} {
						for _, raw := range anySlice(neighbors[direction]) {
							path := raw.(map[string]any)["file_path"].(string)
							if paths[path] {
								t.Fatalf("duplicate neighbor %s", path)
							}
							paths[path] = true
						}
					}
					if paths["far.md"] || paths["center.md"] || !paths["near.md"] || (tc.wantCount == 2 && !paths["side.md"]) {
						t.Fatalf("depth %v neighbors: %v", tc.depth, paths)
					}
					if len(paths) != tc.wantCount || float64Int(payload["total_neighbors"]) != tc.wantCount || boolVal(payload["truncated"]) != tc.truncated {
						t.Fatalf("count/budget result: %v", payload)
					}
				})
			}
		})
	}
}
