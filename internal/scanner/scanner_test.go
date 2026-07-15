package scanner_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/nrkno/plattform-okf-mcp/internal/scanner"
)

// writeFile creates all parent directories and writes content to path.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

// sortedScan calls scanner.Scan and returns a sorted slice so assertions are
// order-independent.
func sortedScan(t *testing.T, dir string) []string {
	t.Helper()
	paths, err := scanner.Scan(dir)
	if err != nil {
		t.Fatalf("Scan(%s): unexpected error: %v", dir, err)
	}
	sort.Strings(paths)
	return paths
}

// TestScan_EmptyDir verifies that an empty directory returns no paths and no
// error.
func TestScan_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	paths, err := scanner.Scan(dir)
	if err != nil {
		t.Fatalf("Scan(%s): unexpected error: %v", dir, err)
	}
	if len(paths) != 0 {
		t.Errorf("got %v, want empty slice", paths)
	}
}

// TestScan_SingleValidFile verifies that a single qualifying .md file is
// returned.
func TestScan_SingleValidFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	want := filepath.Join(dir, "guide.md")
	writeFile(t, want, "# Guide\n")

	got := sortedScan(t, dir)
	if len(got) != 1 || got[0] != want {
		t.Errorf("got %v, want [%s]", got, want)
	}
}

// TestScan_IndexMdSkipped verifies that index.md is never returned (I-4).
func TestScan_IndexMdSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.md"), "# Index\n")
	writeFile(t, filepath.Join(dir, "guide.md"), "# Guide\n")

	got := sortedScan(t, dir)
	for _, p := range got {
		if filepath.Base(p) == "index.md" {
			t.Errorf("index.md must not appear in results, got %v", got)
		}
	}
	if len(got) != 1 || filepath.Base(got[0]) != "guide.md" {
		t.Errorf("got %v, want [guide.md]", got)
	}
}

// TestScan_LogMdSkipped verifies that log.md is never returned (I-4).
func TestScan_LogMdSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "log.md"), "# Log\n")
	writeFile(t, filepath.Join(dir, "guide.md"), "# Guide\n")

	got := sortedScan(t, dir)
	for _, p := range got {
		if filepath.Base(p) == "log.md" {
			t.Errorf("log.md must not appear in results, got %v", got)
		}
	}
	if len(got) != 1 || filepath.Base(got[0]) != "guide.md" {
		t.Errorf("got %v, want [guide.md]", got)
	}
}

// TestScan_HiddenDirGit verifies that .git/ subtrees are skipped entirely (I-5).
func TestScan_HiddenDirGit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".git", "some.md"), "# git file\n")
	writeFile(t, filepath.Join(dir, "guide.md"), "# Guide\n")

	got := sortedScan(t, dir)
	for _, p := range got {
		if filepath.Dir(p) == filepath.Join(dir, ".git") {
			t.Errorf(".git subtree must not appear in results, got %v", got)
		}
	}
	if len(got) != 1 || filepath.Base(got[0]) != "guide.md" {
		t.Errorf("got %v, want [guide.md]", got)
	}
}

// TestScan_HiddenDirOpencode verifies that .opencode/ subtrees are skipped (I-5).
func TestScan_HiddenDirOpencode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".opencode", "architecture", "design.md"), "# Design\n")
	writeFile(t, filepath.Join(dir, "guide.md"), "# Guide\n")

	got := sortedScan(t, dir)
	for _, p := range got {
		rel, _ := filepath.Rel(dir, p)
		if len(rel) > 0 && rel[0:1] == "." {
			t.Errorf(".opencode subtree must not appear in results, got %v", got)
		}
	}
	if len(got) != 1 || filepath.Base(got[0]) != "guide.md" {
		t.Errorf("got %v, want [guide.md]", got)
	}
}

// TestScan_NestedSubdirectories verifies that qualifying .md files in nested
// non-hidden subdirectories are all returned.
func TestScan_NestedSubdirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files := []string{
		filepath.Join(dir, "a.md"),
		filepath.Join(dir, "sub", "b.md"),
		filepath.Join(dir, "sub", "deep", "c.md"),
	}
	for _, f := range files {
		writeFile(t, f, "# Doc\n")
	}

	got := sortedScan(t, dir)
	want := append([]string(nil), files...)
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("got %d paths, want %d: %v", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

// TestScan_NonMdFilesSkipped verifies that .txt and .go files are not returned.
func TestScan_NonMdFilesSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "readme.txt"), "plain text\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n")
	writeFile(t, filepath.Join(dir, "guide.md"), "# Guide\n")

	got := sortedScan(t, dir)
	if len(got) != 1 || filepath.Base(got[0]) != "guide.md" {
		t.Errorf("got %v, want [guide.md]", got)
	}
}

// TestScan_Mixed verifies combined behaviour: valid .md returned; reserved
// filenames, hidden dirs, and non-.md files excluded.
func TestScan_Mixed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	want := []string{
		filepath.Join(dir, "guide.md"),
		filepath.Join(dir, "docs", "intro.md"),
	}
	for _, f := range want {
		writeFile(t, f, "# Doc\n")
	}

	// Must NOT appear in results.
	writeFile(t, filepath.Join(dir, "index.md"), "# Index\n")           // I-4
	writeFile(t, filepath.Join(dir, "log.md"), "# Log\n")               // I-4
	writeFile(t, filepath.Join(dir, ".hidden", "secret.md"), "# Sec\n") // I-5
	writeFile(t, filepath.Join(dir, "notes.txt"), "plain\n")            // not .md
	writeFile(t, filepath.Join(dir, "script.sh"), "#!/bin/sh\n")        // not .md

	got := sortedScan(t, dir)
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("got %d paths %v, want %d paths %v", len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}
