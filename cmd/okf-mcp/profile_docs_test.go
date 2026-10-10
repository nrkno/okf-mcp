package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nrkno/plattform-okf-mcp/internal/scanner"
)

// docFences reads the actual published examples so documentation drift fails tests.
func docFences(t *testing.T, path, language string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var blocks []string
	rest := string(data)
	for {
		_, after, found := strings.Cut(rest, "```"+language+"\n")
		if !found {
			return blocks
		}
		block, remaining, closed := strings.Cut(after, "\n```")
		if !closed {
			t.Fatalf("unclosed %s fence in %s", language, path)
		}
		blocks = append(blocks, block+"\n")
		rest = remaining
	}
}

func TestPublishedProfileExamples(t *testing.T) {
	root := findModuleRoot(t)
	guide := filepath.Join(root, "docs", "okf-profile.md")
	yamlBlocks := docFences(t, guide, "yaml")
	docs := docFences(t, guide, "markdown")
	if len(yamlBlocks) != 1 || len(docs) != 2 {
		t.Fatalf("expected one profile and two documents; got %d, %d", len(yamlBlocks), len(docs))
	}
	// Include the preserved main-branch AOS profile, not only a test vocabulary.
	repoProfile, err := os.ReadFile(filepath.Join(root, ".okf-profile.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	profiles := []struct {
		name string
		yaml string
	}{
		{"guide", yamlBlocks[0]},
		{"repository", string(repoProfile)},
	}
	for _, prof := range profiles {
		t.Run(prof.name, func(t *testing.T) {
			cases := []struct {
				name         string
				anchor       string
				requirement  string
				wantCheck    string
				wantSeverity string
			}{
				{"valid", docs[0], docs[1], "", ""},
				{"missing source", docs[0], strings.ReplaceAll(docs[1], "- [Example anchor](anchor.md)", "No source link."), "cardinality_violation", "error"},
				{"wrong target type", strings.ReplaceAll(docs[0], "type: anchor", "type: design"), docs[1], "profile_violations", "warning"},
				{"nearest heading", docs[0], strings.ReplaceAll(docs[1], "- [Example anchor]", "### Notes\n\n- [Example anchor]"), "cardinality_violation", "error"},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					dir := t.TempDir()
					writeProfile(t, dir, prof.yaml)
					if _, src, err := loadProfile(dir, filepath.Join(dir, ".okf-profile.yaml")); err != nil || src.Kind != "explicit" {
						t.Fatalf("explicit profile: source=%+v err=%v", src, err)
					}
					if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
						t.Fatal(err)
					}
					for path, content := range map[string]string{"anchor.md": tc.anchor, "requirement.md": tc.requirement} {
						if err := os.WriteFile(filepath.Join(dir, "docs", path), []byte(content), 0o644); err != nil {
							t.Fatal(err)
						}
					}
					srv := newFixtureServer(t, dir, scanner.ScanOptions{})
					defer srv.Close()
					var result map[string]any
					unmarshalText(t, callTool(t, srv, "graph_integrity", map[string]any{"checks": []any{"profile_violations"}}), &result)
					if !boolVal(result["profile_loaded"]) {
						t.Fatal("profile did not load")
					}
					findings := anySlice(result["findings"])
					if tc.wantCheck == "" {
						if len(findings) != 0 {
							t.Fatalf("unexpected findings: %v", findings)
						}
						for _, query := range []struct {
							path, direction, typ, endpoint, value string
						}{
							{"docs/requirement.md", "outgoing", "derived-from", "target", "docs/anchor.md"},
							{"docs/anchor.md", "incoming", "derives", "source", "docs/requirement.md"},
						} {
							unmarshalText(t, callTool(t, srv, "graph_relationships", map[string]any{"file_path": query.path, "direction": query.direction, "type": query.typ}), &result)
							edges := anySlice(result[query.direction])
							if len(edges) != 1 || edges[0].(map[string]any)[query.endpoint] != query.value {
								t.Fatalf("%s relationship: %v", query.direction, edges)
							}
						}
					} else {
						if len(findings) != 1 {
							t.Fatalf("expected one %s finding: %v", tc.wantCheck, findings)
						}
						finding := findings[0].(map[string]any)
						if finding["check"] != tc.wantCheck || finding["severity"] != tc.wantSeverity {
							t.Fatalf("wrong finding: %v", finding)
						}
					}
					// Graph errors do not manufacture frontmatter errors.
					unmarshalText(t, callTool(t, srv, "validate_doc", nil), &result)
					if result["summary"].(map[string]any)["Errors"] != float64(0) {
						t.Fatalf("document validation errors: %v", result)
					}
				})
			}
		})
	}
	formatProfiles := docFences(t, filepath.Join(root, "docs", "okf-profile-format.md"), "yaml")
	if len(formatProfiles) != 1 {
		t.Fatalf("expected one format-reference profile, got %d", len(formatProfiles))
	}
	for _, block := range formatProfiles {
		dir := t.TempDir()
		writeProfile(t, dir, block)
		if _, _, err := loadProfile(dir, filepath.Join(dir, ".okf-profile.yaml")); err != nil {
			t.Fatalf("format reference profile: %v", err)
		}
	}
}
