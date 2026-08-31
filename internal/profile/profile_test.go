package profile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeProfile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return p
}

func validProfile() string {
	return `name: "engineering"
version: "1.0"

concept_types:
  - name: "Requirement"
    aliases: ["Behavioral Requirement", "Functional Requirement"]
  - name: "Implementation"
    aliases: ["Implementation Plan"]

relationships:
  - name: "depends_on"
    heading_aliases: ["Depends on", "Dependencies", "Requires"]
    inverse: "depended_on_by"
    allowed_source_types: ["*"]
    allowed_target_types: ["*"]

  - name: "implements"
    heading_aliases: ["Implements", "Implementation of"]
    inverse: "implemented_by"
    allowed_source_types: ["Implementation"]
    allowed_target_types: ["Requirement"]

invariants:
  - id: "P1"
    description: "Every Requirement must be implemented"
    source_type: "Requirement"
    relationship: "implements"
    direction: "incoming"
    min: 1
    max: -1
    severity: "warning"

  - id: "P2"
    description: "No document should supersede more than one"
    source_type: "*"
    relationship: "supersedes"
    direction: "outgoing"
    min: 0
    max: 1
    severity: "error"
`
}

func TestLoad_ValidProfile(t *testing.T) {
	dir := t.TempDir()
	path := writeProfile(t, dir, "profile.yaml", validProfile())

	p, err := Load(path)
	if err != nil {
		t.Fatalf("Load valid profile: unexpected error: %v", err)
	}

	if p.Name != "engineering" {
		t.Errorf("Name = %q, want %q", p.Name, "engineering")
	}
	if p.Version != "1.0" {
		t.Errorf("Version = %q, want %q", p.Version, "1.0")
	}
	if len(p.ConceptTypes) != 2 {
		t.Errorf("len(ConceptTypes) = %d, want 2", len(p.ConceptTypes))
	}
	if len(p.Relationships) != 2 {
		t.Errorf("len(Relationships) = %d, want 2", len(p.Relationships))
	}
	if len(p.Invariants) != 2 {
		t.Errorf("len(Invariants) = %d, want 2", len(p.Invariants))
	}

	if p.Relationships[0].Name != "depends_on" {
		t.Errorf("first relationship name = %q, want %q", p.Relationships[0].Name, "depends_on")
	}
	if len(p.Relationships[0].HeadingAliases) != 3 {
		t.Errorf("depends_on aliases = %d, want 3", len(p.Relationships[0].HeadingAliases))
	}

	inv := p.Invariants[0]
	if inv.ID != "P1" || inv.Min != 1 || inv.Max != -1 || inv.Severity != "warning" {
		t.Errorf("P1 fields mismatch: %+v", inv)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err == nil {
		t.Fatal("Load missing file: expected error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeProfile(t, dir, "bad.yaml", "not: [valid yaml: : :")

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load invalid YAML: expected error")
	}
}

func TestLoad_UnsupportedVersion(t *testing.T) {
	dir := t.TempDir()
	path := writeProfile(t, dir, "v2.yaml", `name: "x"
version: "2.0"
relationships: []
invariants: []
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load unsupported version: expected error")
	}
	want := "unsupported profile version"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err.Error(), want)
	}
}

func TestLoad_EmptyRelationshipName(t *testing.T) {
	dir := t.TempDir()
	path := writeProfile(t, dir, "bad.yaml", `name: "x"
version: "1.0"
relationships:
  - name: ""
    heading_aliases: ["Depends on"]
invariants: []
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty relationship name")
	}
	if !strings.Contains(err.Error(), "relationship name is empty") {
		t.Errorf("error %q does not contain expected text", err.Error())
	}
}

func TestLoad_NoHeadingAliases(t *testing.T) {
	dir := t.TempDir()
	path := writeProfile(t, dir, "bad.yaml", `name: "x"
version: "1.0"
relationships:
  - name: "depends_on"
    heading_aliases: []
invariants: []
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for no heading aliases")
	}
	if !strings.Contains(err.Error(), "has no heading aliases") {
		t.Errorf("error %q does not contain expected text", err.Error())
	}
}

func TestLoad_InvalidInvariantFields(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "empty id",
			content: `name: "x"
version: "1.0"
relationships: []
invariants:
  - id: ""
    relationship: "depends_on"
    direction: "outgoing"
    severity: "warning"
`,
			want: "invariant id is empty",
		},
		{
			name: "empty relationship",
			content: `name: "x"
version: "1.0"
relationships: []
invariants:
  - id: "P1"
    relationship: ""
    direction: "outgoing"
    severity: "warning"
`,
			want: "has empty relationship",
		},
		{
			name: "empty direction",
			content: `name: "x"
version: "1.0"
relationships: []
invariants:
  - id: "P1"
    relationship: "depends_on"
    direction: ""
    severity: "warning"
`,
			want: "invalid direction",
		},
		{
			name: "invalid direction",
			content: `name: "x"
version: "1.0"
relationships: []
invariants:
  - id: "P1"
    relationship: "depends_on"
    direction: "sideways"
    severity: "warning"
`,
			want: "invalid direction",
		},
		{
			name: "invalid severity",
			content: `name: "x"
version: "1.0"
relationships: []
invariants:
  - id: "P1"
    relationship: "depends_on"
    direction: "outgoing"
    severity: "critical"
`,
			want: "invalid severity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeProfile(t, dir, "bad.yaml", tt.content)

			_, err := Load(path)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.want)
			}
		})
	}
}

func TestLoad_CardinalityContradiction(t *testing.T) {
	dir := t.TempDir()
	path := writeProfile(t, dir, "bad.yaml", `name: "x"
version: "1.0"
relationships:
  - name: "depends_on"
    heading_aliases: ["Depends on"]
invariants:
  - id: "P1"
    relationship: "depends_on"
    direction: "outgoing"
    min: 3
    max: 1
    severity: "warning"
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected cardinality contradiction error")
	}
	want := "contradictory cardinality"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err.Error(), want)
	}
}

func TestLoad_AliasUniqueness_CrossRelationship(t *testing.T) {
	dir := t.TempDir()
	path := writeProfile(t, dir, "bad.yaml", `name: "x"
version: "1.0"
relationships:
  - name: "depends_on"
    heading_aliases: ["Depends on"]
  - name: "requires"
    heading_aliases: ["depends on"]
invariants: []
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected I-31 alias uniqueness error")
	}
	for _, want := range []string{"depends on", "depends_on", "requires"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}
}

func TestLoad_AliasUniqueness_IntraRelationship_Dedup(t *testing.T) {
	dir := t.TempDir()
	path := writeProfile(t, dir, "ok.yaml", `name: "x"
version: "1.0"
relationships:
  - name: "depends_on"
    heading_aliases: ["Depends on", "depends on", "DEPENDS ON"]
invariants: []
`)

	p, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Relationships) != 1 {
		t.Fatalf("expected 1 relationship, got %d", len(p.Relationships))
	}
	aliases := p.Relationships[0].HeadingAliases
	if len(aliases) != 1 {
		t.Fatalf("expected 1 deduplicated alias, got %d", len(aliases))
	}
	if aliases[0] != "depends on" {
		t.Errorf("alias = %q, want %q", aliases[0], "depends on")
	}
}

func TestDefault(t *testing.T) {
	p := Default()
	if p == nil {
		t.Fatal("Default() returned nil")
	}
	if len(p.Relationships) != 0 {
		t.Errorf("Default() relationships = %d, want 0", len(p.Relationships))
	}
	if len(p.Invariants) != 0 {
		t.Errorf("Default() invariants = %d, want 0", len(p.Invariants))
	}
}

func TestClassifyHeading(t *testing.T) {
	dir := t.TempDir()
	path := writeProfile(t, dir, "profile.yaml", `name: "x"
version: "1.0"
relationships:
  - name: "depends_on"
    heading_aliases: ["Depends on", "Dependencies"]
  - name: "implements"
    heading_aliases: ["Implements"]
invariants: []
`)

	p, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	tests := []struct {
		heading string
		wantRel string
		wantOK  bool
	}{
		{"Depends on", "depends_on", true},
		{"depends   on", "depends_on", true},
		{"DEPENDS ON", "depends_on", true},
		{"Implements", "implements", true},
		{"Notes", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.heading, func(t *testing.T) {
			rel, ok := p.ClassifyHeading(tt.heading)
			if rel != tt.wantRel || ok != tt.wantOK {
				t.Errorf("ClassifyHeading(%q) = (%q, %v), want (%q, %v)",
					tt.heading, rel, ok, tt.wantRel, tt.wantOK)
			}
		})
	}
}

func TestInverseName(t *testing.T) {
	dir := t.TempDir()
	path := writeProfile(t, dir, "profile.yaml", `name: "x"
version: "1.0"
relationships:
  - name: "depends_on"
    heading_aliases: ["Depends on"]
    inverse: "depended_on_by"
  - name: "untyped_alias"
    heading_aliases: ["X"]
invariants: []
`)

	p, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := p.InverseName("depends_on"); got != "depended_on_by" {
		t.Errorf("InverseName(depends_on) = %q, want %q", got, "depended_on_by")
	}
	if got := p.InverseName("unknown"); got != "referenced_by" {
		t.Errorf("InverseName(unknown) = %q, want %q", got, "referenced_by")
	}
	if got := p.InverseName("untyped_alias"); got != "referenced_by" {
		t.Errorf("InverseName(untyped_alias) = %q, want %q", got, "referenced_by")
	}
}

func TestIsAllowedSource(t *testing.T) {
	dir := t.TempDir()
	path := writeProfile(t, dir, "profile.yaml", `name: "x"
version: "1.0"
relationships:
  - name: "wildcard"
    heading_aliases: ["W"]
    allowed_source_types: ["*"]
  - name: "specific"
    heading_aliases: ["S"]
    allowed_source_types: ["Implementation"]
invariants: []
`)

	p, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !p.IsAllowedSource("wildcard", "Anything") {
		t.Error("wildcard should allow any source")
	}
	if !p.IsAllowedSource("specific", "Implementation") {
		t.Error("specific should allow Implementation")
	}
	if p.IsAllowedSource("specific", "Requirement") {
		t.Error("specific should not allow Requirement")
	}
	if p.IsAllowedSource("unknown", "X") {
		t.Error("unknown relationship should not allow anything")
	}
}

func TestIsAllowedTarget(t *testing.T) {
	dir := t.TempDir()
	path := writeProfile(t, dir, "profile.yaml", `name: "x"
version: "1.0"
relationships:
  - name: "depends_on"
    heading_aliases: ["D"]
    allowed_target_types: ["*"]
  - name: "implements"
    heading_aliases: ["I"]
    allowed_target_types: ["Requirement", "Design"]
invariants: []
`)

	p, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !p.IsAllowedTarget("depends_on", "X") {
		t.Error("depends_on should allow any target")
	}
	if !p.IsAllowedTarget("implements", "Design") {
		t.Error("implements should allow Design")
	}
	if p.IsAllowedTarget("implements", "Verification") {
		t.Error("implements should not allow Verification")
	}
}

func TestConceptTypeAliases(t *testing.T) {
	dir := t.TempDir()
	path := writeProfile(t, dir, "profile.yaml", `name: "x"
version: "1.0"
concept_types:
  - name: "Requirement"
    aliases: ["R1", "R2"]
relationships: []
invariants: []
`)

	p, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := p.ConceptTypeAliases("Requirement")
	want := []string{"R1", "R2"}
	if len(got) != len(want) {
		t.Fatalf("aliases = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("alias[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if p.ConceptTypeAliases("Missing") != nil {
		t.Error("expected nil for missing concept type")
	}
}


