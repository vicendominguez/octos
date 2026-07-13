// Package main — filewatch.go provides filesystem scanning and change detection.
// scanDirectory builds a snapshot of file modification times, and detectFileChanges
// compares two snapshots to identify created, modified, and deleted files.
package main

import (
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

// detectFileChanges compares directory state before/after to find changes
func detectFileChanges(beforeFiles map[string]time.Time) []string {
	afterFiles := scanDirectory(".")
	var changes []string

	// Check for new or modified files
	for path, afterTime := range afterFiles {
		if beforeTime, exists := beforeFiles[path]; !exists {
			changes = append(changes, "+ "+path)
		} else if afterTime.After(beforeTime) {
			changes = append(changes, "M "+path)
		}
	}

	// Check for deleted files
	for path := range beforeFiles {
		if _, exists := afterFiles[path]; !exists {
			changes = append(changes, "- "+path)
		}
	}

	return changes
}

// scanDirectory recursively scans directory and returns file paths with mod times
func scanDirectory(root string) map[string]time.Time {
	files := make(map[string]time.Time)
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if strings.Contains(path, "/.") ||
			strings.Contains(path, "/node_modules/") ||
			strings.Contains(path, OctosDirSlash) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				files[path] = info.ModTime()
			}
		}
		return nil
	})
	return files
}
