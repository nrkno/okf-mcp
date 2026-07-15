package matcher

import (
	"strings"
	"unicode"

	"github.com/nrkno/plattform-okf-mcp/internal/parser"
)

// Score returns the weighted token match score for doc against query and tag filter.
//
// Weights per token:
//   - 3.0 if token is a substring of doc.Title (case-insensitive)
//   - 2.0 if token is a substring of any element of doc.Tags (case-insensitive)
//   - 1.0 if token is a substring of doc.Description (case-insensitive)
//
// Returns -1 if:
//   - matchMode is not "and" or "or"
//   - filterTags is non-empty and doc fails the tag filter
//
// Empty filterTags means no filtering — all docs are eligible regardless of matchMode.
// Empty query means all eligible docs score 0.
//
// Caller is responsible for validating matchMode before calling Score;
// Score itself does not return an error.
func Score(query string, filterTags []string, matchMode string, doc parser.Doc) float64 {
	// Validate matchMode first — return -1 for anything unknown.
	if matchMode != "and" && matchMode != "or" {
		return -1
	}

	// Apply tag filter (only when filterTags is non-empty).
	if len(filterTags) > 0 {
		if !passesTagFilter(filterTags, matchMode, doc.Tags) {
			return -1
		}
	}

	// Tokenise and deduplicate.
	tokens := tokenise(query)
	if len(tokens) == 0 {
		return 0
	}

	titleLower := strings.ToLower(doc.Title)
	descLower := strings.ToLower(doc.Description)
	tagsLower := make([]string, len(doc.Tags))
	for i, t := range doc.Tags {
		tagsLower[i] = strings.ToLower(t)
	}

	var score float64
	for _, tok := range tokens {
		if strings.Contains(titleLower, tok) {
			score += 3.0
		}
		for _, tag := range tagsLower {
			if strings.Contains(tag, tok) {
				score += 2.0
				break // count each token at most once per tag field
			}
		}
		if strings.Contains(descLower, tok) {
			score += 1.0
		}
	}

	return score
}

// FindBest returns the highest-scoring doc from docs.
//
// Tie-break: alphabetical (lexicographic) by doc.FilePath ascending (invariant I-6).
// Returns (Doc{}, false) if:
//   - docs is empty
//   - no doc has score > 0 (after tag filter and matchMode validation)
func FindBest(query string, filterTags []string, matchMode string, docs []parser.Doc) (parser.Doc, bool) {
	var best parser.Doc
	bestScore := 0.0
	found := false

	for _, doc := range docs {
		s := Score(query, filterTags, matchMode, doc)
		if s <= 0 {
			continue
		}
		if !found || s > bestScore || (s == bestScore && doc.FilePath < best.FilePath) {
			best = doc
			bestScore = s
			found = true
		}
	}

	if !found {
		return parser.Doc{}, false
	}
	return best, true
}

// tokenise splits query into lowercased, deduplicated tokens.
// Tokens are split on any non-alphanumeric rune.
func tokenise(query string) []string {
	raw := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, tok := range raw {
		if _, ok := seen[tok]; !ok {
			seen[tok] = struct{}{}
			out = append(out, tok)
		}
	}
	return out
}

// passesTagFilter returns true if docTags satisfies the filterTags constraint.
// matchMode "and": doc must contain ALL filterTags (case-insensitive exact match).
// matchMode "or":  doc must contain AT LEAST ONE filterTag (case-insensitive).
// Caller guarantees matchMode is "and" or "or".
func passesTagFilter(filterTags []string, matchMode string, docTags []string) bool {
	switch matchMode {
	case "and":
		for _, required := range filterTags {
			if !containsTagCI(docTags, required) {
				return false
			}
		}
		return true
	default: // "or"
		for _, wanted := range filterTags {
			if containsTagCI(docTags, wanted) {
				return true
			}
		}
		return false
	}
}

// containsTagCI returns true if any element of tags equals target
// under case-insensitive comparison.
func containsTagCI(tags []string, target string) bool {
	target = strings.ToLower(target)
	for _, t := range tags {
		if strings.ToLower(t) == target {
			return true
		}
	}
	return false
}
