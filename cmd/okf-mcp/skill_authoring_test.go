package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nrkno/plattform-okf-mcp/internal/index"
	"github.com/nrkno/plattform-okf-mcp/internal/linkextract"
	"github.com/nrkno/plattform-okf-mcp/internal/parser"
	"github.com/nrkno/plattform-okf-mcp/internal/profile"
	"github.com/nrkno/plattform-okf-mcp/internal/scanner"
	"github.com/nrkno/plattform-okf-mcp/internal/validator"
	"gopkg.in/yaml.v3"
)

func TestOKFAuthoringPortablePackage(t *testing.T) {
	root := findModuleRoot(t)
	base := filepath.Join(root, "skills", "okf-authoring")
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	for _, ref := range []struct{ source, target string }{
		{"okf-standard.md", "okf-standard.md"},
		{"okf-profile-format.md", "profile-format.md"},
	} {
		source := read(filepath.Join(root, "docs", ref.source))
		fm := parser.DetectFrontmatter(source)
		if !fm.HasFrontmatter {
			t.Fatalf("canonical source lacks frontmatter: %s", ref.source)
		}
		want := strings.TrimLeft(source[fm.BodyOffset:], "\n")
		want = strings.ReplaceAll(want, "](/docs/okf-profile.md)", "](profile-setup.md)")
		if got := read(filepath.Join(base, "references", ref.target)); got != want {
			t.Fatalf("portable reference drift: %s; synchronize canonical body and documented link transformation", ref.target)
		}
	}
	if read(filepath.Join(base, "LICENSE")) != read(filepath.Join(root, "LICENSE")) {
		t.Fatal("portable license differs from repository license")
	}
	// Existence alone previously allowed the setup promise to point to a
	// practitioner file with no connection procedure. Pin the destination's job.
	format := read(filepath.Join(base, "references", "profile-format.md"))
	if !strings.Contains(format, "[Configuring an OKF Profile](profile-setup.md)") {
		t.Fatal("profile setup reference points to the wrong capability")
	}
	setup := read(filepath.Join(base, "references", "profile-setup.md"))
	for _, instruction := range []string{"--profile .okf-profile.yaml", "working directory", "Reconnect/restart", "graph_relationships", "graph_integrity", "unproven"} {
		if !strings.Contains(setup, instruction) {
			t.Fatalf("portable setup lost required instruction: %s", instruction)
		}
	}
	entry := read(filepath.Join(base, "SKILL.md"))
	fm := parser.DetectFrontmatter(entry)
	var metadata struct{ Name, Description, Type string }
	if !fm.HasFrontmatter || yaml.Unmarshal([]byte(fm.YAMLBlock), &metadata) != nil || metadata.Name != "okf-authoring" || metadata.Description == "" || metadata.Type != "" {
		t.Fatalf("invalid native skill metadata: %+v", metadata)
	}
	cases, err := decodeOKFAuthoringEvalCorpus([]byte(read(filepath.Join(base, "evals", "cases.json"))))
	if err != nil {
		t.Fatal(err)
	}
	if cases.Version != "1.0.0" {
		t.Fatal("published package version changed; update this package assertion intentionally")
	}
	if len(cases.Cases) != 9 {
		t.Fatal("missing decision-boundary eval cases")
	}
	seen := map[string]bool{}
	for _, c := range cases.Cases {
		seen[c.ID] = true
	}
	if !seen["mixed-corpus-authoring"] {
		t.Fatal("missing corpus-to-profile methodology case")
	}
	for _, id := range []string{"directional-allow-lists", "incoming-checked-node-selector", "profile-schema-incident", "index-missing-document", "current-log-entry", "nested-bundle-and-native-skill", "content-review-negative-space", "portable-install-and-tool-absence"} {
		if !seen[id] {
			t.Fatalf("missing source/target decision-boundary eval: %s", id)
		}
	}
	// Install a real copy away from the source repository; every live local link
	// must resolve within this package, not through ../../docs or implementation paths.
	installed := t.TempDir()
	err = filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(installed, rel), 0o755)
		}
		if d.Type()&os.ModeSymlink != 0 {
			t.Fatalf("non-portable symlink: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(installed, rel), data, 0o644); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(installed, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		// A wider extraction root exposes links escaping the installed package
		// instead of silently dropping them as normal graph containment would.
		volumeRoot := filepath.VolumeName(installed) + string(filepath.Separator)
		for _, link := range linkextract.Extract(string(data), filepath.Dir(path), volumeRoot) {
			target := filepath.Join(volumeRoot, link.Target)
			rel, err := filepath.Rel(installed, target)
			if err != nil || !filepath.IsLocal(rel) {
				t.Fatalf("reference escapes installed skill: %s -> %s", path, target)
			}
			if _, err := os.Stat(target); err != nil {
				t.Fatalf("installed reference missing: %s -> %s", path, target)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	findings, err := validator.ValidateDoc(filepath.Join(installed, "SKILL.md"), nil)
	if err != nil || len(findings) != 1 || findings[0].Code != "E2" {
		t.Fatalf("native metadata must not be treated as content metadata: %v, %v", findings, err)
	}
	idx := index.New(installed, scanner.ScanOptions{}, profile.Default())
	result := validator.ValidateBundle(idx)
	if result.Summary.Errors != 0 || len(idx.Docs()) != 0 {
		t.Fatalf("whole-corpus validation should skip native/reference assets, not prove their conformance: %+v", result)
	}
}

// This checks this package's selected published artifact shape, not the central
// loader's complete accepted language (which also normalizes legacy shapes).
// It proves neither live skill value nor native integration.
type okfAuthoringEvalCase struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	Trap                 string   `json:"trap"`
	Prompt               string   `json:"prompt"`
	Expectations         []string `json:"expectations"`
	NegativeExpectations []string `json:"negative_expectations"`
}

type okfAuthoringEvalCorpus struct {
	Skill   string                 `json:"skill"`
	Version string                 `json:"version"`
	Shape   int                    `json:"shape"`
	Cases   []okfAuthoringEvalCase `json:"cases"`
}

func decodeOKFAuthoringEvalCorpus(data []byte) (okfAuthoringEvalCorpus, error) {
	var corpus okfAuthoringEvalCorpus
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		return corpus, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return corpus, fmt.Errorf("expected one corpus JSON value, got %v", err)
	}
	if corpus.Skill != "okf-authoring" || strings.TrimSpace(corpus.Version) == "" || corpus.Shape != 1 || len(corpus.Cases) == 0 {
		return corpus, fmt.Errorf("invalid skill/version/shape/cases envelope")
	}
	ids, names := map[string]bool{}, map[string]bool{}
	for i, c := range corpus.Cases {
		for field, value := range map[string]string{"id": c.ID, "name": c.Name, "trap": c.Trap, "prompt": c.Prompt} {
			if strings.TrimSpace(value) == "" {
				return corpus, fmt.Errorf("case %d requires nonempty %s", i, field)
			}
		}
		if ids[c.ID] || names[c.Name] {
			return corpus, fmt.Errorf("case %d duplicates id or name", i)
		}
		ids[c.ID], names[c.Name] = true, true
		for field, values := range map[string][]string{"expectations": c.Expectations, "negative_expectations": c.NegativeExpectations} {
			if len(values) == 0 {
				return corpus, fmt.Errorf("case %d requires nonempty %s", i, field)
			}
			for _, value := range values {
				if strings.TrimSpace(value) == "" {
					return corpus, fmt.Errorf("case %d has empty %s item", i, field)
				}
			}
		}
	}
	return corpus, nil
}

func TestOKFAuthoringPublishedEvalShape(t *testing.T) {
	t.Parallel()
	const valid = `{"skill":"okf-authoring","version":"1.0.0","shape":1,"cases":[{"id":"example","name":"Example boundary","trap":"Green validation hides missing inventory","prompt":"Assess the incomplete index inline","expectations":["Flag missing document"],"negative_expectations":["Approve from validator success"]}]}`
	if _, err := decodeOKFAuthoringEvalCorpus([]byte(valid)); err != nil {
		t.Fatalf("valid contract rejected: %v", err)
	}
	// Version is metadata, not an invented SemVer compatibility constraint.
	for _, version := range []string{"2.0.0", "next"} {
		if _, err := decodeOKFAuthoringEvalCorpus([]byte(strings.ReplaceAll(valid, "1.0.0", version))); err != nil {
			t.Fatalf("nonempty version metadata rejected: %v", err)
		}
	}
	checkMutation := func(name string, mutate func(map[string]any, map[string]any)) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			var envelope map[string]any
			if err := json.Unmarshal([]byte(valid), &envelope); err != nil {
				t.Fatal(err)
			}
			c := envelope["cases"].([]any)[0].(map[string]any)
			mutate(envelope, c)
			data, err := json.Marshal(envelope)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decodeOKFAuthoringEvalCorpus(data); err == nil {
				t.Fatalf("malformed contract accepted: %s", data)
			}
		})
	}
	for _, field := range []string{"skill", "version", "shape", "cases"} {
		checkMutation("missing envelope "+field, func(e, _ map[string]any) { delete(e, field) })
		checkMutation("null envelope "+field, func(e, _ map[string]any) { e[field] = nil })
		checkMutation("wrong envelope type "+field, func(e, _ map[string]any) { e[field] = true })
	}
	for _, field := range []string{"id", "name", "trap", "prompt", "expectations", "negative_expectations"} {
		checkMutation("missing case "+field, func(_, c map[string]any) { delete(c, field) })
		checkMutation("null case "+field, func(_, c map[string]any) { c[field] = nil })
		checkMutation("wrong case type "+field, func(_, c map[string]any) { c[field] = 1 })
	}
	for _, field := range []string{"id", "name", "trap", "prompt"} {
		checkMutation("blank "+field, func(_, c map[string]any) { c[field] = " \n" })
	}
	for _, field := range []string{"expectations", "negative_expectations"} {
		for _, value := range []any{[]any{}, []any{" "}, []any{nil}, []any{1}, "not an array"} {
			checkMutation(fmt.Sprintf("invalid %s %v", field, value), func(_, c map[string]any) { c[field] = value })
		}
	}
	for _, field := range []string{"description", "request", "expect", "reject", "origin"} {
		checkMutation("legacy envelope "+field, func(e, _ map[string]any) { e[field] = "legacy" })
		checkMutation("legacy case "+field, func(_, c map[string]any) { c[field] = "legacy" })
	}
	checkMutation("wrong skill", func(e, _ map[string]any) { e["skill"] = "another-skill" })
	checkMutation("blank version", func(e, _ map[string]any) { e["version"] = " " })
	checkMutation("wrong shape", func(e, _ map[string]any) { e["shape"] = 2 })
	checkMutation("fractional shape", func(e, _ map[string]any) { e["shape"] = 1.5 })
	checkMutation("empty cases", func(e, _ map[string]any) { e["cases"] = []any{} })
	checkMutation("null case", func(e, _ map[string]any) { e["cases"] = []any{nil} })
	for _, field := range []string{"id", "name"} {
		checkMutation("duplicate "+field, func(e, c map[string]any) {
			other := map[string]any{}
			for k, v := range c {
				other[k] = v
			}
			other["id"], other["name"] = "second", "Second boundary"
			other[field] = c[field]
			e["cases"] = []any{c, other}
		})
	}
	for _, suffix := range []string{` {}`, ` trailing`} {
		t.Run("trailing JSON "+suffix, func(t *testing.T) {
			if _, err := decodeOKFAuthoringEvalCorpus([]byte(valid + suffix)); err == nil {
				t.Fatal("trailing input accepted")
			}
		})
	}
}

// Editorial regression for the published rubrics, not a model-judge execution.
// Prompts supply a date, not an instant, and profile names are optional.
func TestOKFAuthoringOracleFairness(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join(findModuleRoot(t), "skills", "okf-authoring", "evals", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := decodeOKFAuthoringEvalCorpus(data)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]okfAuthoringEvalCase{}
	for _, c := range corpus.Cases {
		cases[c.ID] = c
	}
	for _, tc := range []struct {
		id                  string
		required, forbidden []string
	}{
		{"mixed-corpus-authoring", []string{"parseable complete profile YAML", "required version 1.0", "profile name is optional"}, []string{"nonempty name"}},
		{"current-log-entry", []string{"valid complete Markdown artifacts", "valid ISO timestamp dated 2026-10-10", "any supported time of day", "unchanged 2026-10-09 Creation history"}, []string{"2026-10-10T00:00:00Z"}},
		{"portable-install-and-tool-absence", []string{"complete valid document", "valid ISO timestamp dated 2026-10-10", "any supported time of day", "unchanged accepted body/link"}, []string{"2026-10-10T00:00:00Z"}},
	} {
		t.Run(tc.id, func(t *testing.T) {
			oracle := strings.Join(cases[tc.id].Expectations, "\n")
			for _, required := range tc.required {
				if !strings.Contains(oracle, required) {
					t.Fatalf("missing fairness/artifact criterion: %s", required)
				}
			}
			for _, forbidden := range tc.forbidden {
				if strings.Contains(oracle, forbidden) {
					t.Fatalf("unsupported oracle constraint: %s", forbidden)
				}
			}
		})
	}
}

func TestOKFAuthoringValidatorBoundaries(t *testing.T) {
	dir := t.TempDir()
	write := func(path, content string) string {
		t.Helper()
		abs := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return abs
	}
	write("a.md", frontmatter("Architecture", "A", "A document", []string{"example"})+"# A\n")
	write("b.md", frontmatter("Architecture", "B", "B document", []string{"example"})+"# B\n")
	for _, tc := range []struct {
		name, index string
		wantError   bool
	}{
		{"complete", "# Index\n\n- [A](/a.md)\n- [B](/b.md)\n", false},
		{"incomplete but validator green", "# Index\n\n- [A](/a.md)\n", false},
		{"reserved frontmatter", "---\ntype: Architecture\n---\n# Index\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			findings, err := validator.ValidateReserved(write("index.md", tc.index), "index.md")
			if err != nil || (len(findings) > 0) != tc.wantError {
				t.Fatalf("reserved index result: %v %v", findings, err)
			}
		})
	}
	const currentDate = "2026-10-10" // Explicit scenario date, not a wall-clock claim.
	for _, date := range []string{currentDate, "2026-10-09"} {
		t.Run("log "+date, func(t *testing.T) {
			body := "---\ntype: Log\n---\n# Log\n\n## " + date + "\n\n**Update**: [A](/a.md) — Changed document.\n"
			findings, err := validator.ValidateReserved(write("log.md", body), "log.md")
			if err != nil || len(findings) != 0 {
				t.Fatalf("valid structure should pass even for stale log: %v %v", findings, err)
			}
			if strings.Contains(body, "## "+currentDate) != (date == currentDate) {
				t.Fatal("fixture currency expectation failed")
			}
		})
	}
	write("index.md", "# Index\n")
	write("a.md", "---\ntype: Architecture\n---\n# A\n")
	write("b.md", "# B missing metadata\n")
	write("child/index.md", "# Child\n")
	write("child/c.md", "---\ntype: Architecture\n---\n# C\n")
	idx := index.New(dir, scanner.ScanOptions{}, profile.Default())
	if err := idx.Rebuild(); err != nil {
		t.Fatal(err)
	}
	if len(idx.Docs()) != 2 {
		t.Fatalf("missing-type document must be absent from discovery: %v", idx.Docs())
	}
	for _, doc := range idx.Docs() {
		if doc.FilePath == "child/c.md" && doc.Bundle != "child" {
			t.Fatalf("nested document mis-owned: %+v", doc)
		}
	}
}
