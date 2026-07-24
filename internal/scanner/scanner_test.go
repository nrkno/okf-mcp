package scanner_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// TestScan_HiddenDirOpencode_DefaultOff verifies that .opencode/ subtrees are
// skipped when EnableHidden is false (the default, I-5, I-18).
func TestScan_HiddenDirOpencode_DefaultOff(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".opencode", "architecture", "design.md"), "# Design\n")
	writeFile(t, filepath.Join(dir, "guide.md"), "# Guide\n")

	docs, _ := sortedScanAll(t, dir)
	for _, p := range docs {
		rel, _ := filepath.Rel(dir, p)
		if len(rel) > 0 && rel[0:1] == "." {
			t.Errorf(".opencode subtree must not appear in results, got %v", docs)
		}
	}
	if len(docs) != 1 || filepath.Base(docs[0]) != "guide.md" {
		t.Errorf("got %v, want [guide.md]", docs)
	}
}

// TestScan_HiddenDirOpencode_FlagOn verifies that .opencode/ subtrees are
// traversed when EnableHidden is true (I-5).
func TestScan_HiddenDirOpencode_FlagOn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// doc in hidden dir — must appear in Docs
	hiddenDoc := filepath.Join(dir, ".opencode", "architecture", "design.md")
	writeFile(t, hiddenDoc, "# Design\n")
	visibleDoc := filepath.Join(dir, "guide.md")
	writeFile(t, visibleDoc, "# Guide\n")

	result, err := scanner.ScanAll(dir, scanner.ScanOptions{EnableHidden: true})
	if err != nil {
		t.Fatalf("ScanAll(%s, EnableHidden=true): unexpected error: %v", dir, err)
	}
	docs := append([]string(nil), result.Docs...)
	sort.Strings(docs)

	if len(docs) != 2 {
		t.Fatalf("got %d docs %v, want 2 (guide.md + hidden design.md)", len(docs), docs)
	}
	if filepath.Base(docs[0]) != "design.md" {
		t.Errorf("docs[0] = %s, want design.md (hidden dir)", filepath.Base(docs[0]))
	}
	if filepath.Base(docs[1]) != "guide.md" {
		t.Errorf("docs[1] = %s, want guide.md (visible)", filepath.Base(docs[1]))
	}
}

// TestScan_HiddenDirGit_AlwaysSkipped verifies that .git/ subtrees are always
// skipped, even when EnableHidden is true (I-19).
func TestScan_HiddenDirGit_AlwaysSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".git", "some.md"), "# git file\n")
	writeFile(t, filepath.Join(dir, "guide.md"), "# Guide\n")

	result, err := scanner.ScanAll(dir, scanner.ScanOptions{EnableHidden: true})
	if err != nil {
		t.Fatalf("ScanAll(%s, EnableHidden=true): unexpected error: %v", dir, err)
	}

	for _, p := range result.Docs {
		rel, _ := filepath.Rel(dir, p)
		if rel == ".git" || strings.HasPrefix(rel, ".git/") {
			t.Errorf(".git subtree must not appear in Docs even with EnableHidden, got %v", result.Docs)
		}
	}
	if len(result.Docs) != 1 || filepath.Base(result.Docs[0]) != "guide.md" {
		t.Errorf("Docs = %v, want [guide.md]", result.Docs)
	}
}

// TestScan_HiddenDirHg_AlwaysSkipped verifies that .hg/ subtrees are always
// skipped, even when EnableHidden is true (I-19).
func TestScan_HiddenDirHg_AlwaysSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".hg", "some.md"), "# hg file\n")
	writeFile(t, filepath.Join(dir, "guide.md"), "# Guide\n")

	result, err := scanner.ScanAll(dir, scanner.ScanOptions{EnableHidden: true})
	if err != nil {
		t.Fatalf("ScanAll(%s, EnableHidden=true): unexpected error: %v", dir, err)
	}

	for _, p := range result.Docs {
		rel, _ := filepath.Rel(dir, p)
		if rel == ".hg" || strings.HasPrefix(rel, ".hg/") {
			t.Errorf(".hg subtree must not appear in Docs even with EnableHidden, got %v", result.Docs)
		}
	}
	if len(result.Docs) != 1 || filepath.Base(result.Docs[0]) != "guide.md" {
		t.Errorf("Docs = %v, want [guide.md]", result.Docs)
	}
}

// TestScanAll_EnableHidden_ReservedInHiddenDir verifies that reserved files
// (index.md, log.md) inside a hidden directory appear in Reserved, not Docs,
// when EnableHidden is true.
func TestScanAll_EnableHidden_ReservedInHiddenDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".opencode", "architecture", "index.md"), "# Index\n")
	writeFile(t, filepath.Join(dir, ".opencode", "architecture", "log.md"), "# Log\n")
	writeFile(t, filepath.Join(dir, ".opencode", "architecture", "design.md"), "# Design\n")
	writeFile(t, filepath.Join(dir, "guide.md"), "# Guide\n")

	result, err := scanner.ScanAll(dir, scanner.ScanOptions{EnableHidden: true})
	if err != nil {
		t.Fatalf("ScanAll(%s, EnableHidden=true): unexpected error: %v", dir, err)
	}

	docs := append([]string(nil), result.Docs...)
	reserved := append([]string(nil), result.Reserved...)
	sort.Strings(docs)
	sort.Strings(reserved)

	// reserved files must be in Reserved, not Docs
	for _, p := range docs {
		if filepath.Base(p) == "index.md" || filepath.Base(p) == "log.md" {
			t.Errorf("reserved file %s must not appear in Docs, got Docs=%v", p, docs)
		}
	}

	// check specific content of Reserved
	wantReserved := []string{
		filepath.Join(dir, ".opencode", "architecture", "index.md"),
		filepath.Join(dir, ".opencode", "architecture", "log.md"),
	}
	sort.Strings(wantReserved)
	if len(reserved) != len(wantReserved) {
		t.Fatalf("Reserved = %v, want %v", reserved, wantReserved)
	}
	for i := range reserved {
		if reserved[i] != wantReserved[i] {
			t.Errorf("Reserved[%d] = %s, want %s", i, reserved[i], wantReserved[i])
		}
	}

	// design.md (non-reserved) must be in Docs
	if len(docs) != 2 {
		t.Fatalf("got %d docs %v, want 2 (guide.md + hidden design.md)", len(docs), docs)
	}
	if filepath.Base(docs[0]) != "design.md" {
		t.Errorf("docs[0] = %s, want design.md", filepath.Base(docs[0]))
	}
	if filepath.Base(docs[1]) != "guide.md" {
		t.Errorf("docs[1] = %s, want guide.md", filepath.Base(docs[1]))
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

// sortedScanAll calls scanner.ScanAll and returns sorted Docs and Reserved slices.
func sortedScanAll(t *testing.T, dir string) (docs, reserved []string) {
	t.Helper()
	result, err := scanner.ScanAll(dir, scanner.ScanOptions{})
	if err != nil {
		t.Fatalf("ScanAll(%s): unexpected error: %v", dir, err)
	}
	docs = append([]string(nil), result.Docs...)
	reserved = append([]string(nil), result.Reserved...)
	sort.Strings(docs)
	sort.Strings(reserved)
	return docs, reserved
}

// TestScanAll_ReservedAtRoot verifies that index.md and log.md at the root
// appear in Reserved, not in Docs.
func TestScanAll_ReservedAtRoot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.md"), "# Index\n")
	writeFile(t, filepath.Join(dir, "log.md"), "# Log\n")
	writeFile(t, filepath.Join(dir, "guide.md"), "# Guide\n")

	docs, reserved := sortedScanAll(t, dir)

	sort.Strings(reserved)
	wantReserved := []string{
		filepath.Join(dir, "index.md"),
		filepath.Join(dir, "log.md"),
	}
	sort.Strings(wantReserved)

	if len(docs) != 1 || filepath.Base(docs[0]) != "guide.md" {
		t.Errorf("Docs = %v, want [guide.md]", docs)
	}
	if len(reserved) != len(wantReserved) {
		t.Fatalf("Reserved = %v, want %v", reserved, wantReserved)
	}
	for i := range reserved {
		if reserved[i] != wantReserved[i] {
			t.Errorf("Reserved[%d] = %s, want %s", i, reserved[i], wantReserved[i])
		}
	}
}

// TestScanAll_ReservedInNestedDirs verifies that reserved files in nested
// directories are collected into Reserved.
func TestScanAll_ReservedInNestedDirs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "sub", "index.md"), "# Sub Index\n")
	writeFile(t, filepath.Join(dir, "deep", "log.md"), "# Deep Log\n")
	writeFile(t, filepath.Join(dir, "sub", "guide.md"), "# Guide\n")
	writeFile(t, filepath.Join(dir, "deep", "other.md"), "# Other\n")

	docs, reserved := sortedScanAll(t, dir)

	wantDocs := []string{
		filepath.Join(dir, "deep", "other.md"),
		filepath.Join(dir, "sub", "guide.md"),
	}
	wantReserved := []string{
		filepath.Join(dir, "deep", "log.md"),
		filepath.Join(dir, "sub", "index.md"),
	}
	sort.Strings(wantDocs)
	sort.Strings(wantReserved)

	if len(docs) != len(wantDocs) {
		t.Fatalf("Docs = %v, want %v", docs, wantDocs)
	}
	for i := range docs {
		if docs[i] != wantDocs[i] {
			t.Errorf("Docs[%d] = %s, want %s", i, docs[i], wantDocs[i])
		}
	}
	if len(reserved) != len(wantReserved) {
		t.Fatalf("Reserved = %v, want %v", reserved, wantReserved)
	}
	for i := range reserved {
		if reserved[i] != wantReserved[i] {
			t.Errorf("Reserved[%d] = %s, want %s", i, reserved[i], wantReserved[i])
		}
	}
}

// TestScanAll_NoReservedFiles verifies that ScanAll returns an empty Reserved
// slice when no reserved files exist.
func TestScanAll_NoReservedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "guide.md"), "# Guide\n")
	writeFile(t, filepath.Join(dir, "sub", "intro.md"), "# Intro\n")

	docs, reserved := sortedScanAll(t, dir)

	wantDocs := []string{
		filepath.Join(dir, "guide.md"),
		filepath.Join(dir, "sub", "intro.md"),
	}
	sort.Strings(wantDocs)

	if len(docs) != len(wantDocs) {
		t.Fatalf("Docs = %v, want %v", docs, wantDocs)
	}
	for i := range docs {
		if docs[i] != wantDocs[i] {
			t.Errorf("Docs[%d] = %s, want %s", i, docs[i], wantDocs[i])
		}
	}
	if len(reserved) != 0 {
		t.Errorf("Reserved = %v, want empty slice", reserved)
	}
}

// TestScanAll_EmptyDir verifies that ScanAll returns empty slices for an empty dir.
func TestScanAll_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	result, err := scanner.ScanAll(dir, scanner.ScanOptions{})
	if err != nil {
		t.Fatalf("ScanAll(%s): unexpected error: %v", dir, err)
	}
	if len(result.Docs) != 0 {
		t.Errorf("Docs = %v, want empty slice", result.Docs)
	}
	if len(result.Reserved) != 0 {
		t.Errorf("Reserved = %v, want empty slice", result.Reserved)
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
