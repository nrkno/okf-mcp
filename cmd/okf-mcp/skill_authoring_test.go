package main

import (
	"encoding/json"
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
		want = strings.ReplaceAll(want, "](/docs/okf-profile.md)", "](../SKILL.md)")
		if got := read(filepath.Join(base, "references", ref.target)); got != want {
			t.Fatalf("portable reference drift: %s; synchronize canonical body and documented link transformation", ref.target)
		}
	}
	if read(filepath.Join(base, "LICENSE")) != read(filepath.Join(root, "LICENSE")) {
		t.Fatal("portable license differs from repository license")
	}
	entry := read(filepath.Join(base, "SKILL.md"))
	fm := parser.DetectFrontmatter(entry)
	var metadata struct{ Name, Description, Type string }
	if !fm.HasFrontmatter || yaml.Unmarshal([]byte(fm.YAMLBlock), &metadata) != nil || metadata.Name != "okf-authoring" || metadata.Description == "" || metadata.Type != "" {
		t.Fatalf("invalid native skill metadata: %+v", metadata)
	}
	var cases struct {
		Cases []struct {
			ID, Request    string
			Expect, Reject []string
		}
	}
	if err := json.Unmarshal([]byte(read(filepath.Join(base, "evals", "cases.json"))), &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases.Cases) < 6 {
		t.Fatal("missing decision-boundary eval cases")
	}
	seen := map[string]bool{}
	for _, c := range cases.Cases {
		if c.ID == "" || seen[c.ID] || c.Request == "" || len(c.Expect) == 0 || len(c.Reject) == 0 {
			t.Fatalf("incomplete or duplicate eval case: %+v", c)
		}
		seen[c.ID] = true
	}
	// Install a real copy away from the source repository; every live local link
	// must resolve within this package, not through ../../docs or implementation paths.
	installed := t.TempDir()
	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
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
