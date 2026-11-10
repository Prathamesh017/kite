package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"kite/internal/core"
)

var _ core.Tools = Tools{}

// Tools are the tools that work on files in the folder Kite was started in.
type Tools struct{}

func (Tools) Specs() []core.ToolSpec {
	return []core.ToolSpec{listFilesSpec, readFileSpec, writeFileSpec}
}

func (Tools) Run(_ context.Context, name, arguments string) string {
	switch name {
	case "list_files":
		return listFiles(arguments)
	case "read_file":
		return readFile(arguments)
	case "write_file":
		return writeFile(arguments)
	}
	return "error: unknown tool " + name
}

// insideWorkspace resolves path against the folder Kite was started in, and
// reports whether the result stays inside that folder.
func insideWorkspace(path string) (string, bool) {
	root, err := os.Getwd()
	if err != nil {
		return "", false
	}
	full := path
	if !filepath.IsAbs(full) {
		full = filepath.Join(root, full)
	}
	full = filepath.Clean(full)

	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return full, true
}
