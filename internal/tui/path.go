package tui

import (
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// pathDepth is how many trailing folders are kept, like a shell prompt.
const pathDepth = 2

// workingDir is the shortened folder kite was started from, or "" if unknown.
func workingDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return shortPath(cwd)
}

// shortPath writes the home folder as ~ and keeps only the last pathDepth
// folders, so /Users/me/work/projects/kite becomes …/projects/kite.
func shortPath(path string) string {
	if home, err := os.UserHomeDir(); err == nil {
		if rel, err := filepath.Rel(home, path); err == nil && !strings.HasPrefix(rel, "..") {
			path = filepath.Join("~", rel)
		}
	}

	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) <= pathDepth {
		return path
	}
	return "…/" + strings.Join(parts[len(parts)-pathDepth:], "/")
}

// trimLeft drops leading folders from path until it fits in max columns,
// marking the cut with …/. The end of a path is the part worth keeping.
// A single folder that is still too long is cut mid-name.
func trimLeft(path string, max int) string {
	for utf8.RuneCountInString(path) > max {
		rest := strings.TrimPrefix(path, "…/")
		i := strings.Index(rest, "/")
		if i < 0 {
			break
		}
		path = "…/" + rest[i+1:]
	}

	runes := []rune(path)
	if len(runes) <= max {
		return path
	}
	if max < 2 {
		return ""
	}
	return "…" + string(runes[len(runes)-max+1:])
}
