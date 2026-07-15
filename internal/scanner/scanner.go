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

// Scan walks dir recursively and returns the absolute paths of all indexable
// .md files found within.
//
// Skip rules (applied in order):
//  1. Any directory whose name starts with '.' is skipped entirely (I-5).
//  2. Files named exactly "index.md" or "log.md" are skipped (I-4).
//  3. Files whose extension is not ".md" are skipped.
//  4. Directory entries themselves are never included in the result.
//
// dir must be an absolute path; the caller is responsible for providing it.
// Returns nil, nil when dir exists but contains no matching files.
func Scan(dir string) ([]string, error) {
	var paths []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		name := d.Name()

		// Rule 1: skip hidden directories entirely (I-5).
		if d.IsDir() && strings.HasPrefix(name, ".") {
			return fs.SkipDir
		}

		// Directories themselves are never added to results.
		if d.IsDir() {
			return nil
		}

		// Rule 2: skip OKF reserved filenames (I-4).
		if reserved[name] {
			return nil
		}

		// Rule 3: only .md files qualify.
		if filepath.Ext(name) != ".md" {
			return nil
		}

		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return paths, nil
}
