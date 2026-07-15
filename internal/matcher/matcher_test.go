package matcher

import (
	"testing"

	"github.com/nrkno/plattform-okf-mcp/internal/parser"
)

// helpers

func doc(title, desc string, tags ...string) parser.Doc {
	return parser.Doc{
		Title:       title,
		Description: desc,
		Tags:        tags,
		FilePath:    "a/doc.md",
	}
}

func docAt(path, title, desc string, tags ...string) parser.Doc {
	return parser.Doc{
		Title:       title,
		Description: desc,
		Tags:        tags,
		FilePath:    path,
	}
}

// Score — field weight tests

func TestScore_TitleWeight(t *testing.T) {
	t.Parallel()
	d := doc("Kubernetes Guide", "Deploy apps", "docker")
	got := Score("kubernetes", nil, "and", d)
	if got != 3.0 {
		t.Errorf("Score = %v, want 3.0 (title hit)", got)
	}
}

func TestScore_TagWeight(t *testing.T) {
	t.Parallel()
	d := doc("Deploy Guide", "Some description", "kubernetes", "helm")
	got := Score("kubernetes", nil, "and", d)
	if got != 2.0 {
		t.Errorf("Score = %v, want 2.0 (tag hit only)", got)
	}
}

func TestScore_DescriptionWeight(t *testing.T) {
	t.Parallel()
	d := doc("Deploy Guide", "Deploy kubernetes applications", "docker")
	// "kubernetes" hits description (1) only — not in title or tags
	got := Score("kubernetes", nil, "and", d)
	if got != 1.0 {
		t.Errorf("Score = %v, want 1.0 (description hit only)", got)
	}
}

func TestScore_AllFieldsHit(t *testing.T) {
	t.Parallel()
	// token "go" appears in title, a tag, and description → 3+2+1 = 6
	d := doc("Go Programming", "Learn go basics", "go", "programming")
	got := Score("go", nil, "and", d)
	if got != 6.0 {
		t.Errorf("Score = %v, want 6.0 (title+tag+desc hit)", got)
	}
}

func TestScore_MultipleTokens(t *testing.T) {
	t.Parallel()
	// "go" hits title(3)+tag(2)+desc(1)=6, "programming" hits title(3)+tag(2)=5 → 11
	d := doc("Go Programming", "Learn go basics", "go", "programming")
	got := Score("go programming", nil, "and", d)
	if got != 11.0 {
		t.Errorf("Score = %v, want 11.0", got)
	}
}

func TestScore_EmptyQuery(t *testing.T) {
	t.Parallel()
	d := doc("Kubernetes Guide", "Deploy apps", "kubernetes")
	got := Score("", nil, "and", d)
	if got != 0 {
		t.Errorf("Score = %v, want 0 (empty query)", got)
	}
}

// Score — case-insensitive matching

func TestScore_CaseInsensitiveTitle(t *testing.T) {
	t.Parallel()
	d := doc("Kubernetes Guide", "Deploy apps", "docker")
	got := Score("KUBERNETES", nil, "and", d)
	if got != 3.0 {
		t.Errorf("Score = %v, want 3.0 (uppercase query matches lowercase title)", got)
	}
}

func TestScore_CaseInsensitiveTag(t *testing.T) {
	t.Parallel()
	d := doc("Guide", "Deploy apps", "Kubernetes")
	got := Score("kubernetes", nil, "and", d)
	if got != 2.0 {
		t.Errorf("Score = %v, want 2.0 (lowercase query matches mixed-case tag)", got)
	}
}

func TestScore_CaseInsensitiveDescription(t *testing.T) {
	t.Parallel()
	d := doc("Guide", "Deploy KUBERNETES applications", "docker")
	got := Score("kubernetes", nil, "and", d)
	if got != 1.0 {
		t.Errorf("Score = %v, want 1.0 (lowercase query matches uppercase description)", got)
	}
}

// Score — tag filter "and"

func TestScore_TagFilterAnd_AllPresent(t *testing.T) {
	t.Parallel()
	d := doc("Guide", "Some description", "go", "kubernetes", "helm")
	got := Score("guide", []string{"go", "kubernetes"}, "and", d)
	// "guide" hits title (3)
	if got != 3.0 {
		t.Errorf("Score = %v, want 3.0 (all required tags present)", got)
	}
}

func TestScore_TagFilterAnd_MissingOneTag(t *testing.T) {
	t.Parallel()
	d := doc("Guide", "Some description", "go") // missing "kubernetes"
	got := Score("guide", []string{"go", "kubernetes"}, "and", d)
	if got != -1 {
		t.Errorf("Score = %v, want -1 (missing required tag)", got)
	}
}

// Score — tag filter "or"

func TestScore_TagFilterOr_OneMatch(t *testing.T) {
	t.Parallel()
	d := doc("Guide", "Some description", "go") // has "go", not "kubernetes"
	got := Score("guide", []string{"go", "kubernetes"}, "or", d)
	// "guide" hits title (3)
	if got != 3.0 {
		t.Errorf("Score = %v, want 3.0 (at least one required tag present)", got)
	}
}

func TestScore_TagFilterOr_NoneMatch(t *testing.T) {
	t.Parallel()
	d := doc("Guide", "Some description", "docker") // has neither "go" nor "kubernetes"
	got := Score("guide", []string{"go", "kubernetes"}, "or", d)
	if got != -1 {
		t.Errorf("Score = %v, want -1 (none of the required tags present)", got)
	}
}

// Score — empty filterTags (no filtering)

func TestScore_EmptyFilterTags_AndMode(t *testing.T) {
	t.Parallel()
	d := doc("Guide", "Some description")
	got := Score("guide", nil, "and", d)
	if got != 3.0 {
		t.Errorf("Score = %v, want 3.0 (empty filterTags means no filter)", got)
	}
}

func TestScore_EmptyFilterTags_OrMode(t *testing.T) {
	t.Parallel()
	d := doc("Guide", "Some description")
	got := Score("guide", []string{}, "or", d)
	if got != 3.0 {
		t.Errorf("Score = %v, want 3.0 (empty filterTags means no filter)", got)
	}
}

// TestScore_UnrecognisedMatchMode — acceptance criterion test

func TestScore_UnrecognisedMatchMode(t *testing.T) {
	t.Parallel()

	d := doc("Kubernetes Guide", "Deploy kubernetes apps", "kubernetes")

	// Score with an unrecognised matchMode must return -1 regardless of query/tags.
	got := Score("kubernetes", nil, "xor", d)
	if got != -1 {
		t.Errorf("Score with matchMode=%q = %v, want -1", "xor", got)
	}

	// FindBest with an unrecognised matchMode must return (Doc{}, false).
	docs := []parser.Doc{
		doc("Kubernetes Guide", "Deploy kubernetes apps", "kubernetes"),
		doc("Helm Charts", "Package kubernetes apps", "helm", "kubernetes"),
	}
	best, ok := FindBest("kubernetes", nil, "xor", docs)
	if ok {
		t.Errorf("FindBest with matchMode=%q returned ok=true, doc=%+v; want (Doc{}, false)", "xor", best)
	}
	if best.FilePath != "" || best.Title != "" {
		t.Errorf("FindBest with matchMode=%q returned non-zero Doc: %+v", "xor", best)
	}
}

// FindBest tests

func TestFindBest_EmptySlice(t *testing.T) {
	t.Parallel()
	best, ok := FindBest("kubernetes", nil, "and", nil)
	if ok {
		t.Errorf("FindBest on empty slice returned ok=true, doc=%+v", best)
	}
}

func TestFindBest_EmptyQuery(t *testing.T) {
	t.Parallel()
	docs := []parser.Doc{
		doc("Kubernetes Guide", "Deploy apps", "kubernetes"),
	}
	_, ok := FindBest("", nil, "and", docs)
	if ok {
		t.Errorf("FindBest with empty query returned ok=true (all docs score 0, want false)")
	}
}

func TestFindBest_SingleWinner(t *testing.T) {
	t.Parallel()
	docs := []parser.Doc{
		doc("Helm Charts", "Package manager", "helm"),
		doc("Kubernetes Guide", "Deploy kubernetes apps", "kubernetes"),
	}
	best, ok := FindBest("kubernetes", nil, "and", docs)
	if !ok {
		t.Fatal("FindBest returned ok=false, want a result")
	}
	if best.Title != "Kubernetes Guide" {
		t.Errorf("best.Title = %q, want %q", best.Title, "Kubernetes Guide")
	}
}

// TestFindBest_TieBreakAlphabetical — invariant I-6: tie-break on FilePath asc.
func TestFindBest_TieBreakAlphabetical(t *testing.T) {
	t.Parallel()
	// Both docs have identical titles and descriptions; "kubernetes" in title only → 3.0 each.
	docs := []parser.Doc{
		docAt("z/kubernetes.md", "Kubernetes Guide", ""),
		docAt("a/kubernetes.md", "Kubernetes Guide", ""),
	}
	best, ok := FindBest("kubernetes", nil, "and", docs)
	if !ok {
		t.Fatal("FindBest returned ok=false, want a result")
	}
	if best.FilePath != "a/kubernetes.md" {
		t.Errorf("tie-break: best.FilePath = %q, want %q (alphabetically first)", best.FilePath, "a/kubernetes.md")
	}
}

func TestFindBest_AllFilteredOut(t *testing.T) {
	t.Parallel()
	docs := []parser.Doc{
		doc("Guide", "Some description", "docker"),
	}
	_, ok := FindBest("guide", []string{"kubernetes"}, "and", docs)
	if ok {
		t.Errorf("FindBest returned ok=true when all docs are filtered by tag")
	}
}
