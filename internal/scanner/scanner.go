// Package scanner walks a directory tree and returns the paths of all
// indexable OKF markdown files.
package scanner

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// reserved is the set of filenames that OKF reserves for its own use (I-4).
// These files are never returned by Scan even though they carry .md extension.
var reserved = map[string]bool{
	"index.md": true,
	"log.md":   true,
}

// vcsDirs is the set of VCS-internal directory names that are always skipped
// regardless of ScanOptions (I-19).
var vcsDirs = map[string]bool{
	".git": true,
	".hg":  true,
	".svn": true,
}

// ScanOptions controls optional scanner behaviour.
type ScanOptions struct {
	EnableHidden bool // traverse hidden directories (except VCS internals)
}

// ScanResult holds the results of a full directory scan.
type ScanResult struct {
	Docs     []string // absolute paths of indexable .md files
	Reserved []string // absolute paths of reserved files (index.md, log.md)
}

// ScanAll walks dir recursively and returns both indexable and reserved file
// paths in a ScanResult.
//
// Skip rules (applied in order):
//  1. VCS-internal directories (.git, .hg, .svn) are always skipped (I-19).
//     Other hidden directories are skipped unless opts.EnableHidden is true (I-5).
//  2. Reserved filenames (index.md, log.md) are collected in Reserved, not Docs (I-4).
//  3. Files whose extension is not ".md" are skipped.
//  4. Directory entries themselves are never included in either slice.
//
// dir must be an absolute path; the caller is responsible for providing it.
// Returns an empty ScanResult, nil when dir exists but contains no matching files.
func ScanAll(dir string, opts ScanOptions) (ScanResult, error) {
	var result ScanResult

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		name := d.Name()

		// Rule 1: VCS internals always skipped; other hidden dirs skipped unless EnableHidden is set (I-5, I-19).
		if d.IsDir() && strings.HasPrefix(name, ".") {
			if vcsDirs[name] {
				return fs.SkipDir
			}
			if !opts.EnableHidden {
				return fs.SkipDir
			}
		}

		// Directories themselves are never added to results.
		if d.IsDir() {
			return nil
		}

		// Rule 2: reserved filenames go into Reserved, not Docs (I-4).
		if reserved[name] {
			result.Reserved = append(result.Reserved, path)
			return nil
		}

		// Rule 3: only .md files qualify.
		if filepath.Ext(name) != ".md" {
			return nil
		}

		result.Docs = append(result.Docs, path)
		return nil
	})
	if err != nil {
		return ScanResult{}, err
	}

	return result, nil
}

// Scan walks dir recursively and returns the absolute paths of all indexable
// .md files found within. It delegates to ScanAll and returns only the Docs
// slice.
//
// dir must be an absolute path; the caller is responsible for providing it.
// Returns nil, nil when dir exists but contains no matching files.
func Scan(dir string) ([]string, error) {
	r, err := ScanAll(dir, ScanOptions{})
	if err != nil {
		return nil, err
	}
	return r.Docs, nil
}
