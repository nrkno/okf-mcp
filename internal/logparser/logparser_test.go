package logparser

import (
	"testing"
)

func TestParse_EmptyInput(t *testing.T) {
	got := Parse("")
	if got != nil {
		t.Errorf("Parse(\"\") = %v, want nil", got)
	}
}

func TestParse_SkipsLinesBeforeFirstDate(t *testing.T) {
	body := `# Directory Update Log

Some preamble text.

## 2026-07-15

**Creation**: ` + "`docs/architecture.md`" + ` — initial creation.`
	entries := Parse(body)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Date != "2026-07-15" {
		t.Errorf("Date = %q, want 2026-07-15", entries[0].Date)
	}
	if entries[0].Action != "Creation" {
		t.Errorf("Action = %q, want Creation", entries[0].Action)
	}
	if entries[0].Target != "docs/architecture.md" {
		t.Errorf("Target = %q, want docs/architecture.md", entries[0].Target)
	}
	if entries[0].Detail != "initial creation." {
		t.Errorf("Detail = %q, want 'initial creation.'", entries[0].Detail)
	}
}

func TestParse_MultipleDates(t *testing.T) {
	body := `## 2026-07-15

**Creation**: ` + "`docs/architecture.md`" + ` — initial creation.
**Creation**: ` + "`docs/tools.md`" + ` — tool reference.

## 2026-07-18

**Update**: ` + "`docs/tools.md`" + ` — added examples.`
	entries := Parse(body)
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}

	expected := []LogEntry{
		{Date: "2026-07-15", Action: "Creation", Target: "docs/architecture.md", Detail: "initial creation."},
		{Date: "2026-07-15", Action: "Creation", Target: "docs/tools.md", Detail: "tool reference."},
		{Date: "2026-07-18", Action: "Update", Target: "docs/tools.md", Detail: "added examples."},
	}

	for i, want := range expected {
		if entries[i] != want {
			t.Errorf("entry[%d] = %+v, want %+v", i, entries[i], want)
		}
	}
}

func TestParse_MultilineDetail(t *testing.T) {
	body := `## 2026-07-15

**Creation**: ` + "`docs/architecture.md`" + ` — initial creation: internal package structure.
Second line of detail.
Third line of detail.`
	entries := Parse(body)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	want := "initial creation: internal package structure.\nSecond line of detail.\nThird line of detail."
	if entries[0].Detail != want {
		t.Errorf("Detail = %q, want %q", entries[0].Detail, want)
	}
}

func TestParse_NonMatchingLinesBecomesDetail(t *testing.T) {
	body := `## 2026-07-15

**Creation**: ` + "`docs/architecture.md`" + ` — initial creation.
Some random line that is not an entry.
**Update**: ` + "`docs/tools.md`" + ` — updated tools.`
	entries := Parse(body)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Detail != "initial creation.\nSome random line that is not an entry." {
		t.Errorf("entry[0].Detail = %q", entries[0].Detail)
	}
}

func TestParse_DocumentOrder(t *testing.T) {
	body := `## 2026-07-15

**Creation**: ` + "`docs/first.md`" + ` — first.
**Creation**: ` + "`docs/second.md`" + ` — second.
**Creation**: ` + "`docs/third.md`" + ` — third.

## 2026-07-18

**Update**: ` + "`docs/fourth.md`" + ` — fourth.`
	entries := Parse(body)
	if len(entries) != 4 {
		t.Fatalf("got %d entries, want 4", len(entries))
	}
	for i, wantTarget := range []string{"docs/first.md", "docs/second.md", "docs/third.md", "docs/fourth.md"} {
		if entries[i].Target != wantTarget {
			t.Errorf("entry[%d].Target = %q, want %q", i, entries[i].Target, wantTarget)
		}
	}
}

func TestParse_MalformedInput(t *testing.T) {
	body := `This is not a log at all.
No date headings here.
Just random text.`
	entries := Parse(body)
	if entries != nil {
		t.Errorf("malformed input returned %v, want nil", entries)
	}
}

func TestParse_SingleDateNoEntries(t *testing.T) {
	body := "## 2026-07-15\n"
	entries := Parse(body)
	if entries != nil {
		t.Errorf("date heading with no entries returned %v, want nil", entries)
	}
}

func TestParse_EmptyLinesWithinEntrySkipped(t *testing.T) {
	body := `## 2026-07-15

**Creation**: ` + "`docs/architecture.md`" + ` — initial creation.

**Update**: ` + "`docs/tools.md`" + ` — updated tools.`
	entries := Parse(body)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Detail != "initial creation." {
		t.Errorf("entry[0].Detail = %q, want 'initial creation.'", entries[0].Detail)
	}
}

func TestParse_WithRealLogContent(t *testing.T) {
	// Simulates the real docs/log.md format after frontmatter strip
	body := "# Directory Update Log\n\n## 2026-07-15\n\n" +
		"**Creation**: `docs/architecture.md` — initial creation: internal package structure, design invariants, scoring model.\n" +
		"**Creation**: `docs/configuration.md` — initial creation: MCP host registration, opencode and Claude Desktop examples, permission strings.\n" +
		"**Creation**: `docs/okf-standard.md` — initial creation: OKF frontmatter schema, type vocabulary, skip rules, authoring conventions.\n" +
		"**Creation**: `docs/deployment.md` — initial creation: build, install, run, test, and release procedures.\n" +
		"**Creation**: `docs/tools.md` — initial creation: complete reference for list_tags, list_docs, and get_doc."
	entries := Parse(body)
	if len(entries) != 5 {
		t.Fatalf("got %d entries, want 5", len(entries))
	}
	for i, e := range entries {
		if e.Date != "2026-07-15" {
			t.Errorf("entry[%d].Date = %q, want 2026-07-15", i, e.Date)
		}
		if e.Action != "Creation" {
			t.Errorf("entry[%d].Action = %q, want Creation", i, e.Action)
		}
	}
}
