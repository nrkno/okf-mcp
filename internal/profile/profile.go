// Package profile defines the schema for domain-specific relationship vocabulary,
// loads profile YAML files, validates them, and provides lookup helpers.
package profile

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// Profile describes a domain-specific graph relationship vocabulary.
type Profile struct {
	Name          string
	Version       string
	ConceptTypes  []ConceptType
	Relationships []Relationship
	Invariants    []Invariant

	aliasMap       map[string]string   // normalized heading alias -> relationship name
	relByName      map[string]*Relationship
	conceptAliases map[string][]string // concept type name -> aliases
}

// ConceptType is a document category used by a profile.
type ConceptType struct {
	Name    string   // canonical type name
	Aliases []string // alternative names
}

// Relationship maps Markdown heading text to a canonical edge type.
type Relationship struct {
	Name             string   // canonical relationship name (e.g. "depends_on")
	HeadingAliases   []string // normalized heading texts that map to this relationship
	Inverse          string   // inverse relationship name (e.g. "depended_on_by")
	AllowedSources   []string // allowed source concept types ("*" = any)
	AllowedTargets   []string // allowed target concept types ("*" = any)
}

// Invariant expresses a cardinality rule over edges of a given relationship.
type Invariant struct {
	ID           string // e.g. "P1"
	Description  string
	SourceType   string // concept type to check ("*" = all types)
	Relationship string // relationship type to check
	Direction    string // "incoming" or "outgoing"
	Min          int    // minimum required edge count (0 = no minimum)
	Max          int    // maximum allowed edge count (-1 = unlimited, 0 = forbidden)
	Severity     string // "error", "warning", or "notification"
}

// rawProfile mirrors the YAML structure for unmarshalling.
type rawProfile struct {
	Name          string         `yaml:"name"`
	Version       string         `yaml:"version"`
	ConceptTypes  []rawConcept   `yaml:"concept_types"`
	Relationships []rawRelation  `yaml:"relationships"`
	Invariants    []rawInvariant `yaml:"invariants"`
}

type rawConcept struct {
	Name    string   `yaml:"name"`
	Aliases []string `yaml:"aliases"`
}

type rawRelation struct {
	Name             string   `yaml:"name"`
	HeadingAliases   []string `yaml:"heading_aliases"`
	Inverse          string   `yaml:"inverse"`
	AllowedSources   []string `yaml:"allowed_source_types"`
	AllowedTargets   []string `yaml:"allowed_target_types"`
}

type rawInvariant struct {
	ID           string `yaml:"id"`
	Description  string `yaml:"description"`
	SourceType   string `yaml:"source_type"`
	Relationship string `yaml:"relationship"`
	Direction    string `yaml:"direction"`
	Min          int    `yaml:"min"`
	Max          int    `yaml:"max"`
	Severity     string `yaml:"severity"`
}

// Load reads and validates a profile YAML file.
//
// Validation includes: file readable, valid YAML, version == "1.0", non-empty
// relationship names, at least one heading alias per relationship, non-empty
// invariant id/relationship/direction, valid direction and severity values,
// cardinality consistency, and I-31 heading-alias uniqueness across
// relationships. Duplicate aliases within a single relationship are
// deduplicated.
func Load(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read profile: %w", err)
	}

	var raw rawProfile
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse profile: %w", err)
	}

	if raw.Version != "1.0" {
		return nil, fmt.Errorf("unsupported profile version %q (expected %q)", raw.Version, "1.0")
	}

	p := &Profile{
		Name:          raw.Name,
		Version:       raw.Version,
		ConceptTypes:  make([]ConceptType, 0, len(raw.ConceptTypes)),
		Relationships: make([]Relationship, 0, len(raw.Relationships)),
		Invariants:    make([]Invariant, 0, len(raw.Invariants)),
	}

	for _, c := range raw.ConceptTypes {
		p.ConceptTypes = append(p.ConceptTypes, ConceptType{
			Name:    c.Name,
			Aliases: dedupStrings(c.Aliases),
		})
	}

	aliasMap := make(map[string]string)

	for _, r := range raw.Relationships {
		name := strings.TrimSpace(r.Name)
		if name == "" {
			return nil, fmt.Errorf("relationship name is empty")
		}

		aliases := dedupNormalizedAliases(r.HeadingAliases)
		if len(aliases) == 0 {
			return nil, fmt.Errorf("relationship %q has no heading aliases", name)
		}

		for _, alias := range aliases {
			if other, ok := aliasMap[alias]; ok {
				return nil, fmt.Errorf("heading alias %q is shared by relationships %q and %q", alias, other, name)
			}
			aliasMap[alias] = name
		}

		p.Relationships = append(p.Relationships, Relationship{
			Name:           name,
			HeadingAliases: aliases,
			Inverse:        r.Inverse,
			AllowedSources: r.AllowedSources,
			AllowedTargets: r.AllowedTargets,
		})
	}

	for _, inv := range raw.Invariants {
		id := strings.TrimSpace(inv.ID)
		if id == "" {
			return nil, fmt.Errorf("invariant id is empty")
		}
		if strings.TrimSpace(inv.Relationship) == "" {
			return nil, fmt.Errorf("invariant %q has empty relationship", id)
		}
		if inv.Direction != "incoming" && inv.Direction != "outgoing" {
			return nil, fmt.Errorf("invariant %q has invalid direction %q (want \"incoming\" or \"outgoing\")", id, inv.Direction)
		}
		if inv.Severity != "error" && inv.Severity != "warning" && inv.Severity != "notification" {
			return nil, fmt.Errorf("invariant %q has invalid severity %q (want \"error\", \"warning\", or \"notification\")", id, inv.Severity)
		}
		if inv.Min > 0 && inv.Max >= 0 && inv.Min > inv.Max {
			return nil, fmt.Errorf("invariant %q has contradictory cardinality min=%d max=%d", id, inv.Min, inv.Max)
		}

		p.Invariants = append(p.Invariants, Invariant{
			ID:           id,
			Description:  inv.Description,
			SourceType:   inv.SourceType,
			Relationship: inv.Relationship,
			Direction:    inv.Direction,
			Min:          inv.Min,
			Max:          inv.Max,
			Severity:     inv.Severity,
		})
	}

	p.aliasMap = aliasMap
	p.relByName = make(map[string]*Relationship, len(p.Relationships))
	for i := range p.Relationships {
		p.relByName[p.Relationships[i].Name] = &p.Relationships[i]
	}
	p.conceptAliases = make(map[string][]string, len(p.ConceptTypes))
	for i := range p.ConceptTypes {
		p.conceptAliases[p.ConceptTypes[i].Name] = p.ConceptTypes[i].Aliases
	}

	return p, nil
}

// Default returns the empty domain-neutral profile. All links classify as
// "untyped" when this profile is used.
func Default() *Profile {
	return &Profile{
		ConceptTypes:  []ConceptType{},
		Relationships: []Relationship{},
		Invariants:    []Invariant{},
	}
}

// ClassifyHeading maps a Markdown heading to a relationship name.
// The heading is normalized before matching; returns ("", false) when no
// relationship claims the heading.
func (p *Profile) ClassifyHeading(heading string) (string, bool) {
	norm := normalizeHeading(heading)
	if norm == "" {
		return "", false
	}
	name, ok := p.aliasMap[norm]
	return name, ok
}

// InverseName returns the inverse relationship name for relName. If the
// profile does not define an inverse, it returns the generic fallback
// "referenced_by".
func (p *Profile) InverseName(relName string) string {
	if r, ok := p.relByName[relName]; ok && r.Inverse != "" {
		return r.Inverse
	}
	return "referenced_by"
}

// IsAllowedSource reports whether sourceType may be the source of relName.
// The wildcard "*" allows any type.
func (p *Profile) IsAllowedSource(relName, sourceType string) bool {
	r, ok := p.relByName[relName]
	if !ok {
		return false
	}
	return containsWildcardOrMatch(r.AllowedSources, sourceType)
}

// IsAllowedTarget reports whether targetType may be the target of relName.
// The wildcard "*" allows any type.
func (p *Profile) IsAllowedTarget(relName, targetType string) bool {
	r, ok := p.relByName[relName]
	if !ok {
		return false
	}
	return containsWildcardOrMatch(r.AllowedTargets, targetType)
}

// ConceptTypeAliases returns the aliases for a concept type, or nil if the
// type is not defined. The returned slice is owned by the Profile and must not
// be modified by the caller.
func (p *Profile) ConceptTypeAliases(typeName string) []string {
	return p.conceptAliases[typeName]
}

// normalizeHeading lowercases a heading and collapses consecutive whitespace.
func normalizeHeading(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastSpace := true
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
		} else {
			b.WriteRune(r)
			lastSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}

// dedupNormalizedAliases removes duplicate and empty aliases after
// normalization. Aliases within a single relationship are deduplicated rather
// than rejected.
func dedupNormalizedAliases(aliases []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(aliases))
	for _, a := range aliases {
		norm := normalizeHeading(a)
		if norm == "" {
			continue
		}
		if _, ok := seen[norm]; ok {
			continue
		}
		seen[norm] = struct{}{}
		out = append(out, norm)
	}
	return out
}

func dedupStrings(ss []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func containsWildcardOrMatch(list []string, value string) bool {
	for _, item := range list {
		if item == "*" || item == value {
			return true
		}
	}
	return false
}
